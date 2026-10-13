package main

import (
	"fmt"
	"io"
	"os"
)

func init() {
	GeneratedMainPolicy = MainPolicy{
		Initialize: func() error {
			fmt.Fprintln(os.Stderr, "policy initialized")
			return nil
		},
		NewRoot: func() (*RootCmd, error) {
			return NewRoot("policyapp", "custom-version", "custom-commit", "custom-date")
		},
		RenderError: func(out io.Writer, err error) {
			fmt.Fprintf(out, "custom error: %v\n", err)
		},
		ExitCode: func(error) int { return 23 },
	}
}
