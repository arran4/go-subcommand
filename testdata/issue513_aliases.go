package aliasesapp

import (
	"fmt"
	"os"
)

func recordInvocation(marker string) error {
	if path := os.Getenv("ISSUE513_ALIAS_MARKER_FILE"); path != "" {
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
	if os.Getenv("ISSUE513_ALIAS_FAIL_ON_ACTION") == "1" {
		return fmt.Errorf("unexpected action execution during help: %s", marker)
	}
	fmt.Printf("executed:%s\n", marker)
	return nil
}

// ProvideAliasRuntimeState fails if called during help.
func ProvideAliasRuntimeState() (string, error) {
	if err := recordInvocation("provider:alias-runtime"); err != nil {
		return "", err
	}
	return "alias-ready", nil
}

// RunApp is a subcommand `aliasapp` Application root command with custom function name.
//
// Flags:
//
//	config: --config (default: "default.cfg") Configuration path
//	state: (generator: ProvideAliasRuntimeState)
func RunApp(config string, state string) error {
	return recordInvocation(fmt.Sprintf("app-root:config=%s,state=%s", config, state))
}

// RootCmdAction is a subcommand `aliasapp root` Explicit root child with aliases.
// (aka: supervisor, r)
//
// Flags:
//
//	config: --config (from parent)
//	serve: --serve (default: false)
//	state: (generator: ProvideAliasRuntimeState)
func RootCmdAction(config string, serve bool, state string) error {
	return recordInvocation(fmt.Sprintf("child-root:config=%s,serve=%t,state=%s", config, serve, state))
}

// SvcAction is a subcommand `aliasapp svc` Manage services.
// Aliases: service, s
func SvcAction() error {
	return recordInvocation("svc")
}

// SvcListAction is a subcommand `aliasapp svc list` List services.
// (aka: ls)
//
// Flags:
//
//	all: --all (default: false)
func SvcListAction(all bool) error {
	return recordInvocation(fmt.Sprintf("svc-list:all=%t", all))
}
