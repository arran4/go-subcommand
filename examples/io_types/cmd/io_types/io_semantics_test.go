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

	var capturedReader, capturedWriter *os.File
	cmd.CommandAction = func(c *CopyFile) error {
		capturedReader = c.input.(*os.File)
		capturedWriter = c.output.(*os.File)
		return nil
	}

	outPath := filepath.Join(tmpDir, "output.txt")
	err = cmd.Execute([]string{"--", inPath, outPath})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	// Verify it's closed
	_, err = capturedReader.Stat()
	if !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected closed error, got: %v", err)
	}
	_, err = capturedWriter.Stat()
	if !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected writer closed error, got: %v", err)
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

	var capturedReader, capturedWriter *os.File
	cmd.CommandAction = func(c *CopyFile) error {
		capturedReader = c.input.(*os.File)
		capturedWriter = c.output.(*os.File)
		return errors.New("sentinel action error")
	}

	outPath := filepath.Join(tmpDir, "output.txt")
	err = cmd.Execute([]string{"--", inPath, outPath})
	if err == nil || !strings.Contains(err.Error(), "sentinel action error") {
		t.Fatalf("expected action error, got %v", err)
	}

	// Verify it's closed despite error
	_, err = capturedReader.Stat()
	if !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected closed error, got: %v", err)
	}
	_, err = capturedWriter.Stat()
	if !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected writer closed error, got: %v", err)
	}
}

func TestLifecycle_PartialFailure(t *testing.T) {
	originalReader, originalWriter := generatedOpenReader, generatedOpenWriter
	t.Cleanup(func() {
		generatedOpenReader, generatedOpenWriter = originalReader, originalWriter
	})

	reader := &observedReadCloser{name: "reader"}
	generatedOpenReader = func(string) (io.ReadCloser, error) { return reader, nil }
	generatedOpenWriter = func(string) (io.WriteCloser, error) {
		return nil, errors.New("sentinel open error")
	}

	parent := &RootCmd{
		FlagSet:  flag.NewFlagSet("root", flag.ContinueOnError),
		Commands: make(map[string]func() Cmd),
	}
	cmd := parent.NewCopyFile()

	cmd.CommandAction = func(c *CopyFile) error {
		// Should not be called because argument parsing fails
		return nil
	}

	err := cmd.Execute([]string{"--", "input", "output"})
	if err == nil || !strings.Contains(err.Error(), "sentinel open error") {
		t.Fatalf("expected output open error, got %v", err)
	}
	if reader.closeCount != 1 {
		t.Fatalf("first resource close count = %d, want 1", reader.closeCount)
	}
}

func TestLifecycle_ReverseOrderAndBorrowedStreams(t *testing.T) {
	originalReader, originalWriter := generatedOpenReader, generatedOpenWriter
	t.Cleanup(func() {
		generatedOpenReader, generatedOpenWriter = originalReader, originalWriter
	})

	var order []string
	reader := &observedReadCloser{name: "reader", order: &order}
	writer := &observedWriteCloser{name: "writer", order: &order}
	readerOpens, writerOpens := 0, 0
	generatedOpenReader = func(name string) (io.ReadCloser, error) {
		readerOpens++
		if name != "stdin" {
			t.Errorf("reader opener path = %q, want literal stdin", name)
		}
		return reader, nil
	}
	generatedOpenWriter = func(name string) (io.WriteCloser, error) {
		writerOpens++
		if name != "stdout" {
			t.Errorf("writer opener path = %q, want literal stdout", name)
		}
		return writer, nil
	}

	parent := &RootCmd{FlagSet: flag.NewFlagSet("root", flag.ContinueOnError), Commands: make(map[string]func() Cmd)}
	cmd := parent.NewCopyFile()
	cmd.CommandAction = func(*CopyFile) error { return nil }
	if err := cmd.Execute([]string{"--", "stdin", "stdout"}); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if got, want := strings.Join(order, ","), "writer,reader"; got != want {
		t.Fatalf("cleanup order = %q, want %q", got, want)
	}
	if reader.closeCount != 1 || writer.closeCount != 1 {
		t.Fatalf("close counts = reader %d, writer %d; want exactly once", reader.closeCount, writer.closeCount)
	}

	if err := cmd.Execute([]string{"--", "-", "-"}); err != nil {
		t.Fatalf("borrowed stream execute failed: %v", err)
	}
	if readerOpens != 1 || writerOpens != 1 {
		t.Fatalf("borrowed streams invoked openers: reader %d, writer %d", readerOpens, writerOpens)
	}
}

type observedReadCloser struct {
	name       string
	order      *[]string
	closeCount int
}

func (*observedReadCloser) Read([]byte) (int, error) { return 0, io.EOF }

func (c *observedReadCloser) Close() error {
	c.closeCount++
	if c.order != nil {
		*c.order = append(*c.order, c.name)
	}
	return nil
}

type observedWriteCloser struct {
	name       string
	order      *[]string
	closeCount int
}

func (*observedWriteCloser) Write(p []byte) (int, error) { return len(p), nil }

func (c *observedWriteCloser) Close() error {
	c.closeCount++
	if c.order != nil {
		*c.order = append(*c.order, c.name)
	}
	return nil
}
