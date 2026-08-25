package main

import (
	"os"

	"github.com/arran4/go-subcommand/examples/io_types/cmd/io_types"
)

func main() {
	cmd, err := io_types.NewRoot("io_types", "dev", "HEAD", "today")
	if err != nil {
		os.Exit(1)
	}
	if err := cmd.Execute(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
