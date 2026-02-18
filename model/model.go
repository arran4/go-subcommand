package model

import (
	"fmt"
	"go/token"
	"strings"
)

type DataModel struct {
	FileSet     *token.FileSet
	PackageName string
	Commands    []*Command
}

type Command struct {
	*DataModel
	MainCmdName    string
	SubCommands    []*SubCommand
	PackagePath    string
	ImportPath     string
	Description    string
	ExtendedHelp   string
	FunctionName   string
	DefinitionFile string
	DocStart       token.Pos
	DocEnd         token.Pos
	Parameters     []*FunctionParameter
	ReturnsError   bool
	ReturnCount    int
}

type FunctionParameter struct {
	Name               string
	Type               string
	FlagAliases        []string
	Default            string
	Description        string
	IsPositional       bool
	PositionalArgIndex int
	IsVarArg           bool
	VarArgMin          int
	VarArgMax          int
}

func (p *FunctionParameter) FlagString() string {
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
	flags := strings.Join(parts, ", ")

	typeStr := ""
	if p.Type != "bool" {
		typeStr = " " + p.Type
	}
	return flags + typeStr
}

func (p *FunctionParameter) DefaultString() string {
	if p.Default == "" {
		return ""
	}
	if p.Type == "string" && !strings.HasPrefix(p.Default, "\"") {
		return fmt.Sprintf("(default: %q)", p.Default)
	}
	return fmt.Sprintf("(default: %s)", p.Default)
}

type SubCommand struct {
	*Command
	Parent                 *SubCommand
	SubCommands            []*SubCommand
	SubCommandName         string
	SubCommandStructName   string
	SubCommandFunctionName string
	SubCommandDescription  string
	SubCommandExtendedHelp string
	ImportPath             string
	SubCommandPackageName  string
	UsageFileName          string
	DefinitionFile         string
	DocStart               token.Pos
	DocEnd                 token.Pos
	Parameters             []*FunctionParameter
	ReturnsError           bool
	ReturnCount            int
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
	return sc.MainCmdName + " " + sc.SubCommandSequence()
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

func (sc *SubCommand) MaxDefaultLength() int {
	max := 0
	for _, p := range sc.Parameters {
		l := len(p.DefaultString())
		if l > max {
			max = l
		}
	}
	return max
}
