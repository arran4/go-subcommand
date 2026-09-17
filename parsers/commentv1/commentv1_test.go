package commentv1

import (
	"testing"
	"testing/fstest"
)

func TestParseCliParserDirective(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "go flag", text: "CLI-Parser: go-flag", want: "go-flag"},
		{name: "gnu", text: "CLI-Parser: gnu", want: "gnu"},
		{name: "case insensitive", text: "cli-parser: go-flag", want: "go-flag"},
		{name: "unset", text: "ordinary help text", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCliParserDirective(tt.text); got != tt.want {
				t.Fatalf("parseCliParserDirective() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSubCommandCommentsCliParserIsMetadata(t *testing.T) {
	text := "Child is a subcommand `app child`\nCLI-Parser: go-flag\nLong help text"
	_, _, _, extendedHelp, _, _, ok := ParseSubCommandComments(text)
	if !ok {
		t.Fatal("ParseSubCommandComments returned !ok")
	}
	if extendedHelp != "Long help text" {
		t.Fatalf("extended help = %q, want %q", extendedHelp, "Long help text")
	}
}

func TestParseGoFilesCliParserMetadata(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/testcli\ngo 1.21\n")},
		"commands.go": {Data: []byte("package testcli\n\n// Root is a subcommand `app`\n// CLI-Parser: gnu\nfunc Root() {}\n\n// Child is a subcommand `app child`\n// CLI-Parser: go-flag\nfunc Child() {}\n")},
	}

	data, err := ParseGoFiles(fsys, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles: %v", err)
	}
	if len(data.Commands) != 1 {
		t.Fatalf("len(data.Commands) = %d, want 1", len(data.Commands))
	}
	root := data.Commands[0]
	if root.CliParser != "gnu" {
		t.Fatalf("root.CliParser = %q, want gnu", root.CliParser)
	}
	if len(root.SubCommands) != 1 {
		t.Fatalf("len(root.SubCommands) = %d, want 1", len(root.SubCommands))
	}
	if got := root.SubCommands[0].CliParser; got != "go-flag" {
		t.Fatalf("child.CliParser = %q, want go-flag", got)
	}
}
