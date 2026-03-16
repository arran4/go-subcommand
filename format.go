package go_subcommand

import (
	"fmt"
	"go/token"
	"os"
	"sort"
	"strings"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
)

// Format is a subcommand `gosubc format` formats the subcommand definitions
// Format updates the documentation comments for subcommands in the codebase
// to match the defined parameters and standard formatting.
//
// Flags:
//
//	dir:		--dir		(default: ".")		The project root directory
//	inplace:	--inplace				Modify files in place
//	paths:		--path		(default: nil)		Paths to search for subcommands (relative to dir)
//	recursive:	--recursive	(default: true)		Search recursively
func Format(dir string, inplace bool, paths []string, recursive bool) error {
	dataModel, err := parse(dir, "commentv1", &parsers.ParseOptions{
		SearchPaths: paths,
		Recursive:   recursive,
	})
	if err != nil {
		return err
	}

	// Group modifications by file
	editsByFile := make(map[string][]fileEdit)

	for _, cmd := range dataModel.Commands {
		// Root command
		if cmd.DefinitionFile != "" {
			newDoc := generateDocComment(cmd.FunctionName, cmd.MainCmdName, cmd.Description, cmd.ExtendedHelp, cmd.Parameters)
			editsByFile[cmd.DefinitionFile] = append(editsByFile[cmd.DefinitionFile], fileEdit{
				start: cmd.DocStart,
				end:   cmd.DocEnd,
				text:  newDoc,
			})
		}
		collectSubCommandEdits(cmd.SubCommands, editsByFile)
	}

	for filename, edits := range editsByFile {
		// Sort edits by start position (descending) to apply them without messing up offsets
		sort.Slice(edits, func(i, j int) bool {
			return edits[i].start > edits[j].start
		})

		// Read file
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filename, err)
		}

		fset := dataModel.FileSet

		newContent := content
		for _, edit := range edits {
			startOffset := fset.Position(edit.start).Offset
			endOffset := fset.Position(edit.end).Offset

			// Validation
			if startOffset < 0 || endOffset > len(newContent) || startOffset > endOffset {
				return fmt.Errorf("invalid offsets for %s: %d-%d", filename, startOffset, endOffset)
			}

			// Replace
			var buf []byte
			buf = append(buf, newContent[:startOffset]...)
			buf = append(buf, []byte(edit.text)...)
			buf = append(buf, newContent[endOffset:]...)
			newContent = buf
		}

		if inplace {
			if err := os.WriteFile(filename, newContent, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
			fmt.Printf("Formatted %s\n", filename)
		} else {
			fmt.Printf("Would format %s\n", filename)
		}
	}

	return nil
}

type fileEdit struct {
	start token.Pos
	end   token.Pos
	text  string
}

func collectSubCommandEdits(subCmds []*model.SubCommand, editsByFile map[string][]fileEdit) {
	for _, sc := range subCmds {
		if sc.DefinitionFile != "" {
			fullSeq := sc.MainCmdName + " " + sc.SubCommandSequence()

			newDoc := generateDocComment(sc.SubCommandFunctionName, fullSeq, sc.SubCommandDescription, sc.SubCommandExtendedHelp, sc.Parameters)
			editsByFile[sc.DefinitionFile] = append(editsByFile[sc.DefinitionFile], fileEdit{
				start: sc.DocStart,
				end:   sc.DocEnd,
				text:  newDoc,
			})
		}
		collectSubCommandEdits(sc.SubCommands, editsByFile)
	}
}

func generateDocComment(funcName, commandSeq, description, extendedHelp string, params []*model.FunctionParameter) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "// %s is a subcommand `%s` %s\n", funcName, commandSeq, description)

	// Extended Help
	if extendedHelp != "" {
		// Ensure extended help lines are commented
		lines := strings.Split(extendedHelp, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				sb.WriteString("//\n")
			} else {
				if strings.HasPrefix(line, "//") {
					sb.WriteString(line + "\n")
				} else {
					sb.WriteString("// " + line + "\n")
				}
			}
		}
	}

	if len(params) > 0 {
		sb.WriteString("//\n")
		sb.WriteString("// Flags:\n")

		var maxNameLen int
		var maxFlagLen int
		var maxDefaultLen int

		type fmtParam struct {
			nameCol string
			flagCol string
			defCol  string
			desc    string
		}

		var formattedParams []fmtParam

		for _, p := range params {
			nameCol := p.Name + ":"
			// Reconstruct flag string without type
			var parts []string
			if len(p.FlagAliases) > 0 {
				for _, f := range p.FlagAliases {
					prefix := "-"
					if len(f) > 1 {
						prefix = "--"
					}
					parts = append(parts, prefix+f)
				}
			} else {
				prefix := "-"
				if len(p.Name) > 1 {
					prefix = "--"
				}
				parts = append(parts, prefix+p.Name)
			}
			flagCol := strings.Join(parts, ", ")

			defCol := ""
			if p.Default != "" {
				if p.Type == "string" && !strings.HasPrefix(p.Default, "\"") {
					defCol = fmt.Sprintf("(default: %q)", p.Default)
				} else {
					defCol = fmt.Sprintf("(default: %s)", p.Default)
				}
			}

			if len(nameCol) > maxNameLen {
				maxNameLen = len(nameCol)
			}
			if len(flagCol) > maxFlagLen {
				maxFlagLen = len(flagCol)
			}
			if len(defCol) > maxDefaultLen {
				maxDefaultLen = len(defCol)
			}

			formattedParams = append(formattedParams, fmtParam{nameCol, flagCol, defCol, p.Description})
		}

		for _, fp := range formattedParams {
			// padding
			padName := strings.Repeat(" ", maxNameLen-len(fp.nameCol)+1)
			padFlag := strings.Repeat(" ", maxFlagLen-len(fp.flagCol)+1)
			padDef := strings.Repeat(" ", maxDefaultLen-len(fp.defCol)+1)

			// Construct line: //   name: <pad> flag <pad> def <pad> desc
			line := fmt.Sprintf("//   %s%s%s%s%s%s%s", fp.nameCol, padName, fp.flagCol, padFlag, fp.defCol, padDef, fp.desc)
			// Trim trailing whitespace
			line = strings.TrimRight(line, " \t") + "\n"
			sb.WriteString(line)
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}
