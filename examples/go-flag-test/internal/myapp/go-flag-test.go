package myapp

import (
	"io"
	"fmt"
	"strings"
)

// App is a subcommand `app`
// CLI-Parser: go-flag
//
// Flags:
//	in:		--in		(default: "-")	Input source
//	out:	--out		(default: "-")	Output target
//	verbose: --verbose	(default: false)	Verbose
//	a: -a (default: false)
//	b: -b (default: false)
//	c: -c (default: false)
//	pointerInt: -pointerInt (default: nil)
//	args: (positional: true)
func App(in io.Reader, out io.Writer, verbose bool, a bool, b bool, c bool, pointerInt *int, args []string) error {
	inBytes, _ := io.ReadAll(in)
	fmt.Fprintf(out, "App in=%s\n", strings.TrimSpace(string(inBytes)))
	if verbose {
		fmt.Fprintln(out, "verbose")
	}
	if a { fmt.Fprintln(out, "a") }
	if b { fmt.Fprintln(out, "b") }
	if c { fmt.Fprintln(out, "c") }
	if pointerInt != nil { fmt.Fprintln(out, "pointerInt=", *pointerInt) }
	for _, arg := range args {
		fmt.Fprintln(out, "arg=" + arg)
	}
	return nil
}

// Child is a subcommand `app child`
// CLI-Parser: gnu
//
// Flags:
//	v: -v (default: false)
func Child(v bool) error {
	fmt.Println("Child v=", v)
	return nil
}
