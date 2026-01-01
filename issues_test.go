package go_subcommand

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// MockWriter implements FileWriter for in-memory testing
type MockWriter struct {
	Files map[string][]byte
}

func NewMockWriter() *MockWriter {
	return &MockWriter{
		Files: make(map[string][]byte),
	}
}

func (m *MockWriter) WriteFile(path string, content []byte, perm os.FileMode) error {
	m.Files[path] = content
	return nil
}

func (m *MockWriter) MkdirAll(path string, perm os.FileMode) error {
	return nil // No-op for map
}

// setupProject returns an in-memory FS
func setupProject(t *testing.T, sourceCode string) fstest.MapFS {
	return fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(sourceCode)},
	}
}

// runGenerateInMemory runs the generator using in-memory FS and Writer
func runGenerateInMemory(t *testing.T, inputFS fstest.MapFS) *MockWriter {
	writer := NewMockWriter()
	// We use a dummy dir name like "." or "/app"
	if err := GenerateWithFS(inputFS, writer, ".", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	return writer
}


func TestIssue33_HyphenatedCommands_Content(t *testing.T) {
	src := `package main

// ListHeads is a subcommand ` + "`app list-heads`" + `
func ListHeads() {}
`
	fs := setupProject(t, src)
	writer := NewMockWriter()

	err := GenerateWithFS(fs, writer, ".", "")

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
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(src)},
	}

	model, err := ParseGoFiles(fs, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles failed: %v", err)
	}

	if len(model.Commands) != 1 {
		t.Fatalf("Expected 1 root command, got %d", len(model.Commands))
	}
	root := model.Commands[0]

	var admin *SubCommand
	for _, sub := range root.SubCommands {
		if sub.SubCommandName == "admin" {
			admin = sub
			break
		}
	}
	if admin == nil {
		t.Fatal("Expected 'admin' subcommand, but it was flattened or missing")
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
//   retries: --retries (default: 3) Number of retries
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
//   force: -f --force Force execution
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

	expectedNote := "// Generated by github.com/arran4/go-subcommand/cmd/gosubc"
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

func TestIssue42_MissingHelpUsageInGuide(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd() {}
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
