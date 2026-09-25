package myapp
import "fmt"

// Root is a subcommand `app`
// CLI-Parser: gnu
//
// Flags:
//	gnuFlag: -g (default: false)
//	longFlag: --long (default: "test")
//	args: (positional: true)
func Root(gnuFlag bool, longFlag string, args []string) error {
	fmt.Println("Root gnuFlag=", gnuFlag, " longFlag=", longFlag)
	return nil
}

// Child is a subcommand `app child`
// CLI-Parser: go-flag
//
// Flags:
//	goFlag: -go-flag (default: "flag")
//	boolFlag: -b (default: false)
//	args: (positional: true)
func Child(goFlag string, boolFlag bool, args []string) error {
	fmt.Println("Child goFlag=", goFlag, " boolFlag=", boolFlag)
	for _, arg := range args {
		fmt.Println("arg=" + arg)
	}
	return nil
}
