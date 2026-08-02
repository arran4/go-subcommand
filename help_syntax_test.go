package go_subcommand

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHelpSyntax(t *testing.T) {
	// Save original stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	// Create a pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Set stdout to our writer
	os.Stdout = w

	// Run the function
	err = HelpSyntax()
	if err != nil {
		t.Errorf("HelpSyntax() returned an error: %v", err)
	}

	// Close the writer so the reader knows it's done
	_ = w.Close()

	// Read the output
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("Failed to copy from pipe: %v", err)
	}

	// Restore stdout early
	os.Stdout = oldStdout

	output := buf.String()

	// Verify output
	expectedPhrases := []string{
		"Gosubc Syntax Guide",
		"Command Definition:",
		"Arguments / Flags:",
		"Positional Arguments:",
		"Variadic Arguments:",
		"Defaults:",
		"Implicit Parsing:",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("Expected output to contain %q, but it didn't", phrase)
		}
	}
}
