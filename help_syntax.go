package go_subcommand

import (
	"fmt"
)

// HelpSyntax is a subcommand `gosubc syntax` that prints the available forms of function comments
func HelpSyntax() error {
	fmt.Print(`
Gosubc Syntax Guide

The 'gosubc' tool parses Go comments to generate CLI subcommands.

Command Definition:

  // <Function> is a subcommand <command> <subcommand>... that <description>
  func <Function>(...) error { ... }

  Example:
    // MyCommand is a subcommand my-cmd sub-cmd that does something
    func MyCommand(arg string) error { ... }

Arguments / Flags:

  Arguments are defined by function parameters.
  Supported types: string, int, int64, bool, float64, time.Duration.

  You can customize flags using:
  1. Flags Block (Recommended):
     Place a 'Flags:' block in the function comment. Indentation matters (tabs preferred).

     // Flags:
     // 	argName: --flag-name <description> (default: <value>)

  2. Inline Comments:
     Place comments after the parameter definition.

     func MyCommand(
         arg string, // flag: --arg <description> (default: <value>)
     ) error { ... }

  3. Preceding Comments:
     Place comments before the parameter definition.

     // flag: --arg <description> (default: <value>)
     arg string,

Positional Arguments:

  Use '@n' to mark a parameter as positional argument n (1-based).

  func MyCommand(
      input string, // @1 input file
      output string, // @2 output file
  ) error { ... }

Variadic Arguments:

  Use '...' or 'min...max' to specify variadic arguments.

  func MyCommand(
      files ...string, // @3... (min: 1)
  ) error { ... }

Defaults:

  Use '(default: value)' in comments to specify default values.

  // arg: --arg (default: "default value")

Implicit Parsing:

  If no specific flag is defined, parameter names are converted to kebab-case flags.
  e.g. 'MyArg' becomes '--my-arg'.

For more details, see: https://github.com/arran4/go-subcommand
`)
	return nil
}
