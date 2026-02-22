package commentv1

import (
	"testing"

	"github.com/arran4/go-subcommand/model"
)

func TestSubCommandTree_Insert(t *testing.T) {
	tests := []struct {
		name             string
		importPath       string
		packageName      string
		sequence         []string
		expectedLeafName string
	}{
		{
			name:             "Insert Root Level",
			importPath:       "example.com/root",
			packageName:      "main",
			sequence:         []string{},
			expectedLeafName: "root",
		},
		{
			name:             "Insert Child",
			importPath:       "example.com/child",
			packageName:      "childpkg",
			sequence:         []string{"child"},
			expectedLeafName: "child",
		},
		{
			name:             "Insert Nested Child",
			importPath:       "example.com/parent/child",
			packageName:      "childpkg",
			sequence:         []string{"parent", "child"},
			expectedLeafName: "child",
		},
		{
			name:             "Insert Deeply Nested Child",
			importPath:       "example.com/grandparent/parent/child",
			packageName:      "childpkg",
			sequence:         []string{"grandparent", "parent", "child"},
			expectedLeafName: "child",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sct := NewSubCommandTree(nil)
			sc := &model.SubCommand{
				SubCommandName: tt.expectedLeafName,
			}

			sct.Insert(tt.importPath, tt.packageName, tt.sequence, sc)

			// Traverse the tree to find the inserted node
			current := sct
			for _, part := range tt.sequence {
				next, ok := current.SubCommands[part]
				if !ok {
					t.Fatalf("Expected subcommand '%s' in sequence %v, but it was missing", part, tt.sequence)
				}
				current = next
			}

			if current.SubCommand != sc {
				t.Errorf("Expected SubCommand to be set at the leaf node")
			}

			// Fix: QF1008: could remove embedded field "SubCommand" from selector (staticcheck)
			if current.ImportPath != tt.importPath {
				t.Errorf("Expected ImportPath '%s', got '%s'", tt.importPath, current.ImportPath)
			}

			if current.SubCommandPackageName != tt.packageName {
				t.Errorf("Expected SubCommandPackageName '%s', got '%s'", tt.packageName, current.SubCommandPackageName)
			}
		})
	}
}

func TestSubCommandTree_Insert_Merge(t *testing.T) {
	t.Run("Parent then Child", func(t *testing.T) {
		sct := NewSubCommandTree(nil)

		parentSC := &model.SubCommand{SubCommandName: "parent"}
		sct.Insert("pkg/parent", "parent", []string{"parent"}, parentSC)

		childSC := &model.SubCommand{SubCommandName: "child"}
		sct.Insert("pkg/child", "child", []string{"parent", "child"}, childSC)

		// Verify parent
		parentNode, ok := sct.SubCommands["parent"]
		if !ok {
			t.Fatal("Parent node missing")
		}
		if parentNode.SubCommand != parentSC {
			t.Error("Parent subcommand mismatch")
		}

		// Verify child
		childNode, ok := parentNode.SubCommands["child"]
		if !ok {
			t.Fatal("Child node missing")
		}
		if childNode.SubCommand != childSC {
			t.Error("Child subcommand mismatch")
		}
	})

	t.Run("Child then Parent", func(t *testing.T) {
		sct := NewSubCommandTree(nil)

		childSC := &model.SubCommand{SubCommandName: "child"}
		// This creates intermediate "parent" node with nil SubCommand
		sct.Insert("pkg/child", "child", []string{"parent", "child"}, childSC)

		parentSC := &model.SubCommand{SubCommandName: "parent"}
		// This should update the existing "parent" node
		sct.Insert("pkg/parent", "parent", []string{"parent"}, parentSC)

		// Verify parent
		parentNode, ok := sct.SubCommands["parent"]
		if !ok {
			t.Fatal("Parent node missing")
		}
		if parentNode.SubCommand != parentSC {
			t.Error("Parent subcommand mismatch - likely not updated correctly")
		}

		// Verify child still exists
		childNode, ok := parentNode.SubCommands["child"]
		if !ok {
			t.Fatal("Child node missing")
		}
		if childNode.SubCommand != childSC {
			t.Error("Child subcommand mismatch")
		}
	})
}
