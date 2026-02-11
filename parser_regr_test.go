package go_subcommand

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

//go:embed testdata/parser_regr/*.txtar
var parserRegrFS embed.FS

func TestParserRegression(t *testing.T) {
	dirEntries, err := parserRegrFS.ReadDir("testdata/parser_regr")
	if err != nil {
		t.Fatalf("failed to read testdata dir: %v", err)
	}

	for _, entry := range dirEntries {
		if !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			content, err := parserRegrFS.ReadFile("testdata/parser_regr/" + entry.Name())
			if err != nil {
				t.Fatalf("failed to read %s: %v", entry.Name(), err)
			}

			archive := txtar.Parse(content)
			var inputSrc []byte
			var expectedJSON []byte

			for _, f := range archive.Files {
				if f.Name == "input.go" {
					inputSrc = f.Data
				} else if f.Name == "expected.json" {
					expectedJSON = f.Data
				}
			}

			if inputSrc == nil {
				t.Fatalf("input.go not found in %s", entry.Name())
			}
			if expectedJSON == nil {
				t.Fatalf("expected.json not found in %s", entry.Name())
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
			_, _, _, _, gotParams, ok := ParseSubCommandComments(funcDecl.Doc.Text())
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
		})
	}
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
