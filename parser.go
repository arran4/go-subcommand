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
			cmdName, subCommandSequence, description, ok := ParseSubCommandComments(s.Doc.Text())
			if !ok || len(subCommandSequence) == 0 {
				continue
			}

			var params []*FunctionParameter
			if s.Type.Params != nil {
				for _, p := range s.Type.Params.List {
					for _, name := range p.Names {
						params = append(params, &FunctionParameter{
							Name: name.Name,
							Type: p.Type.(*ast.Ident).Name,
						})
					}
				}
			}

			subCommandName := subCommandSequence[len(subCommandSequence)-1]
			cmdTree.Insert(importPath, f.Name.Name, cmdName, subCommandSequence, &SubCommand{
				SubCommandFunctionName: s.Name.Name,
				SubCommandDescription:  description,
				SubCommandName:         subCommandName,
				Parameters:             params,
			})
		}
	}
	return nil
}

func ParseSubCommandComments(text string) (cmdName string, subCommandSequence []string, description string, ok bool) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	var descriptionLines []string
	for scanner.Scan() {
		line := scanner.Text()
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
			}
		} else {
			descriptionLines = append(descriptionLines, line)
		}
	}
	description = strings.TrimSpace(strings.Join(descriptionLines, "\n"))
	return
}
