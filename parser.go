package go_subcommand

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

type SubCommandTree struct {
	SubCommands map[string]*SubCommandTree
	*SubCommand
}

func (sct *SubCommandTree) Insert(importPath, packageName string, sequence []string, s *SubCommand) {
	if len(sequence) == 0 {
		s.ImportPath = importPath
		s.SubCommandPackageName = packageName
		sct.SubCommand = s
		return
	}
	subCommandName := sequence[0]
	subCommandTree, ok := sct.SubCommands[subCommandName]
	if !ok {
		subCommandTree = NewSubCommandTree(nil)
		sct.SubCommands[subCommandName] = subCommandTree
	}
	subCommandTree.Insert(importPath, packageName, sequence[1:], s)
}

type CommandTree struct {
	CommandName string
	*SubCommandTree
}

type CommandsTree struct {
	Commands    map[string]*CommandTree
	PackagePath string
}

func (cst *CommandsTree) Insert(importPath, packageName, cmdName string, subcommandSequence []string, s *SubCommand) {
	ct, ok := cst.Commands[cmdName]
	if !ok {
		ct = &CommandTree{
			CommandName:    cmdName,
			SubCommandTree: NewSubCommandTree(nil),
		}
		cst.Commands[cmdName] = ct
	}
	ct.Insert(importPath, packageName, subcommandSequence, s)
}

func NewSubCommandTree(subCommand *SubCommand) *SubCommandTree {
	return &SubCommandTree{
		SubCommands: map[string]*SubCommandTree{},
		SubCommand:  subCommand,
	}
}

