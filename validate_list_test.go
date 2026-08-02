package go_subcommand

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Run("ValidParser", func(t *testing.T) {
		var buf bytes.Buffer
		err := Validate(".", "commentv1", []string{"."}, false, &buf)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !strings.Contains(buf.String(), "Validation successful.") {
			t.Errorf("Expected output to contain 'Validation successful.', got %s", buf.String())
		}
	})

	t.Run("InvalidParser", func(t *testing.T) {
		var buf bytes.Buffer
		err := Validate(".", "invalid_parser", nil, false, &buf)
		if err == nil {
			t.Error("Expected an error for invalid parser, got nil")
		}
	})

	t.Run("InvalidDirectory", func(t *testing.T) {
		var buf bytes.Buffer
		err := Validate("non_existent_directory_12345", "commentv1", []string{"."}, false, &buf)
		if err == nil {
			t.Error("Expected an error for non-existent directory, got nil")
		}
	})
}

func TestList(t *testing.T) {
	t.Run("ValidParser", func(t *testing.T) {
		var buf bytes.Buffer
		err := List(".", "commentv1", []string{"."}, false, &buf)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Command: gosubc") {
			t.Errorf("Expected output to contain 'Command: gosubc', got %s", output)
		}
	})

	t.Run("InvalidParser", func(t *testing.T) {
		var buf bytes.Buffer
		err := List(".", "invalid_parser", nil, false, &buf)
		if err == nil {
			t.Error("Expected an error for invalid parser, got nil")
		}
	})

	t.Run("InvalidDirectory", func(t *testing.T) {
		var buf bytes.Buffer
		err := List("non_existent_directory_12345", "commentv1", []string{"."}, false, &buf)
		if err == nil {
			t.Error("Expected an error for non-existent directory, got nil")
		}
	})
}
