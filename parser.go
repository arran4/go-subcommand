package go_subcommand

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/modfile"
)

type File struct {
	Path   string
	Reader io.Reader
}

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

func ParseGoFiles(root string, files ...File) (*DataModel, error) {
	fset := token.NewFileSet()

	goModPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("go.mod not found in the root of the repository: %s", goModPath)
	}

	goModBytes, err := ioutil.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}
	modPath := modfile.ModulePath(goModBytes)

	rootCommands := &CommandsTree{
		Commands:    map[string]*CommandTree{},
		PackagePath: modPath,
	}
	for _, file := range files {
		rel, err := filepath.Rel(root, file.Path)
		if err != nil {
			return nil, err
		}
		dir := filepath.Dir(rel)
		if dir == "." {
			dir = ""
		}
		importPath := path.Join(modPath, dir)

		if err := ParseGoFile(fset, importPath, file.Reader, rootCommands); err != nil {
			return nil, err
		}
	}

	d := &DataModel{
		PackageName: "main",
	}

	var commands []*Command
	for cmdName, cmdTree := range rootCommands.Commands {
		cmd := &Command{
			DataModel:   d,
			MainCmdName: cmdName,
			PackagePath: rootCommands.PackagePath,
		}

		var subCommands []*SubCommand
		subCommands = collectSubCommands(cmd, cmdTree.SubCommandTree, nil)
		cmd.SubCommands = subCommands
		commands = append(commands, cmd)
	}
	d.Commands = commands
	return d, nil
}

func collectSubCommands(cmd *Command, sct *SubCommandTree, parent *SubCommand) []*SubCommand {
	var subCommands []*SubCommand
	if sct.SubCommand != nil {
		sct.SubCommand.Command = cmd
		sct.SubCommand.Parent = parent
		subCommands = append(subCommands, sct.SubCommand)
		for _, subTree := range sct.SubCommands {
			sct.SubCommand.SubCommands = append(sct.SubCommand.SubCommands, collectSubCommands(cmd, subTree, sct.SubCommand)...)
		}
	} else {
		for _, subTree := range sct.SubCommands {
			subCommands = append(subCommands, collectSubCommands(cmd, subTree, parent)...)
		}
	}
	return subCommands
}

func ParseGoFile(fset *token.FileSet, importPath string, file io.Reader, cmdTree *CommandsTree) error {
	f, err := parser.ParseFile(fset, "", file, parser.SkipObjectResolution|parser.ParseComments)
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
						fp := &FunctionParameter{
							Name: name.Name,
							Type: p.Type.(*ast.Ident).Name,
						}
						if parsed, ok := parsedParams[name.Name]; ok {
							fp.FlagAliases = parsed.Flags
							fp.Default = parsed.Default
							fp.Description = parsed.Description
						}
						params = append(params, fp)
					}
				}
			}

			subCommandName := subCommandSequence[len(subCommandSequence)-1]
			cmdTree.Insert(importPath, f.Name.Name, cmdName, subCommandSequence, &SubCommand{
				SubCommandFunctionName: s.Name.Name,
				SubCommandDescription:  description,
				SubCommandExtendedHelp: extendedHelp,
				SubCommandName:         subCommandName,
				Parameters:             params,
			})
		}
	}
	return nil
}

type ParsedParam struct {
	Flags       []string
	Default     string
	Description string
}

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
			re := regexp.MustCompile(`^([\w]+)(?:[:\s])\s*(.*)$`)
			matches := re.FindStringSubmatch(paramLine)
			if matches != nil {
				name := matches[1]
				rest := matches[2]
				params[name] = parseParamDetails(rest)
			} else {
				extendedHelpLines = append(extendedHelpLines, trimmedLine)
			}
		} else {
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
