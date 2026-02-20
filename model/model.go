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

// IsSlice returns true if the type is a slice.
func (p *FunctionParameter) IsSlice() bool {
	return strings.HasPrefix(p.Type, "[]")
}

// HasPointer returns true if the type is a pointer (or slice of pointers).
func (p *FunctionParameter) HasPointer() bool {
	t := p.Type
	t = strings.TrimPrefix(t, "[]")
	return strings.HasPrefix(t, "*")
}

// BaseType returns the underlying type (stripping * and []).
func (p *FunctionParameter) BaseType() string {
	t := p.Type
	t = strings.TrimPrefix(t, "[]")
	t = strings.TrimPrefix(t, "*")
	return t
}

func (p *FunctionParameter) IsBool() bool {
	return p.BaseType() == "bool"
}

func (p *FunctionParameter) IsString() bool {
	return p.BaseType() == "string"
}

func (p *FunctionParameter) IsDuration() bool {
	return p.BaseType() == "time.Duration"
}

func (p *FunctionParameter) ParserCall(valName string) string {
	t := p.BaseType()
	if t == "int" {
		return fmt.Sprintf("strconv.Atoi(%s)", valName)
	}
	if t == "time.Duration" {
		return fmt.Sprintf("time.ParseDuration(%s)", valName)
	}
	if t == "bool" {
		return fmt.Sprintf("strconv.ParseBool(%s)", valName)
	}
	if strings.HasPrefix(t, "int") {
		bits := t[3:]
		if bits == "" {
			bits = "0"
		}
		return fmt.Sprintf("strconv.ParseInt(%s, 10, %s)", valName, bits)
	}
	if strings.HasPrefix(t, "uint") {
		bits := t[4:]
		if bits == "" {
			bits = "64"
		}
		return fmt.Sprintf("strconv.ParseUint(%s, 10, %s)", valName, bits)
	}
	if strings.HasPrefix(t, "float") {
		bits := t[5:]
		return fmt.Sprintf("strconv.ParseFloat(%s, %s)", valName, bits)
	}
	return ""
}

func (p *FunctionParameter) CastCode(valName string) string {
	t := p.BaseType()
	// No cast needed if types match the parser return type
	switch t {
	case "int", "int64", "float64", "bool", "time.Duration", "string":
		return valName
	case "uint64":
		return valName // ParseUint returns uint64
	}
	return fmt.Sprintf("%s(%s)", t, valName)
}

func (p *FunctionParameter) TypeDescription() string {
	t := p.BaseType()
	switch t {
	case "int":
		return "integer"
	case "bool":
		return "boolean"
	case "time.Duration":
		return "duration"
	default:
		return t
	}
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
	for _, p := range sc.Parameters {
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

func (sc *SubCommand) MaxDefaultLength() int {
	max := 0
	for _, p := range sc.Parameters {
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
