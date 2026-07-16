package main

// App is a subcommand `app`.
//
// Flags:
//
//	config: (required) --config -c Config path
func App(config string) {}

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
//	values: -v Repeatable values
//	ptr: --ptr Nullable pointer
//	parsed: (parser: "example.com/test/pkg".ParseThing) Parsed value
//	generated: (generator: "example.com/test/pkg".GenThing) Generated value
func Child(dir string, i int, values []string, ptr *int, parsed string, generated string) {}
