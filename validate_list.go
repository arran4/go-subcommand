package go_subcommand

import (
	"fmt"
	"io"
	"io/fs"
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
//	ops:		(parser: ParseAny)		Internal options
func Validate(dir string, parserName string, paths []string, recursive bool, ops ...any) error {
	fsys := fs.FS(os.DirFS(dir))
	var out io.Writer = os.Stdout

	for _, opt := range ops {
		switch o := opt.(type) {
		case fs.FS:
			fsys = o
		case io.Writer:
			out = o
		}
	}

	p, err := parsers.Get(parserName)
	if err != nil {
		return err
	}

	_, err = p.Parse(fsys, ".", &parsers.ParseOptions{
		SearchPaths: paths,
		Recursive:   recursive,
	})
	if err != nil {
		return err
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
//	ops:		(parser: ParseAny)		Internal options
func List(dir string, parserName string, paths []string, recursive bool, ops ...any) error {
	fsys := fs.FS(os.DirFS(dir))
	var out io.Writer = os.Stdout

	for _, opt := range ops {
		switch o := opt.(type) {
		case fs.FS:
			fsys = o
		case io.Writer:
			out = o
		}
	}

	p, err := parsers.Get(parserName)
	if err != nil {
		return err
	}

	dataModel, err := p.Parse(fsys, ".", &parsers.ParseOptions{
		SearchPaths: paths,
		Recursive:   recursive,
	})
	if err != nil {
		return err
	}
	for _, cmd := range dataModel.Commands {
		_, _ = fmt.Fprintf(out, "Command: %s\n", cmd.MainCmdName)
		for _, subCmd := range cmd.SubCommands {
			_, _ = fmt.Fprintf(out, "  Subcommand: %s\n", subCmd.SubCommandSequence())
		}
	}
	return nil
}
