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

func TestFlagAliases_Order_Stability(t *testing.T) {
	// Scenario 1: -n -name
	fsys1 := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`
package main

// MyCmd is a subcommand ` + "`app cmd`" + `
// param Name: -n -name Your name
func MyCmd(Name string) {}
`)},
	}

	// Scenario 2: -name -n
	fsys2 := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`
package main

// MyCmd is a subcommand ` + "`app cmd`" + `
// param Name: -name -n Your name
func MyCmd(Name string) {}
`)},
	}

	model1, err := ParseGoFiles(fsys1, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles 1 failed: %v", err)
	}
	aliases1 := model1.Commands[0].SubCommands[0].Parameters[0].FlagAliases

	model2, err := ParseGoFiles(fsys2, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles 2 failed: %v", err)
	}
	aliases2 := model2.Commands[0].SubCommands[0].Parameters[0].FlagAliases

	// We expect aliases to be sorted, so they should be identical regardless of input order
	if !slicesEqual(aliases1, aliases2) {
		t.Errorf("Aliases order mismatch. Run 1: %v, Run 2: %v. Expected them to be identical (sorted).", aliases1, aliases2)
	}
}

func TestFlagAliases_Order_Stability_FlagsBlock(t *testing.T) {
	// Scenario 1: -n -name inside Flags block
	fsys1 := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`
package main

// MyCmd is a subcommand ` + "`app cmd`" + `
// Flags:
//   Name: -n -name Your name
func MyCmd(Name string) {}
`)},
	}

	// Scenario 2: -name -n inside Flags block
	fsys2 := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(`
package main

// MyCmd is a subcommand ` + "`app cmd`" + `
// Flags:
//   Name: -name -n Your name
func MyCmd(Name string) {}
`)},
	}

	model1, err := ParseGoFiles(fsys1, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles 1 failed: %v", err)
	}
	aliases1 := model1.Commands[0].SubCommands[0].Parameters[0].FlagAliases

	model2, err := ParseGoFiles(fsys2, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles 2 failed: %v", err)
	}
	aliases2 := model2.Commands[0].SubCommands[0].Parameters[0].FlagAliases

	// We expect aliases to be sorted (longest first)
	if !slicesEqual(aliases1, aliases2) {
		t.Errorf("Aliases order mismatch. Run 1: %v, Run 2: %v. Expected them to be identical (sorted).", aliases1, aliases2)
	}

	expected := []string{"name", "n"}
	if !slicesEqual(aliases1, expected) {
		t.Errorf("Aliases order mismatch. Got: %v, Expected: %v", aliases1, expected)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
