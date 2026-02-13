package go_subcommand

import (
	"fmt"
	"strings"

	"github.com/arran4/go-subcommand/model"
)

// Scan is a subcommand `gosubc scan` lists all available subcommands and their flags
// Scan lists all available subcommands and their flags from the parsed codebase.
// It is useful for verifying the command structure and configuration.
//
// Flags:
//   dir: --dir (default: ".") string The project root directory
func Scan(dir string) error {
	dataModel, err := parse(dir, "commentv1")
	if err != nil {
		return err
	}

	for _, cmd := range dataModel.Commands {
		fmt.Printf("Command: %s\n", cmd.MainCmdName)
		printSubCommands(cmd.SubCommands, 1)
	}
	return nil
}

func printSubCommands(subCmds []*model.SubCommand, indentLevel int) {
	indent := strings.Repeat("  ", indentLevel)
	for _, sc := range subCmds {
		fmt.Printf("%sSubcommand: %s\n", indent, sc.SubCommandSequence())
		if len(sc.Parameters) > 0 {
			fmt.Printf("%s  Flags:\n", indent)
			for _, p := range sc.Parameters {
				defaultVal := ""
				if p.Default != "" {
					defaultVal = fmt.Sprintf(" (default: %q)", p.Default)
				}
				fmt.Printf("%s    %s%s %s\n", indent, p.FlagString(), defaultVal, p.Description)
			}
		}
		if len(sc.SubCommands) > 0 {
			printSubCommands(sc.SubCommands, indentLevel+1)
		}
	}
}
