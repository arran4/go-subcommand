package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "testscript")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "io_types")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/io_types")
	if err := cmd.Run(); err != nil {
		panic(err)
	}

	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	os.Exit(m.Run())
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}
