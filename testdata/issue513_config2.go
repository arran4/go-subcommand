package config2

import "fmt"

// AppMain is a subcommand `app`
//
// Flags:
//
//	flag: --flag (default: "root_val") Flag value
func AppMain(flag string) error {
	fmt.Printf("executed:config2-root:flag=%s\n", flag)
	return nil
}

// ChildAction is a subcommand `app child`
//
// Flags:
//
//	val: --val (default: "child_val")
func ChildAction(val string) error {
	fmt.Printf("executed:config2-child:val=%s\n", val)
	return nil
}
