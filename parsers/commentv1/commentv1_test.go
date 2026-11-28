package commentv1

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
)

func TestCommentParser_RespectsBuildConstraints(t *testing.T) {
	files := fstest.MapFS{
		"go.mod":                     {Data: []byte("module example.com/app\n\ngo 1.22\n")},
		"shared.go":                  {Data: []byte("package app\n// Root is a subcommand `app`\nfunc Root() {}\n")},
		"commands_linux_amd64.go":    {Data: []byte("package app\n// Linux is a subcommand `app linux`\nfunc Linux() {}\n")},
		"commands_linux_arm64.go":    {Data: []byte("package app\n// Arm is a subcommand `app arm`\nfunc Arm() {}\n")},
		"commands_windows.go":        {Data: []byte("package app\n// Windows is a subcommand `app windows`\nfunc Windows() {}\n")},
		"commands_legacy.go":         {Data: []byte("// +build darwin\n\npackage app\n// Legacy is a subcommand `app legacy`\nfunc Legacy() {}\n")},
		"commands_tag.go":            {Data: []byte("//go:build special\n\npackage app\n// Tagged is a subcommand `app tagged`\nfunc Tagged() {}\n")},
		"commands_excluded_test.go":  {Data: []byte("this is intentionally not Go source")},
		"nested/commands_linux.go":   {Data: []byte("package nested\n// Nested is a subcommand `app nested`\nfunc Nested() {}\n")},
		"nested/commands_windows.go": {Data: []byte("package nested\n// NestedWin is a subcommand `app nested-windows`\nfunc NestedWin() {}\n")},
	}

	tests := []struct {
		name string
		opts parsers.ParseOptions
		want []string
	}{
		{"linux amd64", parsers.ParseOptions{Recursive: true, GOOS: "linux", GOARCH: "amd64"}, []string{"linux", "nested"}},
		{"linux arm64", parsers.ParseOptions{Recursive: true, GOOS: "linux", GOARCH: "arm64"}, []string{"arm", "nested"}},
		{"windows", parsers.ParseOptions{Recursive: true, GOOS: "windows", GOARCH: "amd64"}, []string{"nested-windows", "windows"}},
		{"legacy darwin", parsers.ParseOptions{Recursive: true, GOOS: "darwin", GOARCH: "amd64"}, []string{"legacy"}},
		{"explicit tag", parsers.ParseOptions{Recursive: true, GOOS: "linux", GOARCH: "amd64", BuildTags: []string{"special"}}, []string{"linux", "nested", "tagged"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := (&CommentParser{}).Parse(files, ".", &tt.opts)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(model.Commands) != 1 {
				t.Fatalf("commands = %d, want one root", len(model.Commands))
			}
			got := make([]string, 0, len(model.Commands[0].SubCommands))
			for _, command := range model.Commands[0].SubCommands {
				got = append(got, command.SubCommandName)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("subcommands = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubCommandTree_Insert(t *testing.T) {
	tests := []struct {
		name             string
		importPath       string
		packageName      string
		sequence         []string
		expectedLeafName string
	}{
		{
			name:             "Insert Root Level",
			importPath:       "example.com/root",
			packageName:      "main",
			sequence:         []string{},
			expectedLeafName: "root",
		},
		{
			name:             "Insert Child",
			importPath:       "example.com/child",
			packageName:      "childpkg",
			sequence:         []string{"child"},
			expectedLeafName: "child",
		},
		{
			name:             "Insert Nested Child",
			importPath:       "example.com/parent/child",
			packageName:      "childpkg",
			sequence:         []string{"parent", "child"},
			expectedLeafName: "child",
		},
		{
			name:             "Insert Deeply Nested Child",
			importPath:       "example.com/grandparent/parent/child",
			packageName:      "childpkg",
			sequence:         []string{"grandparent", "parent", "child"},
			expectedLeafName: "child",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sct := NewSubCommandTree(nil)
			sc := &model.SubCommand{
				SubCommandName: tt.expectedLeafName,
			}

			sct.Insert(tt.importPath, tt.packageName, tt.sequence, sc)

			// Traverse the tree to find the inserted node
			current := sct
			for _, part := range tt.sequence {
				next, ok := current.SubCommands[part]
				if !ok {
					t.Fatalf("Expected subcommand '%s' in sequence %v, but it was missing", part, tt.sequence)
				}
				current = next
			}

			if current.SubCommand != sc {
				t.Errorf("Expected SubCommand to be set at the leaf node")
			}

			if current.ImportPath != tt.importPath {
				t.Errorf("Expected ImportPath '%s', got '%s'", tt.importPath, current.ImportPath)
			}

			if current.SubCommandPackageName != tt.packageName {
				t.Errorf("Expected SubCommandPackageName '%s', got '%s'", tt.packageName, current.SubCommandPackageName)
			}
		})
	}
}

func TestSubCommandTree_Insert_Merge(t *testing.T) {
	t.Run("Parent then Child", func(t *testing.T) {
		sct := NewSubCommandTree(nil)

		parentSC := &model.SubCommand{SubCommandName: "parent"}
		sct.Insert("pkg/parent", "parent", []string{"parent"}, parentSC)

		childSC := &model.SubCommand{SubCommandName: "child"}
		sct.Insert("pkg/child", "child", []string{"parent", "child"}, childSC)

		// Verify parent
		parentNode, ok := sct.SubCommands["parent"]
		if !ok {
			t.Fatal("Parent node missing")
		}
		if parentNode.SubCommand != parentSC {
			t.Error("Parent subcommand mismatch")
		}

		// Verify child
		childNode, ok := parentNode.SubCommands["child"]
		if !ok {
			t.Fatal("Child node missing")
		}
		if childNode.SubCommand != childSC {
			t.Error("Child subcommand mismatch")
		}
	})

	t.Run("Child then Parent", func(t *testing.T) {
		sct := NewSubCommandTree(nil)

		childSC := &model.SubCommand{SubCommandName: "child"}
		// This creates intermediate "parent" node with nil SubCommand
		sct.Insert("pkg/child", "child", []string{"parent", "child"}, childSC)

		parentSC := &model.SubCommand{SubCommandName: "parent"}
		// This should update the existing "parent" node
		sct.Insert("pkg/parent", "parent", []string{"parent"}, parentSC)

		// Verify parent
		parentNode, ok := sct.SubCommands["parent"]
		if !ok {
			t.Fatal("Parent node missing")
		}
		if parentNode.SubCommand != parentSC {
			t.Error("Parent subcommand mismatch - likely not updated correctly")
		}

		// Verify child still exists
		childNode, ok := parentNode.SubCommands["child"]
		if !ok {
			t.Fatal("Child node missing")
		}
		if childNode.SubCommand != childSC {
			t.Error("Child subcommand mismatch")
		}
	})
}

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
		"go.mod":      &fstest.MapFile{Data: []byte("module example.com/testcli\ngo 1.21\n")},
		"commands.go": &fstest.MapFile{Data: []byte("package testcli\n\n// Root is a subcommand `app`\n// CLI-Parser: gnu\nfunc Root() {}\n\n// Child is a subcommand `app child`\n// CLI-Parser: go-flag\nfunc Child() {}\n")},
	}

	data, err := ParseGoFiles(fsys, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles: %v", err)
	}
	if len(data.Commands) != 1 {
		t.Fatalf("len(data.Commands) = %d, want 1", len(data.Commands))
	}
	root := data.Commands[0]
	if root.CLIParser != "gnu" {
		t.Fatalf("root.CLIParser = %q, want gnu", root.CLIParser)
	}
	if len(root.SubCommands) != 1 {
		t.Fatalf("len(root.SubCommands) = %d, want 1", len(root.SubCommands))
	}
	if got := root.SubCommands[0].CLIParser; got != "go-flag" {
		t.Fatalf("child.CLIParser = %q, want go-flag", got)
	}
}

func TestParseSubCommandCommentsHyphenatedProseDoesNotCreateFlags(t *testing.T) {
	text := `Generate is a subcommand ` + "`gosubc generate`" + `
Flags:
	configPath: --config (default: "") Optional version-one JSON generation configuration.
	timeout: --timeout (default: 30) Wait for a zero-value or non-negative timeout.
	format: -f --format (default: "raw") Uses built-in formats with 0-based indexing.
`
	_, _, _, _, _, params, ok := ParseSubCommandComments(text)
	if !ok {
		t.Fatal("ParseSubCommandComments returned !ok")
	}

	configParam, ok := params["configPath"]
	if !ok {
		t.Fatal("missing configPath parameter")
	}
	for _, flag := range configParam.Flags {
		if flag == "one" {
			t.Fatalf("configPath has unintended flag alias 'one': flags = %v", configParam.Flags)
		}
	}
	if len(configParam.Flags) != 1 || configParam.Flags[0] != "config" {
		t.Fatalf("configPath flags = %v, want ['config']", configParam.Flags)
	}
	if !strings.Contains(configParam.Description, "version-one") {
		t.Fatalf("configPath description lost 'version-one': got %q", configParam.Description)
	}

	timeoutParam, ok := params["timeout"]
	if !ok {
		t.Fatal("missing timeout parameter")
	}
	for _, flag := range timeoutParam.Flags {
		if flag == "value" || flag == "negative" {
			t.Fatalf("timeout has unintended flag alias %q: flags = %v", flag, timeoutParam.Flags)
		}
	}
	if !strings.Contains(timeoutParam.Description, "zero-value") {
		t.Fatalf("timeout description lost 'zero-value': got %q", timeoutParam.Description)
	}

	formatParam, ok := params["format"]
	if !ok {
		t.Fatal("missing format parameter")
	}
	for _, flag := range formatParam.Flags {
		if flag == "in" || flag == "based" {
			t.Fatalf("format has unintended flag alias %q: flags = %v", flag, formatParam.Flags)
		}
	}
	if !strings.Contains(formatParam.Description, "built-in") || !strings.Contains(formatParam.Description, "0-based") {
		t.Fatalf("format description lost prose: got %q", formatParam.Description)
	}
}

func TestParseRequiresDirectives_AndDiagnostics(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/reqapp\n\ngo 1.22\n")},
		"auth/auth.go": {Data: []byte(`package auth

type User struct{}
type Session struct{}
type Audit struct{}

func RequireAdmin(u *User) error              { return nil }
func CurrentUser() (*User, error)             { return nil, nil }
func MakeSession(a, b string) (*Session, error) { return nil, nil }
func AuditTrail() (*Audit, error)             { return nil, nil }
func BadNoReturn()                            {}
func BadThree() (int, string, error)          { return 0, "", nil }
func BadSecond() (int, string)                { return 0, "" }
func BadErrErr() (error, error)               { return nil, nil }
func unexportedFn() error                     { return nil }
`)},
		"app/commands.go": {Data: []byte(`package app

import myauth "example.com/reqapp/auth"

// Root is a subcommand ` + "`app`" + ` -- Root cmd.
//
// Flags:
//	a: --a Env a.
//	b: --b Env b.
func Root(a, b string) error { return nil }

// Admin is a command ` + "`app admin`" + ` -- Admin commands.
// Requires: myauth.CurrentUser, sess = myauth.MakeSession(a, b)
// Requires: myauth.RequireAdmin
// Requires: myauth.AuditTrail (always)
// Additional admin usage line.

// Inspect is a subcommand ` + "`app admin inspect`" + ` -- Inspect.
func Inspect(user *myauth.User, sess *myauth.Session, audit *myauth.Audit) error { return nil }
`)},
	}

	dm, err := (&CommentParser{}).Parse(fsys, ".", &parsers.ParseOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	admin := dm.Commands[0].SubCommands[0]
	if len(admin.Requirements) != 4 {
		t.Fatalf("admin.Requirements len = %d, want 4", len(admin.Requirements))
	}
	if admin.Requirements[0].Func.FunctionName != "CurrentUser" || admin.Requirements[0].Func.ImportPath != "example.com/reqapp/auth" {
		t.Fatalf("req[0] = %+v", admin.Requirements[0])
	}
	if admin.Requirements[1].Name != "sess" || len(admin.Requirements[1].ExplicitInputs) != 2 || admin.Requirements[1].ExplicitInputs[0] != "a" || admin.Requirements[1].ExplicitInputs[1] != "b" {
		t.Fatalf("req[1] = %+v", admin.Requirements[1])
	}
	if admin.Requirements[2].Kind != model.RequirementKindGuard || admin.Requirements[2].Policy != model.RequirementPolicyOnDemand {
		t.Fatalf("req[2] = %+v", admin.Requirements[2])
	}
	if admin.Requirements[3].Policy != model.RequirementPolicyAlways || admin.Requirements[3].Kind != model.RequirementKindProvider {
		t.Fatalf("req[3] = %+v", admin.Requirements[3])
	}
	if admin.SubCommandExtendedHelp != "Additional admin usage line." {
		t.Fatalf("admin.SubCommandExtendedHelp = %q, want Requires lines stripped", admin.SubCommandExtendedHelp)
	}

	diagCases := []struct {
		name string
		src  string
		want string
	}{
		{"negated", "package app\n// Run is a subcommand `app`\n// Requires: !auth.Check\nfunc Run() {}\n", "negative requirement"},
		{"empty", "package app\n// Run is a subcommand `app`\n// Requires:   \nfunc Run() {}\n", "empty Requires"},
		{"empty slot", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.CurrentUser, , auth.RequireAdmin\nfunc Run() {}\n", "empty entry"},
		{"unknown modifier", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.CurrentUser (lazy)\nfunc Run() {}\n", "unsupported requirement modifier"},
		{"duplicate always", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.CurrentUser (always) (always)\nfunc Run() {}\n", "duplicate modifier (always)"},
		{"unimported qualifier no imports", "package app\n// Run is a subcommand `app`\n// Requires: auth.CurrentUser\nfunc Run() {}\n", "unimported package qualifier"},
		{"unimported qualifier with imports", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: wrong.CurrentUser\nfunc Run() {}\n", "unimported package qualifier"},
		{"missing function", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.DoesNotExist\nfunc Run() {}\n", "not found"},
		{"unexported function", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.unexportedFn\nfunc Run() {}\n", "must be exported"},
		{"zero returns", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.BadNoReturn\nfunc Run() {}\n", "0 return values"},
		{"three returns", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.BadThree\nfunc Run() {}\n", "returns 3 values"},
		{"second not error", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.BadSecond\nfunc Run() {}\n", "invalid second return type"},
		{"error error", "package app\nimport \"example.com/reqapp/auth\"\n// Run is a subcommand `app`\n// Requires: auth.BadErrErr\nfunc Run() {}\n", "(error, error)"},
	}
	for _, tc := range diagCases {
		t.Run(tc.name, func(t *testing.T) {
			badFS := fstest.MapFS{
				"go.mod":          fsys["go.mod"],
				"auth/auth.go":    fsys["auth/auth.go"],
				"app/commands.go": {Data: []byte(tc.src)},
			}
			_, err := (&CommentParser{}).Parse(badFS, ".", &parsers.ParseOptions{Recursive: true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got err=%v, want substring %q", err, tc.want)
			}
		})
	}
}
