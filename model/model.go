package model

import (
	"fmt"
	"go/token"
	"sort"
	"strings"
)

type DataModel struct {
	FileSet     *token.FileSet
	PackageName string
	Commands    []*Command
	GoVersion   string
}

type Command struct {
	*DataModel
	MainCmdName        string
	SubCommands        []*SubCommand
	PackagePath        string
	ImportPath         string
	CommandPackageName string
	Description        string
	ExtendedHelp       string
	FunctionName       string
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
	DeclaredIn         string
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
	def := p.Default
	if p.Type == "string" && !strings.HasPrefix(def, "\"") {
		def = fmt.Sprintf("%q", def)
	}
	return fmt.Sprintf("(default: %s)", def)
}

type SubCommand struct {
	*Command
	Parent                 *SubCommand
	SubCommands            []*SubCommand
	SubCommandName         string
	Aliases                []string
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
	for _, p := range sc.AllParameters() {
		l := len(p.FlagString())
		if l > max {
			max = l
		}
	}
	return max
}

func (sc *SubCommand) AllParameters() []*FunctionParameter {
	var params []*FunctionParameter
	seen := make(map[string]bool)

	addParams := func(ps []*FunctionParameter) {
		for _, p := range ps {
			if !seen[p.Name] {
				seen[p.Name] = true
				params = append(params, p)
			}
		}
	}

	current := sc
	for current != nil {
		addParams(current.Parameters)
		current = current.Parent
	}

	if sc.Command != nil {
		addParams(sc.Command.Parameters)
	}
	return params
}

type ParameterGroup struct {
	CommandName string
	Parameters  []*FunctionParameter
}

func (sc *SubCommand) ParameterGroups() []ParameterGroup {
	allParams := sc.AllParameters()
	grouped := make(map[string][]*FunctionParameter)
	for _, p := range allParams {
		if p.IsPositional {
			continue
		}
		grouped[p.DeclaredIn] = append(grouped[p.DeclaredIn], p)
	}

	var groups []ParameterGroup

	// Traverse from root to current to ensure order
	var stack []*SubCommand
	current := sc
	for current != nil {
		stack = append(stack, current)
		current = current.Parent
	}

	// Add root command (SubCommand.Command)
	if sc.Command != nil {
		name := sc.MainCmdName
		if params, ok := grouped[name]; ok {
			groups = append(groups, ParameterGroup{
				CommandName: name,
				Parameters:  params,
			})
			delete(grouped, name)
		}
	}

	// Iterate stack in reverse (Root -> Parent -> Child)
	for i := len(stack) - 1; i >= 0; i-- {
		cmd := stack[i]
		name := cmd.SubCommandName
		if params, ok := grouped[name]; ok {
			groups = append(groups, ParameterGroup{
				CommandName: name,
				Parameters:  params,
			})
			delete(grouped, name)
		}
	}

	// Add any remaining groups (shouldn't happen if DeclaredIn is correct)
	var keys []string
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		params := grouped[name]
		if name == "" {
			name = "unknown"
		}
		groups = append(groups, ParameterGroup{
			CommandName: name,
			Parameters:  params,
		})
	}

	return groups
}

func (sc *SubCommand) FullUsageString() string {
	var parts []string

	// Add root command
	if sc.Command != nil {
		parts = append(parts, sc.MainCmdName)
		hasFlags := false
		for _, p := range sc.Command.Parameters {
			if !p.IsPositional {
				hasFlags = true
				break
			}
		}
		if hasFlags {
			parts = append(parts, "[flags...]")
		}
	}

	// Traverse from root to current
	var stack []*SubCommand
	current := sc
	for current != nil {
		stack = append(stack, current)
		current = current.Parent
	}

	for i := len(stack) - 1; i >= 0; i-- {
		cmd := stack[i]
		parts = append(parts, cmd.SubCommandName)

		// Check for flags declared in THIS command
		hasFlags := false
		for _, p := range cmd.Parameters {
			if !p.IsPositional {
				hasFlags = true
				break
			}
		}

		if hasFlags {
			parts = append(parts, "[flags...]")
		}
	}

	// Add positional arguments for the LEAF command (sc)
	// We only show positionals for the command we are running.
	for _, p := range sc.Parameters {
		if p.IsPositional {
			if p.IsVarArg {
				parts = append(parts, fmt.Sprintf("[%s...]", p.Name))
			} else {
				parts = append(parts, fmt.Sprintf("<%s>", p.Name))
			}
		}
	}

	if len(sc.SubCommands) > 0 {
		parts = append(parts, "<subcommand>")
	}

	return strings.Join(parts, " ")
}

func (sc *SubCommand) MaxDefaultLength() int {
	max := 0
	for _, p := range sc.AllParameters() {
		if p.IsPositional {
			continue
		}
		l := len(p.DefaultString())
		if l > max {
			max = l
		}
	}
	return max
}
