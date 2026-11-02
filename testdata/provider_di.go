package app

import "fmt"

func BuildLabel(prefix string) (string, error) {
	return prefix + "-generated", nil
}

// Root is a subcommand `app`.
// Flags:
//
//	prefix: --prefix Prefix supplied by the command line.
//	label: (generator: BuildLabel(prefix)) Label supplied by a provider dependency.
func Root(prefix, label string) {
	fmt.Printf("%s\n", label)
}
