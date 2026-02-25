package commentv1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseParamDetails_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		check    func(*testing.T, ParsedParam)
	}{
		{
			name:  "nested parens in parser",
			input: "(parser: myfunc(true))",
			check: func(t *testing.T, got ParsedParam) {
				if assert.NotNil(t, got.ParserFunc) {
					assert.Equal(t, "myfunc(true)", got.ParserFunc.FunctionName)
				}
			},
		},
		{
			name:  "multiple parens",
			input: "(required) (global)",
			check: func(t *testing.T, got ParsedParam) {
				assert.True(t, got.IsRequired)
				assert.True(t, got.IsPersistent)
			},
		},
		{
			name:  "parens in description",
			input: "some description (with parens)",
			check: func(t *testing.T, got ParsedParam) {
				assert.Contains(t, got.Description, "some description (with parens)")
			},
		},
		{
			name:  "unbalanced parens in description",
			input: "some description (with unbalanced parens",
			check: func(t *testing.T, got ParsedParam) {
				assert.Contains(t, got.Description, "some description (with unbalanced parens")
			},
		},
		{
			name:  "parser with complex args",
			input: `(parser: "fmt".Sprintf("%s", val))`,
			check: func(t *testing.T, got ParsedParam) {
				if assert.NotNil(t, got.ParserFunc) {
					// The current parser splits by last dot for package.
					// "fmt".Sprintf("%s", val) -> pkg: "fmt", func: Sprintf("%s", val)
					// Let's see what happens.
					assert.Equal(t, `Sprintf("%s", val)`, got.ParserFunc.FunctionName)
					assert.Equal(t, `fmt`, got.ParserFunc.ImportPath)
				}
			},
		},
		{
			name:  "default with parens",
			input: "(default: foo())",
			check: func(t *testing.T, got ParsedParam) {
				assert.Equal(t, "foo()", got.Default)
			},
		},
		{
			name:  "semicolon in parser args",
			input: `(parser: split("a;b"))`,
			check: func(t *testing.T, got ParsedParam) {
				if assert.NotNil(t, got.ParserFunc) {
					assert.Equal(t, `split("a;b")`, got.ParserFunc.FunctionName)
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
