package go_subcommand

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/parsers"
)

//go:embed testdata/issue_runtime.go
var issueRuntimeSource string

//go:embed testdata/issue_runtime_parser.go
var issueRuntimeParserSource string

//go:embed testdata/issue_runtime_test.go
var issueRuntimeTestSource string

//go:embed testdata/alias_io.go
var aliasIOSource string

//go:embed testdata/alias_root_io.go
var aliasRootIOSource string

func TestGenerate_Recursive(t *testing.T) {
	fs := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`package main
// Root is a subcommand ` + "`app`" + `
func Root() {}
`)},
		"sub/sub.go": &fstest.MapFile{Data: []byte(`package sub
// Sub is a subcommand ` + "`app sub`" + `
func Sub() {}
`)},
	}

	// Test recursive=true (default)
	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, ok := writer.Files["cmd/app/sub.go"]; !ok {
		t.Errorf("Expected sub.go to be generated with recursive=true")
	}

	// Test recursive=false
	writer = NewCollectingFileWriter()
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: false}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, ok := writer.Files["cmd/app/sub.go"]; ok {
		t.Errorf("Expected sub.go NOT to be generated with recursive=false")
	}
}

func TestGenerate_Paths(t *testing.T) {
	fs := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`package main
// Root is a subcommand ` + "`app`" + `
func Root() {}
`)},
		"pkg1/cmd.go": &fstest.MapFile{Data: []byte(`package pkg1
// Cmd1 is a subcommand ` + "`app cmd1`" + `
func Cmd1() {}
`)},
		"pkg2/cmd.go": &fstest.MapFile{Data: []byte(`package pkg2
// Cmd2 is a subcommand ` + "`app cmd2`" + `
func Cmd2() {}
`)},
	}

	// Test with specific path
	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{
		SearchPaths: []string{"pkg1"},
		Recursive:   true,
	}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, ok := writer.Files["cmd/app/cmd1.go"]; !ok {
		t.Errorf("Expected cmd1.go to be generated")
	}
	if _, ok := writer.Files["cmd/app/cmd2.go"]; ok {
		t.Errorf("Expected cmd2.go NOT to be generated")
	}
}

func TestGenerate_RuntimeRequirements(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/e2e\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issueRuntimeSource)
	writeRuntimeFixture(t, filepath.Join(dir, "parserpkg", "parser.go"), issueRuntimeParserSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	writeRuntimeFixture(t, filepath.Join(dir, "cmd", "app", "runtime_test.go"), issueRuntimeTestSource)

	cmd := exec.Command("go", "test", "./...")

	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		generatedTest, readErr := os.ReadFile(filepath.Join(dir, "cmd", "app", "runtime_test.go"))
		if readErr != nil {
			t.Fatalf("generated module tests failed: %v\n%s", err, output)
		}
		t.Fatalf("generated module tests failed: %v\n%s\nGenerated test:\n%s", err, output, generatedTest)
	}
}

func TestGenerate_AliasedIOCompiles(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/aliasio\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app", "app.go"), aliasIOSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	generatedPath := filepath.Join(dir, "cmd", "aliasio", "run.go")
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated command: %v", err)
	}
	generatedSource := string(generated)
	if got := strings.Count(generatedSource, `"io"`); got != 1 {
		t.Fatalf("io import count = %d, want 1\n%s", got, generatedSource)
	}
	for _, typeName := range []string{"io.Reader", "io.Writer", "io.ReadCloser", "io.WriteCloser"} {
		if !strings.Contains(generatedSource, typeName) {
			t.Errorf("generated source missing %s\n%s", typeName, generatedSource)
		}
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated aliased module did not compile: %v\n%s", err, output)
	}
}

func TestGenerate_AliasedRootIOCompiles(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/aliasroot\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app", "app.go"), aliasRootIOSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	generatedPath := filepath.Join(dir, "cmd", "aliasroot", "root.go")
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated root command: %v", err)
	}
	generatedSource := string(generated)
	if got := strings.Count(generatedSource, `"io"`); got != 1 {
		t.Fatalf("root io import count = %d, want 1\n%s", got, generatedSource)
	}
	if strings.Contains(generatedSource, `stream "io"`) {
		t.Fatalf("root command retained conflicting io alias\n%s", generatedSource)
	}
	for _, typeName := range []string{"io.Reader", "io.Writer"} {
		if !strings.Contains(generatedSource, typeName) {
			t.Errorf("generated root source missing %s\n%s", typeName, generatedSource)
		}
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated aliased root module did not compile: %v\n%s", err, output)
	}
}

func writeRuntimeFixture(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatalf("create %q parent: %v", name, err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", name, err)
	}
}

func TestGenerate_ReplaceTemplates(t *testing.T) {
	fs := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`package main
// Root is a subcommand ` + "`app`" + ` -- Custom App
func Root() {}
`)},
		"custom_usage.gotmpl": &fstest.MapFile{Data: []byte("OVERRIDDEN USAGE FOR {{.FullUsageString}}")},
	}

	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, []string{"usage=custom_usage.gotmpl"}, false, false, "", "", "", fs)
	if err != nil {
		t.Fatalf("GenerateWithFS with replaceTemplates failed: %v", err)
	}

	usageContent, ok := writer.Files["cmd/app/templates/app_usage.txt"]
	if !ok {
		t.Fatalf("Expected cmd/app/templates/app_usage.txt to be generated")
	}

	if string(usageContent) != "OVERRIDDEN USAGE FOR app" {
		t.Errorf("Expected 'OVERRIDDEN USAGE FOR app', got %q", string(usageContent))
	}
}

