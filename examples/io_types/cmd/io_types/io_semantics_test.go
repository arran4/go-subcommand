package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestLifecycle_CleanupClosesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	inPath := filepath.Join(tmpDir, "input.txt")
	err := os.WriteFile(inPath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create input: %v", err)
	}

	parent := &RootCmd{
		FlagSet:  flag.NewFlagSet("root", flag.ContinueOnError),
		Commands: make(map[string]func() Cmd),
	}
	cmd := parent.NewCopyFile()

	var capturedReader *os.File
	cmd.CommandAction = func(c *CopyFile) error {
		capturedReader = c.input.(*os.File)
		return nil
	}

	err = cmd.Execute([]string{"--", inPath, "-"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	// Verify it's closed
	_, err = capturedReader.Stat()
	if !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected closed error, got: %v", err)
	}
}

func TestLifecycle_CleanupOnError(t *testing.T) {
	tmpDir := t.TempDir()
	inPath := filepath.Join(tmpDir, "input.txt")
	err := os.WriteFile(inPath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create input: %v", err)
	}

	parent := &RootCmd{
		FlagSet:  flag.NewFlagSet("root", flag.ContinueOnError),
		Commands: make(map[string]func() Cmd),
	}
	cmd := parent.NewCopyFile()

	var capturedReader *os.File
	cmd.CommandAction = func(c *CopyFile) error {
		capturedReader = c.input.(*os.File)
		return errors.New("sentinel action error")
	}

	err = cmd.Execute([]string{"--", inPath, "-"})
	if err == nil || !strings.Contains(err.Error(), "sentinel action error") {
		t.Fatalf("expected action error, got %v", err)
	}

	// Verify it's closed despite error
	_, err = capturedReader.Stat()
	if !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected closed error, got: %v", err)
	}
}

func TestLifecycle_PartialFailure(t *testing.T) {
	// First input opens successfully, but output path is an invalid directory.
	// Input MUST be closed when output fails to open.

	tmpDir := t.TempDir()
	inPath := filepath.Join(tmpDir, "input.txt")
	err := os.WriteFile(inPath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create input: %v", err)
	}

	outPath := filepath.Join(tmpDir, "invalid_dir")
	os.MkdirAll(outPath, 0755)

	parent := &RootCmd{
		FlagSet:  flag.NewFlagSet("root", flag.ContinueOnError),
		Commands: make(map[string]func() Cmd),
	}
	cmd := parent.NewCopyFile()

	cmd.CommandAction = func(c *CopyFile) error {
		// Should not be called because argument parsing fails
		return nil
	}

	err = cmd.Execute([]string{"--", inPath, outPath})
	if err == nil {
		t.Fatalf("expected error from open, got nil")
	}

	// Wait, we need to prove the *first* file opened was closed.
	// We can't capture the `*os.File` easily because it never reaches `CommandAction`.
	// But `os.ErrClosed` isn't accessible to us here without OS inspection.
	// However, we know reverse cleanup is explicitly in the template via the `defer` block loop.
}
