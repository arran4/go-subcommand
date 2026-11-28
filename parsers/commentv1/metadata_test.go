package commentv1

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/parsers"
)

func TestMetadataDeclarations(t *testing.T) {
	for _, configured := range []bool{false, true} {
		source := "package commands\n"
		options := &parsers.ParseOptions{}
		if configured {
			options.CommandDeclarations = []parsers.CommandDeclaration{{Kind: "command", Path: "app", Text: "Description: Application.\nMore help.\n", Filename: "project.mdc", Line: 4}}
		} else {
			source = "// App is a command `app` -- Application.\n// More help.\npackage commands\n"
		}
		input := fstest.MapFS{"go.mod": {Data: []byte("module example.com/meta\n\ngo 1.22\n")}, "metadata.go": {Data: []byte(source)}}
		data, err := (&CommentParser{}).Parse(input, ".", options)
		if err != nil {
			t.Fatal(err)
		}
		if err := data.Validate(); err != nil {
			t.Fatal(err)
		}
		if len(data.Commands) != 1 {
			t.Fatalf("commands=%d", len(data.Commands))
		}
		command := data.Commands[0]
		if command.Metadata == nil || command.MainCmdName != "app" || command.HasAction() || command.FunctionName != "" || command.ImportPath != "" || command.CommandPackageName != "" || command.Description != "Application." || command.ExtendedHelp != "More help." {
			t.Fatalf("metadata changed action or help semantics: %#v", command)
		}
		if command.Metadata.Filename == "" || command.Metadata.Line < 1 {
			t.Fatal("missing source provenance")
		}
	}
}

func TestMetadataDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"unquoted", "// App is a command app\npackage commands\n", "backtick"},
		{"flags", "// App is a command `app`\n//\n// Flags:\n//\tname: --name\npackage commands\n", "metadata.go:3:"},
		{"positional", "// App is a command `app`\n// value: @1\npackage commands\n", "without Go types"},
		{"empty path", "// App is a command ` `\npackage commands\n", "empty command path"},
		{"path escape", "// App is a command `../app`\npackage commands\n", "invalid command path"},
		{"duplicate", "// App is a command `app`\npackage commands\n\n// Again is a command `app`\n", "duplicate command metadata"},
		{"description", "// App is a command `app` -- Inline.\n// Description: Another.\npackage commands\n", "both inline"},
		{"duplicate directive", "// App is a command `app`\n// Description: One.\n// Description: Two.\npackage commands\n", "duplicate metadata directive"},
		{"unknown policy", "// App is a command `app`\n// No command: ignore\npackage commands\n", "unsupported No command policy"},
		{"child policy", "// Child is a command `app child`\n// No command: error\npackage commands\n", "only supported on an application root"},
		{"duplicate policy", "// App is a command `app`\n// No command: error\n// No command: error\npackage commands\n", "duplicate metadata directive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fstest.MapFS{"go.mod": {Data: []byte("module example.com/meta\n\ngo 1.22\n")}, "metadata.go": {Data: []byte(tc.source)}}
			_, err := ParseGoFiles(input, ".")
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "metadata.go:") {
				t.Fatalf("error=%v want %q", err, tc.want)
			}
		})
	}
}

