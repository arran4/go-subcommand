package commentv1

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/arran4/go-subcommand/model"
	"golang.org/x/tools/txtar"
)

func TestCircularParsing(t *testing.T) {
	// Walk over txtar files in testdata recursively
	err := filepath.WalkDir("testdata", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".txtar") {
			return nil
		}

		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}
			archive := txtar.Parse(data)

			var inputComment string
			for _, f := range archive.Files {
				if f.Name == "input.comment" {
					inputComment = string(f.Data)
					break
				}
			}

			if inputComment == "" {
				// Skip tests without input comment
				return
			}

			// 1. Parse initial comment
			cmdName, subCommandSequence, description, extendedHelp, aliases, params, ok := ParseSubCommandComments(inputComment)
			if !ok {
				t.Logf("Skipping %s: not a subcommand comment", d.Name())
				return
			}

			// 2. Construct Model
			sc := &model.SubCommand{
				Command: &model.Command{
					MainCmdName: cmdName,
				},
				SubCommandDescription:  description,
				SubCommandExtendedHelp: extendedHelp,
				Aliases:                aliases,
				SubCommandFunctionName: "DummyFunc", // Dummy function name
			}

			// Build parent chain
			if len(subCommandSequence) > 0 {
				sc.SubCommandName = subCommandSequence[len(subCommandSequence)-1]
				current := sc
				for i := len(subCommandSequence) - 2; i >= 0; i-- {
					parent := &model.SubCommand{
						Command: &model.Command{
							MainCmdName: cmdName,
						},
						SubCommandName: subCommandSequence[i],
					}
					current.Parent = parent
					current = parent
				}
			} else {
				sc.SubCommandName = cmdName
			}

			// Convert params
			var funcParams []*model.FunctionParameter
			for name, p := range params {
				fp := &model.FunctionParameter{
					Name:               name,
					Description:        p.Description,
					Default:            p.Default,
					IsPositional:       p.IsPositional,
					PositionalArgIndex: p.PositionalArgIndex,
					IsVarArg:           p.IsVarArg,
					VarArgMin:          p.VarArgMin,
					VarArgMax:          p.VarArgMax,
					IsRequired:         p.IsRequired,
					FlagAliases:        p.Flags,
					Parser:             p.Parser,
					Generator:          p.Generator,
					DeclaredIn:         sc.SubCommandName, // Assume local for this test unless Inherited
				}
				if p.Inherited {
					// Logic for finding parent name
					if sc.Parent != nil {
						fp.DeclaredIn = sc.Parent.SubCommandName
					} else {
						fp.DeclaredIn = "" // Unknown parent
					}
				}
				funcParams = append(funcParams, fp)
			}
			sc.Parameters = funcParams

			// 3. Generate Comment
			parser := &CommentParser{}
			generatedComment, err := parser.Format(sc)
			if err != nil {
				t.Fatalf("failed to format subcommand: %v", err)
			}

			// 4. Parse Generated Comment
			// ParseSubCommandComments expects text content (like s.Doc.Text()), not raw comment with //
			cleanComment := stripCommentMarkers(generatedComment)
			cmdName2, subCommandSequence2, description2, extendedHelp2, aliases2, params2, ok2 := ParseSubCommandComments(cleanComment)

			if !ok2 {
				t.Fatalf("Generated comment failed to parse as subcommand:\n%s\nCleaned:\n%s", generatedComment, cleanComment)
			}

			// 5. Compare Metadata

			if cmdName != cmdName2 {
				t.Errorf("CmdName mismatch: got %q, want %q", cmdName2, cmdName)
			}
			if !slicesEqual(subCommandSequence, subCommandSequence2) {
				t.Errorf("Sequence mismatch: got %v, want %v", subCommandSequence2, subCommandSequence)
			}
			// Description might differ by whitespace normalization, but parser usually trims.
			if strings.TrimSpace(description) != strings.TrimSpace(description2) {
				t.Errorf("Description mismatch: got %q, want %q", description2, description)
			}
			if strings.TrimSpace(extendedHelp) != strings.TrimSpace(extendedHelp2) {
				t.Errorf("ExtendedHelp mismatch: got %q, want %q", extendedHelp2, extendedHelp)
			}

			// Aliases: sort before compare
			sort.Strings(aliases)
			sort.Strings(aliases2)
			if !slicesEqual(aliases, aliases2) {
				t.Errorf("Aliases mismatch: got %v, want %v", aliases2, aliases)
			}

			// Params comparison
			// Iterate expected params
			for name, p1 := range params {
				p2, exists := params2[name]
				if !exists {
					t.Errorf("Parameter %s missing in reparsed result", name)
					continue
				}

				// Compare fields
				if p1.Description != p2.Description {
					t.Errorf("Param %s Description mismatch: got %q, want %q", name, p2.Description, p1.Description)
				}
				if p1.Default != p2.Default {
					// Handle quoting issue if any, but we expect raw match
					t.Errorf("Param %s Default mismatch: got %q, want %q", name, p2.Default, p1.Default)
				}
				if p1.IsRequired != p2.IsRequired {
					t.Errorf("Param %s IsRequired mismatch", name)
				}
				if p1.IsPositional != p2.IsPositional {
					t.Errorf("Param %s IsPositional mismatch", name)
				}
				if p1.PositionalArgIndex != p2.PositionalArgIndex {
					t.Errorf("Param %s PositionalArgIndex mismatch: %d vs %d", name, p2.PositionalArgIndex, p1.PositionalArgIndex)
				}
				if p1.IsVarArg != p2.IsVarArg {
					t.Errorf("Param %s IsVarArg mismatch", name)
				}

				// Flags
				sort.Strings(p1.Flags)
				sort.Strings(p2.Flags)

				if !slicesEqual(p1.Flags, p2.Flags) {
					t.Errorf("Param %s Flags mismatch: got %v, want %v", name, p2.Flags, p1.Flags)
				}

				// Parser/Generator
				if p1.Parser.Type != p2.Parser.Type {
					t.Errorf("Param %s Parser Type mismatch", name)
				}
				// Check Func names if Custom
			}

			if len(params) != len(params2) {
				t.Errorf("Parameter count mismatch: got %d, want %d", len(params2), len(params))
				// List extra keys
				for k := range params2 {
					if _, ok := params[k]; !ok {
						t.Errorf("Extra param found: %s", k)
					}
				}
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk testdata: %v", err)
	}
}

func stripCommentMarkers(comment string) string {
	var sb strings.Builder
	lines := strings.Split(comment, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "//" {
			sb.WriteString("") // Empty line
		} else if strings.HasPrefix(trimmed, "// ") {
			sb.WriteString(strings.TrimPrefix(trimmed, "// "))
		} else if strings.HasPrefix(trimmed, "//") {
			sb.WriteString(strings.TrimPrefix(trimmed, "//"))
		} else {
			sb.WriteString(line)
		}
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
