package app

import (
	"fmt"
	"io"
)

// Issue185 is a subcommand `app`
//
//	flag: --reader (default: "-") (description: "Input reader")
//	flag: --writer (default: "-") (description: "Output writer")
func Issue185(reader io.Reader, writer io.Writer) error {
	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "Received: %s", string(b))
	return err
}