// ParseGoFiles parses the Go files in the provided filesystem to build the command model.
// It expects a go.mod file at the root of the filesystem (or root directory).
func ParseGoFiles(fsys fs.FS, root string) (*DataModel, error) {
	fset := token.NewFileSet()

	// Read go.mod from FS
	goModBytes, err := fs.ReadFile(fsys, filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("go.mod not found in the root of the repository: %w", err)
	}

	modPath := modfile.ModulePath(goModBytes)

	rootCommands := &CommandsTree{
		Commands:    map[string]*CommandTree{},
		PackagePath: modPath,
	}

	// Walk the FS
	err = fs.WalkDir(fsys, root, func(pathStr string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(pathStr, ".go") {
			return nil
		}

		// Calculate import path
		rel, err := filepath.Rel(root, pathStr)
		if err != nil {
			rel = pathStr // Fallback
		}
		dir := filepath.Dir(rel)
		if dir == "." {
			dir = ""
		}
		importPath := path.Join(modPath, dir)

		f, err := fsys.Open(pathStr)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := ParseGoFile(fset, pathStr, importPath, f, rootCommands); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	d := &DataModel{
		PackageName: "main",
	}

	var commands []*Command
	var cmdNames []string
	for cmdName := range rootCommands.Commands {
		cmdNames = append(cmdNames, cmdName)
	}
	sort.Strings(cmdNames)
	for _, cmdName := range cmdNames {
		cmdTree := rootCommands.Commands[cmdName]
		cmd := &Command{
			DataModel:   d,
			MainCmdName: cmdName,
			PackagePath: rootCommands.PackagePath,
		}

		allocator := NewNameAllocator()
		var subCommands []*SubCommand
		subCommands = collectSubCommands(cmd, "", cmdTree.SubCommandTree, nil, allocator)
		cmd.SubCommands = subCommands
		commands = append(commands, cmd)
	}
	d.Commands = commands
	return d, nil
}

func collectSubCommands(cmd *Command, name string, sct *SubCommandTree, parent *SubCommand, allocator *NameAllocator) []*SubCommand {
	var subCommands []*SubCommand
	var subCommandNames []string
	for name := range sct.SubCommands {
		subCommandNames = append(subCommandNames, name)
	}
	sort.Strings(subCommandNames)
	if sct.SubCommand != nil {
		sct.SubCommand.Command = cmd
		sct.SubCommand.Parent = parent
		// Allocate unique struct name
		sct.SubCommand.SubCommandStructName = allocator.Allocate(sct.SubCommand.SubCommandName)

		subCommands = append(subCommands, sct.SubCommand)
		for _, name := range subCommandNames {
			subTree := sct.SubCommands[name]
			sct.SubCommand.SubCommands = append(sct.SubCommand.SubCommands, collectSubCommands(cmd, name, subTree, sct.SubCommand, allocator)...)
		}
	} else {
		if name == "" {
			for _, name := range subCommandNames {
				subTree := sct.SubCommands[name]
				subCommands = append(subCommands, collectSubCommands(cmd, name, subTree, parent, allocator)...)
			}
		} else {
			// Missing intermediate node -> Synthetic command
			syntheticCmd := &SubCommand{
				Command:                cmd,
				Parent:                 parent,
				SubCommandName:         name,
				SubCommandFunctionName: "", // Empty to indicate synthetic
			}
			subCommands = append(subCommands, syntheticCmd)
			for _, childName := range subCommandNames {
				subTree := sct.SubCommands[childName]
				syntheticCmd.SubCommands = append(syntheticCmd.SubCommands, collectSubCommands(cmd, childName, subTree, syntheticCmd, allocator)...)
			}
		}
	}
	return subCommands
}

func ParseGoFile(fset *token.FileSet, filename, importPath string, file io.Reader, cmdTree *CommandsTree) error {
	src, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	f, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return err
	}


	// packageName := f.Name.Name
	for _, s := range f.Decls {
		switch s := s.(type) {
		case *ast.FuncDecl:
			if s.Recv != nil {
				continue
			}
			cmdName, subCommandSequence, description, extendedHelp, parsedParams, ok := ParseSubCommandComments(s.Doc.Text())
			if !ok || len(subCommandSequence) == 0 {
				continue
			}

			var params []*FunctionParameter
			if s.Type.Params != nil {
				for _, p := range s.Type.Params.List {
					for _, name := range p.Names {
						typeName := ""
						isVarArg := false
						switch t := p.Type.(type) {
						case *ast.Ident:
							typeName = t.Name
						case *ast.SelectorExpr:
							if ident, ok := t.X.(*ast.Ident); ok {
								typeName = fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)
							} else {
								// Fallback or panic, for now panic to discover what's wrong
								panic(fmt.Sprintf("Unsupported selector type: %T", t.X))
							}
						case *ast.Ellipsis:
							isVarArg = true
							if ident, ok := t.Elt.(*ast.Ident); ok {
								typeName = ident.Name
							} else if sel, ok := t.Elt.(*ast.SelectorExpr); ok {
								if ident, ok := sel.X.(*ast.Ident); ok {
									typeName = fmt.Sprintf("%s.%s", ident.Name, sel.Sel.Name)
								} else {
									panic(fmt.Sprintf("Unsupported selector type in ellipsis: %T", sel.X))
								}
							} else {
								panic(fmt.Sprintf("Unsupported type in ellipsis: %T", t.Elt))
							}
						default:
							panic(fmt.Sprintf("Unsupported type: %T", t))
						}
						fp := &FunctionParameter{
							Name:     name.Name,
							Type:     typeName,
							IsVarArg: isVarArg,
						}
						if parsed, ok := parsedParams[name.Name]; ok {
							fp.FlagAliases = parsed.Flags
							fp.Default = parsed.Default
							fp.Description = parsed.Description
							fp.IsPositional = parsed.IsPositional
							fp.PositionalArgIndex = parsed.PositionalArgIndex
							fp.VarArgMin = parsed.VarArgMin
							fp.VarArgMax = parsed.VarArgMax
						} else {
							var commentText string
							if p.Doc != nil {
								commentText = p.Doc.Text()
							}
							if p.Comment != nil {
								commentText += p.Comment.Text()
							}
							if commentText == "" {
								// Fallback: try to find a comment on the same line from global comments
								pLine := fset.Position(p.Pos()).Line
								for _, cg := range f.Comments {
									cPos := fset.Position(cg.Pos())
									if cPos.Line == pLine {
										commentText = cg.Text()
										break
									}
								}
							}

							if commentText != "" {
								parsed := parseParamDetails(commentText)
								fp.FlagAliases = parsed.Flags
								fp.Default = parsed.Default
								fp.Description = parsed.Description
							}
						}

						if len(fp.FlagAliases) == 0 {
							kebab := ToKebabCase(name.Name)
							if kebab != name.Name {
								fp.FlagAliases = []string{kebab}
							}
						}

						// If detected as VarArg in signature, force IsPositional to true
						if fp.IsVarArg {
							fp.IsPositional = true
						}

						params = append(params, fp)
					}
				}
			}

			returnsError := false
			returnCount := 0
			if s.Type.Results != nil {
				returnCount = len(s.Type.Results.List)
				for _, r := range s.Type.Results.List {
					if ident, ok := r.Type.(*ast.Ident); ok && ident.Name == "error" {
						returnsError = true
						break
					}
				}
			}

			subCommandName := subCommandSequence[len(subCommandSequence)-1]
			cmdTree.Insert(importPath, f.Name.Name, cmdName, subCommandSequence, &SubCommand{
				SubCommandFunctionName: s.Name.Name,
				SubCommandDescription:  description,
				SubCommandExtendedHelp: extendedHelp,
				SubCommandName:         subCommandName,
				// SubCommandStructName is assigned during collection
				Parameters:   params,
				ReturnsError: returnsError,
				ReturnCount:  returnCount,
			})
		}
	}
	return nil
}

