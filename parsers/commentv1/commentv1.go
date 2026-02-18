package commentv1

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"log"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
	"golang.org/x/mod/modfile"
)

type SubCommandTree struct {
	SubCommands map[string]*SubCommandTree
	*model.SubCommand
}

func (sct *SubCommandTree) Insert(importPath, packageName string, sequence []string, s *model.SubCommand) {
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
	FunctionName   string
	DefinitionFile string
	DocStart       token.Pos
	DocEnd         token.Pos
	Parameters     []*model.FunctionParameter
	ReturnsError   bool
	ReturnCount    int
	Description    string
	ExtendedHelp   string
}

type CommandsTree struct {
	Commands    map[string]*CommandTree
	PackagePath string
}

func (cst *CommandsTree) Insert(importPath, packageName, cmdName string, subcommandSequence []string, s *model.SubCommand) {
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

func NewSubCommandTree(subCommand *model.SubCommand) *SubCommandTree {
	return &SubCommandTree{
		SubCommands: map[string]*SubCommandTree{},
		SubCommand:  subCommand,
	}
}

func init() {
	parsers.Register("commentv1", &CommentParser{})
}

type CommentParser struct{}

// ParseGoFiles parses the Go files in the provided filesystem to build the command model.
// It expects a go.mod file at the root of the filesystem (or root directory).
func ParseGoFiles(fsys fs.FS, root string) (*model.DataModel, error) {
	return (&CommentParser{}).Parse(fsys, root)
}

func (p *CommentParser) Parse(fsys fs.FS, root string) (*model.DataModel, error) {
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
		if d.IsDir() {
			if d.Name() == "examples" || d.Name() == "testdata" || d.Name() == ".git" {
				return fs.SkipDir
			}
			// Skip directories that are submodules (have go.mod, unless it's the root)
			if pathStr != root {
				if _, err := fs.Stat(fsys, filepath.Join(pathStr, "go.mod")); err == nil {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(pathStr, ".go") {
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
		defer func() {
			_ = f.Close()
		}()

		if err := ParseGoFile(fset, pathStr, importPath, f, rootCommands); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	d := &model.DataModel{
		FileSet:     fset,
		PackageName: "main",
	}

	var commands []*model.Command
	var cmdNames []string
	for cmdName := range rootCommands.Commands {
		cmdNames = append(cmdNames, cmdName)
	}
	sort.Strings(cmdNames)
	for _, cmdName := range cmdNames {
		cmdTree := rootCommands.Commands[cmdName]
		cmd := &model.Command{
			DataModel:      d,
			MainCmdName:    cmdName,
			PackagePath:    rootCommands.PackagePath,
			ImportPath:     rootCommands.PackagePath, // Root command logic usually in root package
			FunctionName:   cmdTree.FunctionName,
			DefinitionFile: cmdTree.DefinitionFile,
			DocStart:       cmdTree.DocStart,
			DocEnd:         cmdTree.DocEnd,
			Parameters:     cmdTree.Parameters,
			ReturnsError:   cmdTree.ReturnsError,
			ReturnCount:    cmdTree.ReturnCount,
			Description:    cmdTree.Description,
			ExtendedHelp:   cmdTree.ExtendedHelp,
		}

		allocator := parsers.NewNameAllocator()
		subCommands := collectSubCommands(cmd, "", cmdTree.SubCommandTree, nil, allocator)
		cmd.SubCommands = subCommands
		commands = append(commands, cmd)
	}
	d.Commands = commands
	return d, nil
}

func collectSubCommands(cmd *model.Command, name string, sct *SubCommandTree, parent *model.SubCommand, allocator *parsers.NameAllocator) []*model.SubCommand {
	var subCommands []*model.SubCommand
	var subCommandNames []string
	for name := range sct.SubCommands {
		subCommandNames = append(subCommandNames, name)
	}
	sort.Strings(subCommandNames)
	if sct.SubCommand != nil {
		sct.Command = cmd
		sct.Parent = parent
		// Allocate unique struct name
		if sct.SubCommandFunctionName != "" {
			sct.SubCommandStructName = allocator.Allocate(sct.SubCommandFunctionName)
		} else {
			sct.SubCommandStructName = allocator.Allocate(sct.SubCommandName)
		}

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
			syntheticCmd := &model.SubCommand{
				Command:                cmd,
				Parent:                 parent,
				SubCommandName:         name,
				SubCommandStructName:   allocator.Allocate(name),
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
			if !ok {
				continue
			}

			if len(subCommandSequence) > 0 && description == "" {
				fullCmdName := cmdName + " " + strings.Join(subCommandSequence, " ")
				log.Printf("Warning: Subcommand '%s' (function %s) is missing a short description.", fullCmdName, s.Name.Name)
			}

			var params []*model.FunctionParameter
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
						fp := &model.FunctionParameter{
							Name:     name.Name,
							Type:     typeName,
							IsVarArg: isVarArg,
						}
						// Extract details from different sources with priority
						// Priority:
						// 1. Flags: block in main documentation (Top)
						// 2. Inline comment (same line) (2nd)
						// 3. Preceding comment (line before) (3rd)

						var candidates []ParsedParam

						// 1. Flags block
						if parsed, ok := parsedParams[name.Name]; ok {
							candidates = append(candidates, parsed)
						} else {
							candidates = append(candidates, ParsedParam{})
						}

						// 2. Inline comment
						var inlineComment string
						if p.Comment != nil {
							inlineComment = p.Comment.Text()
						}
						if inlineComment == "" {
							pLine := fset.Position(p.Pos()).Line
							for _, cg := range f.Comments {
								cPos := fset.Position(cg.Pos())
								if cPos.Line == pLine {
									inlineComment = cg.Text()
									break
								}
							}
						}
						if inlineComment != "" {
							candidates = append(candidates, parseParamDetails(inlineComment))
						} else {
							candidates = append(candidates, ParsedParam{})
						}

						// 3. Preceding comment
						var precedingComment string
						if p.Doc != nil {
							precedingComment = p.Doc.Text()
						}
						// Fallback if p.Doc is empty: look for comment ending on line-1
						if precedingComment == "" {
							pLine := fset.Position(p.Pos()).Line
							for _, cg := range f.Comments {
								// If the comment is the function's doc, ignore it
								if s.Doc != nil && cg.Pos() == s.Doc.Pos() {
									continue
								}
								cEndLine := fset.Position(cg.End()).Line
								if cEndLine == pLine-1 {
									precedingComment = cg.Text()
									break
								}
							}
						}
						if precedingComment != "" {
							candidates = append(candidates, parseParamDetails(precedingComment))
						} else {
							candidates = append(candidates, ParsedParam{})
						}

						// Merge Logic based on Priority
						// We fill the 'fp' fields starting from the lowest priority (Preceding)
						// and overwrite with higher priority if the field is present/non-empty in the higher one.
						// EXCEPT for Description: if higher priority has description, it overwrites.
						// But what if higher priority has NO description? Then we keep the lower one.

						// Start with Preceding (3rd)
						if len(candidates) > 2 {
							c := candidates[2]
							if len(c.Flags) > 0 {
								fp.FlagAliases = c.Flags
							}
							if c.Default != "" {
								fp.Default = c.Default
							}
							if c.Description != "" {
								fp.Description = c.Description
							}
							if c.IsPositional {
								fp.IsPositional = true
								fp.PositionalArgIndex = c.PositionalArgIndex
							}
							if c.IsVarArg {
								fp.IsVarArg = true
								fp.VarArgMin = c.VarArgMin
								fp.VarArgMax = c.VarArgMax
							}
						}

						// Merge Inline (2nd)
						if len(candidates) > 1 {
							c := candidates[1]
							if len(c.Flags) > 0 {
								fp.FlagAliases = c.Flags
							}
							if c.Default != "" {
								fp.Default = c.Default
							}
							if c.Description != "" {
								fp.Description = c.Description
							}
							if c.IsPositional {
								fp.IsPositional = true
								fp.PositionalArgIndex = c.PositionalArgIndex
							}
							if c.IsVarArg {
								fp.IsVarArg = true
								fp.VarArgMin = c.VarArgMin
								fp.VarArgMax = c.VarArgMax
							}
						}

						// Merge Flags Block (Top)
						if len(candidates) > 0 {
							c := candidates[0]
							if len(c.Flags) > 0 {
								fp.FlagAliases = c.Flags
							}
							if c.Default != "" {
								fp.Default = c.Default
							}
							if c.Description != "" {
								fp.Description = c.Description
							}
							if c.IsPositional {
								fp.IsPositional = true
								fp.PositionalArgIndex = c.PositionalArgIndex
							}
							if c.IsVarArg {
								fp.IsVarArg = true
								fp.VarArgMin = c.VarArgMin
								fp.VarArgMax = c.VarArgMax
							}
						}

						if len(fp.FlagAliases) == 0 {
							kebab := parsers.ToKebabCase(name.Name)
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

			hasDescription := false
			var missingDescription []string
			for _, p := range params {
				if p.Description != "" {
					hasDescription = true
				} else {
					missingDescription = append(missingDescription, p.Name)
				}
			}
			if hasDescription && len(missingDescription) > 0 {
				log.Printf("Warning: In command '%s' (function %s), the following parameters are missing descriptions while others have them: %s", cmdName, s.Name.Name, strings.Join(missingDescription, ", "))
			}

			returnsError := false
			returnCount := 0
			if s.Type.Results != nil {
				for _, r := range s.Type.Results.List {
					if len(r.Names) > 0 {
						returnCount += len(r.Names)
					} else {
						returnCount++
					}
					if ident, ok := r.Type.(*ast.Ident); ok && ident.Name == "error" {
						returnsError = true
					}
				}
				if returnCount > 1 {
					return fmt.Errorf("function %s has multiple return values, which is not implemented yet", s.Name.Name)
				}
			}

			if len(subCommandSequence) == 0 {
				ct, ok := cmdTree.Commands[cmdName]
				if !ok {
					ct = &CommandTree{
						CommandName:    cmdName,
						SubCommandTree: NewSubCommandTree(nil),
					}
					cmdTree.Commands[cmdName] = ct
				}
				ct.FunctionName = s.Name.Name
				ct.DefinitionFile = filename
				ct.DocStart = s.Doc.Pos()
				ct.DocEnd = s.Doc.End()
				ct.Parameters = params
				ct.ReturnsError = returnsError
				ct.ReturnCount = returnCount
				ct.Description = description
				ct.ExtendedHelp = extendedHelp
				continue
			}

			subCommandName := subCommandSequence[len(subCommandSequence)-1]
			cmdTree.Insert(importPath, f.Name.Name, cmdName, subCommandSequence, &model.SubCommand{
				SubCommandFunctionName: s.Name.Name,
				SubCommandDescription:  description,
				SubCommandExtendedHelp: extendedHelp,
				SubCommandName:         subCommandName,
				// SubCommandStructName is assigned during collection
				DefinitionFile: filename,
				DocStart:       s.Doc.Pos(),
				DocEnd:         s.Doc.End(),
				Parameters:     params,
				ReturnsError:   returnsError,
				ReturnCount:    returnCount,
			})
		}
	}
	return nil
}

var (
	reParamDefinition = regexp.MustCompile(`^([\w]+)(?:[:\s])\s*(.*)$`)
	reImplicitCheck   = regexp.MustCompile(`@\d+|\.\.\.`)
	reImplicitFormat  = regexp.MustCompile(`^(\w+):\s+(.*)$`)
)

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

var reImplicitParam = regexp.MustCompile(`^([\w]+):\s*(.*)$`)

func ParseSubCommandComments(text string) (cmdName string, subCommandSequence []string, description string, extendedHelp string, params map[string]ParsedParam, ok bool) {
	params = make(map[string]ParsedParam)
	scanner := bufio.NewScanner(strings.NewReader(text))
	var extendedHelpLines []string

	inFlagsBlock := false
	justEnteredFlagsBlock := false

	for scanner.Scan() {
		line := scanner.Text() // Keep whitespace for indentation check
		trimmedLine := strings.TrimSpace(line)

		if justEnteredFlagsBlock {
			if trimmedLine != "" {
				log.Printf("Warning: 'Flags:' block for command '%s' should be followed by an empty line", cmdName)
			}
			justEnteredFlagsBlock = false
		}

		if trimmedLine == "" {
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
			justEnteredFlagsBlock = true
			continue
		}

		parsedParam := false
		var paramLine string

		if inFlagsBlock {
			// Check if line is indented (starts with space or tab)
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				if strings.HasPrefix(line, " ") {
					log.Printf("Warning: Parameter '%s' in command '%s' uses spaces for indentation. Use tabs.", trimmedLine, cmdName)
				}
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
			} else if matches := reImplicitFormat.FindStringSubmatch(trimmedLine); matches != nil {
				if reImplicitCheck.MatchString(matches[2]) {
					paramLine = trimmedLine
					parsedParam = true
				}
			}
		}

		if parsedParam {
			matches := reParamDefinition.FindStringSubmatch(paramLine)
			//matches := reExplicitParam.FindStringSubmatch(paramLine)
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
		if _, err := fmt.Sscanf(posArgMatches[1], "%d", &p.PositionalArgIndex); err != nil {
			// fallback/ignore, though usually this regex implies digits
			p.PositionalArgIndex = 0
		}
	}

	// Varargs constraints: 1...3 or ...
	varArgRangeRegex := regexp.MustCompile(`(\d+)\.\.\.(\d+)|(\.\.\.)`)
	varArgRangeMatches := varArgRangeRegex.FindStringSubmatch(text)
	if varArgRangeMatches != nil {
		p.IsVarArg = true
		if varArgRangeMatches[3] == "..." {
			// Just "..." means no specific limits parsed here
		} else {
			if _, err := fmt.Sscanf(varArgRangeMatches[1], "%d", &p.VarArgMin); err != nil {
				p.VarArgMin = 0
			}
			if _, err := fmt.Sscanf(varArgRangeMatches[2], "%d", &p.VarArgMax); err != nil {
				p.VarArgMax = 0
			}
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

	sort.Slice(p.Flags, func(i, j int) bool {
		if len(p.Flags[i]) != len(p.Flags[j]) {
			return len(p.Flags[i]) > len(p.Flags[j])
		}
		return p.Flags[i] < p.Flags[j]
	})

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
