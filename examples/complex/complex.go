package complex

import "fmt"

// TopLevel is a subcommand `complex toplevel`
// TopLevel command description
func TopLevel(name string) {
	fmt.Printf("TopLevel command executed with name: %s\n", name)
}

// Nested is a subcommand `complex toplevel nested`
// Nested command description
func Nested(count int, verbose bool) {
	fmt.Printf("Nested command executed with count: %d, verbose: %v\n", count, verbose)
}

// Another is a subcommand `complex another`
// Another command description
func Another() {
	fmt.Println("Another command executed")
}
