package go_subcommand

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()

	modGo := `module example.com/test

go 1.22
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(modGo), 0644); err != nil {
		t.Fatal(err)
	}

	mainGo := `package main
// Root is a subcommand ` + "`app`" + ` -- Some root description
func Root() {}

// Sub is a subcommand ` + "`app sub`" + ` -- Some description
//
// Flags:
//
//	flag1:	--flag1	(default: "val")	Some flag
func Sub(flag1 string) {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Scan(dir, "commentv1", nil, true)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("Scan failed: %v", err)
	}

	if !strings.Contains(output, "Command: app") {
		t.Errorf("Expected output to contain 'Command: app', got: %s", output)
	}
	if !strings.Contains(output, "Subcommand: sub") {
		t.Errorf("Expected output to contain 'Subcommand: sub', got: %s", output)
	}
	if !strings.Contains(output, "--flag1") {
		t.Errorf("Expected output to contain '--flag1', got: %s", output)
	}
}

func TestScan_Error(t *testing.T) {
	err := Scan(".", "unknown_parser", nil, false)
	if err == nil {
		t.Errorf("Expected error with unknown parser, got nil")
	}
}
