package go_subcommand

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	goparser "go/parser"
)

func TestFormatSourceComments_NoBlankLine(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "format_test_blank")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	src := `package main

// MyCmd is a subcommand ` + "`" + `test mycmd` + "`" + `
// param verbose (default: false)
// param extra (default: "foo")
// param another (default: "bar")
// Flags:
// 	verbose: -v (default: false)
func MyCmd(verbose bool) {}
`
	filename := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(filename, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	if err := FormatSourceComments(tempDir); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Formatted content:\n%s", string(content))

	// Check for blank line
	if strings.Contains(string(content), "\n\nfunc MyCmd") {
		t.Errorf("Formatted content has blank line before func:\n%s", string(content))
	}

	// Verify parsing
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, filename, content, goparser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "MyCmd" {
			if fn.Doc == nil {
				t.Errorf("MyCmd has no Doc comment after formatting")
			} else {
				found = true
				if !strings.Contains(fn.Doc.Text(), "is a subcommand") {
					t.Errorf("MyCmd Doc text does not contain 'is a subcommand': %q", fn.Doc.Text())
				}
			}
		}
	}
	if !found {
		t.Errorf("MyCmd function not found")
	}
}
