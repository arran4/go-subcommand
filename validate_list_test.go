package go_subcommand

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	dir := t.TempDir()

	modContent := "module example.com/test\n\ngo 1.22\n"
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(modContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	content := `package main
// Root is a subcommand ` + "`app`" + `
func Root() {}

// Sub is a subcommand ` + "`app sub`" + `
func Sub() {}
`
	err = os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = List(dir, "commentv1", []string{}, true)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("List returned unexpected error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Command: app") {
		t.Errorf("Expected output to contain 'Command: app', got: %s", output)
	}
	if !strings.Contains(output, "Subcommand: sub") {
		t.Errorf("Expected output to contain 'Subcommand: sub', got: %s", output)
	}
}

func TestValidate(t *testing.T) {
	dir := t.TempDir()

	modContent := "module example.com/test\n\ngo 1.22\n"
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(modContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	content := `package main
// Root is a subcommand ` + "`app`" + `
func Root() {}
`
	err = os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = Validate(dir, "commentv1", []string{}, true)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Validation successful.") {
		t.Errorf("Expected output to contain 'Validation successful.', got: %s", output)
	}
}

func TestList_Error(t *testing.T) {
	err := List("non_existent_dir", "invalidparser", []string{}, true)
	if err == nil {
		t.Error("Expected error for invalid parser, got nil")
	}
}

func TestValidate_Error(t *testing.T) {
	err := Validate("non_existent_dir", "invalidparser", []string{}, true)
	if err == nil {
		t.Error("Expected error for invalid parser, got nil")
	}
}
