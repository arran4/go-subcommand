package go_subcommand

import (
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers/commentv1"
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

func (m *MockWriter) ReadFile(path string) ([]byte, error) {
	if content, ok := m.Files[path]; ok {
		return content, nil
	}
	return nil, os.ErrNotExist
}

func (m *MockWriter) ReadDir(path string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	seen := make(map[string]bool)
	// Normalize path to have trailing slash if not empty
	prefix := path
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for k := range m.Files {
		if strings.HasPrefix(k, prefix) {
			rel := strings.TrimPrefix(k, prefix)
			parts := strings.Split(rel, "/")
			if len(parts) > 0 && parts[0] != "" {
				name := parts[0]
				if seen[name] {
					continue
				}
				seen[name] = true
				isDir := len(parts) > 1
				entries = append(entries, &mockDirEntry{name: name, isDir: isDir})
			}
		}
	}
	return entries, nil
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
	if err := GenerateWithFS(inputFS, writer, ".", "", "commentv1", nil, false); err != nil {
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

	err := GenerateWithFS(fs, writer, ".", "", "commentv1", nil, false)

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

	expectedNote := "// Code generated by github.com/arran4/go-subcommand/cmd/gosubc. DO NOT EDIT."
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

	// Issue 11 & 42: Help and usage should work (be visible)
	// UPDATE per user request: Functionless parent subcommands (like 'nested' here) are equiv to usage/help
	// and thus don't need to output them as subcommands.
	present := []string{}
	unexpected := []string{"help", "usage"}
	for _, unexp := range unexpected {
		if strings.Contains(usageText, unexp+" ") {
			present = append(present, unexp)
		}
	}
	if len(present) > 0 {
		t.Errorf("Issue 11/42: Expected nested usage text to NOT contain %v (functionless parent), but they were present.\nContent:\n%s", present, usageText)
	}

	// Issue 41/52: Version should NOT be at nested level
	if strings.Contains(usageText, "version      Print version information") {
		t.Errorf("Issue 41/52: Nested usage text contains 'version' command which should only be at top level.\nContent:\n%s", usageText)
	}

	// Check child command usage (explicitly functional)
	childUsagePath := "cmd/app/templates/child_usage.txt"
	childContent, ok := writer.Files[childUsagePath]
	if !ok {
		t.Fatalf("Child usage file not found: %s", childUsagePath)
	}
	childUsageText := string(childContent)

	// Issue 11 & 42: Help and usage SHOULD be present for normal commands
	missing := []string{}
	expected := []string{"help", "usage"}
	for _, exp := range expected {
		if !strings.Contains(childUsageText, exp+" ") {
			missing = append(missing, exp)
		}
	}
	if len(missing) > 0 {
		keys := []string{}
		for k := range writer.Files {
			keys = append(keys, k)
		}
		t.Errorf("Issue 11/42: Expected child usage text to contain %v (normal command), but they were missing.\nContent:\n%s\nFiles:\n%v", missing, childUsageText, keys)
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

	writer := NewMockWriter()
	// Generate code
	if err := GenerateWithFS(fs, writer, ".", "", "commentv1", nil, false); err != nil {
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

	if strings.Contains(nestedCode, `""`) {
		t.Errorf("Issue 221: Generated code contains empty import `\"\"`")
	}

	// Also check for trailing blank line in import block
	// We expect import block to end with `strings"\n)` or `strings")` (if formatted by gofmt)
	// But definitely NOT `\n\n)`
	if strings.Contains(nestedCode, "\n\n)") {
		t.Errorf("Issue 221: Found empty line before closing parenthesis in import block")
	}
}

func TestGenerate_OverwriteProtection(t *testing.T) {
	src := `package main
// Cmd is a subcommand ` + "`app cmd`" + `
func Cmd() {}
`
	fs := setupProject(t, src)

	// 1. Initial generation
	writer := NewMockWriter()
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", nil, false)
	if err != nil {
		t.Fatalf("Initial generation failed: %v", err)
	}

	cmdFile := "cmd/app/cmd2.go"
	if _, ok := writer.Files[cmdFile]; !ok {
		t.Fatalf("File %s not generated", cmdFile)
	}

	// 2. Modify file (simulating manual edit removing header)
	writer.Files[cmdFile] = []byte("package main\n// Manual edit\nfunc Cmd() {}")

	// 3. Generate without force -> Should fail
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", nil, false)
	if err == nil {
		t.Errorf("Expected failure when overwriting non-generated file without force")
	} else if !strings.Contains(err.Error(), "exists and was not generated by gosubc") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// 4. Generate with force -> Should succeed
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", nil, true)
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
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", nil, false)
	if err == nil {
		t.Errorf("Expected failure with extraneous file without force")
	} else if !strings.Contains(err.Error(), "present in the directory but not in the generated set") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// 7. Generate with force -> Should succeed
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", nil, true)
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
