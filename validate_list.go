package go_subcommand

import (
	"fmt"
)

// Validate is a subcommand `gosubc validate`
// Validates the subcommand code
func Validate(dir string) error {
	_, err := parse(dir)
	if err != nil {
		return err
	}
	fmt.Println("Validation successful.")
	return nil
}

// List is a subcommand `gosubc list`
// Lists the subcommands
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
