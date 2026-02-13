package go_subcommand

import (
	"fmt"
)

// Validate is a subcommand `gosubc validate` that validates the subcommand code
//
// Flags:
//
//	dir:        --dir         (default: ".") The project root directory containing go.mod
//	parserName: --parser-name (default: "comment") Name of the parser to use
func Validate(dir string, parserName string) error {
	_, err := parse(dir, parserName)
	if err != nil {
		return err
	}
	fmt.Println("Validation successful.")
	return nil
}

// List is a subcommand `gosubc list` that lists the subcommands
//
// Flags:
//
//	dir:        --dir         (default: ".") The project root directory containing go.mod
//	parserName: --parser-name (default: "comment") Name of the parser to use
func List(dir string, parserName string) error {
	dataModel, err := parse(dir, parserName)
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
