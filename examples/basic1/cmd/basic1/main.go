package main

import (
	"fmt"
	"os"
)

func main() {
	root := NewRootCmd()

	root.Commands["example1"] = root.Newexample1Cmd()

	if len(os.Args) < 2 {
		root.Usage()
		os.Exit(1)
	}

	cmd, ok := root.Commands[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		root.Usage()
		os.Exit(1)
	}

	if err := cmd.Execute(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
