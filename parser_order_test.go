package go_subcommand

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoFiles_Order(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module example.com/test\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mainGoPath := filepath.Join(tmpDir, "main.go")
	mainGoContent := `
package main

// CmdA is a subcommand ` + "`app cmd-a`" + `
func CmdA() {}

// CmdB is a subcommand ` + "`app cmd-b`" + `
func CmdB() {}

// CmdC is a subcommand ` + "`app cmd-c`" + `
func CmdC() {}
`

	// Run multiple times to detect instability
	for i := 0; i < 20; i++ {
		files := []File{
			{
				Path:   mainGoPath,
				Reader: strings.NewReader(mainGoContent),
			},
		}

		model, err := ParseGoFiles(tmpDir, files...)
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
