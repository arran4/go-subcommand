package main

import (
	"flag"
	"fmt"
	"os"

	go_subcommand "github.com/arran4/go-subcommand"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("expected 'generate', 'validate' or 'list' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)
		var dir string
		generateCmd.StringVar(&dir, "dir", ".", "directory to look for subcommands")
		generateCmd.Parse(os.Args[2:])
		go_subcommand.Generate(dir)
	case "validate":
		validateCmd := flag.NewFlagSet("validate", flag.ExitOnError)
		var dir string
		validateCmd.StringVar(&dir, "dir", ".", "directory to validate")
		validateCmd.Parse(os.Args[2:])
		go_subcommand.Validate(dir)
	case "list":
		listCmd := flag.NewFlagSet("list", flag.ExitOnError)
		var dir string
		listCmd.StringVar(&dir, "dir", ".", "directory to list subcommands from")
		listCmd.Parse(os.Args[2:])
		go_subcommand.List(dir)
	default:
		fmt.Println("expected 'generate', 'validate' or 'list' subcommands")
		os.Exit(1)
	}
}