type ParsedParam struct {
	Flags              []string
	Default            string
	Description        string
	IsPositional       bool
	PositionalArgIndex int
	IsVarArg           bool
	VarArgMin          int
	VarArgMax          int
}

var (
	reExplicitParam = regexp.MustCompile(`^([\w]+)(?:[:\s])\s*(.*)$`)
	reImplicitParam = regexp.MustCompile(`^([\w]+):\s*(.*)$`)
)

func ParseSubCommandComments(text string) (cmdName string, subCommandSequence []string, description string, extendedHelp string, params map[string]ParsedParam, ok bool) {
	params = make(map[string]ParsedParam)
	scanner := bufio.NewScanner(strings.NewReader(text))
	var extendedHelpLines []string

	inFlagsBlock := false

	for scanner.Scan() {
		line := scanner.Text() // Keep whitespace for indentation check
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" {
			if inFlagsBlock {
				inFlagsBlock = false
			}
			if len(extendedHelpLines) > 0 {
				extendedHelpLines = append(extendedHelpLines, "")
			}
			continue
		}

		if strings.Contains(line, "is a subcommand `") {
			ok = true
			start := strings.Index(line, "`")
			end := strings.LastIndex(line, "`")
			if start != -1 && end != -1 && start < end {
				commandPart := line[start+1 : end]
				parts := strings.Fields(commandPart)
				if len(parts) > 0 {
					cmdName = parts[0]
					if len(parts) > 1 {
						subCommandSequence = parts[1:]
					}
				}

				rest := strings.TrimSpace(line[end+1:])
				if strings.HasPrefix(rest, "that ") {
					description = strings.TrimPrefix(rest, "that ")
				} else if strings.HasPrefix(rest, "-- ") {
					description = strings.TrimPrefix(rest, "-- ")
				} else if rest != "" {
					description = rest
				}
			}
			continue
		}

		if trimmedLine == "Flags:" {
			inFlagsBlock = true
			continue
		}

		parsedParam := false
		var paramLine string

		if inFlagsBlock {
			// Check if line is indented (starts with space or tab)
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				paramLine = trimmedLine
				parsedParam = true
			} else {
				inFlagsBlock = false
			}
		}

		if !parsedParam {
			if strings.HasPrefix(trimmedLine, "flag ") {
				paramLine = strings.TrimPrefix(trimmedLine, "flag ")
				parsedParam = true
			} else if strings.HasPrefix(trimmedLine, "param ") {
				paramLine = strings.TrimPrefix(trimmedLine, "param ")
				parsedParam = true
			}
		}

		if parsedParam {
			matches := reExplicitParam.FindStringSubmatch(paramLine)
			if matches != nil {
				name := matches[1]
				rest := matches[2]
				params[name] = parseParamDetails(rest)
			} else {
				extendedHelpLines = append(extendedHelpLines, trimmedLine)
			}
		} else {
			// Attempt to parse as parameter if it looks like one, even without prefix/block
			matches := reImplicitParam.FindStringSubmatch(trimmedLine)
			if matches != nil {
				name := matches[1]
				rest := matches[2]
				details := parseParamDetails(rest)

				// We only accept it as a parameter if it has explicit configuration
				// that strongly suggests it is a parameter definition.
				// e.g. @N for positional, or defined flags, or default value.
				// This prevents false positives from general description text.
				if details.IsPositional || details.IsVarArg {
					params[name] = details
					continue
				}
			}
			extendedHelpLines = append(extendedHelpLines, trimmedLine)
		}
	}
	extendedHelp = strings.TrimSpace(strings.Join(extendedHelpLines, "\n"))
	return
}

