package go_subcommand

import (
	"testing"
	"testing/fstest"
)

func TestParseGoFiles_Order(t *testing.T) {
	// In-memory FS
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`
package main

// CmdA is a subcommand ` + "`app cmd-a`" + `
func CmdA() {}

// CmdB is a subcommand ` + "`app cmd-b`" + `
func CmdB() {}

// CmdC is a subcommand ` + "`app cmd-c`" + `
func CmdC() {}
`)},
	}

	// Run multiple times to detect instability
	for i := 0; i < 20; i++ {
		// Use ParseGoFiles directly with FS
		model, err := ParseGoFiles(fsys, ".")
		if err != nil {
			t.Fatalf("ParseGoFiles failed: %v", err)
		}

		if len(model.Commands) != 1 {
			t.Fatalf("Expected 1 command, got %d", len(model.Commands))
		}

		cmd := model.Commands[0]
		if len(cmd.SubCommands) != 3 {
			t.Fatalf("Expected 3 subcommands, got %d", len(cmd.SubCommands))
		}

		// Check order
		expected := []string{"cmd-a", "cmd-b", "cmd-c"}
		for j, sub := range cmd.SubCommands {
			if sub.SubCommandName != expected[j] {
				t.Fatalf("Iteration %d: Expected subcommand at index %d to be %s, got %s. Order was: %v",
					i, j, expected[j], sub.SubCommandName, getSubCommandNames(cmd.SubCommands))
			}
		}
	}
}

func getSubCommandNames(subs []*SubCommand) []string {
	var names []string
	for _, s := range subs {
		names = append(names, s.SubCommandName)
	}
	return names
}
