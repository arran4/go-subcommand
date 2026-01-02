package returns

import (
	"errors"
	"fmt"
)

//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate || go run github.com/arran4/go-subcommand/cmd/gosubc generate"

// SimpleError is a subcommand `returns simple`
// Returns a simple error if the flag is set.
// Flags:
//   fail: --fail -f (default: false) Make the command fail
func SimpleError(fail bool) error {
	if fail {
		return errors.New("simple error occurred")
	}
	fmt.Println("SimpleError success")
	return nil
}

// MultipleReturns is a subcommand `returns multiple`
// Returns a value and an error. The value is ignored by the generator, but the error is checked.
// Flags:
//   fail: --fail -f (default: false) Make the command fail
func MultipleReturns(fail bool) (int, error) {
	if fail {
		return 0, errors.New("multiple returns error occurred")
	}
	fmt.Println("MultipleReturns success")
	return 42, nil
}
