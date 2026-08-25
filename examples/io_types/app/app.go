package app

import (
	"fmt"
	"io"
	"os"
)

// App is a subcommand `io_types`
func App() error {
	return nil
}

// RunCopyFile is a subcommand `io_types copy-file` that copies a file
func RunCopyFile(
	input io.Reader, // @1 The input file
	output io.Writer, // @2 The output file
) error {
	_, err := io.Copy(output, input)
	if err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}
	return nil
}

// RunInspectFile is a subcommand `io_types inspect-file` that inspects an os.File
func RunInspectFile(
	file *os.File, // @1 The file to inspect
) error {
	fi, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat failed: %w", err)
	}
	fmt.Printf("File size: %d\n", fi.Size())
	return nil
}
