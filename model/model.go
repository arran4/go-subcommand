package model

import (
	"fmt"
	"go/token"
	"path"
	"slices"
	"sort"
	"strings"
)

var ReservedKeywords = []string{
	"error", "string", "int", "bool", "byte", "rune", "float32", "float64",
	"complex64", "complex128", "uint", "uint8", "uint16", "uint32", "uint64",
	"uintptr", "true", "false", "iota", "nil",
	"break", "default", "func", "interface", "select",
	"case", "defer", "go", "map", "struct",
	"chan", "else", "goto", "package", "switch",
	"const", "fallthrough", "if", "range", "type",
	"continue", "for", "import", "return", "var",
	"strconv", "time", "flag", "fmt", "os", "strings", "slices",
}

type DataModel struct {
	FileSet     *token.FileSet
	PackageName string
	Commands    []*Command
	GoVersion   string
}

type FuncRef struct {
	PackagePath        string
	ImportPath         string
	CommandPackageName string
	FunctionName       string
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
	DefinitionFile     string
	DocStart           token.Pos
	DocEnd             token.Pos
	Parameters         []*FunctionParameter
	ReturnsError       bool
	ReturnCount        int
	UsageFileName      string
}

func (c *Command) ImportAlias() string {
	if c.CommandPackageName == "" || c.CommandPackageName == "main" {
		return ""
	}
	if c.ImportPath == "" {
		return ""
	}
	base := path.Base(c.ImportPath)
	if c.CommandPackageName != base || slices.Contains(ReservedKeywords, c.CommandPackageName) {
		return c.CommandPackageName
	}
	return ""
}

// SourceType defines how a parameter's value is populated.
type SourceType string

const (
	// SourceTypeFlag indicates that the parameter value is derived from a command-line flag.
	// This is the default source type.
	SourceTypeFlag SourceType = "flag"
	// SourceTypeGenerator indicates that the parameter value is produced by a generator function.
	// Parameters with this source type are not exposed as command-line flags.
	// Limitations: The generator function must be defined in the project or imported, and it is executed before the command's action.
	SourceTypeGenerator SourceType = "generator"
)

// ParserType defines how a string value (typically from a flag) is converted to the target type.
type ParserType string

const (
	// ParserTypeImplicit uses the default parsing logic based on the parameter's type (e.g., strconv for integers).
	// It is used when no custom parser is specified.
	ParserTypeImplicit ParserType = "implicit"
	// ParserTypeCustom uses a user-provided function to parse the string value.
	// The function must accept a string and return the target type (and optionally an error).
	ParserTypeCustom ParserType = "custom"
	// ParserTypeIdentity indicates that no parsing is performed.
	// This is typically used when the value is not a string flag, such as when it comes from a generator or is already in the correct format.
	ParserTypeIdentity ParserType = "identity"
)

// GeneratorConfig holds configuration for value generation.
type GeneratorConfig struct {
	// Type specifies the source of the parameter value.
	Type SourceType
	// Func refers to the generator function. It is only required/used when Type is SourceTypeGenerator.
	Func *FuncRef // Non-nil if Type == SourceTypeGenerator
}

