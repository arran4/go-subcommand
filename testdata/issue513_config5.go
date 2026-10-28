package config5

import "fmt"

// RootAction is a subcommand `app root`
//
// Flags:
//
//	serve: --serve (default: false)
func RootAction(serve bool) error {
	fmt.Printf("executed:config5-child-root:serve=%t\n", serve)
	return nil
}

// OtherAction is a subcommand `app other`
func OtherAction() error {
	fmt.Printf("executed:config5-other\n")
	return nil
}
