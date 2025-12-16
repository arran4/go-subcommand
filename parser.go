package go_subcommand

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"path"
	"strings"

	"golang.org/x/mod/modfile"
)

type SubCommandTree struct {
	SubCommands map[string]*SubCommandTree
	*SubCommand
}

func (sct * SubCommandTree) Insert(importPath string, sequence []string, s *SubCommand) {
	if len(sequence) == 0 {
		sct.SubCommand = s
		return
	}
	subCommandName := sequence[0]
	subCommandTree, ok := sct.SubCommands[subCommandName]
	if !ok {
		subCommandTree = NewSubCommandTree(nil)
		sct.SubCommands[subCommandName] = subCommandTree
	}
	subCommandTree.Insert(importPath, sequence[1:], s)
}

type CommandTree struct {
	CommandName string
	*SubCommandTree
}

type CommandsTree struct {
	Commands map[string]*CommandTree
	PackagePath string
}

func (cst *CommandsTree) Insert(importPath string, cmdName string, subcommandSequence[]string, s *SubCommand) {
	ct, ok := cst.Commands[cmdName]
	if !ok {
		ct = &CommandTree{
			CommandName:    cmdName,
			SubCommandTree: NewSubCommandTree(nil),
		}
		cst.Commands[cmdName] = ct
	}
	ct.Insert(importPath, subcommandSequence, s)
}

func NewSubCommandTree(subCommand *SubCommand) *SubCommandTree {
	return &SubCommandTree{
		SubCommands: map[string]*SubCommandTree{},
		SubCommand:  subCommand,
	}
}

func ParseGoFiles(importPrefix string, files ...io.Reader) (*DataModel, error) {
	fset := token.NewFileSet()

	goModBytes, err := ioutil.ReadFile("go.mod")
	if err != nil {
		return nil, err
	}
	modPath := modfile.ModulePath(goModBytes)

	rootCommands := &CommandsTree{
		Commands:    map[string]*CommandTree{},
		PackagePath: modPath,
	}
	for _, file := range files {
		if err := ParseGoFile(fset, importPrefix, file, rootCommands); err != nil {
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
		}

		var subCommands []*SubCommand
		var collectSubCommands func(sct *SubCommandTree, parent *SubCommand)
		collectSubCommands = func(sct *SubCommandTree, parent *SubCommand) {
			if sct.SubCommand != nil {
				sct.SubCommand.Command = cmd
				sct.SubCommand.Parent = parent
				subCommands = append(subCommands, sct.SubCommand)
			}
			for _, subTree := range sct.SubCommands {
				currentParent := parent
				if sct.SubCommand != nil {
					currentParent = sct.SubCommand
				}
				collectSubCommands(subTree, currentParent)
			}
		}

		collectSubCommands(cmdTree.SubCommandTree, nil)
		cmd.SubCommands = subCommands
		commands = append(commands, cmd)
	}
	d.Commands = commands

	return d, nil
}

func ParseGoFile(fset *token.FileSet, importPrefix string, file io.Reader, cmdTree *CommandsTree) error {
	f, err := parser.ParseFile(fset, "", file, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return err
	}
	importPath := path.Join(cmdTree.PackagePath, importPrefix)
	for _, s := range f.Decls {
		switch s := s.(type) {
		case *ast.FuncDecl:
			if s.Recv != nil {
				continue
			}
			cmdName, subCommandSequence, description, ok := ParseSubCommandComments(s.Doc.Text())
			if !ok || len(subCommandSequence) == 0 {
				continue
			}
			parentSubCommandSequence := subCommandSequence[:len(subCommandSequence)-1]
			subCommandName := subCommandSequence[len(subCommandSequence)-1]
			cmdTree.Insert(importPath, cmdName, parentSubCommandSequence, &SubCommand{
				SubCommandFunctionName:       s.Name.Name,
				SubCommandDescription: description,
				SubCommandName: subCommandName,
				ImportPath:             importPath,
				SubCommandPackageName:  f.Name.Name,
/*				FunctionParams: slices.Collect(func(yield func(*NameType) bool) {
					if s.Type.Params == nil {
						return
					}
					for _, each := range s.Type.Params.List {
						for _, eachName := range each.Names {
							if each.Names == nil {
								if !yield(&NameType{
									Type: GetType(each.Type),
								}) {
									return
								}
								continue
							}
							if !yield(&NameType{
								Name: eachName.Name,
								Type: GetType(each.Type),
							}) {
								return
							}
						}
					}
				}),
				ReturnValues: slices.Collect(func(yield func(*NameType) bool) {
					if s.Type.Results == nil {
						return
					}
					for _, each := range s.Type.Results.List {
						if each.Names == nil {
							if !yield(&NameType{
								Type: GetType(each.Type),
							}) {
								return
							}
							continue
						}
						for _, eachName := range each.Names {
							if !yield(&NameType{
								Name: eachName.Name,
								Type: GetType(each.Type),
							}) {
								return
							}
						}
					}
				}),*/
			})
		}
	}
	return nil
}

func ParseSubCommandComments(text string) (cmdName string, subCommandSequence []string, description string, ok bool) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "go-subcommand:") {
			line = strings.TrimPrefix(line, "go-subcommand:")
			parts := strings.Fields(line)
			if len(parts) > 0 {
				cmdName = parts[0]
				if len(parts) > 1 {
					subCommandSequence = parts[1:]
				}
				ok = true
			}
		} else if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return
}

