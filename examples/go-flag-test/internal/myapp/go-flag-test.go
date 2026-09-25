package myapp

import "fmt"

// App is a subcommand `app`
// CLI-Parser: go-flag
//
// Flags:
//
//	in:		--in		(default: "stdin")	Input source
//	out:	--out		(default: "stdout")	Output target
//	verbose: --verbose	(default: false)	Verbose
//	a: -a (default: false)
//	b: -b (default: false)
//	c: -c (default: false)
//	args: (positional: true)
func App(in string, out string, verbose bool, a bool, b bool, c bool, args []string) error {
	fmt.Println("App in=" + in + " out=" + out)
	if verbose {
		fmt.Println("verbose")
	}
	if a {
		fmt.Println("a")
	}
	if b {
		fmt.Println("b")
	}
	if c {
		fmt.Println("c")
	}
	for _, arg := range args {
		fmt.Println("arg=" + arg)
	}
	return nil
}

// Child is a subcommand `app child`
// CLI-Parser: gnu
//
// Flags:
//
//	v: -v (default: false)
func Child(v bool) error {
	fmt.Println("Child v=", v)
	return nil
}
