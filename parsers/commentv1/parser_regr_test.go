package commentv1

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"testing"

	"github.com/arran4/go-subcommand/parsers"
	"golang.org/x/tools/txtar"
)

//go:embed testdata/parser_regr/*.txtar
var parserRegrFS embed.FS

func TestParserRegression(t *testing.T) {
	// Handler for "commentv1 regression tests"
	runRegressionTest := func(t *testing.T, archive *txtar.Archive) {
		var inputSrc []byte
		var expectedJSON []byte

		for _, f := range archive.Files {
			switch f.Name {
			case "input.go":
				inputSrc = f.Data
			case "expected.json":
				expectedJSON = f.Data
			}
		}

		if inputSrc == nil {
			t.Fatalf("input.go not found")
		}
		if expectedJSON == nil {
			t.Fatalf("expected.json not found")
		}

		// 1. Verify input source is gofmt-compliant
		formattedSrc, err := format.Source(inputSrc)
		if err != nil {
			t.Fatalf("Failed to format source: %v", err)
		}

		if !bytes.Equal(bytes.TrimSpace(formattedSrc), bytes.TrimSpace(inputSrc)) {
			t.Errorf("input.go is not gofmt compliant. Please run gofmt on the txtar content.\nDiff:\n%s",
				diff(string(inputSrc), string(formattedSrc)))
		}

		// 2. Parse the formatted source
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "", formattedSrc, parser.ParseComments)
		if err != nil {
			t.Fatalf("Failed to parse source: %v", err)
		}

		// Find the function doc comment
		// Assuming there is at least one function
		var funcDecl *ast.FuncDecl
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				if fn.Doc != nil {
					funcDecl = fn
					break
				}
			}
		}

		if funcDecl == nil {
			t.Fatalf("No function with doc comment found in input.go")
		}

		// 3. Extract params using the parser
		_, _, _, _, _, gotParams, ok := ParseSubCommandComments(funcDecl.Doc.Text())
		if !ok {
			t.Fatalf("ParseSubCommandComments returned !ok")
		}

		// 4. Compare against expected JSON
		var expectedParams map[string]ParsedParam
		if err := json.Unmarshal(expectedJSON, &expectedParams); err != nil {
			t.Fatalf("Failed to unmarshal expected.json: %v", err)
		}

		for name, expected := range expectedParams {
			got, found := gotParams[name]
			if !found {
				t.Errorf("Expected param %q not found", name)
				continue
			}

			if !compareParams(got, expected) {
				t.Errorf("Param %q mismatch.\nGot:      %+v\nExpected: %+v", name, got, expected)
			}
		}
		// Also check for extra params
		for name := range gotParams {
			if _, found := expectedParams[name]; !found {
				t.Errorf("Unexpected param %q found", name)
			}
		}
	}

	parsers.RunTxtarTests(t, parserRegrFS, "testdata/parser_regr", map[string]func(*testing.T, *txtar.Archive){
		"commentv1 regression tests": runRegressionTest,
	}, runRegressionTest)
}

func compareParams(got, expected ParsedParam) bool {
	if got.Default != expected.Default {
		return false
	}
	if got.Description != expected.Description {
		return false
	}
	if got.IsPositional != expected.IsPositional {
		return false
	}
	if got.PositionalArgIndex != expected.PositionalArgIndex {
		return false
	}
	if got.IsVarArg != expected.IsVarArg {
		return false
	}
	if got.VarArgMin != expected.VarArgMin {
		return false
	}
	if got.VarArgMax != expected.VarArgMax {
		return false
	}

	if len(got.Flags) != len(expected.Flags) {
		return false
	}
	for i, f := range got.Flags {
		if f != expected.Flags[i] {
			return false
		}
	}

	return true
}

func diff(a, b string) string {
	// Simple diff for error reporting
	if a == b {
		return ""
	}
	return fmt.Sprintf("Original:\n%q\nFormatted:\n%q", a, b)
}
