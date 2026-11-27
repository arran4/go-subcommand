package go_subcommand

import (
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/parsers"
)

//go:embed testdata/issue490_dispatcher_main.go
var issue490DispatcherMainSource string

func TestGenerate_ImportableDispatcher(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/contextapp\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "application.go"), issue483RootContextSource)
	writeRuntimeFixture(t, filepath.Join(dir, "buildinfo", "buildinfo.go"), `package buildinfo

type BuildInfo struct { Version string }
`)

	err := GenerateWithFS(os.DirFS(dir), &OSFileWriter{}, dir, "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "", GenerateOptions{DispatcherPackage: "dispatch"})
	if err != nil {
		t.Fatalf("GenerateWithFS dispatcher mode failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "cmd", "contextapp", "main.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dispatcher mode generated main.go, stat error = %v", err)
	}
	rootSource, err := os.ReadFile(filepath.Join(dir, "cmd", "contextapp", "root.go"))
	if err != nil {
		t.Fatalf("read generated dispatcher root: %v", err)
	}
	if !strings.Contains(string(rootSource), "package dispatch") {
		t.Fatalf("dispatcher root is not importable:\n%s", rootSource)
	}
	if _, err := os.Stat(filepath.Join(dir, "cmd", "contextapp", "templates", "contextapp_usage.txt")); err != nil {
		t.Fatalf("dispatcher usage resource missing: %v", err)
	}

	writeRuntimeFixture(t, filepath.Join(dir, "launcher", "main.go"), issue490DispatcherMainSource)
	writeRuntimeFixture(t, filepath.Join(dir, "launcher", "main_test.go"), `package main

import "testing"

func TestExternalMainUsesGeneratedDispatcher(t *testing.T) {
	if err := run([]string{"help"}); err != nil {
		t.Fatalf("help dispatch: %v", err)
	}
	if err := run([]string{"child"}); err != nil {
		t.Fatalf("child dispatch: %v", err)
	}
	if err := run([]string{"child", "grandchild"}); err != nil {
		t.Fatalf("nested dispatch: %v", err)
	}
}
`)

	compile := exec.Command("go", "test", "./...")
	compile.Dir = dir
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("generated dispatcher module failed: %v\n%s", err, output)
	}
	deps := exec.Command("go", "list", "-deps", "./launcher")
	deps.Dir = dir
	if output, err := deps.CombinedOutput(); err != nil {
		t.Fatalf("list generated dispatcher dependencies: %v\n%s", err, output)
	} else if strings.Contains(string(output), "github.com/arran4/go-subcommand") {
		t.Fatalf("generated dispatcher retained a gosubc runtime dependency:\n%s", output)
	}

	second := NewCollectingFileWriter()
	if err := GenerateWithFS(os.DirFS(dir), second, dir, "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "", GenerateOptions{DispatcherPackage: "dispatch"}); err != nil {
		t.Fatalf("repeat dispatcher generation failed: %v", err)
	}
	if _, ok := second.Files[filepath.Join(dir, "cmd", "contextapp", "main.go")]; ok {
		t.Fatal("repeat dispatcher generation collected main.go")
	}
}

func TestGenerate_ImportableDispatcherRejectsUnimportablePackage(t *testing.T) {
	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.22\n")},
		"app.go": &fstest.MapFile{Data: []byte("package app\n// Root is a subcommand `app`\nfunc Root() {}\n")},
	}, writer, ".", "", "commentv1", nil, false, false, nil, false, false, "", "", "", GenerateOptions{DispatcherPackage: "main"})
	if err == nil || !strings.Contains(err.Error(), "dispatcher package") {
		t.Fatalf("invalid dispatcher package error = %v", err)
	}
}
