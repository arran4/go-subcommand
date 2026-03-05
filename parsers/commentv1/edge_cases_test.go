package commentv1

import (
	"strings"
	"testing"
)

func TestParseParamDetails_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(*testing.T, ParsedParam)
	}{
		{
			name:  "nested parens in parser",
			input: "(parser: myfunc(true))",
			check: func(t *testing.T, got ParsedParam) {
				if got.Parser.Func == nil {
					t.Fatal("ParserFunc is nil")
				}
				if got.Parser.Func.FunctionName != "myfunc(true)" {
					t.Errorf("expected FunctionName 'myfunc(true)', got '%s'", got.Parser.Func.FunctionName)
				}
			},
		},
		{
			name:  "multiple parens",
			input: "(required) (global)",
			check: func(t *testing.T, got ParsedParam) {
				if !got.Required {
					t.Error("expected IsRequired to be true")
				}
				// Global tag logic handled separately but check if it was consumed
				if strings.Contains(got.Description, "global") {
					t.Error("expected 'global' to be consumed")
				}
			},
		},
		{
			name:  "parens in description",
			input: "some description (with parens)",
			check: func(t *testing.T, got ParsedParam) {
				if !strings.Contains(got.Description, "some description (with parens)") {
					t.Errorf("expected description to contain 'some description (with parens)', got '%s'", got.Description)
				}
			},
		},
		{
			name:  "unbalanced parens in description",
			input: "some description (with unbalanced parens",
			check: func(t *testing.T, got ParsedParam) {
				if !strings.Contains(got.Description, "some description (with unbalanced parens") {
					t.Errorf("expected description to contain 'some description (with unbalanced parens', got '%s'", got.Description)
				}
			},
		},
		{
			name:  "parser with complex args",
			input: `(parser: "fmt".Sprintf("%s", val))`,
			check: func(t *testing.T, got ParsedParam) {
				if got.Parser.Func == nil {
					t.Fatal("ParserFunc is nil")
				}
				// The current parser splits by last dot for package.
				// "fmt".Sprintf("%s", val) -> pkg: "fmt", func: Sprintf("%s", val)
				// Let's see what happens.
				if got.Parser.Func.FunctionName != `Sprintf("%s", val)` {
					t.Errorf("expected FunctionName `Sprintf(\"%%s\", val)`, got `%s`", got.Parser.Func.FunctionName)
				}
				if got.Parser.Func.ImportPath != `fmt` {
					t.Errorf("expected ImportPath `fmt`, got `%s`", got.Parser.Func.ImportPath)
				}
			},
		},
		{
			name:  "default with parens",
			input: "(default: foo())",
			check: func(t *testing.T, got ParsedParam) {
				if got.Default != "foo()" {
					t.Errorf("expected Default 'foo()', got '%s'", got.Default)
				}
			},
		},
		{
			name:  "semicolon in parser args",
			input: `(parser: split("a;b"))`,
			check: func(t *testing.T, got ParsedParam) {
				if got.Parser.Func == nil {
					t.Fatal("ParserFunc is nil")
				}
				if got.Parser.Func.FunctionName != `split("a;b")` {
					t.Errorf("expected FunctionName `split(\"a;b\")`, got `%s`", got.Parser.Func.FunctionName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseParamDetails(tt.input)
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
