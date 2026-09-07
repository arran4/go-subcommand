package app

import (
	"fmt"
	"io"
)

// App is a subcommand `app`
//
//	flag: --reader (default: "-") (description: "Input reader")
//	flag: --writer (default: "-") (description: "Output writer")
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
//	flag: --reader (default: "-") (description: "Input reader")
//	flag: --writer (default: "-") (description: "Output writer")
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
//	arg: @1 (default: "-") (description: "Input reader")
//	arg: @2 (default: "-") (description: "Output writer")
func MyCmd2(reader io.Reader, writer io.Writer) error {
	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "Received pos: %s", string(b))
	return err
}
