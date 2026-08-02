package go_subcommand

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Run("ValidParser", func(t *testing.T) {
		// Test against the project root which has a go.mod and valid Go code.
		err := Validate(".", "commentv1", []string{"."}, false)
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
		// Capture standard output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Test against the project root
		err := List(".", "commentv1", []string{"."}, false)

		_ = w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Command: gosubc") {
			t.Errorf("Expected output to contain 'Command: gosubc', got %s", output)
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
