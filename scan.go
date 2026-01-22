package go_subcommand

import (
	"fmt"
	"strings"
)

// Scan is a subcommand `gosubc scan` scans for subcommands and prints their flags
//
// Flags:
//   dir: --dir string (default: ".") The project root directory
func Scan(dir string) error {
	dataModel, err := parse(dir)
	if err != nil {
		return err
	}

	for _, cmd := range dataModel.Commands {
		fmt.Printf("Command: %s\n", cmd.MainCmdName)
		printSubCommands(cmd.SubCommands, 1)
	}
	return nil
}

func printSubCommands(subCmds []*SubCommand, indentLevel int) {
	indent := strings.Repeat("  ", indentLevel)
	for _, sc := range subCmds {
		fmt.Printf("%sSubcommand: %s\n", indent, sc.SubCommandSequence())
		if len(sc.Parameters) > 0 {
			fmt.Printf("%s  Flags:\n", indent)
			for _, p := range sc.Parameters {
				fmt.Printf("%s    %s (default: %q) %s\n", indent, p.FlagString(), p.Default, p.Description)
			}
		}
		if len(sc.SubCommands) > 0 {
			printSubCommands(sc.SubCommands, indentLevel+1)
		}
	}
}
