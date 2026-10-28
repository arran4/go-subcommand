package go_subcommand

import (
	"bytes"
	_ "embed"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"
	"unicode/utf8"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
	"github.com/arran4/go-subcommand/parsers/commentv1"
)

//go:embed testdata/issue350_388.go
var issue350388Source string

//go:embed testdata/issue330.go
var issue330Source string

//go:embed testdata/issue18.go
var issue18Source string

//go:embed testdata/dashdash.go
var dashdashSource string

//go:embed testdata/issue481.go
var issue481Source string

func runtimeDisplayWidth(s string) int {
	width := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == ';') {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		width++
	}
	return width
}

func runtimePadRight(s string, width int) string {
	w := runtimeDisplayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func runtimeWrapText(text string, indent int, width int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if width <= indent {
		width = indent + 10
	}
	avail := width - indent
	indentStr := "\n" + strings.Repeat(" ", indent)

	var out strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		words := strings.Fields(line)
		if len(words) == 0 {
			if i > 0 {
				out.WriteString("\n")
			}
			continue
		}
		if i > 0 {
			out.WriteString(indentStr)
		}

		currentLen := 0
		for j, word := range words {
			wordLen := runtimeDisplayWidth(word)
			if j > 0 && currentLen+1+wordLen > avail {
				out.WriteString(indentStr)
				currentLen = 0
			} else if j > 0 {
				out.WriteString(" ")
				currentLen++
			}
			out.WriteString(word)
			currentLen += wordLen
		}
	}
	return out.String()
}

func runtimeFormatUsageRow(prefix, descStr string, width int) string {
	if strings.TrimSpace(descStr) == "" {
		return strings.TrimRight(prefix, " ")
	}
	indent := 4 + runtimeDisplayWidth(prefix)
	return prefix + runtimeWrapText(descStr, indent, width)
}

func executeGeneratedUsageTemplateAtWidth(t *testing.T, tmplText string, recursive bool, width int) string {
	t.Helper()
	funcs := template.FuncMap{
		"formatHelpRow": func(labelWidth int, label, descStr string) string {
			if strings.TrimSpace(descStr) == "" {
				return label
			}
			prefix := runtimePadRight(label, labelWidth) + " "
			return runtimeFormatUsageRow(prefix, descStr, width)
		},
		"formatFlagRow": func(maxFlag, maxDef int, flagStr, defStr, descStr string) string {
			var prefix string
			if maxDef > 0 {
				prefix = runtimePadRight(flagStr, maxFlag) + " " + runtimePadRight(defStr, maxDef) + " "
			} else if defStr != "" {
				prefix = runtimePadRight(flagStr, maxFlag) + " " + defStr + " "
			} else {
				prefix = runtimePadRight(flagStr, maxFlag) + " "
			}
			return runtimeFormatUsageRow(prefix, descStr, width)
		},
		"wrapFlag": func(maxFlag, maxDef int, flagStr, defStr, descStr string) string {
			if strings.TrimSpace(descStr) == "" {
				return ""
			}
			indent := 4 + maxFlag + 1
			if maxDef > 0 {
				indent += maxDef + 1
			}
			return runtimeWrapText(descStr, indent, width)
		},
	}
	parsed, err := template.New("usage").Funcs(funcs).Parse(tmplText)
	if err != nil {
		t.Fatalf("failed to parse generated usage template: %v\n%s", err, tmplText)
	}
	var buf bytes.Buffer
	err = parsed.Execute(&buf, struct {
		Recursive bool
		Injected  string
		Exploit   string
	}{
		Recursive: recursive,
		Injected:  "INJECTED_VALUE",
		Exploit:   "EXPLOITED_VALUE",
	})
	if err != nil {
		t.Fatalf("failed to execute generated usage template (recursive=%v, width=%d): %v\n%s", recursive, width, err, tmplText)
	}
	return buf.String()
}

func executeGeneratedUsageTemplate(t *testing.T, tmplText string, recursive bool) string {
	t.Helper()
	return executeGeneratedUsageTemplateAtWidth(t, tmplText, recursive, 120)
}

// setupProject returns an in-memory FS
func setupProject(t *testing.T, sourceCode string) fstest.MapFS {
	return fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(sourceCode)},
	}
}

