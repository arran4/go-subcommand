package go_subcommand

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"path"
	"slices"
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
		SubCommands: map[string]*SubCommandTree{
		},
		SubCommand:  subCommand,
	}
}

func ParseGoFiles(importPrefix string, files ...io.Reader) (*DataModel, error) {
	fset := token.NewFileSet()
	var rootCommands = &CommandsTree{
		Commands: map[string]*CommandTree{},
		PackagePath: "", // TODO
	}
	for _, file := range files {
		if err := ParseGoFile(fset, importPrefix, file, rootCommands); err != nil {
			return nil, err
		}
	}
	d := &DataModel{
		d.PackageName: , // Populate from go.mod
	}
	var commands map[string]*Command
	for _, subcommand := range subcommands {
		if cmd, ok := commands[subcommand.MainCmdName]; ok {
			subcommand.Command = cmd
			cmd.SubCommands = append(cmd.SubCommands, subcommand)
		} else {
			commands[subcommand.MainCmdName] = subcommand.Command
			subcommand.Command.DataModel = d
		}
	}
	d.Commands = slices.Collect(maps.Values(commands))
	return d, nil
}

func ParseGoFile(fset *token.FileSet, importPrefix string, file io.Reader, cmdTree *CommandsTree) error {
	f, err := parser.ParseFile(fset, "", file, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return err
	}
	importPath := path.Join(importPrefix, f.Name.Name)
	for _, s := range f.Decls {
		switch s := s.(type) {
		case *ast.FuncDecl:
			if s.Recv != nil {
				continue
			}
			cmdName, subCommandSequence, description, ok := ParseSubCommandComments(s.Doc.Text())
			if !ok {
				continue
			}
			cmdTree.Insert(importPath, cmdName, subCommandSequence[:len(subCommandSequence) - 2], &SubCommand{
				SubCommandFunctionName:       s.Name.Name,
				SubCommandDescription: description,
				SubCommandName: subCommandSequence[len(subCommandSequence) - 1],
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
	// TODO
}

