package issue50

import "fmt"

// Root is a subcommand `issue50`
func Root() error {
	return nil
}

// Parent is a subcommand `issue50 parent`
// flag: verbose --verbose -v "Verbosity level"
func Parent(verbose bool) error {
	fmt.Printf("Parent verbose: %v\n", verbose)
	return nil
}

// Child is a subcommand `issue50 parent child`
// parent-flag: verbose
func Child(verbose bool) error {
	fmt.Printf("Child verbose: %v\n", verbose)
	return nil
}
