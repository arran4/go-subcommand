package go_subcommand

import (
	"fmt"
)

func Validate(dir string) {
	_ = parse(dir)
	fmt.Println("Validation successful.")
}

func List(dir string) {
	dataModel := parse(dir)
	for _, cmd := range dataModel.Commands {
		fmt.Printf("Command: %s\n", cmd.MainCmdName)
		for _, subCmd := range cmd.SubCommands {
			fmt.Printf("  Subcommand: %s\n", subCmd.SubCommandSequence())
		}
	}
}
