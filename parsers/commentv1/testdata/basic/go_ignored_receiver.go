package main

type T struct{}

// Method is a subcommand that should be ignored
func (t *T) Method() {}
