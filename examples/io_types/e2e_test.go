package main

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/arran4/go-subcommand/examples/io_types/cmd/io_types"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"io_types": func() int {
			cmd, err := io_types.NewRoot("io_types", "dev", "HEAD", "today")
			if err != nil {
				return 1
			}
			if err := cmd.Execute(os.Args[1:]); err != nil {
				return 1
			}
			return 0
		},
	}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}
