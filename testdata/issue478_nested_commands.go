package commands

import "fmt"

// Generate is a subcommand `gogenmux generate`.
//
// Flags:
//
//	name: --name Name to generate
func Generate(name string) {
	fmt.Printf("generate:%s\n", name)
}

// Inspect is a subcommand `gogenmux inspect`.
//
// Flags:
//	path: @1 Path to inspect
func Inspect(path string) {
	fmt.Printf("inspect:%s\n", path)
}

// Validate is a subcommand `gogenmux validate`.
//
// Flags:
//	strict: --strict Validate strictly
func Validate(strict bool) {
	fmt.Printf("validate:%t\n", strict)
}

// Show is a subcommand `gogenmux config show`.
//
// Flags:
//	format: --format Output format
func Show(format string) {
	fmt.Printf("show:%s\n", format)
}
