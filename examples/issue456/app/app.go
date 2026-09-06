package app

import (
	"fmt"
	myflag "github.com/arran4/go-subcommand/examples/issue456/app/flag"
)

// App is a subcommand `app`
func App(f myflag.MyFlagType) error {
	fmt.Printf("Flag: %s\n", f)
	return nil
}
