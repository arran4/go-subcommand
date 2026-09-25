package main

import (
	"os"
	"testing"
	"github.com/arran4/go-subcommand/examples/gnu-root-goflag-child/cmd/app"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"test-app": Main,
	}))
}

func Main() int {
	root, _ := app.NewRoot("app", "1.0", "commit", "date")
	err := root.Execute(os.Args[1:])
	if err != nil {
		return 1
	}
	return 0
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}