// runGenerateInMemory runs the generator using in-memory FS and Writer
func runGenerateInMemory(t *testing.T, inputFS fstest.MapFS) *CollectingFileWriter {
	writer := NewCollectingFileWriter()
	// We use a dummy dir name like "." or "/app"
	if err := GenerateWithFS(inputFS, writer, ".", "", "commentv1", &parsers.ParseOptions{}, false, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	return writer
}

func TestIssue350_388_GeneratedRequirements(t *testing.T) {
	input := fstest.MapFS{
		"go.mod":     &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go":    &fstest.MapFile{Data: []byte(issue350388Source)},
		"pkg/pkg.go": &fstest.MapFile{Data: []byte("package pkg\n\nfunc ParseThing(s string) (string, error) { return s, nil }\nfunc GenThing() (string, error) { return \"generated\", nil }\n")},
	}
	writer := runGenerateInMemory(t, input)

	rootUsage := string(mustGeneratedFile(t, writer, "cmd/app/templates/app_usage.txt"))
	if !strings.Contains(rootUsage, "Usage: app [flags...]") {
		t.Fatalf("root usage template was not generated from usage.txt.gotmpl:\n%s", rootUsage)
	}

	rootGo := string(mustGeneratedFile(t, writer, "cmd/app/root.go"))
	assertContains(t, rootGo, `executeUsage(os.Stderr, "app_usage.txt"`, "root usage should use embedded usage template")
	assertContains(t, rootGo, `seenFlags := make(map[string]bool)`, "required root flag should create seenFlags")
	assertContains(t, rootGo, `required flag --config not provided`, "required root flag should be validated")
	if action := strings.Index(rootGo, "if c.CommandAction != nil"); action == -1 {
		t.Fatal("root command action missing")
	} else if dispatch := strings.Index(rootGo, "if !dashDashSeen && len(remainingArgs) > 0"); dispatch == -1 {
		t.Fatal("root command dispatch missing")
	} else if dispatch > action {
		t.Fatalf("root command dispatch should run before the root action\nroot.go:\n%s", rootGo)
	}
	assertContains(t, rootGo, "if err := c.CommandAction(c); err != nil {\n\t\t\treturn err\n\t\t}", "RootCmd.Execute should not add duplicate action context")

	childGo := string(mustGeneratedFile(t, writer, "cmd/app/parent_child.go"))
	assertContains(t, childGo, `"example.com/test/pkg"`, "external parser/generator package should be imported")
	assertContains(t, childGo, `pkg.ParseThing(value)`, "custom parser should be called")
	assertContains(t, childGo, `pkg.GenThing()`, "generator should be called")
	assertNotContains(t, childGo, `case "generated":`, "generator-backed parameter should not be exposed as a flag")
	assertContains(t, childGo, `c.values = append(c.values, value)`, "slice flags should be repeatable")
	assertContains(t, childGo, `c.ptr = &val`, "pointer flag should preserve omitted-vs-zero state")
	assertContains(t, childGo, `value = shorts[j+1:]`, "short value flags should consume the rest of a GNU-style short cluster")
	assertContains(t, childGo, "if err := c.CommandAction(c); err != nil {\n\t\t\treturn err\n\t\t}", "child Execute should not add duplicate action context")

	childUsage := string(mustGeneratedFile(t, writer, "cmd/app/templates/child_usage.txt"))
	assertContains(t, childUsage, "`parent` Flags:", "from-parent flag should be grouped under parent")
	assertContains(t, childUsage, "`child` Flags:", "child-local flags should be grouped under child")
	assertNotContains(t, childUsage, "generated", "generator-backed parameter should not appear in usage flags")
}

func mustGeneratedFile(t *testing.T, writer *CollectingFileWriter, path string) []byte {
	t.Helper()
	content, ok := writer.Files[path]
	if !ok {
		t.Fatalf("generated file %q missing; generated files: %v", path, writer.Files)
	}
	return content
}

func assertContains(t *testing.T, s, substr, msg string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("%s: missing %q in:\n%s", msg, substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr, msg string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("%s: found unexpected %q in:\n%s", msg, substr, s)
	}
}

func TestDashDashExecutionLogic(t *testing.T) {
	input := setupProject(t, dashdashSource)
	writer := runGenerateInMemory(t, input)

	rootGo := string(mustGeneratedFile(t, writer, "cmd/app/root.go"))
	assertContains(t, rootGo, `dashDashSeen := false`, "dashDashSeen variable should be tracked")
	assertContains(t, rootGo, `if arg == "--" {`, "dash dash should be evaluated")
	assertContains(t, rootGo, `dashDashSeen = true`, "dashDashSeen should be set to true")
	assertContains(t, rootGo, `if !dashDashSeen && len(remainingArgs) > 0 {`, "subcommand dispatch should check dashDashSeen")

	cmdGo := string(mustGeneratedFile(t, writer, "cmd/app/command.go"))
	assertContains(t, cmdGo, `dashDashSeen := false`, "dashDashSeen variable should be tracked in subcommand")
	assertContains(t, cmdGo, `if !dashDashSeen && len(remainingArgs) > 0 {`, "subcommand dispatch should check dashDashSeen in subcommand")
}

func TestGeneratedGoEscapesUserDescriptions(t *testing.T) {
	description := "delimiter is \\\"\\n\\\"\\twith {{template-like}} Unicode: café"
	for _, parserName := range []string{"gnu", "go-flag"} {
		t.Run(parserName, func(t *testing.T) {
			input := fstest.MapFS{
				"go.mod": &fstest.MapFile{Data: []byte("module example.com/escape\n\ngo 1.22\n")},
				"app.go": &fstest.MapFile{Data: []byte("package app\n\n// Root is a subcommand `escape`\n// CLI-Parser: " + parserName + "\n// Flags:\n//\tdelim: -d --delim " + description + "\nfunc Root(delim string) {}\n")},
			}
			writer := NewCollectingFileWriter()
			if err := GenerateWithFS(input, writer, ".", "", "commentv1", nil, false, false, nil, false, false, "", "", ""); err != nil {
				t.Fatalf("GenerateWithFS: %v", err)
			}
			root := string(mustGeneratedFile(t, writer, "cmd/escape/root.go"))
			if _, err := parser.ParseFile(token.NewFileSet(), "root.go", root, parser.AllErrors); err != nil {
				t.Fatalf("generated source does not escape its description: %v\n%s", err, root)
			}
			if !strings.Contains(root, `\\\"`) || !strings.Contains(root, `\\t`) {
				t.Fatalf("generated source did not preserve escaped description content:\n%s", root)
			}
		})
	}
}

func TestIssue18_MoreBasicTypes(t *testing.T) {
	writer := runGenerateInMemory(t, setupProject(t, issue18Source))
	code := string(mustGeneratedFile(t, writer, "cmd/app/mycmd.go"))

	for _, typ := range []string{
		"int64", "int32", "int16", "int8",
		"uint", "uint64", "uint32", "uint16", "uint8",
		"float64", "float32", "[]int64", "*int64",
	} {
		if strings.Contains(code, "TODO: Implement parsing for flag type "+typ) {
			t.Errorf("Missing parsing for flag type %s", typ)
		}
	}
}

func TestIssue55_SanitizeToIdentifier(t *testing.T) {
	tests := []struct{ input, expected string }{
		{"", "Cmd"}, {"123test", "Cmd123test"}, {"123-test", "Cmd123Test"},
		{"123", "Cmd123"}, {"@foo", "Foo"}, {" foo", "Foo"},
		{"-foo", "Foo"}, {"_123", "Cmd123"}, {"foo@bar", "FooBar"},
		{"foo#bar", "FooBar"}, {"foo$bar", "FooBar"}, {"foo123bar", "Foo123bar"},
		{"foo-123-bar", "Foo123Bar"}, {"123-foo@bar.com", "Cmd123FooBarCom"},
		{"!@#$", "Cmd"}, {"héllo", "Héllo"}, {"éllo", "Éllo"},
		{"1éllo", "Cmd1éllo"}, {"１test", "Cmd１test"}, {"a💩b", "AB"},
		{"💩", "Cmd"}, {"💩a", "A"},
	}
	for _, tt := range tests {
		if got := parsers.SanitizeToIdentifier(tt.input); got != tt.expected {
			t.Errorf("SanitizeToIdentifier(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIssue55_ToKebabCase(t *testing.T) {
	tests := []struct{ input, expected string }{
		{"Test", "test"}, {"123Test", "123-test"}, {"123test", "123test"},
		{"Cmd123Test", "cmd123-test"}, {"Cmd123test", "cmd123test"},
		{"JSONData", "json-data"}, {"JSON123Data", "json123-data"},
		{"JSON123data", "json123data"}, {"My123Test", "my123-test"},
		{"My123test", "my123test"},
	}
	for _, tt := range tests {
		if got := parsers.ToKebabCase(tt.input); got != tt.expected {
			t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIssue330_ParentFlagsReporting(t *testing.T) {
	writer := runGenerateInMemory(t, setupProject(t, issue330Source))

	usageText := string(mustGeneratedFile(t, writer, "cmd/app/templates/child_usage.txt"))
	if !strings.Contains(usageText, "`child` Flags:") {
		t.Error("Missing '`child` Flags:' section in usage")
	}
	if !strings.Contains(usageText, "`parent` Flags:") {
		t.Error("Missing '`parent` Flags:' section in usage")
	}

	parentIndex := strings.Index(usageText, "`parent` Flags:")
	childIndex := strings.Index(usageText, "`child` Flags:")
	dirIndex := strings.Index(usageText, "--dir")
	if parentIndex == -1 || childIndex == -1 || dirIndex == -1 {
		t.Fatalf("incomplete parent/child usage output:\n%s", usageText)
	}
	if parentIndex > childIndex || dirIndex < parentIndex || dirIndex > childIndex {
		t.Errorf("--dir should be listed in the parent group before child flags:\n%s", usageText)
	}
	assertNotContains(t, usageText, "-d string", "child argument d should inherit --dir")
	assertNotContains(t, usageText, "--ir int", "child argument ir should use the documented --i flag")
	assertContains(t, usageText, "-i int", "child argument ir should use -i")
	assertContains(t, usageText, "(default: 0)", "child argument ir should retain its documented default")
	childCode := string(mustGeneratedFile(t, writer, "cmd/app/parent_child.go"))
	assertNotContains(t, childCode, `case "dir"`, "inherited parent flags should not be reparsed by the child")
	assertContains(t, childCode, "Child(c.dir, c.ir)", "child action should read the embedded parent value")

	grandchildUsage := string(mustGeneratedFile(t, writer, "cmd/app/templates/grandchild_usage.txt"))
	for _, expected := range []string{"`parent` Flags:", "--dir", "The directory"} {
		if !strings.Contains(grandchildUsage, expected) {
			t.Errorf("Grandchild usage missing %q:\n%s", expected, grandchildUsage)
		}
	}
}

func TestIssue33_HyphenatedCommands_Content(t *testing.T) {
	src := `package main

// ListHeads is a subcommand ` + "`app list-heads`" + `
func ListHeads() {}
`
	fs := setupProject(t, src)
	writer := NewCollectingFileWriter()

	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{}, false, false, nil, false, false, "", "", "")

	if err != nil {
		// This test verifies that the issue is still present (OPEN).
		// The generator currently produces invalid Go code for hyphenated commands,
		// causing format.Source to fail.
		// We log the failure as expected behavior for this verification test.
		t.Logf("Issue #33 verified as OPEN: Generation failed as expected: %v", err)
		t.Fail()
	}
}

func TestIssue41_VersionInUsageForNestedCommand(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd() {}

// Child is a subcommand ` + "`app mycmd child`" + `
func Child() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	usageText := string(content)
	if strings.Contains(usageText, "version      Print version information") {
		t.Errorf("Issue #41: Usage text contains 'version' command which is not implemented for nested commands")
	}
}

func TestIssue19_HyphenatedCommands_Content(t *testing.T) {
	TestIssue33_HyphenatedCommands_Content(t)
}

func TestIssue21_EmptyFileGeneration(t *testing.T) {
	src := `package main
// Cmd is a subcommand ` + "`app cmd`" + `
func Cmd() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	for path, content := range writer.Files {
		if len(content) == 0 {
			t.Errorf("Found empty generated file: %s", path)
		}
	}
}

func TestIssue20_NestedSubcommandsFlattened_Model(t *testing.T) {
	src := `package main
	// ListUsers is a subcommand ` + "`address admin list-users`" + `
	func ListUsers() {}
	`

	fs := fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(src)},
	}

	m, err := commentv1.ParseGoFiles(fs, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles failed: %v", err)
	}

	if len(m.Commands) != 1 {
		t.Fatalf("Expected 1 root command, got %d", len(m.Commands))
	}
	root := m.Commands[0]

	var admin *model.SubCommand
	for _, sub := range root.SubCommands {
		if sub.SubCommandName == "admin" {
			admin = sub
			break
		}
	}
	if admin == nil {
		t.Fatal("Expected 'admin' subcommand, but it was flattened or missing")
		return
	}

	found := false
	for _, sub := range admin.SubCommands {
		if sub.SubCommandName == "list-users" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected 'list-users' to be nested under 'admin'")
	}
}

func TestIssue25_MissingHelpText_FromParamComments(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd(
	username string, // User name for login
) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	if !strings.Contains(string(content), "User name for login") {
		t.Errorf("Expected usage text to contain 'User name for login', got:\n%s", string(content))
	}
}

func TestIssue10_MissingFlagDescriptions(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd(
	// Output format
	output string,
	columns int, // Number of columns
	// Verbose mode
	verbose bool, // Enable verbose logging
) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	usageText := string(content)

	// Check "Line before"
	if !strings.Contains(usageText, "Output format") {
		t.Errorf("Issue #10: Usage text missing description 'Output format' (line before) for flag output. Got:\n%s", usageText)
	}

	// Check "Same line after"
	if !strings.Contains(usageText, "Number of columns") {
		t.Errorf("Issue #10: Usage text missing description 'Number of columns' (same line) for flag columns. Got:\n%s", usageText)
	}

	// Check "Priority" for Verbose (Line before vs Same line after)
	// With priority logic (Inline > Preceding), "Enable verbose logging" should overwrite "Verbose mode".
	// So "Enable verbose logging" MUST be present.
	if !strings.Contains(usageText, "Enable verbose logging") {
		t.Errorf("Issue #10: Usage text missing description 'Enable verbose logging' (same line) for flag verbose. Got:\n%s", usageText)
	}
	// And "Verbose mode" might NOT be present (since it was overwritten).
	// If it IS present, that implies concatenation, which is NOT what the user asked for in Priority test.
	// But let's just assert the winner is there.
}

func TestFlagDescriptionPriority_Exhaustive(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		expected string
	}{
		{
			name: "All 3 Present -> Flags Block Wins",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
// Flags:
//
//	verbose: Top Priority
func Cmd(
	// Preceding Priority
	verbose bool, // Inline Priority
) {}
`,
			expected: "Top Priority",
		},
		{
			name: "Flags + Inline -> Flags Block Wins",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
// Flags:
//
//	verbose: Top Priority
func Cmd(
	verbose bool, // Inline Priority
) {}
`,
			expected: "Top Priority",
		},
		{
			name: "Flags + Preceding -> Flags Block Wins",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
// Flags:
//
//	verbose: Top Priority
func Cmd(
	// Preceding Priority
	verbose bool,
) {}
`,
			expected: "Top Priority",
		},
		{
			name: "Inline + Preceding -> Inline Wins",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
func Cmd(
	// Preceding Priority
	verbose bool, // Inline Priority
) {}
`,
			expected: "Inline Priority",
		},
		{
			name: "Only Flags -> Flags Wins",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
// Flags:
//
//	verbose: Top Priority
func Cmd(verbose bool) {}
`,
			expected: "Top Priority",
		},
		{
			name: "Only Inline -> Inline Wins",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
func Cmd(
	verbose bool, // Inline Priority
) {}
`,
			expected: "Inline Priority",
		},
		{
			name: "Only Preceding -> Preceding Wins",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
func Cmd(
	// Preceding Priority
	verbose bool,
) {}
`,
			expected: "Preceding Priority",
		},
		{
			name: "None -> Empty",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
func Cmd(verbose bool) {}
`,
			expected: "",
		},
		{
			name: "Flags Block Empty Description -> Inline Fallback",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
// Flags:
//
//	verbose: --verbose
func Cmd(
	verbose bool, // Inline Priority
) {}
`,
			expected: "Inline Priority",
		},
		{
			name: "Inline Empty -> Preceding Fallback",
			src: `package main
// Cmd is a subcommand ` + "`app cmd`" + `
func Cmd(
	// Preceding Priority
	verbose bool,
) {}
`,
			expected: "Preceding Priority",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := setupProject(t, tt.src)
			writer := runGenerateInMemory(t, fs)

			usagePath := "cmd/app/templates/cmd_usage.txt"
			content, ok := writer.Files[usagePath]
			if !ok {
				t.Fatalf("Usage file not found: %s", usagePath)
			}
			usageText := string(content)

			if tt.expected == "" {
				if !strings.Contains(usageText, "--verbose") {
					t.Errorf("Expected usage to contain --verbose")
				}
			} else {
				if !strings.Contains(usageText, tt.expected) {
					t.Errorf("Expected description '%s', got usage:\n%s", tt.expected, usageText)
				}
				if strings.Contains(tt.src, "Preceding Priority") && tt.expected != "Preceding Priority" {
					if strings.Contains(usageText, "Preceding Priority") {
						t.Errorf("Lower priority 'Preceding Priority' leaked into usage:\n%s", usageText)
					}
				}
				if strings.Contains(tt.src, "Inline Priority") && tt.expected != "Inline Priority" {
					if strings.Contains(usageText, "Inline Priority") {
						t.Errorf("Lower priority 'Inline Priority' leaked into usage:\n%s", usageText)
					}
				}
			}
		})
	}
}

func TestPriorityFlagDescriptions(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
// Flags:
//
//	verbose: Top Priority
func MyCmd(
	// 3rd priority
	verbose bool, // 2nd Priority
) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	usageText := string(content)

	// The user expects priority:
	// 1. Flags (Top Priority)
	// 2. Inline (2nd Priority)
	// 3. Line before (3rd Priority)

	// In this test case, we have Top Priority provided in Flags block.
	// So we expect "Top Priority" to be present, and potentially the others NOT present if it overrides?
	// The request was "Test this", suggesting verifying that "Top Priority" wins.

	if !strings.Contains(usageText, "Top Priority") {
		t.Errorf("Expected 'Top Priority' description from Flags block, but missing. Got:\n%s", usageText)
	}

	// If we implement strict override, 2nd and 3rd might be absent.
	// But if we concat, they might be present.
	// Based on "Priority", override seems more logical.
}

func TestIssue24_FlagNamingConvention(t *testing.T) {
	src := `package main
// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd(projectId string) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	if !strings.Contains(string(content), "--project-id") {
		t.Errorf("Expected flag '--project-id', got content:\n%s", string(content))
	}
}

func TestIssue26_DefaultValuesInHelp(t *testing.T) {
	srcWithFlags := `package main
// MyCmd is a subcommand ` + "`app mycmd`" + `
// Flags:
//
//	retries: --retries (default: 3) Number of retries
func MyCmd(retries int) {}
`
	fs := setupProject(t, srcWithFlags)
	writer := runGenerateInMemory(t, fs)

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	if !strings.Contains(string(content), "(default: 3)") {
		t.Errorf("Expected usage text to contain default value '(default: 3)', got:\n%s", string(content))
	}
}

func TestIssue23_ShortFlagAliases(t *testing.T) {
	src := `package main
// MyCmd is a subcommand ` + "`app mycmd`" + `
// Flags:
//
//	force: -f --force Force execution
func MyCmd(force bool) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	if !strings.Contains(string(content), "-f") {
		t.Errorf("Expected usage text to contain short flag '-f', got:\n%s", string(content))
	}
}

func TestIssue16_GenerationNote(t *testing.T) {
	src := `package main
// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	generatedFile := "cmd/app/mycmd.go"
	content, ok := writer.Files[generatedFile]
	if !ok {
		t.Fatalf("Generated file not found: %s", generatedFile)
	}

	expectedNote := "// Code generated by github.com/arran4/go-subcommand"
	if !strings.HasPrefix(string(content), expectedNote) {
		t.Errorf("Expected file to start with generation note '%s', got:\n%s...", expectedNote, string(content)[:50])
	}
}

func TestIssue17_NonFlaggedArguments(t *testing.T) {
	src := `package main
// MyCmd is a subcommand ` + "`app mycmd`" + `
// filename: @1 Filename to process
func MyCmd(filename string) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	if strings.Contains(string(content), "-filename") {
		t.Errorf("Argument 'filename' treated as flag. Should be positional.")
	}
	if !strings.Contains(string(content), "<filename>") {
		t.Errorf("Usage string missing positional arg <filename>")
	}
}

func TestPositionalArgsAndVarArgs(t *testing.T) {
	src := `package main
// MyCmd is a subcommand ` + "`app mycmd`" + `
// id: @1 ID
// name: @2 Name
// files: 1...3 Files
func MyCmd(id int, name string, files ...string) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	cmdPath := "cmd/app/mycmd.go"
	content, ok := writer.Files[cmdPath]
	if !ok {
		t.Fatalf("Generated command file not found: %s", cmdPath)
	}

	code := string(content)
	if strings.Contains(code, "set.IntVar(&v.id") {
		t.Errorf("Positional arg 'id' generated as flag")
	}
	if !strings.Contains(code, "strconv.Atoi(argVal)") {
		t.Errorf("Missing int conversion for positional arg")
	}
	if !strings.Contains(code, "expected at least 2 positional arguments") {
		t.Errorf("Missing positional count validation")
	}
	if !strings.Contains(code, "expected at least 1 arguments for files") {
		t.Errorf("Missing vararg min validation")
	}
	if !strings.Contains(code, "expected at most 3 arguments for files") {
		t.Errorf("Missing vararg max validation")
	}

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	usageContent, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}
	usage := string(usageContent)
	if !strings.Contains(usage, "<id> <name> [files...]") {
		t.Errorf("Usage string incorrect format: %s", usage)
	}
}

func TestOptionalPositionalArgs(t *testing.T) {
	src := "package main\n" +
		"// MyCmd is a subcommand " + "\x60" + "app mycmd" + "\x60\n" +
		"// req: @1 Required arg\n" +
		"// opt1: @2 (default: \"hello\") Optional arg 1\n" +
		"// opt2: @3 (default: \"\") Optional arg 2 (explicit empty string)\n" +
		"func MyCmd(req string, opt1 string, opt2 string) {}\n"

	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	cmdPath := "cmd/app/mycmd.go"
	codeBytes, ok := writer.Files[cmdPath]
	if !ok {
		t.Fatalf("Generated command file not found: %s", cmdPath)
	}

	code := string(codeBytes)
	if !strings.Contains(code, "expected at least 1 positional arguments") {
		t.Errorf("Missing positional count validation for required only")
	}
	if !strings.Contains(code, "c.opt1 = \"hello\"") {
		t.Errorf("Missing default assignment for omitted optional positional arg")
	}
	if !strings.Contains(code, "c.opt2 = \"\"") {
		t.Errorf("Missing default assignment for omitted optional positional arg (empty string)")
	}

	usagePath := "cmd/app/templates/mycmd_usage.txt"
	usageContent, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}
	usage := string(usageContent)
	if !strings.Contains(usage, "<req>") {
		t.Errorf("Usage string missing <req>: %s", usage)
	}
	if !strings.Contains(usage, "[opt1]") {
		t.Errorf("Usage string missing [opt1]: %s", usage)
	}
	if !strings.Contains(usage, "[opt2]") {
		t.Errorf("Usage string missing [opt2]: %s", usage)
	}
}

func TestPositionalArgsValidationOrder(t *testing.T) {
	src := "package main\n" +
		"// MyCmd is a subcommand " + "\x60" + "app mycmd" + "\x60\n" +
		"// opt: @1 (default: \"hello\") Optional arg\n" +
		"// req: @2 Required arg\n" +
		"func MyCmd(opt string, req string) {}\n"

	fs := setupProject(t, src)
	dataModel, err := parse(".", "commentv1", &parsers.ParseOptions{
		SearchPaths: []string{"."},
		Recursive:   true,
	}, fs)
	if err == nil {
		err = dataModel.Validate()
	}
	if err == nil {
		t.Errorf("Expected validation error for required positional after optional")
	} else if !strings.Contains(err.Error(), "required positional argument req appears after an optional positional argument") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestIssue42_MissingHelpUsageInGuide(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd() {}

// Child is a subcommand ` + "`app mycmd child`" + `
func Child() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	// Check subcommand usage
	usagePath := "cmd/app/templates/mycmd_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	usageText := string(content)
	missing := []string{}
	expected := []string{"help", "usage"}

	for _, exp := range expected {
		if !strings.Contains(usageText, exp) {
			missing = append(missing, exp)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Expected usage text to contain %v, but they were missing. Content:\n%s", missing, usageText)
	}
}

func TestIssue11_RootLevelHelpUsageVersion(t *testing.T) {
	src := `package main

// Nested is a subcommand ` + "`app nested`" + `
func Nested() {}

// Child is a subcommand ` + "`app nested child`" + `
func Child() {}

// GrandChild is a subcommand ` + "`app nested child grandchild`" + `
func GrandChild() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	// Check root command code
	rootPath := "cmd/app/root.go"
	content, ok := writer.Files[rootPath]
	if !ok {
		t.Fatalf("Root file not found: %s", rootPath)
	}

	rootCode := string(content)

	// Requirement: Version only at top level.
	// We verify RootCmd supports Version.
	if !strings.Contains(rootCode, "Version") || !strings.Contains(rootCode, "string") {
		t.Errorf("Root command should support Version")
	}

	// Requirement: Help and usage should work at every level (including Root).
	// We expect 'help' and 'usage' to be registered as commands in the RootCmd.
	// The current generator populates commands like: c.Commands["name"] = ...

	if !strings.Contains(rootCode, `c.Commands["help"]`) {
		t.Errorf("Issue 11: Root command should have 'help' command registered")
	}
	if !strings.Contains(rootCode, `c.Commands["usage"]`) {
		t.Errorf("Issue 11: Root command should have 'usage' command registered")
	}

	// Check if 'version' is registered as a command if it is explicitly listed in usage.
	// The prompt says "version only at top level".
	// If it is just a flag or handled in Execute, that might be fine, but if it appears in "Commands:" list
	// it should be in the map.
	// Current usage template lists "version".
	// So we expect it to be a command.
	if !strings.Contains(rootCode, `c.Commands["version"]`) {
		t.Errorf("Issue 11: Root command should have 'version' command registered")
	}
}

func TestIssue11_42_52_HelpUsageVersionVisibility(t *testing.T) {
	src := `package main

// Child is a subcommand ` + "`app nested child`" + `
func Child() {}

// GrandChild is a subcommand ` + "`app nested child grandchild`" + `
func GrandChild() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	// Check nested command usage
	usagePath := "cmd/app/templates/nested_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	usageText := string(content)
	renderedNestedUsage := executeGeneratedUsageTemplate(t, usageText, false)

	// Issue 11 & 42: Help and usage should work (be visible)
	// UPDATE per user request: Functionless parent subcommands (like 'nested' here) are equiv to usage/help
	// and thus don't need to output them as subcommands.
	present := []string{}
	unexpected := []string{"help", "usage"}
	for _, unexp := range unexpected {
		if strings.Contains(renderedNestedUsage, unexp+" ") {
			present = append(present, unexp)
		}
	}
	if len(present) > 0 {
		t.Errorf("Issue 11/42: Expected nested usage text to NOT contain %v (functionless parent), but they were present.\nContent:\n%s", present, renderedNestedUsage)
	}

	// Issue 41/52: Version should NOT be at nested level
	if strings.Contains(renderedNestedUsage, "version      Print version information") {
		t.Errorf("Issue 41/52: Nested usage text contains 'version' command which should only be at top level.\nContent:\n%s", renderedNestedUsage)
	}

	// Check child command usage (explicitly functional)
	childUsagePath := "cmd/app/templates/child_usage.txt"
	childContent, ok := writer.Files[childUsagePath]
	if !ok {
		t.Fatalf("Child usage file not found: %s", childUsagePath)
	}
	childUsageText := string(childContent)
	renderedChildUsage := executeGeneratedUsageTemplate(t, childUsageText, false)

	// Issue 11 & 42: Help and usage SHOULD be present for normal commands
	missing := []string{}
	expected := []string{"help", "usage"}
	for _, exp := range expected {
		if !strings.Contains(renderedChildUsage, exp+" ") {
			missing = append(missing, exp)
		}
	}
	if len(missing) > 0 {
		keys := []string{}
		for k := range writer.Files {
			keys = append(keys, k)
		}
		t.Errorf("Issue 11/42: Expected child usage text to contain %v (normal command), but they were missing.\nContent:\n%s\nFiles:\n%v", missing, renderedChildUsage, keys)
	}
}

func TestIssue67_DeepFlag(t *testing.T) {
	src := `package main

// Root is a subcommand ` + "`app`" + `
func Root() {}

// Child is a subcommand ` + "`app child`" + `
func Child() {}

// GrandChild is a subcommand ` + "`app child grandchild`" + `
func GrandChild() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	// 1. Verify RootCmd supports UsageRecursive logic (or equivalent in Execute)
	// We check if the generated root.go contains logic to handle "-deep"
	rootPath := "cmd/app/root.go"
	rootContent, ok := writer.Files[rootPath]
	if !ok {
		t.Fatalf("Root file not found: %s", rootPath)
	}
	rootCode := string(rootContent)

	if !strings.Contains(rootCode, "-deep") {
		t.Errorf("RootCmd does not check for '-deep' flag")
	}

	// 2. Verify SubCommand supports UsageRecursive
	childPath := "cmd/app/child.go"
	childContent, ok := writer.Files[childPath]
	if !ok {
		t.Fatalf("Child file not found: %s", childPath)
	}
	childCode := string(childContent)

	if !strings.Contains(childCode, "-deep") {
		t.Errorf("SubCommand does not check for '-deep' flag")
	}
	if !strings.Contains(childCode, "UsageRecursive()") {
		t.Errorf("SubCommand does not have UsageRecursive() method")
	}

	// 3. Verify existence of combined usage template with toggle
	usagePath := "cmd/app/templates/child_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Errorf("Usage file not found: %s", usagePath)
	} else {
		// Check content includes toggle and grandchild
		usageText := string(content)
		if !strings.Contains(usageText, "{{if .Recursive}}") {
			t.Errorf("Usage text does not contain Recursive toggle:\n%s", usageText)
		}
		if !strings.Contains(usageText, "child grandchild") {
			t.Errorf("Usage text does not contain recursive grandchild info:\n%s", usageText)
		}
	}
}

func TestIssue67_CollisionMitigation(t *testing.T) {
	src := `package main

// Cattail is a subcommand ` + "`app Cattail`" + `
func Cattail() {}

// CatTail is a subcommand ` + "`app CatTail`" + `
func CatTail() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	// Verify that case-insensitive collisions are mitigated by assigning distinct filenames.
	// "Cattail" and "CatTail" differ only by case.
	// Alphabetical order: CatTail (T=84) < Cattail (t=116).
	// So CatTail is processed first -> "cattail_usage.txt".
	// Cattail is processed second -> "cattail_1_usage.txt".

	catTailUsage := "cmd/app/templates/cattail_usage.txt"
	cattailUsage := "cmd/app/templates/cattail_1_usage.txt"

	_, ok1 := writer.Files[catTailUsage]
	_, ok2 := writer.Files[cattailUsage]

	if !ok1 {
		t.Errorf("Expected %s to exist", catTailUsage)
	}
	if !ok2 {
		t.Errorf("Expected %s to exist (mitigation for collision)", cattailUsage)
	}
}

func TestIssue470_NestedUsageFilesUseFullPath(t *testing.T) {
	src := `package main

// Serve is a subcommand ` + "`app serve`" + `.
// Flags:
//
//	port: --port Port to serve.
func Serve(port int) {}

// ScenarioServe is a subcommand ` + "`app scenario serve`" + `.
// Flags:
//
//	name: @1 Scenario name.
func ScenarioServe(name string) {}
`
	input := setupProject(t, src)
	first := runGenerateInMemory(t, input)
	second := runGenerateInMemory(t, input)

	for path, content := range first.Files {
		if got, ok := second.Files[path]; !ok || string(got) != string(content) {
			t.Fatalf("generation is not deterministic for %s", path)
		}
	}
	if len(first.Files) != len(second.Files) {
		t.Fatalf("generated file count changed: %d then %d", len(first.Files), len(second.Files))
	}

	directUsage := string(mustGeneratedFile(t, first, "cmd/app/templates/serve_usage.txt"))
	nestedUsage := string(mustGeneratedFile(t, first, "cmd/app/templates/scenario_serve_usage.txt"))
	if !strings.Contains(directUsage, "Usage: app serve [flags...]") {
		t.Fatalf("direct usage has wrong command path or flags:\n%s", directUsage)
	}
	if !strings.Contains(nestedUsage, "Usage: app scenario serve <name>") {
		t.Fatalf("nested usage has wrong command path or positional argument:\n%s", nestedUsage)
	}

	directCode := string(mustGeneratedFile(t, first, "cmd/app/serve.go"))
	nestedCode := string(mustGeneratedFile(t, first, "cmd/app/scenario_serve.go"))
	assertContains(t, directCode, `"serve_usage.txt"`, "direct command should select its own usage resource")
	assertContains(t, nestedCode, `"scenario_serve_usage.txt"`, "nested command should select its own usage resource")
}

func TestErrorHandlingGeneration(t *testing.T) {
	src := `package main

import "errors"

// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd() error { return nil }
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	cmdPath := "cmd/app/mycmd.go"
	content, ok := writer.Files[cmdPath]
	if !ok {
		t.Fatalf("Generated file not found: %s", cmdPath)
	}

	code := string(content)
	if !strings.Contains(code, "errors.Is(err, cmd.ErrPrintHelp)") {
		t.Errorf("Generated code should handle ErrPrintHelp")
	}
	if !strings.Contains(code, "errors.Is(err, cmd.ErrHelp)") {
		t.Errorf("Generated code should handle ErrHelp")
	}
	if !strings.Contains(code, "\"example.com/test/cmd\"") {
		t.Errorf("Generated code should import the cmd package")
	}

	// Verify errors.go was generated
	errorsPath := "cmd/errors.go"
	errorsContent, ok := writer.Files[errorsPath]
	if !ok {
		t.Fatalf("Generated file not found: %s", errorsPath)
	}
	if !strings.Contains(string(errorsContent), "var ErrPrintHelp = errors.New(\"print help\")") {
		t.Errorf("errors.go should define ErrPrintHelp")
	}
}

func TestIssue221_ImportFormatting(t *testing.T) {
	src := `package main

// Child is a subcommand ` + "`app nested child`" + `
func Child() {}
`
	fs := fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.25\n")},
		"main.go": &fstest.MapFile{Data: []byte(src)},
	}

	writer := NewCollectingFileWriter()
	// Generate code
	if err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{}, false, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check cmd/app/nested.go (the synthetic command)
	nestedPath := "cmd/app/nested.go"
	content, ok := writer.Files[nestedPath]
	if !ok {
		t.Fatalf("File not found: %s", nestedPath)
	}
	nestedCode := string(content)

	t.Logf("Generated nested.go:\n%s", nestedCode)

	// Issue 221: "If there is no import needed for a subcommand then the import is not formatted right"
	// The diff shows removal of blank line.
	// But generated code also contains invalid empty import `""` for synthetic commands.

	if strings.Contains(nestedCode, "\t\"\"\n") {
		t.Errorf("Issue 221: Generated code contains empty import `\"\"`")
	}

	// Also check for trailing blank line in import block
	// We expect import block to end with `strings"\n)` or `strings")` (if formatted by gofmt)
	// But definitely NOT `\n\n)`
	if strings.Contains(nestedCode, "\n\n)") {
		t.Errorf("Issue 221: Found empty line before closing parenthesis in import block")
	}
}

// TestGenerate_OverwriteProtection verifies Issue #203: "Silent Overwrite of Source Files".
// It ensures that files not generated by gosubc are not overwritten without --force,
// and that extraneous files in the target directory are detected.
func TestGenerate_OverwriteProtection(t *testing.T) {
	src := `package main
// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd() {}
`
	fs := setupProject(t, src)

	// 1. Initial generation
	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("Initial generation failed: %v", err)
	}

	cmdFile := "cmd/app/mycmd.go"
	if _, ok := writer.Files[cmdFile]; !ok {
		keys := make([]string, 0, len(writer.Files))
		for k := range writer.Files {
			keys = append(keys, k)
		}
		t.Fatalf("File %s not generated. Generated files: %v", cmdFile, keys)
	}

	// 2. Modify file (simulating manual edit removing header)
	writer.Files[cmdFile] = []byte("package main\n// Manual edit\nfunc MyCmd() {}")

	// 3. Generate without force -> Should fail
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{}, false, false, nil, false, false, "", "", "")
	if err == nil {
		t.Errorf("Expected failure when overwriting non-generated file without force")
	} else if !strings.Contains(err.Error(), "exists and was not generated by gosubc") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// 4. Generate with force -> Should succeed
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{}, true, false, nil, false, false, "", "", "")
	if err != nil {
		t.Errorf("Expected success with force: %v", err)
	}
	// Verify content reverted to generated
	if newContent, ok := writer.Files[cmdFile]; !ok {
		t.Fatalf("File %s missing after regeneration", cmdFile)
	} else if !strings.Contains(string(newContent), "Code generated by") {
		t.Errorf("Expected file to be overwritten with generated content")
	}

	// 5. Add extraneous file
	extraFile := "cmd/app/extra.txt"
	writer.Files[extraFile] = []byte("I shouldn't be here")

	// 6. Generate without force -> Should fail
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{}, false, false, nil, false, false, "", "", "")
	if err == nil {
		t.Errorf("Expected failure with extraneous file without force")
	} else if !strings.Contains(err.Error(), "present in the directory but not in the generated set") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// 7. Generate with force -> Should succeed
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{}, true, false, nil, false, false, "", "", "")
	if err != nil {
		t.Errorf("Expected success with force: %v", err)
	}
}

