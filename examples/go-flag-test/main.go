package main

import (
	"fmt"
	"github.com/arran4/go-subcommand/examples/go-flag-test/cmd/app"
	"os"
)

func main() {
	root, _ := app.NewRoot("app", "1.0", "commit", "date")
	err := root.Execute(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
