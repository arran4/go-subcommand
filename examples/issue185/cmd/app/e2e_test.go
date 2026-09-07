package main

import (
	"os"
	"testing"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"app": func() int {
			root, err := NewRoot("app", "dev", "none", "unknown")
			if err != nil {
				os.Stderr.WriteString(err.Error() + "\n")
				return 1
			}
			if err := root.Execute(os.Args[1:]); err != nil {
				os.Stderr.WriteString(err.Error() + "\n")
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
