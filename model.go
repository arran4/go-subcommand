package go_subcommand

import (
	"strings"
)

type DataModel struct {
	PackageName string
	Commands    []*Command
}

type Command struct {
	*DataModel
	MainCmdName string
	SubCommands []*SubCommand
	PackagePath string
}

type FunctionParameter struct {
	Name        string
	Type        string
	FlagAliases []string
	Default     string
	Description string
}

func (p *FunctionParameter) FlagString() string {
	var parts []string
	if len(p.FlagAliases) > 0 {
		for _, f := range p.FlagAliases {
			parts = append(parts, "-"+f)
		}
	} else {
		parts = append(parts, "-"+p.Name)
	}
	flags := strings.Join(parts, ", ")

	typeStr := ""
	if p.Type != "bool" {
		typeStr = " " + p.Type
	}
	return flags + typeStr
}

type SubCommand struct {
	*Command
	Parent                 *SubCommand
	SubCommands            []*SubCommand
	SubCommandName         string
	SubCommandFunctionName string
	SubCommandDescription  string
	SubCommandExtendedHelp string
	ImportPath             string
	SubCommandPackageName  string
	Parameters             []*FunctionParameter
}

func (sc *SubCommand) SubCommandSequence() string {
	if sc.Parent == nil {
		return sc.SubCommandName
	}
	return sc.Parent.SubCommandSequence() + " " + sc.SubCommandName
}

func (sc *SubCommand) ParentCmdName() string {
	if sc.Parent != nil {
		return sc.Parent.SubCommandName
	}
	return ""
}

func (sc *SubCommand) HasSubcommands() bool {
	return len(sc.SubCommands) > 0
}

func (sc *SubCommand) ProgName() string {
	return sc.Command.MainCmdName + " " + sc.SubCommandSequence()
}

func (sc *SubCommand) MaxFlagLength() int {
	max := 0
	for _, p := range sc.Parameters {
		l := len(p.FlagString())
		if l > max {
			max = l
		}
	}
	return max
}
