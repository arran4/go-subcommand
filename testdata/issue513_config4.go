package config4

import "fmt"

// ChildAction is a subcommand `app child`
//
// Flags:
//
//	val: --val (default: "child_val")
func ChildAction(val string) error {
	fmt.Printf("executed:config4-child:val=%s\n", val)
	return nil
}
