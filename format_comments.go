package go_subcommand

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
	"github.com/arran4/go-subcommand/parsers/commentv1"
)

// FormatSourceComments is a subcommand `gosubc format-source-comments` formats source comments to match gofmt style
//
// Flags:
//   dir: --dir (default: ".") The project root directory containing go.mod
func FormatSourceComments(dir string) error {
	fset := token.NewFileSet()
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "examples" || info.Name() == "testdata" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Read file content
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		f, err := goparser.ParseFile(fset, path, src, goparser.ParseComments)
		if err != nil {
			return err
		}

		modified := false

		for _, decl := range f.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if funcDecl.Doc == nil {
				continue
			}

			// Check if it's a subcommand
			text := funcDecl.Doc.Text()
			if !strings.Contains(text, "is a subcommand `") {
				continue
			}

			// We need to parse parameters to regenerate the flags block
			// We reuse logic similar to ParseGoFile but locally adapted
			if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) == 0 {
				continue
			}

			// Extract existing parameters info to preserve descriptions/defaults not in signature
			// ParseSubCommandComments returns map[name]ParsedParam
			_, _, _, _, parsedParams, _ := commentv1.ParseSubCommandComments(text)

			var params []*model.FunctionParameter
			for _, p := range funcDecl.Type.Params.List {
				for _, name := range p.Names {
					typeName := ""
					isVarArg := false
					switch t := p.Type.(type) {
					case *ast.Ident:
						typeName = t.Name
					case *ast.SelectorExpr:
						if ident, ok := t.X.(*ast.Ident); ok {
							typeName = fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)
						}
					case *ast.Ellipsis:
						isVarArg = true
						if ident, ok := t.Elt.(*ast.Ident); ok {
							typeName = ident.Name
						} else if sel, ok := t.Elt.(*ast.SelectorExpr); ok {
							if ident, ok := sel.X.(*ast.Ident); ok {
								typeName = fmt.Sprintf("%s.%s", ident.Name, sel.Sel.Name)
							}
						}
					}

					fp := &model.FunctionParameter{
						Name:     name.Name,
						Type:     typeName,
						IsVarArg: isVarArg,
					}

					// Merge logic (simplified)
					// Priority: Flags block > Inline > Preceding
					// We only care about description, flags, default

					// 1. Flags block
					if parsed, ok := parsedParams[name.Name]; ok {
						if len(parsed.Flags) > 0 {
							fp.FlagAliases = parsed.Flags
						}
						if parsed.Default != "" {
							fp.Default = parsed.Default
						}
						if parsed.Description != "" {
							fp.Description = parsed.Description
						}
					}

					if len(fp.FlagAliases) == 0 {
						kebab := parsers.ToKebabCase(name.Name)
						if kebab != name.Name {
							fp.FlagAliases = []string{kebab}
						} else {
							fp.FlagAliases = []string{name.Name}
						}
					}

					// Ensure flags are sorted by length then alpha
					sort.Slice(fp.FlagAliases, func(i, j int) bool {
						if len(fp.FlagAliases[i]) != len(fp.FlagAliases[j]) {
							return len(fp.FlagAliases[i]) < len(fp.FlagAliases[j])
						}
						return fp.FlagAliases[i] < fp.FlagAliases[j]
					})

					params = append(params, fp)
				}
			}

			if len(params) == 0 {
				continue
			}

			// Generate new Flags block
			var buf bytes.Buffer
			w := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)
			for _, p := range params {
				// Name:
				namePart := p.Name + ":"
				// Flags:
				flagsPart := ""
				for _, f := range p.FlagAliases {
					prefix := "-"
					if len(f) > 1 {
						prefix = "--"
					}
					flagsPart += prefix + f + " "
				}
				flagsPart = strings.TrimSpace(flagsPart)

				// Default:
				defaultPart := ""
				if p.Default != "" {
					if p.Type == "string" && !strings.HasPrefix(p.Default, "\"") {
						p.Default = fmt.Sprintf("%q", p.Default)
					}
					defaultPart = fmt.Sprintf("(default: %s)", p.Default)
				}

				// Description:
				descPart := p.Description

				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", namePart, flagsPart, defaultPart, descPart); err != nil {
					return err
				}
			}
			if err := w.Flush(); err != nil {
				return err
			}

			newFlagsBlock := buf.String()

			// Replace in existing doc
			lines := strings.Split(text, "\n")
			var newLines []string

			inFlags := false
			flagsInserted := false

			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "param ") || strings.HasPrefix(trimmed, "flag ") {
					continue
				}

				if trimmed == "Flags:" {
					inFlags = true
					// We will insert our new block here
					newLines = append(newLines, "Flags:", "")
					// Append the generated block (which has newlines)
					blockLines := strings.Split(strings.TrimSpace(newFlagsBlock), "\n")
					for _, bl := range blockLines {
						newLines = append(newLines, "\t"+bl)
					}
					flagsInserted = true
					continue
				}

				if inFlags {
					// Check if we are still in flags block
					if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") || trimmed == "" {
						continue // Skip old flags lines
					}
					inFlags = false
				}

				newLines = append(newLines, line)
			}

			// If "Flags:" block wasn't present, append it?
			if !flagsInserted {
				// Find where to insert. Usually at the end.
				if len(newLines) > 0 && newLines[len(newLines)-1] != "" {
					newLines = append(newLines, "")
				}
				newLines = append(newLines, "Flags:", "")
				blockLines := strings.Split(strings.TrimSpace(newFlagsBlock), "\n")
				for _, bl := range blockLines {
					newLines = append(newLines, "\t"+bl)
				}
			}

			// Reconstruct comment
			var newComments []*ast.Comment
			startPos := funcDecl.Doc.Pos() // Keep start pos

			for i, nl := range newLines {
				pos := token.NoPos
				if i == 0 {
					pos = startPos
				}
				c := &ast.Comment{
					Text:  "// " + nl,
					Slash: pos,
				}
				// Fix empty lines to be just "//"
				if strings.TrimSpace(nl) == "" {
					c.Text = "//"
				}
				newComments = append(newComments, c)
			}

			// Verify if content changed
			var oldTextBuilder strings.Builder
			for _, c := range funcDecl.Doc.List {
				oldTextBuilder.WriteString(c.Text)
				oldTextBuilder.WriteString("\n")
			}

			var newTextBuilder strings.Builder
			for _, c := range newComments {
				newTextBuilder.WriteString(c.Text)
				newTextBuilder.WriteString("\n")
			}

			if oldTextBuilder.String() != newTextBuilder.String() {
				// Remove old Doc from f.Comments to avoid duplication/misplacement
				if funcDecl.Doc != nil {
					var newCommentsList []*ast.CommentGroup
					for _, cg := range f.Comments {
						if cg != funcDecl.Doc {
							newCommentsList = append(newCommentsList, cg)
						}
					}
					f.Comments = newCommentsList
				}

				funcDecl.Doc.List = newComments
				// Strip positions from the function declaration to ensure the printer
				// uses default formatting/spacing instead of original line numbers,
				// avoiding insertion of blank lines when comment length changes.
				stripPositions(funcDecl)
				modified = true
			}
		}

		if modified {
			// Write back
			var buf bytes.Buffer
			if err := format.Node(&buf, fset, f); err != nil {
				return err
			}
			return os.WriteFile(path, buf.Bytes(), info.Mode())
		}
		return nil
	})
}
func stripPositions(node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		val := reflect.ValueOf(n)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return true
		}
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			if field.Type() == reflect.TypeOf(token.NoPos) {
				if field.CanSet() {
					field.SetInt(int64(token.NoPos))
				}
			}
		}
		return true
	})
}
