package main

import (
	"fmt"
	"os"
	"testing"
	"github.com/arran4/go-subcommand/examples/go-flag-test/cmd/app"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"go-flag-test": Main,
	}))
}

func Main() int {
	err := execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}

func execute() error {
	root, _ := app.NewRoot("app", "1.0", "commit", "date")
	return root.Execute(os.Args[1:])
}
