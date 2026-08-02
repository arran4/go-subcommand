package go_subcommand

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Run("ValidParser", func(t *testing.T) {
		// Create a temporary directory with a simple valid go file and a go.mod
		dir := t.TempDir()
		goModContent := []byte("module example.com/test\n\ngo 1.22\n")
		err := os.WriteFile(filepath.Join(dir, "go.mod"), goModContent, 0644)
		if err != nil {
			t.Fatal(err)
		}

		goFileContent := []byte(`package main

// Root is a subcommand ` + "`" + `app` + "`" + `
func Root() {}
`)
		err = os.WriteFile(filepath.Join(dir, "main.go"), goFileContent, 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = Validate(dir, "commentv1", []string{"."}, false)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("InvalidParser", func(t *testing.T) {
		err := Validate(".", "invalid_parser", nil, false)
		if err == nil {
			t.Error("Expected an error for invalid parser, got nil")
		}
	})

	t.Run("InvalidDirectory", func(t *testing.T) {
		err := Validate("non_existent_directory_12345", "commentv1", []string{"."}, false)
		if err == nil {
			t.Error("Expected an error for non-existent directory, got nil")
		}
	})
}

func TestList(t *testing.T) {
	t.Run("ValidParser", func(t *testing.T) {
		dir := t.TempDir()
		goModContent := []byte("module example.com/test\n\ngo 1.22\n")
		err := os.WriteFile(filepath.Join(dir, "go.mod"), goModContent, 0644)
		if err != nil {
			t.Fatal(err)
		}

		goFileContent := []byte(`package main

// Root is a subcommand ` + "`" + `app` + "`" + `
func Root() {}

// Sub is a subcommand ` + "`" + `app sub` + "`" + `
func Sub() {}
`)
		err = os.WriteFile(filepath.Join(dir, "main.go"), goFileContent, 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = List(dir, "commentv1", []string{"."}, false)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("InvalidParser", func(t *testing.T) {
		err := List(".", "invalid_parser", nil, false)
		if err == nil {
			t.Error("Expected an error for invalid parser, got nil")
		}
	})

	t.Run("InvalidDirectory", func(t *testing.T) {
		err := List("non_existent_directory_12345", "commentv1", []string{"."}, false)
		if err == nil {
			t.Error("Expected an error for non-existent directory, got nil")
		}
	})
}
