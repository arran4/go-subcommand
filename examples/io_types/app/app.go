package app

import (
	"fmt"
	"io"
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

// RunInspectFile is a subcommand `io_types inspect-file` that inspects an io.Reader
func RunInspectFile(
	file io.Reader, // @1 The file to inspect
) error {
	b, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}
	fmt.Printf("File size: %d\n", len(b))
	return nil
}
