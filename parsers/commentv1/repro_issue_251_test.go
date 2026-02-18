package commentv1

import (
	"testing"
	"testing/fstest"
)

func TestIssue251_Collision(t *testing.T) {
	fs := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/test\n\ngo 1.21\n")},
		"main.go": {Data: []byte(`
package main

// Cmd1 is a subcommand ` + "`app foo-bar`" + `
func Cmd1() {}

// Cmd2 is a subcommand ` + "`app foo_bar`" + `
func Cmd2() {}
`)},
	}

	model, err := ParseGoFiles(fs, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles failed: %v", err)
	}

	if len(model.Commands) != 1 {
		t.Fatalf("Expected 1 root command, got %d", len(model.Commands))
	}

	root := model.Commands[0]
	if root.MainCmdName != "app" {
		t.Errorf("Expected root command 'app', got '%s'", root.MainCmdName)
	}

	if len(root.SubCommands) != 2 {
		t.Fatalf("Expected 2 subcommands, got %d", len(root.SubCommands))
	}

	// Check generated struct names
	structNames := make(map[string]string)
	for _, sc := range root.SubCommands {
		t.Logf("SubCommand: Name=%s, StructName=%s, Function=%s", sc.SubCommandName, sc.SubCommandStructName, sc.SubCommandFunctionName)
		structNames[sc.SubCommandName] = sc.SubCommandStructName
	}

	if structNames["foo-bar"] == structNames["foo_bar"] {
		t.Errorf("Struct names collision: %s", structNames["foo-bar"])
	}

	if structNames["foo-bar"] != "Cmd1" {
		t.Errorf("Expected StructName 'Cmd1' for 'foo-bar', got '%s'", structNames["foo-bar"])
	}
	if structNames["foo_bar"] != "Cmd2" {
		t.Errorf("Expected StructName 'Cmd2' for 'foo_bar', got '%s'", structNames["foo_bar"])
	}
}
