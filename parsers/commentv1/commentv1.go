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
	FunctionName       string
	CommandPackageName string
	DefinitionFile     string
	DocStart           token.Pos
	DocEnd             token.Pos
	Parameters         []*model.FunctionParameter
	ReturnsError       bool
	ReturnCount        int
	Description    string
	ExtendedHelp   string
	ImportPath     string
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
	return (&CommentParser{}).Parse(fsys, root, nil)
}

func (p *CommentParser) Parse(fsys fs.FS, root string, options *parsers.ParseOptions) (*model.DataModel, error) {
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

	searchPaths := []string{root}
	recursive := true
	if options != nil {
		if len(options.SearchPaths) > 0 {
			searchPaths = options.SearchPaths
		}
		recursive = options.Recursive
	}

	for _, startDir := range searchPaths {
		if startDir == "" {
			startDir = "."
		}
		// Walk the FS
		err = fs.WalkDir(fsys, startDir, func(pathStr string, d fs.DirEntry, err error) error {
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
				if !recursive && pathStr != startDir {
					return fs.SkipDir
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
			DataModel:          d,
			MainCmdName:        cmdName,
			PackagePath:        rootCommands.PackagePath,
			ImportPath:         cmdTree.ImportPath,
			FunctionName:       cmdTree.FunctionName,
			CommandPackageName: cmdTree.CommandPackageName,
			DefinitionFile:     cmdTree.DefinitionFile,
			DocStart:           cmdTree.DocStart,
			DocEnd:             cmdTree.DocEnd,
			Parameters:         cmdTree.Parameters,
			ReturnsError:       cmdTree.ReturnsError,
			ReturnCount:        cmdTree.ReturnCount,
			Description:        cmdTree.Description,
			ExtendedHelp:       cmdTree.ExtendedHelp,
		}

		allocator := parsers.NewNameAllocator()
		subCommands := collectSubCommands(cmd, "", cmdTree.SubCommandTree, nil, allocator)
		cmd.SubCommands = subCommands
		commands = append(commands, cmd)

		cmd.ResolveInheritance()
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
		allocateName := sct.SubCommandName
		if parent != nil {
			allocateName = parent.SubCommandStructName + " " + allocateName
		}
		sct.SubCommandStructName = allocator.Allocate(allocateName)

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
			allocateName := name
			if parent != nil {
				allocateName = parent.SubCommandStructName + " " + allocateName
			}
			syntheticCmd := &model.SubCommand{
				Command:                cmd,
				Parent:                 parent,
				SubCommandName:         name,
				SubCommandStructName:   allocator.Allocate(allocateName),
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
			cmdName, subCommandSequence, description, extendedHelp, aliases, parsedParams, ok := ParseSubCommandComments(s.Doc.Text())
			if !ok {
				continue
			}

			if cmdName == "" && len(subCommandSequence) == 0 {
				cmdName = parsers.ToKebabCase(s.Name.Name)
			}

			if len(subCommandSequence) > 0 && description == "" {
				fullCmdName := cmdName + " " + strings.Join(subCommandSequence, " ")
				log.Printf("Warning: Subcommand '%s' (function %s) is missing a short description.", fullCmdName, s.Name.Name)
			}

			currentCmdName := cmdName
			if len(subCommandSequence) > 0 {
				currentCmdName = subCommandSequence[len(subCommandSequence)-1]
			}

			var params []*model.FunctionParameter
			if s.Type.Params != nil {
				for _, p := range s.Type.Params.List {
					for _, name := range p.Names {
						typeName := ""
						isVarArg := false
						expr := p.Type
						if ellipsis, ok := expr.(*ast.Ellipsis); ok {
							isVarArg = true
							expr = ellipsis.Elt
						}

						var err error
						typeName, err = formatType(expr)
						if err != nil {
							return fmt.Errorf("error processing parameter %s in function %s: %w", name.Name, s.Name.Name, err)
						}

						fp := &model.FunctionParameter{
							Name:       name.Name,
							Type:       typeName,
							IsVarArg:   isVarArg,
							DeclaredIn: currentCmdName,
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

						// Merge Logic Priority:
						// 1. Flags: block (Top)
						// 2. Inline comment (2nd)
						// 3. Preceding comment (3rd)
						// Fields are filled from lowest priority up, overwriting if present.

						inherited := false

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
							if c.Inherited {
								inherited = true
							}
							if c.IsRequired {
								fp.IsRequired = true
							}
							// IsGlobal removed
							if c.Parser.Type != "" {
								fp.Parser = c.Parser
							}
							if c.Generator.Type != "" {
								fp.Generator = c.Generator
							}
							if c.Generator != "" {
								fp.Generator = c.Generator
							}
							if c.ParserFunc != "" {
								fp.ParserFunc = c.ParserFunc
								fp.ParserPkg = c.ParserPkg
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
							if c.Inherited {
								inherited = true
							}
							if c.IsRequired {
								fp.IsRequired = true
							}
							if c.Generator.Type != "" {
								fp.Generator = c.Generator
							}
							if c.Required {
								fp.Required = true
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
							if c.Inherited {
								inherited = true
							}
							if c.IsRequired {
								fp.IsRequired = true
							}
							// IsGlobal removed
							if c.Parser.Type != "" {
								fp.Parser = c.Parser
							}
							if c.Generator.Type != "" {
								fp.Generator = c.Generator
							}
							if c.Generator != "" {
								fp.Generator = c.Generator
							}
							if c.ParserFunc != "" {
								fp.ParserFunc = c.ParserFunc
								fp.ParserPkg = c.ParserPkg
              }
						}

						// Apply defaults if not set
						if fp.Generator.Type == "" {
							fp.Generator.Type = model.SourceTypeFlag
						}
						if fp.Parser.Type == "" {
							if fp.Generator.Type == model.SourceTypeFlag {
								fp.Parser.Type = model.ParserTypeImplicit
							} else {
								fp.Parser.Type = model.ParserTypeIdentity
							}
						}

						if inherited {
							parentCmdName := cmdName
							if len(subCommandSequence) > 1 {
								parentCmdName = subCommandSequence[len(subCommandSequence)-2]
							} else if len(subCommandSequence) == 0 {
								// No parent
								parentCmdName = ""
							}
							if parentCmdName != "" {
								fp.DeclaredIn = parentCmdName
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
					// If declared in parent, we expect description to be inherited later
					if p.DeclaredIn == currentCmdName {
						missingDescription = append(missingDescription, p.Name)
					}
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
				ct.ImportPath = importPath
				ct.FunctionName = s.Name.Name
				ct.CommandPackageName = f.Name.Name
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
				Aliases:                aliases,
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
	reAlias           = regexp.MustCompile(`\((?i:aliases|alias|aka):\s*([^)]+)\)`)
	reRequired        = regexp.MustCompile(`\(required\)`)
	reGlobal          = regexp.MustCompile(`\(global\)`)
	reParser          = regexp.MustCompile(`\(parser:\s*([^)]+)\)`)
	reGenerator       = regexp.MustCompile(`\(generator:\s*([^)]+)\)`)
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
	Inherited          bool
	IsRequired         bool
	// IsGlobal removed
	Parser    model.ParserConfig
	Generator model.GeneratorConfig
}

var reImplicitParam = regexp.MustCompile(`^([\w]+):\s*(.*)$`)

func ParseSubCommandComments(text string) (cmdName string, subCommandSequence []string, description string, extendedHelp string, aliases []string, params map[string]ParsedParam, ok bool) {
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

		if idx := strings.Index(line, DirectiveIsSubcommand); idx != -1 {
			ok = true
			subCmdPart := line[idx+len(DirectiveIsSubcommand):]

			start := strings.Index(subCmdPart, "`")
			end := strings.LastIndex(subCmdPart, "`")
			if start != -1 && end != -1 && start < end {
				commandPart := subCmdPart[start+1 : end]
				parts := strings.Fields(commandPart)
				if len(parts) > 0 {
					cmdName = parts[0]
					if len(parts) > 1 {
						subCommandSequence = parts[1:]
					}
				}
				subCmdPart = subCmdPart[end+1:]
			}

			rest := strings.TrimSpace(subCmdPart)

			// Check for inline aliases
			// Format: (aliases: a, b) or (aka: a, b)
			// Regex to capture content inside parens
			if matches := reAlias.FindStringSubmatch(rest); matches != nil {
				aliasStr := matches[1]
				parts := strings.FieldsFunc(aliasStr, func(r rune) bool {
					return r == ',' || r == ';'
				})
				for _, p := range parts {
					a := strings.TrimSpace(p)
					if a != "" {
						aliases = append(aliases, a)
					}
				}
				// Remove the alias part from rest to keep description clean
				rest = strings.TrimSpace(strings.Replace(rest, matches[0], "", 1))
			}

			if strings.HasPrefix(rest, "that ") {
				description = strings.TrimPrefix(rest, "that ")
			} else if strings.HasPrefix(rest, "-- ") {
				description = strings.TrimPrefix(rest, "-- ")
			} else if rest != "" {
				description = rest
			}
			continue
		}

		lowerTrimmedLine := strings.ToLower(trimmedLine)
		if strings.HasPrefix(lowerTrimmedLine, DirectiveAliasesPrefix) || strings.HasPrefix(lowerTrimmedLine, DirectiveAliasPrefix) {
			lineParts := strings.SplitN(trimmedLine, ":", 2)
			if len(lineParts) > 1 {
				parts := strings.FieldsFunc(lineParts[1], func(r rune) bool {
					return r == ',' || r == ';'
				})
				for _, p := range parts {
					a := strings.TrimSpace(p)
					if a != "" {
						aliases = append(aliases, a)
					}
				}
			}
			continue
		} else if matches := reAlias.FindStringSubmatch(trimmedLine); matches != nil {
			aliasStr := matches[1]
			parts := strings.FieldsFunc(aliasStr, func(r rune) bool {
				return r == ',' || r == ';'
			})
			for _, p := range parts {
				a := strings.TrimSpace(p)
				if a != "" {
					aliases = append(aliases, a)
				}
			}
			continue
		}

		if trimmedLine == DirectiveFlags {
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
			if strings.HasPrefix(trimmedLine, PrefixFlag) {
				paramLine = strings.TrimPrefix(trimmedLine, PrefixFlag)
				parsedParam = true
			} else if strings.HasPrefix(trimmedLine, PrefixParam) {
				paramLine = strings.TrimPrefix(trimmedLine, PrefixParam)
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
				if details.IsPositional || details.IsVarArg || len(details.Flags) > 0 || details.Parser.Type != "" || details.IsRequired || details.Generator.Type != "" {
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

func extractParamAttributes(text string) (string, string) {
	// Check Start
	if strings.HasPrefix(text, "(") {
		open := 0
		for i, r := range text {
			switch r {
			case '(':
				open++
			case ')':
				open--
			}
			if open == 0 {
				// Matched
				return text[1:i], strings.TrimSpace(text[i+1:])
			}
		}
	}
	// Check End
	if strings.HasSuffix(text, ")") {
		open := 0
		for i := len(text) - 1; i >= 0; i-- {
			switch text[i] {
			case ')':
				open++
			case '(':
				open--
			}
			if open == 0 {
				// Matched
				return text[i+1 : len(text)-1], strings.TrimSpace(text[:i])
			}
		}
	}
	return "", text
}

func splitSafe(s string, sep rune) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	inQuote := false

	for _, r := range s {
		if r == '"' {
			inQuote = !inQuote
		}
		if !inQuote {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
			}
		}

		if depth == 0 && !inQuote && r == sep {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func parseAttributes(attrs string, p *ParsedParam) {
	parts := splitSafe(attrs, ';')
	// Check if semicolon split looks valid (keys shouldn't contain commas)
	// If invalid, try comma split (legacy support)
	useComma := false
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		key := strings.TrimSpace(kv[0])
		if strings.Contains(key, ",") {
			useComma = true
			break
		}
	}

	if useComma {
		parts = splitSafe(attrs, ',')
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := ""
		if len(kv) > 1 {
			val = strings.TrimSpace(kv[1])
		}

		if strings.HasPrefix(key, "-") {
			p.Flags = append(p.Flags, strings.TrimLeft(key, "-"))
			continue
		}

		switch key {
		case AttributeRequired:
			p.Required = true
		case AttributeGlobal:
			p.Inherited = true
		case AttributeGenerator:
			p.Generator = val
		case AttributeParser:
			if strings.Contains(val, "\"") {
				// parser: "pkg/path".Func
				// parser: "pkg".Func
				lastDot := strings.LastIndex(val, ".")
				if lastDot != -1 {
					p.ParserPkg = strings.Trim(val[:lastDot], "\"")
					p.ParserFunc = val[lastDot+1:]
				} else {
					p.ParserFunc = val
				}
			} else {
				// parser: Func
				// parser: pkg.Func
				lastDot := strings.LastIndex(val, ".")
				if lastDot != -1 {
					p.ParserPkg = val[:lastDot]
					p.ParserFunc = val[lastDot+1:]
				} else {
					p.ParserFunc = val
				}
			}
		case AttributeAka, AttributeAlias, AttributeAliases:
			vals := strings.Split(val, ",")
			for _, v := range vals {
				v = strings.TrimSpace(v)
				if v != "" {
					p.Flags = append(p.Flags, strings.TrimLeft(v, "-"))
				}
			}
		case AttributeDefault:
			p.Default = val
			if strings.HasPrefix(p.Default, "\"") && strings.HasSuffix(p.Default, "\"") {
				p.Default = strings.Trim(p.Default, "\"")
			}
		case AttributeFromParent, AttributeInherited:
			p.Inherited = true
		}
	}
}

func parseParamDetails(text string) ParsedParam {
	var p ParsedParam

	// Preprocess: split (a; b) into (a) (b)
	parenRegex := regexp.MustCompile(`\(([^)]*)\)`)
	text = parenRegex.ReplaceAllStringFunc(text, func(match string) string {
		content := match[1 : len(match)-1] // strip parens
		if strings.Contains(content, ";") {
			parts := strings.Split(content, ";")
			var newParts []string
			for _, part := range parts {
				newParts = append(newParts, "("+strings.TrimSpace(part)+")")
			}
			return strings.Join(newParts, " ")
		}
		return match
	})

	if reRequired.MatchString(text) {
		p.IsRequired = true
		text = reRequired.ReplaceAllString(text, "")
	}

	if reGlobal.MatchString(text) {
		// p.IsGlobal = true // Removed
		text = reGlobal.ReplaceAllString(text, "")
	}

	if matches := reGenerator.FindStringSubmatch(text); matches != nil {
		generatorVal := strings.TrimSpace(matches[1])
		p.Generator.Type = model.SourceTypeGenerator
		if idx := strings.LastIndex(generatorVal, "."); idx != -1 {
			p.Generator.Func = &model.FuncRef{
				ImportPath:   strings.Trim(generatorVal[:idx], "\""),
				FunctionName: generatorVal[idx+1:],
			}
			p.Generator.Func.PackagePath = p.Generator.Func.ImportPath
			p.Generator.Func.CommandPackageName = filepath.Base(p.Generator.Func.ImportPath)
		} else {
			p.Generator.Func = &model.FuncRef{
				FunctionName: generatorVal,
			}
		}
		text = strings.Replace(text, matches[0], "", 1)
	}

	if matches := reParser.FindStringSubmatch(text); matches != nil {
		parserVal := strings.TrimSpace(matches[1])
		p.Parser.Type = model.ParserTypeCustom
		if idx := strings.LastIndex(parserVal, "."); idx != -1 {
			p.Parser.Func = &model.FuncRef{
				ImportPath:   strings.Trim(parserVal[:idx], "\""),
				FunctionName: parserVal[idx+1:],
			}
			p.Parser.Func.PackagePath = p.Parser.Func.ImportPath
			p.Parser.Func.CommandPackageName = filepath.Base(p.Parser.Func.ImportPath)
		} else {
			p.Parser.Func = &model.FuncRef{
				FunctionName: parserVal,
			}
		}
		text = strings.Replace(text, matches[0], "", 1)
	}

	if strings.Contains(text, "(from parent)") {
		p.Inherited = true
		text = strings.ReplaceAll(text, "(from parent)", "")
	}

	defaultRegex := regexp.MustCompile(`(?:default:\s*)((?:"[^"]*"|[^),]+))`)
	loc := defaultRegex.FindStringSubmatchIndex(text)
	if loc != nil {
		p.Default = strings.TrimSpace(text[loc[2]:loc[3]])
		text = text[:loc[0]] + text[loc[1]:]
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
		text = posArgRegex.ReplaceAllString(text, "")
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
		text = varArgRangeRegex.ReplaceAllString(text, "")
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

func formatType(expr ast.Expr) (string, error) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, nil
	case *ast.StarExpr:
		s, err := formatType(t.X)
		if err != nil {
			return "", err
		}
		return "*" + s, nil
	case *ast.SelectorExpr:
		x, err := formatType(t.X)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%s", x, t.Sel.Name), nil
	case *ast.ArrayType:
		s, err := formatType(t.Elt)
		if err != nil {
			return "", err
		}
		return "[]" + s, nil
	case *ast.Ellipsis:
		s, err := formatType(t.Elt)
		if err != nil {
			return "", err
		}
		return "..." + s, nil
	default:
		return "", fmt.Errorf("unsupported type: %T", t)
	}
}
