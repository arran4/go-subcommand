package go_subcommand

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
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
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, nil)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, ok := writer.Files["cmd/app/sub.go"]; !ok {
		t.Errorf("Expected sub.go to be generated with recursive=true")
	}

	// Test recursive=false
	writer = NewCollectingFileWriter()
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: false}, false, nil)
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
	}, false, nil)
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

	if err := Generate(dir, "", "commentv1", nil, true, true, nil); err != nil {
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
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, []string{"usage=custom_usage.gotmpl"}, fs)
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
