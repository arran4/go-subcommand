package main

import (
	"fmt"
)

type CustomType string

func ParseCustom(s string) (CustomType, error) {
	return CustomType("parsed:" + s), nil
}

// Root is a subcommand `app`
// (global)
// Flags:
//   verbose: -v (default: false) Verbose output
//   name: --name (required) Name of the user
//   custom: --custom (parser: ParseCustom) Custom type parsing
//   ptr: --ptr *int (default: nil) Pointer to int
func Root(verbose bool, name string, custom CustomType, ptr *int) error {
	fmt.Printf("Root: verbose=%v name=%q custom=%v ptr=%v\n", verbose, name, custom, ptr)
	return nil
}

// Child is a subcommand `app child`
// Flags:
//   slice: -s (default: []) Slice of strings
//   repeat: -r (default: []) Repeatable slice
func Child(slice []string, repeat []string) {
	fmt.Printf("Child: slice=%v repeat=%v\n", slice, repeat)
}
