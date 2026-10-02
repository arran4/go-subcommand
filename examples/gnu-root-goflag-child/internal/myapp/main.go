package myapp
import "fmt"

// Root is a subcommand `app`
// CLI-Parser: gnu
//
// Flags:
//	gnuFlag: -g (default: false) Enable GNU flag
//	longFlag: --long (default: "test") Long GNU value
//	dir: --dir Parent directory
//	sameDir: --same-dir Parent shared directory
//	args: (positional: true)
func Root(gnuFlag bool, longFlag, dir, sameDir string, args []string) error {
	fmt.Println("Root gnuFlag=", gnuFlag, " longFlag=", longFlag)
	return nil
}

// Child is a subcommand `app child` -- Go-flag child
// CLI-Parser: go-flag
//
// Flags:
//	dir: --dir (from parent)
//	sameDir: --same-dir (from parent)
//	goFlag: -go-flag (default: "flag") Go-flag value
//	boolFlag: -b (default: false) Enable child flag
//	args: (positional: true)
func Child(d, sameDir, goFlag string, boolFlag bool, args []string) error {
	fmt.Println("Child dir=", d, " sameDir=", sameDir, " goFlag=", goFlag, " boolFlag=", boolFlag)
	for _, arg := range args {
		fmt.Println("arg=" + arg)
	}
	return nil
}
