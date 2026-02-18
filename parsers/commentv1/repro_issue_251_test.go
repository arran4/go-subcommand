package commentv1

import (
	"testing"
	"testing/fstest"
	"github.com/arran4/go-subcommand/model"
)

func TestIssue251_CollisionAndHierarchyNaming(t *testing.T) {
	fs := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/test\n\ngo 1.21\n")},
		"main.go": {Data: []byte(`
package main

// Cmd1 is a subcommand ` + "`app foo-bar`" + `
func Cmd1() {}

// Cmd2 is a subcommand ` + "`app foo_bar`" + `
func Cmd2() {}

// MyChild is a subcommand ` + "`app nested child`" + `
func MyChild() {}

// Bottom is a subcommand ` + "`app nested deep bottom`" + `
func Bottom() {}
`)},
	}

	dataModel, err := ParseGoFiles(fs, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles failed: %v", err)
	}

	if len(dataModel.Commands) != 1 {
		t.Fatalf("Expected 1 root command, got %d", len(dataModel.Commands))
	}

	root := dataModel.Commands[0]

	// Check generated struct names
	structNames := make(map[string]string)

	var checkSubCommands func(subs []*model.SubCommand)
	checkSubCommands = func(subs []*model.SubCommand) {
		for _, sc := range subs {
			t.Logf("SubCommand: Name=%s, StructName=%s, Function=%s", sc.SubCommandName, sc.SubCommandStructName, sc.SubCommandFunctionName)
			structNames[sc.SubCommandName] = sc.SubCommandStructName
			if len(sc.SubCommands) > 0 {
				checkSubCommands(sc.SubCommands)
			}
		}
	}
	checkSubCommands(root.SubCommands)

	// 1. Verify collision resolution (foo-bar vs foo_bar)
	// Parent is nil, so StructName = FuncName
	if structNames["foo-bar"] != "Cmd1" {
		t.Errorf("Expected StructName 'Cmd1' for 'foo-bar', got '%s'", structNames["foo-bar"])
	}
	if structNames["foo_bar"] != "Cmd2" {
		t.Errorf("Expected StructName 'Cmd2' for 'foo_bar', got '%s'", structNames["foo_bar"])
	}

	// 2. Verify hierarchy prefixing
	// nested -> Nested (Synthetic, Parent nil)
	if structNames["nested"] != "Nested" {
		t.Errorf("Expected StructName 'Nested' for 'nested', got '%s'", structNames["nested"])
	}

	// child -> NestedMyChild (Func MyChild, Parent Nested)
	if structNames["child"] != "NestedMyChild" {
		t.Errorf("Expected StructName 'NestedMyChild' for 'child', got '%s'", structNames["child"])
	}

	// deep -> NestedDeep (Synthetic, Parent Nested)
	if structNames["deep"] != "NestedDeep" {
		t.Errorf("Expected StructName 'NestedDeep' for 'deep', got '%s'", structNames["deep"])
	}

	// bottom -> NestedDeepBottom (Func Bottom, Parent NestedDeep)
	if structNames["bottom"] != "NestedDeepBottom" {
		t.Errorf("Expected StructName 'NestedDeepBottom' for 'bottom', got '%s'", structNames["bottom"])
	}
}
