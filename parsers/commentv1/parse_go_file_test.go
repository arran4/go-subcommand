package commentv1

import (
	"go/token"
	"strings"
	"testing"
)

func TestParseGoFile_Implicit(t *testing.T) {
	src := `package main

// Parent is a subcommand that Does work in a directory
func Parent(dir string) {}
`
	fset := token.NewFileSet()
	cmdTree := &CommandsTree{
		Commands:    make(map[string]*CommandTree),
		PackagePath: "example.com/test",
	}

	err := ParseGoFile(fset, "test.go", "example.com/test", strings.NewReader(src), cmdTree)
	if err != nil {
		t.Fatalf("ParseGoFile failed: %v", err)
	}

	// Expect command "parent"
	if _, ok := cmdTree.Commands["parent"]; !ok {
		t.Errorf("Expected command 'parent' to be created, but got keys: %v", getKeys(cmdTree.Commands))
	} else {
		ct := cmdTree.Commands["parent"]
		if ct.Description != "Does work in a directory" {
			t.Errorf("Expected description 'Does work in a directory', got '%s'", ct.Description)
		}
	}
}

func getKeys(m map[string]*CommandTree) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