func TestIssue251_SubcommandNameConflict(t *testing.T) {
	src := `package main

// Parent1 is a subcommand ` + "`app parent1`" + `
func Parent1() {}

// Parent1Child is a subcommand ` + "`app parent1 child`" + `
func Parent1Child() {}

// Parent2 is a subcommand ` + "`app parent2`" + `
func Parent2() {}

// Parent2Child is a subcommand ` + "`app parent2 child`" + `
func Parent2Child() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	// Verify unique files are created
	p1cPath := "cmd/app/parent1_child.go"
	p2cPath := "cmd/app/parent2_child.go"

	if _, ok := writer.Files[p1cPath]; !ok {
		t.Errorf("Expected %s to exist", p1cPath)
	} else {
		content := string(writer.Files[p1cPath])
		if !strings.Contains(content, "type Parent1Child struct") {
			t.Errorf("%s should contain Parent1Child struct", p1cPath)
		}
	}

	if _, ok := writer.Files[p2cPath]; !ok {
		t.Errorf("Expected %s to exist", p2cPath)
	} else {
		content := string(writer.Files[p2cPath])
		if !strings.Contains(content, "type Parent2Child struct") {
			t.Errorf("%s should contain Parent2Child struct", p2cPath)
		}
	}
}

func TestIssue481_RuntimeUsageAlignmentAndEscaping(t *testing.T) {
	input := setupProject(t, issue481Source)
	first := runGenerateInMemory(t, input)
	second := runGenerateInMemory(t, input)

	// 6. Verify that generation is deterministic across repeated runs.
	if len(first.Files) != len(second.Files) {
		t.Fatalf("generated file count changed across runs: %d vs %d", len(first.Files), len(second.Files))
	}
	for path, content := range first.Files {
		got, ok := second.Files[path]
		if !ok || !bytes.Equal(got, content) {
			t.Fatalf("generation is not deterministic for %s", path)
		}
	}

	// Verify templates.go exposes runtime row helpers without go-subcommand runtime dependency or subcommand collections.
	templatesGo := string(mustGeneratedFile(t, first, "cmd/app/templates/templates.go"))
	assertContains(t, templatesGo, `"formatHelpRow":`, "templates.go should register formatHelpRow helper")
	assertContains(t, templatesGo, `"formatFlagRow":`, "templates.go should register formatFlagRow helper")
	assertContains(t, templatesGo, `"wrapFlag":`, "templates.go should retain wrapFlag compatibility helper")
	assertNotContains(t, templatesGo, "usageSubcommand", "templates.go must not define usageSubcommand collection types")
	assertNotContains(t, templatesGo, "newUsageSubcommand", "templates.go must not define newUsageSubcommand")
	assertNotContains(t, templatesGo, `"github.com/arran4/go-subcommand/`, "generated templates.go must not import go-subcommand")

	// 1. Verify generated *_usage.txt files are text-first templates with inline formatHelpRow and formatFlagRow calls.
	appUsage := string(mustGeneratedFile(t, first, "cmd/app/templates/app_usage.txt"))
	serviceUsage := string(mustGeneratedFile(t, first, "cmd/app/templates/service_usage.txt"))
	serveUsage := string(mustGeneratedFile(t, first, "cmd/app/templates/serve_usage.txt"))
	for name, tmplText := range map[string]string{
		"app_usage.txt":     appUsage,
		"service_usage.txt": serviceUsage,
		"serve_usage.txt":   serveUsage,
	} {
		assertContains(t, tmplText, "{{formatHelpRow ", name+" should format command rows via formatHelpRow")
		assertNotContains(t, tmplText, "subcommands", name+" must not build runtime subcommand collections")
		assertNotContains(t, tmplText, "subcommandWidth", name+" must not use subcommandWidth")
		assertNotContains(t, tmplText, "%-40s", name+" must not contain hardcoded %-40s")
		assertNotContains(t, tmplText, "%-10s", name+" must not contain hardcoded %-10s")
	}
	assertContains(t, serveUsage, `{{formatFlagRow 19 18 "--port int" "(default: 8080)"`, "serve_usage.txt should format flags via formatFlagRow")
	assertContains(t, serveUsage, `{{formatFlagRow 19 18 "--endpoint string" ""`, "serve_usage.txt should format default-less flags via formatFlagRow")
	assertContains(t, serveUsage, `{{formatFlagRow 19 18 "--quiet" "(default: false)" ""}}`, "serve_usage.txt should format description-less flags via formatFlagRow")

	// 2, 3, 4. Execute generated templates in shallow and recursive modes for long/short names,
	// nested commands, and commands without descriptions.
	serviceShallow := executeGeneratedUsageTemplate(t, serviceUsage, false)
	wantServiceShallow := strings.Join([]string{
		"Usage: app service <subcommand>",
		"",
		"Manage services.",
		"",
		"Subcommands:",
		"",
		"    silent-group",
		"    status       Show service status and health details across all configured environments.",
		"    help         Print this help message",
		"    usage        Print this usage message",
		"",
		"",
	}, "\n")
	if serviceShallow != wantServiceShallow {
		t.Errorf("service shallow usage mismatch:\nwant:\n%q\ngot:\n%q", wantServiceShallow, serviceShallow)
	}

	serviceRecursive := executeGeneratedUsageTemplate(t, serviceUsage, true)
	wantServiceRecursive := strings.Join([]string{
		"Usage: app service <subcommand>",
		"",
		"Manage services.",
		"",
		"Subcommands:",
		"",
		"    service silent-group",
		"    service silent-group deep-leaf-command Deep nested leaf action.",
		"    service status                         Show service status and health details across all configured environments.",
		"    help                                   Print this help message",
		"    usage                                  Print this usage message",
		"",
		"",
	}, "\n")
	if serviceRecursive != wantServiceRecursive {
		t.Errorf("service recursive usage mismatch:\nwant:\n%q\ngot:\n%q", wantServiceRecursive, serviceRecursive)
	}

	// Verify narrow terminal wrapping for both command help and flag help.
	serviceShallowNarrow := executeGeneratedUsageTemplateAtWidth(t, serviceUsage, false, 44)
	assertContains(t, serviceShallowNarrow, strings.Join([]string{
		"    status       Show service status and",
		"                 health details across all",
		"                 configured environments.",
	}, "\n"), "service shallow narrow output should wrap with hanging indent")

	serveNarrow := executeGeneratedUsageTemplateAtWidth(t, serveUsage, false, 64)
	assertContains(t, serveNarrow, strings.Join([]string{
		"    --port int          (default: 8080)    Port to listen on",
		"                                           when starting the",
		"                                           HTTP listener and",
		"                                           accepting client",
		"                                           connections",
	}, "\n"), "serve flag help should wrap with hanging indent at narrow width")
	assertContains(t, serveNarrow, "https://service.example.com/very/long/endpoint/path/without/spaces", "serve flag help should preserve long word without truncation")
	assertContains(t, serveNarrow, "\n    --quiet             (default: false)\n", "serve flag without description should have no trailing spaces")

	for _, line := range strings.Split(serviceRecursive, "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("service recursive line has trailing space: %q", line)
		}
	}
	for _, line := range strings.Split(serveNarrow, "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("serve narrow line has trailing space: %q", line)
		}
	}

	// 5 & 7. Verify PR #493 unique usage-resource naming and byte-identical output when shallow and recursive entries match.
	scenarioServeUsage := string(mustGeneratedFile(t, first, "cmd/app/templates/scenario_serve_usage.txt"))
	serveShallow := executeGeneratedUsageTemplate(t, serveUsage, false)
	serveRecursive := executeGeneratedUsageTemplate(t, serveUsage, true)
	if serveShallow != serveRecursive {
		t.Errorf("serve shallow and recursive outputs should be byte-identical:\nshallow:\n%q\nrecursive:\n%q", serveShallow, serveRecursive)
	}
	assertContains(t, serveShallow, "Usage: app serve [flags...]", "serve usage should retain its direct command path")
	assertContains(t, serveShallow, "    help  Print this help message\n    usage Print this usage message", "serve usage should align built-in help rows")

	scenarioShallow := executeGeneratedUsageTemplate(t, scenarioServeUsage, false)
	scenarioRecursive := executeGeneratedUsageTemplate(t, scenarioServeUsage, true)
	if scenarioShallow != scenarioRecursive {
		t.Errorf("scenario serve shallow and recursive outputs should be byte-identical:\nshallow:\n%q\nrecursive:\n%q", scenarioShallow, scenarioRecursive)
	}
	assertContains(t, scenarioShallow, "Usage: app scenario serve <name>", "scenario serve usage should retain its nested command path")
	assertContains(t, scenarioShallow, "Scenario {{.Injected}} name", "scenario serve positional description should render literal braces")

	// 8. Ensure literal descriptions containing {{ or }} cannot inject template actions.
	renderUsage := string(mustGeneratedFile(t, first, "cmd/app/templates/render_usage.txt"))
	for _, recursive := range []bool{false, true} {
		renderedApp := executeGeneratedUsageTemplate(t, appUsage, recursive)
		renderedRender := executeGeneratedUsageTemplate(t, renderUsage, recursive)
		for _, out := range []string{renderedApp, renderedRender} {
			if strings.Contains(out, "INJECTED_VALUE") || strings.Contains(out, "EXPLOITED_VALUE") {
				t.Fatalf("literal description injected template action (recursive=%v):\n%s", recursive, out)
			}
		}
		assertContains(t, renderedApp, "Short root description with {{.Injected}} literal braces.", "root description should preserve literal braces")
		assertContains(t, renderedApp, `Extended help with {{printf "%s" .Exploit}} and unmatched }} or {{.`, "root extended help should preserve literal braces")
		assertContains(t, renderedApp, "Render {{.Injected}} template output }} safely {{.", "subcommand row description should preserve literal braces")
		assertContains(t, renderedRender, "Output format like {{.Name}}", "flag description should preserve literal braces")
	}
}
