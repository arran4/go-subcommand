package nestedaliasapp

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

// RunProg is a subcommand `prog` Root application action.
// Aliases: tools docs simple, tools docs s
//
// Flags:
//
//	config: --config (default: "default.cfg") Configuration path
//	state: (generator: ProvideState)
func RunProg(config, state string) error {
	return recordInvocation(fmt.Sprintf("app-root:config=%s,state=%s", config, state))
}

// ToolsAction is a subcommand `prog tools` Developer tools.
// Aliases: t
func ToolsAction() error {
	return recordInvocation("tools")
}

// DocsAction is a subcommand `prog tools docs` Documentation tools.
// Aliases: d
func DocsAction() error {
	return recordInvocation("docs")
}

// RootAction is a subcommand `prog root` Explicit root child supervisor.
// (aka: supervisor, r)
//
// Flags:
//
//	serve: --serve (default: false)
func RootAction(serve bool) error {
	return recordInvocation(fmt.Sprintf("child-root:serve=%t", serve))
}
