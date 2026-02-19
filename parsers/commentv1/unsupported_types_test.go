package commentv1

import (
	"go/token"
	"strings"
	"testing"
)

func TestParseGoFile_UnsupportedTypes(t *testing.T) {
	// Case 1: Map (Should return error, not panic)
	t.Run("MapType", func(t *testing.T) {
		src := `package mypkg

// MyCmd is a subcommand ` + "`app cmd`" + `
func MyCmd(m map[string]int) {}
`
		fset := token.NewFileSet()
		cmdTree := &CommandsTree{Commands: make(map[string]*CommandTree)}

		err := ParseGoFile(fset, "test.go", "main", strings.NewReader(src), cmdTree)
		if err == nil {
			t.Fatal("Expected error for unsupported type (map), got nil")
		}
		if !strings.Contains(err.Error(), "unsupported type") && !strings.Contains(err.Error(), "Unsupported type") {
			t.Errorf("Expected 'unsupported type' error, got: %v", err)
		}
	})

	// Case 2: Pointer (Should be supported)
	t.Run("PointerType", func(t *testing.T) {
		src := `package mypkg

// MyCmd is a subcommand ` + "`app cmd`" + `
func MyCmd(p *int) {}
`
		fset := token.NewFileSet()
		cmdTree := &CommandsTree{Commands: make(map[string]*CommandTree)}

		err := ParseGoFile(fset, "test.go", "main", strings.NewReader(src), cmdTree)
		if err != nil {
			t.Errorf("Expected success for pointer type, got error: %v", err)
		}
	})
}
