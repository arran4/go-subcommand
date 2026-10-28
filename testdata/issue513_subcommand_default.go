package subdefaultapp

import (
	"fmt"
	"os"
)

func recordInvocation(marker string) error {
	if path := os.Getenv("ISSUE513_MARKER_FILE"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintln(f, marker)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if os.Getenv("ISSUE513_FAIL_ON_ACTION") == "1" {
		return fmt.Errorf("unexpected action execution during help: %s", marker)
	}
	fmt.Printf("executed:%s\n", marker)
	return nil
}

// SimpleAction is a subcommand `prog tools docs simple` (default)
//
// Flags:
//
//	val: --val (default: "nested-default") Value
func SimpleAction(val string) error {
	return recordInvocation("simple:val=" + val)
}

// RootAction is a subcommand `prog root` Explicit root child.
func RootAction() error {
	return recordInvocation("child-root")
}
