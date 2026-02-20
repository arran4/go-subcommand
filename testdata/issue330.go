package main

// Parent is a subcommand `app parent` that Does work in a directory
//
// Flags:
//
//	dir: --dir (default: ".") The directory
func Parent(dir string) {}

// Child is a subcommand `app parent child` that Does work in a directory
//
// Flags:
//
//	dir: --dir (from parent)
//	i: --i (default: 0) A random int
func Child(d string, ir int) {}

// GrandChild is a subcommand `app parent child grandchild`
func GrandChild() {}
