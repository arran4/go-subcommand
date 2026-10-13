package application

import "fmt"

// Root is a subcommand `toggle`.
//
// Flags:
//	enabled: -x --enabled Enable the feature
//	verbose: -v --verbose Enable verbose output
//	count: --count Count value
//	args: ... Remaining arguments
func Root(enabled bool, verbose bool, count int, args ...string) {
	fmt.Printf("enabled=%t verbose=%t count=%d args=%v\n", enabled, verbose, count, args)
}
