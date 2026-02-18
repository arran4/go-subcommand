package commentv1

import (
	"go/token"
	"strings"
	"testing"
)

func TestParseGoFile_UnsupportedTypes(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
// It does things.
func MyCmd(
	supported string,
	myMap map[string]int,
) error {
	return nil
}
`
	fset := token.NewFileSet()
	cmdTree := &CommandsTree{
		Commands:    map[string]*CommandTree{},
		PackagePath: "example.com/test",
	}

	// This should return an error, not panic
	err := ParseGoFile(fset, "test.go", "example.com/test", strings.NewReader(src), cmdTree)
	if err == nil {
		t.Fatal("Expected error for unsupported type map[string]int, got nil")
	}
	expectedErr := "unsupported type: *ast.MapType"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected error containing %q, got %q", expectedErr, err.Error())
	}
}

func TestParseGoFile_SlicesAndPointers(t *testing.T) {
	src := `package main

// MyCmd2 is a subcommand ` + "`app mycmd2`" + `
// It does things.
func MyCmd2(
	mySlice []string,
	myPtr *int,
) error {
	return nil
}
`
	fset := token.NewFileSet()
	cmdTree := &CommandsTree{
		Commands:    map[string]*CommandTree{},
		PackagePath: "example.com/test",
	}

	// This should succeed if slices and pointers are supported
	err := ParseGoFile(fset, "test2.go", "example.com/test", strings.NewReader(src), cmdTree)
	if err != nil {
		t.Fatalf("Expected no error for slices and pointers, got: %v", err)
	}

	cmd := cmdTree.Commands["app"]
	subCmd := cmd.SubCommandTree.SubCommands["mycmd2"].SubCommand

	foundSlice := false
	foundPtr := false

	for _, p := range subCmd.Parameters {
		if p.Name == "mySlice" {
			foundSlice = true
			if p.Type != "[]string" {
				t.Errorf("Expected type []string, got %s", p.Type)
			}
		}
		if p.Name == "myPtr" {
			foundPtr = true
			if p.Type != "*int" {
				t.Errorf("Expected type *int, got %s", p.Type)
			}
		}
	}

	if !foundSlice {
		t.Error("Did not find mySlice parameter")
	}
	if !foundPtr {
		t.Error("Did not find myPtr parameter")
	}
}
