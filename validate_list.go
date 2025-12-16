package go_subcommand

import (
	"fmt"
)

func Validate(dir string) error {
	_, err := parse(dir)
	if err != nil {
		return err
	}
	fmt.Println("Validation successful.")
	return nil
}

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
