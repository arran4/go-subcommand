package downloadsd

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

// ProvideRuntimeState is a provider that fails if invoked during help.
func ProvideRuntimeState() (string, error) {
	if err := recordInvocation("provider:runtime-state"); err != nil {
		return "", err
	}
	return "ready", nil
}

// Downloadsd is a subcommand `downloadsd` Manage downloads and services.
//
// Flags:
//
//	config: --config (default: "/etc/downloadsd.conf") Path to configuration file
//	host: --host (default: "127.0.0.1") Listen host
//	state: (generator: ProvideRuntimeState) Runtime state
func Downloadsd(config string, host string, state string) error {
	return recordInvocation(fmt.Sprintf("app-root:config=%s,host=%s,state=%s", config, host, state))
}

// Root is a subcommand `downloadsd root` Run the privileged system supervisor.
//
// Flags:
//
//	config: --config (from parent)
//	serve: --serve (default: false) Run supervisor loop
//	state: (generator: ProvideRuntimeState) Supervisor runtime state
func Root(config string, serve bool, state string) error {
	return recordInvocation(fmt.Sprintf("child-root:config=%s,serve=%t,state=%s", config, serve, state))
}

// ServicesList is a subcommand `downloadsd services list` List configured services.
//
// Flags:
//
//	config: --config (from parent)
//	all: --all (default: false) Include inactive services
//	filter: @1 (default: "active") Service filter pattern
func ServicesList(config string, all bool, filter string) error {
	return recordInvocation(fmt.Sprintf("services-list:config=%s,all=%t,filter=%s", config, all, filter))
}

// QueueList is a subcommand `downloadsd queue list` List queued downloads.
//
// Flags:
//
//	config: --config (from parent)
//	limit: --limit (default: 10) Maximum queue items
//	target: @1 Queue target name
//	extra: 0...5 Additional queue filters
func QueueList(config string, limit int, target string, extra ...string) error {
	return recordInvocation(fmt.Sprintf("queue-list:config=%s,limit=%d,target=%s,extra=%v", config, limit, target, extra))
}
