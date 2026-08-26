package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
	"path/filepath"
)

func TestCopyFile_IO_CleanupAndSemantics(t *testing.T) {
	tmpDir := t.TempDir()
	inPath := filepath.Join(tmpDir, "input.txt")
	outPath := filepath.Join(tmpDir, "output.txt")
	err := os.WriteFile(inPath, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("failed to create input file: %v", err)
	}

	parent := &RootCmd{
		FlagSet:  flag.NewFlagSet("root", flag.ContinueOnError),
		Commands: make(map[string]func() Cmd),
	}
	cmd := parent.NewCopyFile()

	cmd.CommandAction = func(c *CopyFile) error {
		b, err := io.ReadAll(c.input)
		if err != nil {
			return err
		}
		_, err = c.output.Write(bytes.ToUpper(b))
		if err != nil {
			return err
		}
		return nil
	}

	// 1. Test normal files
	err = cmd.Execute([]string{"--", inPath, outPath})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	outBytes, _ := os.ReadFile(outPath)
	if string(outBytes) != "HELLO" {
		t.Errorf("got %q, want HELLO", string(outBytes))
	}

	// 2. Test literal "stdin" strings are treated as files
	err = cmd.Execute([]string{"--", "stdin", outPath})
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("expected file not found error for literal 'stdin', got %v", err)
	}

	// 3. "-" borrows stdin/stdout
	cmd.CommandAction = func(c *CopyFile) error {
		if c.input != os.Stdin {
			t.Errorf("expected os.Stdin for '-'")
		}
		if c.output != os.Stdout {
			t.Errorf("expected os.Stdout for '-'")
		}
		return nil
	}
	err = cmd.Execute([]string{"--", "-", "-"})
	if err != nil {
		t.Fatalf("execute with '-' failed: %v", err)
	}
}
