package application

import "errors"

// Root is a subcommand `policyapp`.
func Root() {}

// Fail is a subcommand `policyapp fail`.
func Fail() error {
	return errors.New("application failure")
}
