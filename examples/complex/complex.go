package complex

import (
	"fmt"
	"time"
)

//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate || go run github.com/arran4/go-subcommand/cmd/gosubc generate"

// TopLevel is a subcommand `complex toplevel` -- A top level command
//
// This command demonstrates a top level command with a string argument.
// It prints the name provided.
//
// Flags:
//
//	name: -n --name (default: "world") The name to greet
func TopLevel(name string) {
	fmt.Printf("TopLevel command executed with name: %s\n", name)
}

// Nested is a subcommand `complex toplevel nested` -- A nested command
//
// This command demonstrates a nested command under 'toplevel'.
// It takes an integer and a boolean.
//
// Flags:
//
//	count:   -c --count   (default: 1)     Number of times to repeat
//	verbose: -v --verbose (default: false) Enable verbose output
func Nested(count int, verbose bool) {
	fmt.Printf("Nested command executed with count: %d, verbose: %v\n", count, verbose)
}

// Another is a subcommand `complex another` -- Another top level command
//
// This command demonstrates using a duration parameter.
//
// Flags:
//
//	wait: -w --wait (default: 1s) How long to wait
func Another(wait time.Duration) {
	fmt.Printf("Another command executed, waiting for %s\n", wait)
}
