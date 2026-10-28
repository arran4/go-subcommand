package go_subcommand

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
)

// Scan is a subcommand `gosubc scan` lists all available subcommands and their flags
// Scan lists all available subcommands and their flags from the parsed codebase.
// It is useful for verifying the command structure and configuration.
//
// Flags:
//
//	dir:		--dir		(default: ".")		The project root directory
//	parserName:	--parser-name	(default: "commentv1")	Name of the parser to use
//	paths:		--path		(default: nil)		Paths to search for subcommands (relative to dir)
//	recursive:	--recursive	(default: true)		Search recursively
func Scan(dir string, parserName string, paths []string, recursive bool) error {
	return ScanWithFS(os.DirFS(dir), os.Stdout, parserName, paths, recursive)
}

// ScanWithFS lists subcommands from inputFS and writes the stable report to
// output. It is the side-effect-free core used by Scan and tests.
func ScanWithFS(inputFS fs.FS, output io.Writer, parserName string, paths []string, recursive bool) error {
	dataModel, err := parse(".", parserName, &parsers.ParseOptions{
		SearchPaths: paths,
		Recursive:   recursive,
	}, inputFS)
	if err != nil {
		return err
	}

	for _, cmd := range dataModel.Commands {
		if _, err := fmt.Fprintf(output, "Command: %s\n", cmd.MainCmdName); err != nil {
			return err
		}
		if err := printSubCommands(output, cmd.SubCommands, 1); err != nil {
			return err
		}
	}
	return nil
}

func printSubCommands(output io.Writer, subCmds []*model.SubCommand, indentLevel int) error {
	indent := strings.Repeat("  ", indentLevel)
	for _, sc := range subCmds {
		if _, err := fmt.Fprintf(output, "%sSubcommand: %s\n", indent, sc.SubCommandSequence()); err != nil {
			return err
		}
		if len(sc.Parameters) > 0 {
			if _, err := fmt.Fprintf(output, "%s  Flags:\n", indent); err != nil {
				return err
			}
			for _, p := range sc.Parameters {
				defaultVal := ""
				if p.Default != "" {
					defaultVal = fmt.Sprintf(" (default: %q)", p.Default)
				}
				if _, err := fmt.Fprintf(output, "%s    %s%s %s\n", indent, p.FlagString(), defaultVal, p.Description); err != nil {
					return err
				}
			}
		}
		if len(sc.SubCommands) > 0 {
			if err := printSubCommands(output, sc.SubCommands, indentLevel+1); err != nil {
				return err
			}
		}
	}
	return nil
}
