package go_subcommand

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"golang.org/x/tools/txtar"
)

func TestCollectExportedTemplateFiles(t *testing.T) {
	files, err := collectExportedTemplateFiles(fstest.MapFS{
		"templates/cmd/main.go.gotmpl":   &fstest.MapFile{Data: []byte("main")},
		"templates/common/common.gotmpl": &fstest.MapFile{Data: []byte("common")},
	})
	if err != nil {
		t.Fatalf("collect exported template files: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("collected %d template files, want 2", len(files))
	}
	if files[0].name != "cmd/main.go.gotmpl" || string(files[0].data) != "main" {
		t.Fatalf("first collected file = %#v, want cmd/main.go.gotmpl", files[0])
	}
	if files[1].name != "common/common.gotmpl" || string(files[1].data) != "common" {
		t.Fatalf("second collected file = %#v, want common/common.gotmpl", files[1])
	}
}

func TestExportDirPreflightsExistingFilesAndPreservesUnrelatedContent(t *testing.T) {
	output := filepath.Join(t.TempDir(), "templates")
	if err := os.MkdirAll(filepath.Join(output, "nested"), 0755); err != nil {
		t.Fatalf("create partial destination: %v", err)
	}
	writeTemplateExportTestFile(t, filepath.Join(output, "nested", "second.gotmpl"), "user second")
	writeTemplateExportTestFile(t, filepath.Join(output, "notes.txt"), "keep me")
	files := []exportedTemplateFile{
		{name: "first.gotmpl", data: []byte("first")},
		{name: "nested/second.gotmpl", data: []byte("second")},
	}

	if err := exportDir(output, files, false); err == nil {
		t.Fatal("exportDir unexpectedly overwrote an existing template")
	}
	if _, err := os.Stat(filepath.Join(output, "first.gotmpl")); !os.IsNotExist(err) {
		t.Fatalf("preflight wrote first.gotmpl before reporting the existing file: %v", err)
	}
	if got := readTemplateExportTestFile(t, filepath.Join(output, "nested", "second.gotmpl")); got != "user second" {
		t.Fatalf("existing template changed after rejected export: %q", got)
	}
	if got := readTemplateExportTestFile(t, filepath.Join(output, "notes.txt")); got != "keep me" {
		t.Fatalf("unrelated file changed after rejected export: %q", got)
	}

	if err := exportDir(output, files, true); err != nil {
		t.Fatalf("forced exportDir: %v", err)
	}
	if got := readTemplateExportTestFile(t, filepath.Join(output, "first.gotmpl")); got != "first" {
		t.Fatalf("first.gotmpl = %q, want first", got)
	}
	if got := readTemplateExportTestFile(t, filepath.Join(output, "nested", "second.gotmpl")); got != "second" {
		t.Fatalf("second.gotmpl = %q, want second", got)
	}
	if got := readTemplateExportTestFile(t, filepath.Join(output, "notes.txt")); got != "keep me" {
		t.Fatalf("forced export changed unrelated file: %q", got)
	}
}

func TestExportTxtarRefusesExistingFileUnlessForced(t *testing.T) {
	output := filepath.Join(t.TempDir(), "templates.txtar")
	writeTemplateExportTestFile(t, output, "user archive")
	files := []exportedTemplateFile{{name: "cmd/main.go.gotmpl", data: []byte("main")}}

	if err := exportTxtar(output, files, false); err == nil {
		t.Fatal("exportTxtar unexpectedly overwrote an existing archive")
	}
	if got := readTemplateExportTestFile(t, output); got != "user archive" {
		t.Fatalf("existing archive changed after rejected export: %q", got)
	}

	if err := exportTxtar(output, files, true); err != nil {
		t.Fatalf("forced exportTxtar: %v", err)
	}
	archive := txtar.Parse([]byte(readTemplateExportTestFile(t, output)))
	if len(archive.Files) != 1 || archive.Files[0].Name != "cmd/main.go.gotmpl" || string(archive.Files[0].Data) != "main\n" {
		t.Fatalf("forced archive = %#v, want one exported template", archive.Files)
	}
}

func TestTemplateExportRefusesSymlinkTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs additional Windows privileges")
	}

	base := t.TempDir()
	outside := filepath.Join(base, "outside.gotmpl")
	writeTemplateExportTestFile(t, outside, "outside")
	output := filepath.Join(base, "templates")
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(output, "main.gotmpl")); err != nil {
		t.Fatalf("create target symlink: %v", err)
	}

	err := exportDir(output, []exportedTemplateFile{{name: "main.gotmpl", data: []byte("replacement")}}, true)
	if err == nil {
		t.Fatal("exportDir unexpectedly followed a template symlink")
	}
	if got := readTemplateExportTestFile(t, outside); got != "outside" {
		t.Fatalf("symlink export changed outside file: %q", got)
	}
}

func writeTemplateExportTestFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readTemplateExportTestFile(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}
