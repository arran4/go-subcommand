package main

import "fmt"

// Root is a subcommand `app`
//
// Flags:
//
//   flag: --flag
func Root(flag bool, args ...string) error {
	fmt.Printf("root: flag=%v args=%v\n", flag, args)
	return nil
}

// Command is a subcommand `app command`
//
// Flags:
//
//   empty: --empty
func Command(empty bool, args ...string) error {
	fmt.Printf("command: args=%v\n", args)
	return nil
}

// Child is a subcommand `app command child`
//
// Flags:
//
//   childflag: --childflag
func Child(childflag bool, args ...string) error {
	fmt.Printf("child: childflag=%v args=%v\n", childflag, args)
	return nil
}
