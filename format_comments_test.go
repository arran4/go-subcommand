package go_subcommand

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestFormatSourceComments(t *testing.T) {
	archive, err := txtar.ParseFile("testdata/format_regr.txtar")
	if err != nil {
		t.Fatal(err)
	}

	tempDir, err := os.MkdirTemp("", "format_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	var inputFile string
	var outputContent []byte

	for _, f := range archive.Files {
		if f.Name == "input.go" {
			inputFile = filepath.Join(tempDir, "main.go")
			if err := os.WriteFile(inputFile, f.Data, 0644); err != nil {
				t.Fatal(err)
			}
		} else if f.Name == "output.go" {
			outputContent = f.Data
		}
	}

	if inputFile == "" || outputContent == nil {
		t.Fatal("input.go or output.go not found in txtar")
	}

	if err := FormatSourceComments(tempDir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatal(err)
	}

	gotStr := strings.TrimSpace(string(got))
	wantStr := strings.TrimSpace(string(outputContent))

	if gotStr != wantStr {
		t.Errorf("FormatSourceComments() mismatch:\nGot:\n%s\nWant:\n%s", string(got), string(outputContent))
	}
}
