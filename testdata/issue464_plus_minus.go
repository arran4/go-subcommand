package application

import "fmt"

// Root is a subcommand `toggle`.
//
// Flags:
//	enabled: -x --enabled Enable the feature
//	verbose: -v --verbose Enable verbose output
//	count: --count Count value
//	x: -q One-letter boolean
//	args: ... Remaining arguments
func Root(enabled bool, verbose bool, count int, x bool, args ...string) {
	fmt.Printf("enabled=%t verbose=%t x=%t count=%d args=%v\n", enabled, verbose, x, count, args)
}