func parseParamDetails(text string) ParsedParam {
	var p ParsedParam

	defaultRegex := regexp.MustCompile(`(?:default:\s*)((?:"[^"]*"|[^),]+))`)
	loc := defaultRegex.FindStringSubmatchIndex(text)
	if loc != nil {
		p.Default = strings.TrimSpace(text[loc[2]:loc[3]])
	}

	// Positional arguments: @1, @2, etc.
	posArgRegex := regexp.MustCompile(`@(\d+)`)
	posArgMatches := posArgRegex.FindStringSubmatch(text)
	if posArgMatches != nil {
		p.IsPositional = true
		fmt.Sscanf(posArgMatches[1], "%d", &p.PositionalArgIndex)
	}

	// Varargs constraints: 1...3 or ...
	varArgRangeRegex := regexp.MustCompile(`(\d+)\.\.\.(\d+)|(\.\.\.)`)
	varArgRangeMatches := varArgRangeRegex.FindStringSubmatch(text)
	if varArgRangeMatches != nil {
		p.IsVarArg = true
		if varArgRangeMatches[3] == "..." {
			// Just "..." means no specific limits parsed here
		} else {
			fmt.Sscanf(varArgRangeMatches[1], "%d", &p.VarArgMin)
			fmt.Sscanf(varArgRangeMatches[2], "%d", &p.VarArgMax)
		}
	}

	flagRegex := regexp.MustCompile(`-[\w-]+`)
	flagMatches := flagRegex.FindAllString(text, -1)

	seenFlags := make(map[string]bool)
	for _, f := range flagMatches {
		stripped := strings.TrimLeft(f, "-")
		if !seenFlags[stripped] {
			p.Flags = append(p.Flags, stripped)
			seenFlags[stripped] = true
		}
	}

	clean := flagRegex.ReplaceAllString(text, "")
	clean = defaultRegex.ReplaceAllString(clean, "")
	clean = posArgRegex.ReplaceAllString(clean, "")
	clean = varArgRangeRegex.ReplaceAllString(clean, "")

	clean = strings.ReplaceAll(clean, "()", "")
	clean = strings.TrimSpace(clean)
	clean = strings.TrimPrefix(clean, ":")
	clean = strings.TrimPrefix(clean, ",")
	clean = strings.TrimPrefix(clean, "(")
	clean = strings.TrimPrefix(clean, ":") // Also remove leading colon if not handled
	clean = strings.TrimSuffix(clean, ")")

	clean = strings.ReplaceAll(clean, ",", " ")
	clean = strings.ReplaceAll(clean, "(", " ")
	clean = strings.ReplaceAll(clean, ")", " ")
	clean = strings.Join(strings.Fields(clean), " ")

	p.Description = clean

	if strings.HasPrefix(p.Default, "\"") && strings.HasSuffix(p.Default, "\"") {
		p.Default = strings.Trim(p.Default, "\"")
	}

	return p
}

