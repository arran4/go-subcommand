package app

import (
	"fmt"
	"io"
)

// App is a subcommand `app`
//
// Flags:
//
//	reader: --reader (default: "-") Input reader
//	writer: --writer (default: "-") Output writer
func App(reader io.Reader, writer io.Writer) error {
	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "Received: %s", string(b))
	return err
}

// MyCmd is a subcommand `app mycmd`
//
// Flags:
//
//	reader: --reader (default: "-") Input reader
//	writer: --writer (default: "-") Output writer
func MyCmd(reader io.Reader, writer io.Writer) error {
	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "Received: %s", string(b))
	return err
}

// MyCmd2 is a subcommand `app mycmd2`
//
// Flags:
//
//	reader: @1 (default: "-") Input reader
//	writer: @2 (default: "-") Output writer
func MyCmd2(reader io.Reader, writer io.Writer) error {
	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "Received pos: %s", string(b))
	return err
}
