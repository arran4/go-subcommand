package go_subcommand

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/parsers"
	"golang.org/x/tools/txtar"
)

func TestGeneratedGoFlagRuntime(t *testing.T) {
	data, err := os.ReadFile("testdata/goflag_runtime_test.txtar")
	if err != nil {
		t.Fatal(err)
	}
	archive := txtar.Parse(data)

	inFS := make(fstest.MapFS)
	for _, f := range archive.Files {
		if strings.HasPrefix(f.Name, "in/") {
			inFS[strings.TrimPrefix(f.Name, "in/")] = &fstest.MapFile{Data: f.Data}
		}
	}

	writer := NewCollectingFileWriter()

	err = GenerateWithFS(inFS, writer, ".", "", "commentv1", "gnu", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	for name, file := range inFS {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, name)), 0777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, name), file.Data, 0666); err != nil {
			t.Fatal(err)
		}
	}
	for name, file := range writer.Files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, name)), 0777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, name), file, 0666); err != nil {
			t.Fatal(err)
		}
	}

	// Create required dummy files
	dummyFile := `package main
func AppMain(config string, one bool, out string) {}
func AppChild(dir string) {}
func AppGrandChild(dir string, enable bool) {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "cmd", "app", "dummy.go"), []byte(dummyFile), 0666); err != nil {
		t.Fatal(err)
	}

	modReplace := exec.Command("go", "mod", "edit", "-replace=github.com/arran4/go-subcommand=../../..")
	modReplace.Dir = tmpDir
	if out, err := modReplace.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit: %v\n%s", err, out)
	}

	modTidy := exec.Command("go", "mod", "tidy")
	modTidy.Dir = tmpDir
	if out, err := modTidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmpDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test failed: %v\n%s", err, out.String())
	}
}
