package helprouteapp

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

// ProvideState fails if called during help.
func ProvideState() (string, error) {
	if err := recordInvocation("provider:state"); err != nil {
		return "", err
	}
	return "ready", nil
}

// SimpleAction is a subcommand `prog simple` Simple demonstration command.
// Default route: help simple
//
// Flags:
//
//	name: -n --name (default: "world") Target name
//	state: (generator: ProvideState)
func SimpleAction(name, state string) error {
	return recordInvocation(fmt.Sprintf("action:simple:name=%s,state=%s", name, state))
}

// OtherAction is a subcommand `prog other` Other command.
//
// Flags:
//
//	state: (generator: ProvideState)
func OtherAction(state string) error {
	return recordInvocation(fmt.Sprintf("action:other:state=%s", state))
}

// RootAction is a subcommand `prog root` Explicit root child supervisor.
//
// Flags:
//
//	serve: --serve (default: false)
func RootAction(serve bool) error {
	return recordInvocation(fmt.Sprintf("action:root:serve=%t", serve))
}
