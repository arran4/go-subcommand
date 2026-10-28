package config1

import "fmt"

// AppMain is a subcommand `app`
//
// Flags:
//
//	flag: --flag (default: "default_val") Flag value
func AppMain(flag string) error {
	fmt.Printf("executed:config1:flag=%s\n", flag)
	return nil
}
