package go_subcommand

import (
	"fmt"
	"io"
	"os"

	"github.com/arran4/go-subcommand/parsers"
)

// Validate is a subcommand `gosubc validate` validates the subcommand code
//
// Flags:
//
//	dir:		--dir		(default: ".")		The project root directory containing go.mod
//	parserName:	--parser-name	(default: "commentv1")	Name of the parser to use
//	paths:		--path		(default: nil)		Paths to search for subcommands (relative to dir)
//	recursive:	--recursive	(default: true)		Search recursively
//	ops:		(parser: ParseAny)	Optional dependency injection arguments
func Validate(dir string, parserName string, paths []string, recursive bool, ops ...any) error {
	_, err := parse(dir, parserName, &parsers.ParseOptions{
		SearchPaths: paths,
		Recursive:   recursive,
	}, ops...)
	if err != nil {
		return err
	}
	var out io.Writer = os.Stdout
	for _, opt := range ops {
		if w, ok := opt.(io.Writer); ok {
			out = w
			break
		}
	}
	_, _ = fmt.Fprintln(out, "Validation successful.")
	return nil
}

// List is a subcommand `gosubc list` lists the subcommands
//
// Flags:
//
//	dir:		--dir		(default: ".")		The project root directory containing go.mod
//	parserName:	--parser-name	(default: "commentv1")	Name of the parser to use
//	paths:		--path		(default: nil)		Paths to search for subcommands (relative to dir)
//	recursive:	--recursive	(default: true)		Search recursively
//	ops:		(parser: ParseAny)	Optional dependency injection arguments
func List(dir string, parserName string, paths []string, recursive bool, ops ...any) error {
	dataModel, err := parse(dir, parserName, &parsers.ParseOptions{
		SearchPaths: paths,
		Recursive:   recursive,
	}, ops...)
	if err != nil {
		return err
	}
	var out io.Writer = os.Stdout
	for _, opt := range ops {
		if w, ok := opt.(io.Writer); ok {
			out = w
			break
		}
	}
	for _, cmd := range dataModel.Commands {
		_, _ = fmt.Fprintf(out, "Command: %s\n", cmd.MainCmdName)
		for _, subCmd := range cmd.SubCommands {
			_, _ = fmt.Fprintf(out, "  Subcommand: %s\n", subCmd.SubCommandSequence())
		}
	}
	return nil
}

// parseAny is a dummy parser for dependency injection
func ParseAny(s string) (any, error) { return s, nil }
