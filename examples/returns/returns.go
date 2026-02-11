package returns

import (
	"errors"
	"fmt"
)

//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate || go run github.com/arran4/go-subcommand/cmd/gosubc generate"

// SimpleError is a subcommand `returns simple`
// Returns a simple error if the flag is set.
// Flags:
//
//	fail: -f --fail (default: false) Make the command fail
func SimpleError(fail bool) error {
	if fail {
		return errors.New("simple error occurred")
	}
	fmt.Println("SimpleError success")
	return nil
}