// ParserConfig holds configuration for value parsing.
type ParserConfig struct {
	// Type specifies how the parameter value is parsed.
	Type ParserType
	// Func refers to the custom parser function. It is only required/used when Type is ParserTypeCustom.
	Func *FuncRef // Non-nil if Type == ParserTypeCustom
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
	// DeclaredIn specifies the name of the command where this parameter was originally declared.
	// This is used for parameter inheritance and grouping in help output.
	DeclaredIn string
	// IsRequired indicates that the parameter is mandatory.
	// If a required flag is missing, execution will fail.
	IsRequired bool
	// Parser holds the configuration for parsing the parameter value.
	Parser ParserConfig
	// Generator holds the configuration for generating the parameter value.
	Generator GeneratorConfig
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
	if p.IsRequired {
		return "(required)"
	}
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
	if p.Parser.Type == ParserTypeCustom && p.Parser.Func != nil {
		if p.Parser.Func.CommandPackageName != "" {
			return fmt.Sprintf("%s.%s(%s)", p.Parser.Func.CommandPackageName, p.Parser.Func.FunctionName, valName)
		}
		return fmt.Sprintf("%s(%s)", p.Parser.Func.FunctionName, valName)
	}
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

func (sc *SubCommand) ImportAlias() string {
	if sc.SubCommandPackageName == "" || sc.SubCommandPackageName == "main" {
		return ""
	}
	if sc.ImportPath == "" {
		return ""
	}
	base := path.Base(sc.ImportPath)
	if sc.SubCommandPackageName != base || slices.Contains(ReservedKeywords, sc.SubCommandPackageName) {
		return sc.SubCommandPackageName
	}
	return ""
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

// RequiredImports returns a deduplicated list of imports needed for custom parsers.
func (sc *SubCommand) RequiredImports() []string {
	var imports []string
	seen := make(map[string]bool)
	for _, p := range sc.AllParameters() {
		if p.Parser.Type == ParserTypeCustom && p.Parser.Func != nil && p.Parser.Func.ImportPath != "" {
			if !seen[p.Parser.Func.ImportPath] {
				imports = append(imports, p.Parser.Func.ImportPath)
				seen[p.Parser.Func.ImportPath] = true
			}
		}
		if p.Generator.Type == SourceTypeGenerator && p.Generator.Func != nil && p.Generator.Func.ImportPath != "" {
			if !seen[p.Generator.Func.ImportPath] {
				imports = append(imports, p.Generator.Func.ImportPath)
				seen[p.Generator.Func.ImportPath] = true
			}
		}
	}
	sort.Strings(imports)
	return imports
}

func (sc *SubCommand) ResolveInheritance() {
	for _, p := range sc.Parameters {
		if p.DeclaredIn != sc.SubCommandName && p.DeclaredIn != "" {
			ancestor := sc.FindAncestor(p.DeclaredIn)
			if ancestor != nil {
				// Find matching parameter in ancestor
				var parentParam *FunctionParameter
				for _, pp := range ancestor.Parameters {
					if pp.Name == p.Name {
						parentParam = pp
						break
					}
				}

				if parentParam != nil {
					if p.Description == "" {
						p.Description = parentParam.Description
					}
					if p.Default == "" {
						p.Default = parentParam.Default
					}
					if len(p.FlagAliases) == 0 {
						p.FlagAliases = parentParam.FlagAliases
					}
					// IsGlobal removed
					if p.Generator.Type == "" && parentParam.Generator.Type != "" {
						p.Generator = parentParam.Generator
					}
					if p.Parser.Type == "" && parentParam.Parser.Type != "" {
						p.Parser = parentParam.Parser
					}
				}
			} else if sc.Command != nil && sc.MainCmdName == p.DeclaredIn {
				// Declared in Root Command
				for _, pp := range sc.Command.Parameters {
					if pp.Name == p.Name {
						if p.Description == "" {
							p.Description = pp.Description
						}
						if p.Default == "" {
							p.Default = pp.Default
						}
						if len(p.FlagAliases) == 0 {
							p.FlagAliases = pp.FlagAliases
						}
						// IsGlobal removed
						if p.Generator.Type == "" && pp.Generator.Type != "" {
							p.Generator = pp.Generator
						}
						if p.Parser.Type == "" && pp.Parser.Type != "" {
							p.Parser = pp.Parser
						}
						break
					}
				}
			}
		}
	}
	for _, child := range sc.SubCommands {
		child.ResolveInheritance()
	}
}

func (sc *SubCommand) FindAncestor(name string) *SubCommand {
	curr := sc.Parent
	for curr != nil {
		if curr.SubCommandName == name {
			return curr
		}
		curr = curr.Parent
	}
	return nil
}

func (cmd *Command) ResolveInheritance() {
	for _, sc := range cmd.SubCommands {
		sc.ResolveInheritance()
	}
}

// RequiredImports returns a deduplicated list of imports needed for custom parsers.
func (cmd *Command) RequiredImports() []string {
	var imports []string
	seen := make(map[string]bool)
	for _, p := range cmd.Parameters {
		if p.Parser.Type == ParserTypeCustom && p.Parser.Func != nil && p.Parser.Func.ImportPath != "" {
			if !seen[p.Parser.Func.ImportPath] {
				imports = append(imports, p.Parser.Func.ImportPath)
				seen[p.Parser.Func.ImportPath] = true
			}
		}
		if p.Generator.Type == SourceTypeGenerator && p.Generator.Func != nil && p.Generator.Func.ImportPath != "" {
			if !seen[p.Generator.Func.ImportPath] {
				imports = append(imports, p.Generator.Func.ImportPath)
				seen[p.Generator.Func.ImportPath] = true
			}
		}
	}
	sort.Strings(imports)
	return imports
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
		appendFlagsUsage(&parts, sc.Command.Parameters)
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
		appendFlagsUsage(&parts, cmd.Parameters)
	}

	// Add positional arguments for the LEAF command (sc)
	// We only show positionals for the command we are running.
	for _, p := range sc.Parameters {
		if p.IsPositional {
			if p.IsRequired {
				if p.IsVarArg {
					parts = append(parts, fmt.Sprintf("<%s...>", p.Name))
				} else {
					parts = append(parts, fmt.Sprintf("<%s>", p.Name))
				}
			} else {
				if p.IsVarArg {
					parts = append(parts, fmt.Sprintf("[%s...]", p.Name))
				} else {
					parts = append(parts, fmt.Sprintf("[%s]", p.Name))
				}
			}
		}
	}

	if len(sc.SubCommands) > 0 {
		parts = append(parts, "<subcommand>")
	}

	return strings.Join(parts, " ")
}

func appendFlagsUsage(parts *[]string, parameters []*FunctionParameter) {
	var flags []*FunctionParameter
	for _, p := range parameters {
		if !p.IsPositional {
			flags = append(flags, p)
		}
	}

	if len(flags) == 0 {
		return
	}

	if len(flags) <= 3 {
		for _, f := range flags {
			name := f.Name
			if len(f.FlagAliases) > 0 {
				// Pick the shortest or first alias
				name = f.FlagAliases[0]
				for _, alias := range f.FlagAliases {
					if len(alias) < len(name) {
						name = alias
					}
				}
			}
			prefix := "-"
			if len(name) > 1 {
				prefix = "--"
			}

			if f.IsBool() {
				*parts = append(*parts, fmt.Sprintf("[%s%s]", prefix, name))
			} else {
				*parts = append(*parts, fmt.Sprintf("[%s%s <%s>]", prefix, name, f.Name)) // Use param name as value placeholder
			}
		}
	} else {
		*parts = append(*parts, "[flags...]")
	}
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
