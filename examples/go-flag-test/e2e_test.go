package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"fmt"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "testscript")
	if err != nil {
		panic(err)
	}

	binPath := filepath.Join(tmpDir, "go-flag-test")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/app")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n%s\n", err, out)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	code := m.Run()
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}
