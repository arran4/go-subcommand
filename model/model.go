package model

import (
	"fmt"
	"go/token"
	"path"
	"slices"
	"sort"
	"strings"
)

// ReservedKeywords is a list of Go keywords and other reserved words that cannot be used as package names or identifiers
// without collision issues in the generated code.
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

// DataModel represents the parsed data model of the Go files, containing commands and package information.
type DataModel struct {
	// FileSet is the token.FileSet used for parsing.
	FileSet     *token.FileSet
	// PackageName is the name of the package where the main command is defined.
	PackageName string
	// Commands is the list of top-level commands found.
	Commands    []*Command
	// GoVersion is the Go version from go.mod.
	GoVersion   string
}

// Command represents a top-level command.
type Command struct {
	*DataModel
	// MainCmdName is the name of the command (usually derived from the function name).
	MainCmdName        string
	// SubCommands is the list of direct subcommands for this command.
	SubCommands        []*SubCommand
	// PackagePath is the full package path (module path + relative path).
	PackagePath        string
	// ImportPath is the import path for the package containing the command.
	ImportPath         string
	// CommandPackageName is the package name where the command function is defined.
	CommandPackageName string
	// Description is a short description of the command.
	Description        string
	// ExtendedHelp is the long description/help text for the command.
	ExtendedHelp       string
	// FunctionName is the name of the function definition.
	FunctionName       string
	// DefinitionFile is the path to the file where the command is defined.
	DefinitionFile string
	// DocStart is the starting position of the documentation comment.
	DocStart       token.Pos
	// DocEnd is the ending position of the documentation comment.
	DocEnd         token.Pos
	// Parameters is the list of parameters (flags and arguments) for the command.
	Parameters     []*FunctionParameter
	// ReturnsError indicates if the command function returns an error.
	ReturnsError   bool
	// ReturnCount is the number of return values.
	ReturnCount    int
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

// FunctionParameter represents a parameter of a command function, which can be a flag or a positional argument.
type FunctionParameter struct {
	// Name is the name of the parameter in the function signature.
	Name               string
	// Type is the Go type of the parameter (e.g., "string", "int", "[]string").
	Type               string
	// FlagAliases is a list of alternative names for the flag (e.g., "v" for "verbose").
	FlagAliases        []string
	// Default is the default value for the parameter if not provided.
	Default            string
	// Description is the help text for the parameter.
	Description        string
	// IsPositional indicates if the parameter is a positional argument (not a flag).
	IsPositional       bool
	// PositionalArgIndex is the index of the positional argument (0-based).
	PositionalArgIndex int
	// IsVarArg indicates if the parameter captures remaining arguments (variadic).
	IsVarArg           bool
	// VarArgMin is the minimum number of arguments required for a variadic parameter.
	VarArgMin          int
	// VarArgMax is the maximum number of arguments allowed for a variadic parameter.
	VarArgMax          int
	// DeclaredIn is the name of the command where this parameter was originally declared (used for inheritance).
	DeclaredIn         string
	// Required indicates if the parameter is mandatory.
	Required           bool
	// Generator is the name of a function that generates the value for this parameter.
	// If set, the parameter is not parsed from the command line but generated.
	Generator          string
	// ParserFunc is the name of a custom parser function to use for this parameter.
	ParserFunc         string
	// ParserPkg is the package path where the custom parser function is defined.
	ParserPkg          string
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

// SubCommand represents a subcommand within a command tree.
type SubCommand struct {
	*Command
	// Parent points to the parent command.
	Parent                 *SubCommand
	// SubCommands is the list of children subcommands.
	SubCommands            []*SubCommand
	// SubCommandName is the name of this subcommand (the word used in the CLI).
	SubCommandName         string
	// Aliases is a list of alternative names for this subcommand.
	Aliases                []string
	// SubCommandStructName is the name of the generated struct for this subcommand.
	SubCommandStructName   string
	// SubCommandFunctionName is the name of the function that implements this subcommand.
	SubCommandFunctionName string
	// SubCommandDescription is a short description.
	SubCommandDescription  string
	// SubCommandExtendedHelp is the long help text.
	SubCommandExtendedHelp string
	// ImportPath is the import path where the subcommand is defined.
	ImportPath             string
	// SubCommandPackageName is the package name where the subcommand is defined.
	SubCommandPackageName  string
	// UsageFileName is the name of the file containing usage documentation.
	UsageFileName          string
	// DefinitionFile is the path to the source file.
	DefinitionFile         string
	// DocStart is the starting position of the docs.
	DocStart               token.Pos
	// DocEnd is the ending position of the docs.
	DocEnd                 token.Pos
	// Parameters is the list of parameters for this subcommand.
	Parameters             []*FunctionParameter
	// ReturnsError indicates if the function returns an error.
	ReturnsError           bool
	// ReturnCount is the number of return values.
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

// ParameterGroup represents a group of parameters belonging to a specific command level.
// Used for displaying flags grouped by where they are defined (e.g. Global Flags vs Local Flags).
type ParameterGroup struct {
	// CommandName is the name of the command that defines these parameters.
	CommandName string
	// Parameters is the list of parameters in this group.
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
