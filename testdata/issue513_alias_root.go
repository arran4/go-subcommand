package aliasroot

import "fmt"

// StatusAction is a subcommand `app status`
// (alias: root)
func StatusAction() error {
	fmt.Printf("executed:status-via-root-alias\n")
	return nil
}
