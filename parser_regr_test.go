package go_subcommand

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestParseSubCommandCommentsGofmt(t *testing.T) {
	// Initial text that mimics what a user might write
	initialText := `MyFunc is a subcommand ` + "`app cmd`" + `
Flags:

	username: --username -u (default: "guest") The user to greet`

	// Create a dummy source file
	// We use ReplaceAll to prefix lines with "//". Note that empty lines in initialText become "//"
	src := "package main\n\n// " + strings.ReplaceAll(initialText, "\n", "\n// ") + "\nfunc MyFunc() {}"

	// Format it using standard go/format to ensure it matches gofmt behavior
	formattedSrc, err := format.Source([]byte(src))
	if err != nil {
		t.Fatalf("Failed to format source: %v", err)
	}

	// Parse the formatted source to extract the exact comment text
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", formattedSrc, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse formatted source: %v", err)
	}

	// Extract the doc comment from the function
	if len(f.Decls) == 0 {
		t.Fatal("No declarations found")
	}
	funcDecl, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok {
		t.Fatal("Declaration is not a function")
	}

	if funcDecl.Doc == nil {
		t.Fatal("Function has no doc comment after formatting")
	}

	// The Text() method returns the comment text without // markers, preserving the structure
	commentText := funcDecl.Doc.Text()

	// Verify that the comment text still contains the blank line we expect gofmt to preserve/enforce
	if !strings.Contains(commentText, "Flags:\n\n") {
		t.Logf("Formatted comment text does not contain expected blank line after Flags:\n%q", commentText)
	}

	// Now run the parser on the extracted, gofmt-compliant text
	_, _, _, _, gotParams, _ := ParseSubCommandComments(commentText)

	if _, ok := gotParams["username"]; !ok {
		t.Errorf("Failed to parse username parameter from gofmt-formatted comment:\n%s", commentText)
	}
}
