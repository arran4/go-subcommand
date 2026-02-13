package go_subcommand

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatSourceComments(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "format_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	src := `package main

// MyCmd is a subcommand ` + "`" + `test mycmd` + "`" + `
// Flags:
//   verbose: -v --verbose (default: false) Enable verbose output
//   name: -n --name (default: "world") Name to greet
func MyCmd(verbose bool, name string) {}
`
	err = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(src), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = FormatSourceComments(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tempDir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}

	got := string(content)

	// Check for empty line after Flags:
	if !strings.Contains(got, "// Flags:\n//\n") {
		t.Errorf("Expected empty line after Flags:. Got:\n%s", got)
	}

	// Check for tab indentation
	if !strings.Contains(got, "// \tverbose:") {
		t.Errorf("Expected tab indentation for 'verbose'. Got:\n%s", got)
	}

	// Check alignment
	// verbose: (8) + 1 space = 9
	// name:    (5) + 4 spaces = 9
	// So "name:    -n"
	if !strings.Contains(got, "name:    -n") {
		t.Errorf("Expected alignment for 'name'. Got:\n%s", got)
	}

	// Check quoting
	if !strings.Contains(got, "(default: \"world\")") {
		t.Errorf("Expected quoted default string. Got:\n%s", got)
	}
}
