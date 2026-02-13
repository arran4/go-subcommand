package go_subcommand

import (
	"fmt"
)

// Validate is a subcommand `gosubc validate` that validates the subcommand code
// param dir (default: ".") The project root directory containing go.mod
//
// Flags:
//
//	dir: --dir (default: ".") The project root directory containing go.mod
func Validate(dir string) error {
	_, err := parse(dir)
	if err != nil {
		return err
	}
	fmt.Println("Validation successful.")
	return nil
}

// List is a subcommand `gosubc list` that lists the subcommands
// param dir (default: ".") The project root directory containing go.mod
//
// Flags:
//
//	dir: --dir (default: ".") The project root directory containing go.mod
func List(dir string) error {
	dataModel, err := parse(dir)
	if err != nil {
		return err
	}
	for _, cmd := range dataModel.Commands {
		fmt.Printf("Command: %s\n", cmd.MainCmdName)
		for _, subCmd := range cmd.SubCommands {
			fmt.Printf("  Subcommand: %s\n", subCmd.SubCommandSequence())
		}
	}
	return nil
}