func TestCollectingFileWriter_ReadDir(t *testing.T) {
	writer := NewCollectingFileWriter()

	_ = writer.WriteFile(filepath.Join("dir", "file1.txt"), []byte("content1"), 0o644)
	_ = writer.WriteFile(filepath.Join("dir", "file2.txt"), []byte("content2"), 0o644)
	_ = writer.WriteFile(filepath.Join("dir", "subdir", "file3.txt"), []byte("content3"), 0o644)
	_ = writer.MkdirAll(filepath.Join("dir", "emptydir"), 0o755)
	// In collecting file writer, directory is just stored in Dirs, but let's check how it handles.
	// ReadDir for CollectingFileWriter uses w.Files to find entries.
	// Let's also check a file in root
	_ = writer.WriteFile("rootfile.txt", []byte("root"), 0o644)

	entries, err := writer.ReadDir("dir")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("Expected 4 entries, got %d", len(entries))
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	expectedNames := []string{"file1.txt", "file2.txt", "subdir", "emptydir"}
	for _, expectedName := range expectedNames {
		found := false
		for _, name := range names {
			if name == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected entry %q not found in %v", expectedName, names)
		}
	}

	// Test error path?
	// CollectingFileWriter ReadDir always returns no error.
}

func TestOSFileWriter_ReadDir(t *testing.T) {
	// Create an in-memory file system using fstest.MapFS
	mockFS := fstest.MapFS{
		"testdir/file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"testdir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
		"testdir/subdir":    &fstest.MapFile{Mode: 0o755 | os.ModeDir},
	}

	writer := &OSFileWriter{}

	// Call ReadDir with the injected mockFS
	entries, err := writer.ReadDir("testdir", mockFS)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	expectedNames := []string{"file1.txt", "file2.txt", "subdir"}
	for _, expectedName := range expectedNames {
		found := false
		for _, name := range names {
			if name == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected entry %q not found in %v", expectedName, names)
		}
	}
}

func TestGenerate_DefaultExpressions(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`package main
// Root is a subcommand ` + "`app`" + `
// Flags:
//   cores: (default: runtime.NumCPU())
//   limit: (default: math.MaxInt32)
func Root(cores int, limit int) {}
`)},
	}

	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fsys, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("GenerateWithFS failed: %v", err)
	}

	content, ok := writer.Files["cmd/app/root.go"]
	if !ok {
		t.Fatalf("root.go not generated")
	}

	s := string(content)
	if !strings.Contains(s, "\"runtime\"") {
		t.Errorf("Missing import for runtime")
	}
	if !strings.Contains(s, "\"math\"") {
		t.Errorf("Missing import for math")
	}
	if !strings.Contains(s, "runtime.NumCPU()") {
		t.Errorf("Missing assignment for cores expression: %s", s)
	}
	if !strings.Contains(s, "math.MaxInt32") {
		t.Errorf("Missing assignment for limit expression: %s", s)
	}
}

func TestGenerate_Clean(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/cleantest\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issueRuntimeSource)
	writeRuntimeFixture(t, filepath.Join(dir, "parserpkg", "parser.go"), issueRuntimeParserSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	customFile := filepath.Join(dir, "cmd", "app", "custom.go")
	writeRuntimeFixture(t, customFile, "package app\n// Custom user file\n")

	if err := Generate(dir, "", "commentv1", nil, true, true, true, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate with clean failed: %v", err)
	}

	if _, err := os.Stat(customFile); os.IsNotExist(err) {
		t.Errorf("Custom user file custom.go was deleted by --clean!")
	}

	generatedFile := filepath.Join(dir, "cmd", "app", "main.go")
	if _, err := os.Stat(generatedFile); os.IsNotExist(err) {
		t.Errorf("Generated main.go was not created after clean")
	}
}

func TestGetProvenance(t *testing.T) {
	_ = os.Setenv("SOURCE_DATE_EPOCH", "1234567890")
	defer func() { _ = os.Unsetenv("SOURCE_DATE_EPOCH") }()

	prov := GetProvenance([]string{"usage=foo.txt"}, true, true, "", "", "")
	if prov.Timestamp != "1234567890" {
		t.Errorf("Expected Timestamp 1234567890, got %s", prov.Timestamp)
	}
	if prov.TemplateID != "usage=foo.txt" {
		t.Errorf("Expected TemplateID usage=foo.txt, got %s", prov.TemplateID)
	}
}

func TestGetProvenanceExtensive(t *testing.T) {
	// Test empty / defaults
	prov := GetProvenance(nil, false, false, "", "", "")
	if prov.ProjectCommit != "" || prov.Timestamp != "" || prov.ReplacedTemplates {
		t.Errorf("Unexpected default provenance values: %+v", prov)
	}

	// Test opt-in timestamp without epoch
	_ = os.Unsetenv("SOURCE_DATE_EPOCH")
	prov = GetProvenance(nil, false, true, "", "", "")
	if prov.Timestamp == "" {
		t.Error("Expected timestamp to be generated")
	}

	// Test with SOURCE_DATE_EPOCH
	_ = os.Setenv("SOURCE_DATE_EPOCH", "987654321")
	defer func() { _ = os.Unsetenv("SOURCE_DATE_EPOCH") }()
	prov = GetProvenance(nil, false, false, "", "", "") // should override the missing timestamp flag
	if prov.Timestamp != "987654321" {
		t.Errorf("Expected 987654321, got %v", prov.Timestamp)
	}

	// Test template replacments
	prov = GetProvenance([]string{"a=b", "c=d"}, false, false, "", "", "")
	if !prov.ReplacedTemplates {
		t.Error("Expected replaced templates to be true")
	}
	if prov.TemplateID != "a=b,c=d" {
		t.Errorf("Expected template ID to be a=b,c=d, got %v", prov.TemplateID)
	}
}
