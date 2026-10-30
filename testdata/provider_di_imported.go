package app

import "fmt"

// Root is a subcommand `app`.
// Flags:
//
//	prefix: --prefix Prefix supplied by the command line.
//	label: (generator: "example.com/providersimport/deps".BuildLabel(prefix)) An imported provider dependency.
func Root(prefix, label string) {
	fmt.Println(label)
}
