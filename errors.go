package go_subcommand

import "errors"

// ErrPrintHelp when returned by any function anywhere it will switch the command from whatever it is to help.
var ErrPrintHelp = errors.New("print help")

// ErrHelp tells the user to use help.
var ErrHelp = errors.New("help requested")