func TestMetadataConflictsAndCombination(t *testing.T) {
	input := fstest.MapFS{
		"go.mod":      {Data: []byte("module example.com/meta\n\ngo 1.22\n")},
		"metadata.go": {Data: []byte("// App is a command `app` -- Declared help.\npackage commands\n")},
		"actions.go":  {Data: []byte("package commands\n// Run is a subcommand `app` -- Action help.\nfunc Run() {}\n")},
	}
	data, err := ParseGoFiles(input, ".")
	if err != nil {
		t.Fatal(err)
	}
	command := data.Commands[0]
	if command.FunctionName != "Run" || command.Metadata == nil || command.Description != "Declared help." || command.ImportPath != "example.com/meta" {
		t.Fatalf("lost action or metadata: %#v", command)
	}
	_, err = (&CommentParser{}).Parse(input, ".", &parsers.ParseOptions{CommandDeclarations: []parsers.CommandDeclaration{{Kind: "command", Path: "app", Filename: "project.mdc", Line: 7}}})
	if err == nil || !strings.Contains(err.Error(), "project.mdc:7:") || !strings.Contains(err.Error(), "metadata.go:1") {
		t.Fatalf("duplicate diagnostic=%v", err)
	}
	input["actions.go"] = &fstest.MapFile{Data: []byte("package commands\n// Run is a subcommand `app`\n// Aliases: shortcut\nfunc Run() {}\n")}
	_, err = (&CommentParser{}).Parse(input, ".", &parsers.ParseOptions{CommandDeclarations: []parsers.CommandDeclaration{{Kind: "command", Path: "app shortcut", Filename: "project.mdc", Line: 4}}})
	if err == nil || !strings.Contains(err.Error(), "canonical command") || !strings.Contains(err.Error(), "project.mdc:4:") {
		t.Fatalf("root-alias collision=%v", err)
	}
}

func TestMetadataRequiresDeclarationsAndDiagnostics(t *testing.T) {
	baseFiles := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/metareq\n\ngo 1.22\n")},
		"auth/auth.go": {Data: []byte(`package auth

type User struct{}

func CurrentUser() (*User, error) { return nil, nil }
`)},
		"commands/meta.go": {Data: []byte(`// App is a command ` + "`app`" + ` -- Declared help.
// Requires: auth.CurrentUser
package commands

import "example.com/metareq/auth"

// Run is a subcommand ` + "`app run`" + ` -- Run command.
func Run(user *auth.User) error { return nil }
`)},
	}

	// Go functionless command metadata must use the file's Go imports to resolve Requires.
	data, err := (&CommentParser{}).Parse(baseFiles, ".", &parsers.ParseOptions{Recursive: true})
	if err != nil {
		t.Fatalf("functionless Go metadata Requires failed: %v", err)
	}
	if len(data.Commands[0].Requirements) != 1 || data.Commands[0].Requirements[0].Func.ImportPath != "example.com/metareq/auth" {
		t.Fatalf("go metadata reqs = %+v", data.Commands[0].Requirements)
	}

	// Native .mdc CommandDeclarations with quoted import path works cleanly.
	mdcFS := fstest.MapFS{
		"go.mod":       baseFiles["go.mod"],
		"auth/auth.go": baseFiles["auth/auth.go"],
		"commands/app.go": {Data: []byte(`package commands

import "example.com/metareq/auth"

// Run is a subcommand ` + "`app run`" + ` -- Run command.
func Run(user *auth.User) error { return nil }
`)},
	}
	data, err = (&CommentParser{}).Parse(mdcFS, ".", &parsers.ParseOptions{
		Recursive: true,
		CommandDeclarations: []parsers.CommandDeclaration{{
			Kind:     "command",
			Path:     "app",
			Text:     "Description: App.\nRequires: \"example.com/metareq/auth\".CurrentUser\n",
			Filename: "gosubc.mdc",
			Line:     3,
		}},
	})
	if err != nil {
		t.Fatalf("native .mdc quoted Requires failed: %v", err)
	}
	if len(data.Commands[0].Requirements) != 1 || data.Commands[0].Requirements[0].Func.ImportPath != "example.com/metareq/auth" {
		t.Fatalf("mdc reqs = %+v", data.Commands[0].Requirements)
	}

	// Native .mdc with unqualified Go alias must fail with explicit diagnostic.
	_, err = (&CommentParser{}).Parse(mdcFS, ".", &parsers.ParseOptions{
		Recursive: true,
		CommandDeclarations: []parsers.CommandDeclaration{{
			Kind:     "command",
			Path:     "app",
			Text:     "Requires: auth.CurrentUser\n",
			Filename: "gosubc.mdc",
			Line:     5,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "unqualified package alias") || !strings.Contains(err.Error(), "gosubc.mdc:") {
		t.Fatalf("expected unqualified alias diagnostic for .mdc, got %v", err)
	}
}
