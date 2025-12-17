package main

import (
	"fmt"
	"os"

	go_subcommand "github.com/arran4/go-subcommand"
)

func main() {
	if err := go_subcommand.Generate("."); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating gosub command: %v\n", err)
		os.Exit(1)
	}
}
