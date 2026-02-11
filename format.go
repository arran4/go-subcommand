package go_subcommand

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Format is a subcommand `gosubc format` that formats the subcommand code comments
// param dir (default: ".") Project root directory containing go.mod
func Format(dir string) error {
	return FormatWithFS(os.DirFS(dir), dir)
}

func FormatWithFS(fsys fs.FS, root string) error {
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "testdata" || d.Name() == "vendor" {
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

		// Parse file
		fileNode, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return err
		}

		var replacements []replacement

		for _, decl := range fileNode.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Doc == nil {
				continue
			}

			// Check if it is a subcommand
			if !isSubCommand(fn.Doc.Text()) {
				continue
			}

			// Get parameter types
			paramTypes := make(map[string]string)
			if fn.Type.Params != nil {
				for _, p := range fn.Type.Params.List {
					typeName := ""
					switch t := p.Type.(type) {
					case *ast.Ident:
						typeName = t.Name
					case *ast.SelectorExpr:
						if ident, ok := t.X.(*ast.Ident); ok {
							typeName = fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)
						}
					case *ast.Ellipsis:
						if ident, ok := t.Elt.(*ast.Ident); ok {
							typeName = ident.Name
						}
					}
					for _, name := range p.Names {
						paramTypes[name.Name] = typeName
					}
				}
			}

			if r := formatFlagsBlock(fset, fn.Doc, paramTypes); r != nil {
				replacements = append(replacements, *r)
			}
		}

		if len(replacements) > 0 {
			// Apply replacements in reverse order
			sort.Slice(replacements, func(i, j int) bool {
				return replacements[i].start > replacements[j].start
			})

			newSrc := src
			for _, r := range replacements {
				// Safety check
				if r.start > len(newSrc) || r.end > len(newSrc) {
					continue
				}
				newSrc = append(newSrc[:r.start], append([]byte(r.text), newSrc[r.end:]...)...)
			}

			// Format the source code using go/format
			formattedSrc, err := format.Source(newSrc)
			if err != nil {
				return fmt.Errorf("failed to format source for %s: %w", path, err)
			}

			if err := os.WriteFile(path, formattedSrc, 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", path, err)
			}
			fmt.Printf("Formatted %s\n", path)
		}
		return nil
	})
	return err
}

type replacement struct {
	start int
	end   int
	text  string
}

func isSubCommand(text string) bool {
	return strings.Contains(text, "is a subcommand `")
}

func formatFlagsBlock(fset *token.FileSet, doc *ast.CommentGroup, paramTypes map[string]string) *replacement {
	var startIdx, endIdx int
	foundFlags := false

	// Locate "Flags:" line
	for i, comment := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		if text == "Flags:" {
			startIdx = i
			foundFlags = true
			break
		}
	}

	if !foundFlags {
		return nil
	}

	// Identify block range
	endIdx = startIdx
	// Find the end of the indented block
	// Look ahead from startIdx + 1
	for i := startIdx + 1; i < len(doc.List); i++ {
		text := doc.List[i].Text
		content := strings.TrimPrefix(text, "//")

		// If line is empty comment "//", it's part of the block (spacer)
		if len(strings.TrimSpace(content)) == 0 {
			endIdx = i
			continue
		}

		// If line is indented, it's part of the block
		if strings.HasPrefix(content, "\t") || strings.HasPrefix(content, " ") {
			endIdx = i
			continue
		}

		// If line is NOT indented and NOT empty, block ends
		break
	}

	// Collect lines to parse
	var params []parsedFlagLine

	for i := startIdx + 1; i <= endIdx; i++ {
		line := doc.List[i].Text
		content := strings.TrimPrefix(line, "//")
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			continue
		}

		// Parse "name: details"
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			detailsStr := strings.TrimSpace(parts[1])

			// Parse details using parser.go logic
			details := parseParamDetails(detailsStr)

			// Infer type if needed
			pType := paramTypes[name]

			params = append(params, parsedFlagLine{
				Name:        name,
				Details:     details,
				OriginalStr: detailsStr,
				Type:        pType,
			})
		} else {
			// Handle continuation or unparsed line?
			// For now, assume it's part of previous param description or ignore
			if len(params) > 0 {
				params[len(params)-1].Details.Description += " " + trimmed
			}
		}
	}

	if len(params) == 0 {
		return nil
	}

	// Format lines
	var formattedLines []string
	formattedLines = append(formattedLines, "// Flags:")
	formattedLines = append(formattedLines, "//") // Empty spacer line

	// Calculate widths
	maxNameLen := 0
	maxFlagsLen := 0
	maxDefaultLen := 0

	for i := range params {
		p := &params[i]
		if len(p.Name) > maxNameLen {
			maxNameLen = len(p.Name)
		}
		// Add dashes
		var flagParts []string
		for _, f := range p.Details.Flags {
			prefix := "-"
			if len(f) > 1 {
				prefix = "--"
			}
			flagParts = append(flagParts, prefix+f)
		}
		// Sort by length
		sort.Slice(flagParts, func(i, j int) bool {
			return len(flagParts[i]) < len(flagParts[j])
		})
		p.FlagsStr = strings.Join(flagParts, " ")

		if len(p.FlagsStr) > maxFlagsLen {
			maxFlagsLen = len(p.FlagsStr)
		}

		// Format default
		defStr := ""
		if p.Details.Default != "" {
			val := p.Details.Default
			// Quote if string
			if p.Type == "string" && !strings.HasPrefix(val, "\"") {
				val = fmt.Sprintf("%q", val)
			}
			defStr = fmt.Sprintf("(default: %s)", val)
		}
		p.DefaultStr = defStr
		if len(defStr) > maxDefaultLen {
			maxDefaultLen = len(defStr)
		}
	}

	// Add colon to maxNameLen
	maxNameLen++ // for ":"

	for _, p := range params {
		// Align
		// Name:
		nameField := p.Name + ":"

		// Line format: //\tName: Flags Default Description
		// Use tab for indentation?
		// User example: //\tpadCommits: ...

		var sb strings.Builder
		sb.WriteString("//\t")
		sb.WriteString(fmt.Sprintf("%-*s", maxNameLen, nameField))

		if p.FlagsStr != "" {
			sb.WriteString(" ")
			sb.WriteString(fmt.Sprintf("%-*s", maxFlagsLen, p.FlagsStr))
		} else if maxFlagsLen > 0 {
			sb.WriteString(" ")
			sb.WriteString(strings.Repeat(" ", maxFlagsLen))
		}

		if p.DefaultStr != "" {
			sb.WriteString(" ")
			sb.WriteString(fmt.Sprintf("%-*s", maxDefaultLen, p.DefaultStr))
		} else if maxDefaultLen > 0 {
			sb.WriteString(" ")
			sb.WriteString(strings.Repeat(" ", maxDefaultLen))
		}

		if p.Details.Description != "" {
			sb.WriteString(" ")
			sb.WriteString(p.Details.Description)
		}

		formattedLines = append(formattedLines, sb.String())
	}

	// Join lines
	newText := strings.Join(formattedLines, "\n")

	// Calculate range
	startPos := fset.Position(doc.List[startIdx].Pos()).Offset
	endPos := fset.Position(doc.List[endIdx].End()).Offset

	return &replacement{
		start: startPos,
		end:   endPos,
		text:  newText,
	}
}

type parsedFlagLine struct {
	Name        string
	Details     ParsedParam
	OriginalStr string
	Type        string
	FlagsStr    string
	DefaultStr  string
}
