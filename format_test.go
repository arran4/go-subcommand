package go_subcommand

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "format_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module github.com/test/mod\n\ngo 1.22"), 0644); err != nil {
		t.Fatal(err)
	}

	src := `package main

// MyCmd is a subcommand ` + "`" + `test mycmd` + "`" + `
func MyCmd(verbose bool, paths []string) {}
`
	filename := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(filename, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	err = Format(".", true, nil, true)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "// Flags:") {
		t.Errorf("Expected Flags section in formatted comment, got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "verbose") {
		t.Errorf("Expected verbose param in formatted comment, got:\n%s", contentStr)
	}
}

func TestFormat_NotInPlace(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "format_test_not_inplace")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module github.com/test/mod\n\ngo 1.22"), 0644); err != nil {
		t.Fatal(err)
	}

	src := `package main

// MyCmd is a subcommand ` + "`" + `test mycmd` + "`" + `
func MyCmd(verbose bool) {}
`
	filename := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(filename, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	err = Format(".", false, nil, true)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "// Flags:") {
		t.Errorf("Expected Flags section to NOT be added when inplace=false, got:\n%s", contentStr)
	}
}

func TestFormat_InvalidParse(t *testing.T) {
	err := Format("non_existent_dir", true, nil, true)
	if err == nil {
		t.Errorf("Expected Format to fail with non-existent directory")
	}
}
