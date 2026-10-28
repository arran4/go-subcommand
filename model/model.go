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

// Provenance holds metadata about the code generation process.
type Provenance struct {
	Version           string
	Commit            string
	Dirty             bool
	ReplacedTemplates bool
	TemplateID        string
	ProjectCommit     string
	ProjectDirty      bool
	Timestamp         string
}

// DataModel represents the parsed data model of the Go files, containing commands and package information.
type DataModel struct {
	// FileSet is the token.FileSet used for parsing.
	FileSet *token.FileSet
	// PackageName is the name of the package where the main command is defined.
	PackageName string
	// Commands is the list of top-level commands found.
	Commands []*Command
	// GoVersion is the Go version from go.mod.
	GoVersion string
	// Provenance holds code generation metadata.
	Provenance *Provenance
}

type SourceType string

const (
	SourceTypeFlag      SourceType = "flag"
	SourceTypeGenerator SourceType = "generator"
)

type ParserType string

const (
	ParserTypeImplicit ParserType = "implicit"
	ParserTypeCustom   ParserType = "custom"
	ParserTypeIdentity ParserType = "identity"
)

type FuncRef struct {
	PackagePath        string
	ImportPath         string
	CommandPackageName string
	FunctionName       string
}

type GeneratorConfig struct {
	Type SourceType
	Func *FuncRef
	// Inputs names parameters made available to a provider. They are resolved
	// within the command's generated field graph, not through globals.
	Inputs []string
}

type ParserConfig struct {
	Type ParserType
	Func *FuncRef
}

// Command represents a top-level command.
type Command struct {
	*DataModel
	// MainCmdName is the name of the command (usually derived from the function name).
	MainCmdName string
	// SubCommands is the list of direct subcommands for this command.
	SubCommands []*SubCommand
	// PackagePath is the full package path (module path + relative path).
	PackagePath string
	// ImportPath is the import path for the package containing the command.
	ImportPath string
	// CommandPackageName is the package name where the command function is defined.
	CommandPackageName string
	// Description is a short description of the command.
	Description string
	// ExtendedHelp is the long description/help text for the command.
	ExtendedHelp string
	// FunctionName is the name of the function definition.
	FunctionName string
	// DefinitionFile is the path to the file where the command is defined.
	DefinitionFile string
	// DocStart is the starting position of the documentation comment.
	DocStart token.Pos
	// DocEnd is the ending position of the documentation comment.
	DocEnd token.Pos
	// Parameters is the list of parameters (flags and arguments) for the command.
	Parameters []*FunctionParameter
	// ReturnsError indicates if the command function returns an error.
	ReturnsError bool
	// ReturnCount is the number of return values.
	ReturnCount int
	// Aliases holds aliases defined on the application root.
	Aliases []string
	// DefaultRoute specifies an explicit default route for bare invocation (e.g. "help simple" or "tools docs simple").
	DefaultRoute string
	// CLIParser is the explicit runtime CLI parser backend selected for this command.
	CLIParser string
	// ResolvedCLIParser is the validated runtime CLI parser backend after inheritance and generator defaults.
	ResolvedCLIParser string
	// SuppressGoGenerate omits the nested regeneration directive from generated
	// main.go. Its zero value retains the compatible default of emitting it.
	SuppressGoGenerate bool
}

// EffectiveCLIParser returns the nearest explicit parser selection. It does not
// apply a generator default or validate that a backend is implemented.
func (c *Command) EffectiveCLIParser() string {
	if c == nil {
		return ""
	}
	return c.CLIParser
}

// FunctionParameter represents a parameter of a command function, which can be a flag or a positional argument.

func (c *Command) ImportAlias() string {
	if c == nil || c.CommandPackageName == "" || c.CommandPackageName == "main" {
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

func (c *Command) CallPackage() string {
	if c == nil || c.CommandPackageName == "" || c.CommandPackageName == "main" {
		return ""
	}
	alias := c.ImportAlias()
	if alias != "" {
		return alias
	}
	return c.CommandPackageName
}

func (c *Command) HasAction() bool {
	return c != nil && c.FunctionName != ""
}

type FunctionParameter struct {
	// Name is the name of the parameter in the function signature.
	Name string
	// Type is the Go type of the parameter (e.g., "string", "int", "[]string").
	Type string
	// FlagAliases is a list of alternative names for the flag (e.g., "v" for "verbose").
	FlagAliases []string
	// Default is the default value for the parameter if not provided.
	Default string
	// HasDefaultValue indicates if a default value was explicitly provided.
	HasDefaultValue bool
	// Description is the help text for the parameter.
	Description string
	// IsPositional indicates if the parameter is a positional argument (not a flag).
	IsPositional bool
	// PositionalArgIndex is the index of the positional argument (0-based).
	PositionalArgIndex int
	// IsVarArg indicates if the parameter captures remaining arguments (variadic).
	IsVarArg bool
	// VarArgMin is the minimum number of arguments required for a variadic parameter.
	VarArgMin int
	// VarArgMax is the maximum number of arguments allowed for a variadic parameter.
	VarArgMax int
	// DeclaredIn is the name of the command where this parameter was originally declared (used for inheritance).
	DeclaredIn string
	// Required indicates if the parameter is mandatory.
	Required bool
	// Generator specifies the source of the parameter value.
	Generator GeneratorConfig
	// Parser holds configuration for value parsing.
	Parser ParserConfig
	// DefaultExpr holds metadata if the default value is a function call or constant expression.
	DefaultExpr *FuncRef
	// Inherited indicates if the parameter was inherited from a parent command.
	Inherited bool
	// InheritedFrom is the parent parameter name referenced by a differently named child parameter.
	// It is parser metadata and is intentionally excluded from serialized models.
	InheritedFrom string `json:"-"`

	TypeImportPath  string
	TypePackageName string
	TypeName        string
}

func (dm *DataModel) Validate() error {
	for _, cmd := range dm.Commands {
		if err := cmd.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (cmd *Command) Validate() error {
	if cmd.DefaultRoute != "" {
		if err := cmd.validateDefaultRoute(); err != nil {
			return err
		}
	}
	if err := cmd.validateRootAliases(); err != nil {
		return err
	}
	if err := validateParameters(cmd.Parameters, cmd.MainCmdName); err != nil {
		return err
	}
	if err := validateSubCommandCollisions(cmd.MainCmdName, true, cmd.SubCommands); err != nil {
		return err
	}
	for _, sc := range cmd.SubCommands {
		if err := sc.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (cmd *Command) validateDefaultRoute() error {
	parts := cmd.DefaultRouteParts()
	if len(parts) == 0 {
		return fmt.Errorf("command %s: empty default route", cmd.MainCmdName)
	}

	// Self-reference check:
	if len(parts) == 1 && strings.EqualFold(parts[0], cmd.MainCmdName) {
		return fmt.Errorf("command %s: default route cannot reference the root command itself", cmd.MainCmdName)
	}

	if cmd.IsHelpDefaultRoute() {
		targetPath := parts[1:]
		if len(targetPath) == 0 {
			// Bare "help" or "usage"
			return nil
		}
		targetSc := cmd.FindSubCommand(targetPath)
		if targetSc == nil {
			return fmt.Errorf("command %s: default route target %q not found in command tree", cmd.MainCmdName, cmd.DefaultRoute)
		}
		return nil
	}

	// An action default route is ambiguous if the root command already defines its own action
	if cmd.FunctionName != "" {
		return fmt.Errorf("command %s: cannot declare default route %q when root command already has an action (%s)", cmd.MainCmdName, cmd.DefaultRoute, cmd.FunctionName)
	}

	targetSc := cmd.FindSubCommand(parts)
	if targetSc == nil {
		return fmt.Errorf("command %s: default route target %q not found in command tree", cmd.MainCmdName, cmd.DefaultRoute)
	}
	if targetSc.IsAppRoot() {
		return fmt.Errorf("command %s: default route cycle detected", cmd.MainCmdName)
	}
	return nil
}

func (cmd *Command) validateRootAliases() error {
	seenAliases := make(map[string]bool)
	for _, rawAlias := range cmd.Aliases {
		alias := strings.TrimSpace(rawAlias)
		aliasLower := strings.ToLower(alias)
		if aliasLower == "" {
			return fmt.Errorf("command %s: invalid empty alias on root command", cmd.MainCmdName)
		}
		segments := strings.Fields(alias)
		if len(segments) == 1 && strings.EqualFold(segments[0], cmd.MainCmdName) {
			return fmt.Errorf("command %s: alias %q cannot alias the root command itself", cmd.MainCmdName, rawAlias)
		}
		if strings.EqualFold(segments[0], "help") || strings.EqualFold(segments[0], "usage") || strings.EqualFold(segments[0], "version") {
			return fmt.Errorf("command %s: alias %q conflicts with built-in command %q", cmd.MainCmdName, rawAlias, strings.ToLower(segments[0]))
		}
		if seenAliases[aliasLower] {
			return fmt.Errorf("command %s: duplicate alias %q on root command", cmd.MainCmdName, rawAlias)
		}
		seenAliases[aliasLower] = true

		if cmd.FunctionName == "" {
			return fmt.Errorf("command %s: aliases (%s) cannot be declared on root command without an action", cmd.MainCmdName, rawAlias)
		}

		if existing := cmd.FindSubCommand(segments); existing != nil {
			if existing.SubCommandFunctionName != "" && existing.SubCommandFunctionName != cmd.FunctionName {
				return fmt.Errorf("command %s: alias %q on root command conflicts with subcommand %q", cmd.MainCmdName, rawAlias, strings.Join(segments, " "))
			}
		}
	}
	return nil
}

func (sc *SubCommand) Validate() error {
	for _, alias := range sc.Aliases {
		if strings.EqualFold(alias, sc.SubCommandName) {
			return fmt.Errorf("command %s: alias %q on subcommand %q cannot alias itself", sc.ProgName(), alias, sc.SubCommandName)
		}
	}
	if err := validateParameters(sc.Parameters, sc.SubCommandName); err != nil {
		return err
	}
	if err := validateSubCommandCollisions(sc.ProgName(), false, sc.SubCommands); err != nil {
		return err
	}
	for _, child := range sc.SubCommands {
		if err := child.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// HasIOParameters reports whether this command tree needs generated file
// acquisition helpers for reader or writer parameters.
func (cmd *Command) HasIOParameters() bool {
	for _, p := range cmd.Parameters {
		if p.IsReader() || p.IsWriter() {
			return true
		}
	}
	for _, sc := range cmd.SubCommands {
		if sc.HasIOParameters() {
			return true
		}
	}
	return false
}

// HasIOParameters reports whether this subcommand tree needs generated file
// acquisition helpers for reader or writer parameters.
func (sc *SubCommand) HasIOParameters() bool {
	for _, p := range sc.Parameters {
		if p.IsReader() || p.IsWriter() {
			return true
		}
	}
	for _, child := range sc.SubCommands {
		if child.HasIOParameters() {
			return true
		}
	}
	return false
}

func validateSubCommandCollisions(parentName string, isRoot bool, subCommands []*SubCommand) error {
	seenCanonical := make(map[string]string)
	for _, sc := range subCommands {
		canonLower := strings.ToLower(sc.SubCommandName)
		if canonLower == "help" || canonLower == "usage" || (isRoot && canonLower == "version") {
			return fmt.Errorf("command %s: subcommand %q conflicts with built-in command %q", parentName, sc.SubCommandName, canonLower)
		}
		if _, ok := seenCanonical[sc.SubCommandName]; ok {
			return fmt.Errorf("command %s: duplicate subcommand %q", parentName, sc.SubCommandName)
		}
		seenCanonical[sc.SubCommandName] = sc.SubCommandName
	}

	seenAliases := make(map[string]string)
	for _, sc := range subCommands {
		seenChildAliases := make(map[string]bool)
		for _, rawAlias := range sc.Aliases {
			alias := strings.TrimSpace(rawAlias)
			aliasLower := strings.ToLower(alias)
			if aliasLower == "" || strings.ContainsAny(aliasLower, " \t\r\n") {
				return fmt.Errorf("command %s: invalid alias %q on subcommand %q", parentName, rawAlias, sc.SubCommandName)
			}
			if aliasLower == "help" || aliasLower == "usage" || (isRoot && aliasLower == "version") {
				return fmt.Errorf("command %s: alias %q on subcommand %q conflicts with built-in command %q", parentName, rawAlias, sc.SubCommandName, aliasLower)
			}
			for canon := range seenCanonical {
				if strings.EqualFold(alias, canon) {
					return fmt.Errorf("command %s: alias %q on subcommand %q conflicts with subcommand %q", parentName, rawAlias, sc.SubCommandName, canon)
				}
			}
			if seenChildAliases[aliasLower] {
				return fmt.Errorf("command %s: duplicate alias %q on subcommand %q", parentName, rawAlias, sc.SubCommandName)
			}
			if otherOwner, exists := seenAliases[aliasLower]; exists {
				return fmt.Errorf("command %s: alias %q on subcommand %q conflicts with alias on subcommand %q", parentName, rawAlias, sc.SubCommandName, otherOwner)
			}
			seenChildAliases[aliasLower] = true
			seenAliases[aliasLower] = sc.SubCommandName
		}
	}
	return nil
}

func validateParameters(params []*FunctionParameter, cmdName string) error {
	var hasOptionalPos bool
	var hasVarArg bool
	for _, p := range params {
		if !p.IsPositional {
			continue
		}

		if p.IsVarArg {
			hasVarArg = true
			continue
		}

		if hasVarArg {
			return fmt.Errorf("command %s: positional argument %s appears after a variadic argument", cmdName, p.Name)
		}

		if p.HasDefaultValue {
			hasOptionalPos = true
		} else {
			if hasOptionalPos {
				return fmt.Errorf("command %s: required positional argument %s appears after an optional positional argument", cmdName, p.Name)
			}
		}
	}

	for _, p := range params {
		if p.IsFile() {
			return fmt.Errorf("parameter %q has type *os.File but no implicit file access mode; use io.Reader/io.ReadCloser for input, io.Writer/io.WriteCloser for output, or configure an explicit provider", p.Name)
		}
	}
	return validateGeneratorDependencies(params, cmdName)
}

func validateGeneratorDependencies(params []*FunctionParameter, cmdName string) error {
	all := make(map[string]*FunctionParameter)
	providers := make(map[string]*FunctionParameter)
	for _, param := range params {
		all[param.Name] = param
		if param.HasGenerator() && !param.Inherited {
			providers[param.Name] = param
		}
	}
	state := make(map[string]uint8)
	var visit func(*FunctionParameter) error
	visit = func(provider *FunctionParameter) error {
		switch state[provider.Name] {
		case 1:
			return fmt.Errorf("command %s: generator dependency cycle includes %s", cmdName, provider.Name)
		case 2:
			return nil
		}
		state[provider.Name] = 1
		for _, input := range provider.Generator.Inputs {
			param, ok := all[input]
			if !ok {
				return fmt.Errorf("command %s: generator %s references unknown dependency %s", cmdName, provider.Name, input)
			}
			if dependency := providers[param.Name]; dependency != nil {
				if err := visit(dependency); err != nil {
					return err
				}
			}
		}
		state[provider.Name] = 2
		return nil
	}
	for _, provider := range providers {
		if err := visit(provider); err != nil {
			return err
		}
	}
	return nil
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

func (p *FunctionParameter) PrimaryFlagName() string {
	name := p.Name
	if len(p.FlagAliases) > 0 {
		name = p.FlagAliases[0]
	}
	if len(name) > 1 {
		return "--" + name
	}
	return "-" + name
}

// ValueFieldName returns the generated command field that supplies this argument.
func (p *FunctionParameter) ValueFieldName() string {
	if p.InheritedFrom != "" {
		return p.InheritedFrom
	}
	return p.Name
}

func (p *FunctionParameter) DefaultString() string {
	if p.Required {
		return "(required)"
	}
	// For flags, we historically printed empty string if Default is empty string and HasDefaultValue is false
	// Let's restore the original logic for formatting flag output correctly.
	if p.Default == "" && !p.HasDefaultValue {
		return ""
	}
	def := p.Default
	if p.Type == "string" && !strings.HasPrefix(def, "\"") {
		def = fmt.Sprintf("%q", def)
	}
	if p.Type == "string" && def == "\"\"" && !p.HasDefaultValue {
		return ""
	}
	if def == "" && p.HasDefaultValue {
		def = "\"\""
	} else if def == "" {
		return ""
	}
	return fmt.Sprintf("(default: %s)", def)
}

// UsageDefaultString returns the default text shown in generated usage. IO
// parameters use '-' as a stream selector, so their help must describe the
// stream rather than leave users to infer that '-' is not a literal filename.
func (p *FunctionParameter) UsageDefaultString() string {
	if p.HasDefaultValue && p.Default == "-" {
		if p.IsReader() {
			return "(default: stdin via '-')"
		}
		if p.IsWriter() {
			return "(default: stdout via '-')"
		}
	}
	return p.DefaultString()
}

// UsageDescription returns help text with the explicit standard-stream
// spelling for IO parameters. Other parameter descriptions remain unchanged.
func (p *FunctionParameter) UsageDescription() string {
	stream := ""
	if p.IsReader() {
		stream = "stdin"
	} else if p.IsWriter() {
		stream = "stdout"
	}
	if stream == "" {
		return p.Description
	}
	if p.Description == "" {
		return fmt.Sprintf("Use '-' for %s", stream)
	}
	return fmt.Sprintf("%s; use '-' for %s", p.Description, stream)
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

// BaseType returns the underlying Go type spelling (stripping * and []).
// It deliberately preserves source import qualifiers so the result remains
// valid Go syntax (for example, stream.Reader).
func (p *FunctionParameter) BaseType() string {
	t := p.Type
	t = strings.TrimPrefix(t, "[]")
	t = strings.TrimPrefix(t, "*")
	return t
}

// CanonicalBaseType returns the semantic identity of the underlying type.
// Unlike BaseType, an imported type is qualified by its import path and must
// not be emitted as Go source.
func (p *FunctionParameter) CanonicalBaseType() string {
	if p.TypeImportPath != "" && p.TypeName != "" {
		return p.TypeImportPath + "." + p.TypeName
	}
	return p.BaseType()
}

// GeneratedType returns the valid Go type spelling used in generated files.
// Built-in IO provider types use the generator's canonical io identifier;
// other imported types retain their source qualifier.
func (p *FunctionParameter) GeneratedType() string {
	if !p.IsReader() && !p.IsWriter() {
		return p.Type
	}
	prefix := strings.TrimSuffix(p.Type, p.BaseType())
	return prefix + p.CanonicalBaseType()
}

func (p *FunctionParameter) IsBool() bool {
	return p.BaseType() == "bool"
}

func (p *FunctionParameter) IsString() bool {
	return p.BaseType() == "string"
}

func (p *FunctionParameter) IsDuration() bool {
	return p.CanonicalBaseType() == "time.Duration"
}

func (p *FunctionParameter) IsWriter() bool {
	return p.CanonicalBaseType() == "io.Writer" || p.CanonicalBaseType() == "io.WriteCloser"
}

func (p *FunctionParameter) IsReader() bool {
	return p.CanonicalBaseType() == "io.Reader" || p.CanonicalBaseType() == "io.ReadCloser"
}

func (p *FunctionParameter) IsCloser() bool {
	return p.CanonicalBaseType() == "io.WriteCloser" || p.CanonicalBaseType() == "io.ReadCloser" || p.CanonicalBaseType() == "os.File"
}

func (p *FunctionParameter) IsFile() bool {
	return p.CanonicalBaseType() == "os.File"
}

func (p *FunctionParameter) HasCustomParser() bool {
	return p.Parser.Type == ParserTypeCustom && p.Parser.Func != nil && p.Parser.Func.FunctionName != ""
}

func (p *FunctionParameter) ParserCall(valName string) string {
	if p.HasCustomParser() {
		if p.Parser.Func.CommandPackageName != "" {
			return fmt.Sprintf("%s.%s(%s)", p.Parser.Func.CommandPackageName, p.Parser.Func.FunctionName, valName)
		}
		return fmt.Sprintf("%s(%s)", p.Parser.Func.FunctionName, valName)
	}

	t := p.BaseType()
	if p.IsWriter() || p.IsReader() || p.IsFile() {
		// Handled specially in templates
		return ""
	}
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
	t := p.CanonicalBaseType()
	switch t {
	case "int":
		return "integer"
	case "bool":
		return "boolean"
	case "time.Duration":
		return "duration"
	case "io.Writer", "io.WriteCloser":
		return "file path (or '-' for stdout)"
	case "io.Reader", "io.ReadCloser":
		return "file path (or '-' for stdin)"
	default:
		return t
	}
}

// SubCommand represents a subcommand within a command tree.
type SubCommand struct {
	*Command
	// Parent points to the parent command.
	Parent *SubCommand
	// SubCommands is the list of children subcommands.
	SubCommands []*SubCommand
	// SubCommandName is the name of this subcommand (the word used in the CLI).
	SubCommandName string
	// Aliases is a list of alternative names for this subcommand.
	Aliases []string
	// DefaultRoute specifies an explicit default route configured on or inherited by this subcommand.
	DefaultRoute string
	// IsDefault indicates whether this subcommand is marked as the application's default command on bare invocation.
	IsDefault bool
	// SubCommandStructName is the name of the generated struct for this subcommand.
	SubCommandStructName string
	// ConstructorMethodName is the name of the generated constructor method.
	ConstructorMethodName string
	// SubCommandFunctionName is the name of the function that implements this subcommand.
	SubCommandFunctionName string
	// SubCommandDescription is a short description.
	SubCommandDescription string
	// SubCommandExtendedHelp is the long help text.
	SubCommandExtendedHelp string
	// ImportPath is the import path where the subcommand is defined.
	ImportPath string
	// SubCommandPackageName is the package name where the subcommand is defined.
	SubCommandPackageName string
	// UsageFileName is the name of the file containing usage documentation.
	UsageFileName string
	// DefinitionFile is the path to the source file.
	DefinitionFile string
	// DocStart is the starting position of the docs.
	DocStart token.Pos
	// DocEnd is the ending position of the docs.
	DocEnd token.Pos
	// Parameters is the list of parameters for this subcommand.
	Parameters []*FunctionParameter
	// ReturnsError indicates if the function returns an error.
	ReturnsError bool
	// ReturnCount is the number of return values.
	ReturnCount int
	// CLIParser is the explicit runtime CLI parser backend selected for this subcommand.
	CLIParser string
	// ResolvedCLIParser is the validated runtime CLI parser backend after inheritance and generator defaults.
	ResolvedCLIParser string
}

// EffectiveCLIParser returns the nearest explicit parser selection. It does not
// apply a generator default or validate that a backend is implemented.
func (sc *SubCommand) EffectiveCLIParser() string {
	if sc == nil {
		return ""
	}
	if sc.CLIParser != "" {
		return sc.CLIParser
	}
	if sc.Parent != nil {
		return sc.Parent.EffectiveCLIParser()
	}
	if sc.Command != nil {
		return sc.Command.EffectiveCLIParser()
	}
	return ""
}

func (sc *SubCommand) ImportAlias() string {
	if sc == nil || sc.SubCommandPackageName == "" || sc.SubCommandPackageName == "main" {
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

func (sc *SubCommand) CallPackage() string {
	if sc == nil || sc.SubCommandPackageName == "" || sc.SubCommandPackageName == "main" {
		return ""
	}
	alias := sc.ImportAlias()
	if alias != "" {
		return alias
	}
	return sc.SubCommandPackageName
}

func (sc *SubCommand) HasAction() bool {
	return sc != nil && sc.SubCommandFunctionName != ""
}

// IsAppRoot reports whether c represents the application-level root command.
func (c *Command) IsAppRoot() bool {
	return c != nil
}

// CommandPath returns the subcommand path segments for the application root (always nil).
func (c *Command) CommandPath() []string {
	return nil
}

// DefaultRouteParts splits DefaultRoute into whitespace-delimited tokens.
func (c *Command) DefaultRouteParts() []string {
	if c == nil || c.DefaultRoute == "" {
		return nil
	}
	return strings.Fields(c.DefaultRoute)
}

// IsHelpDefaultRoute reports whether DefaultRoute targets the built-in help system.
func (c *Command) IsHelpDefaultRoute() bool {
	parts := c.DefaultRouteParts()
	return len(parts) > 0 && (strings.EqualFold(parts[0], "help") || strings.EqualFold(parts[0], "usage"))
}

// FindSubCommand looks up a subcommand path (e.g. ["tools", "docs", "simple"]) within the command tree,
// resolving canonical names and aliases at each intermediate and leaf step.
func (c *Command) FindSubCommand(path []string) *SubCommand {
	if c == nil || len(path) == 0 {
		return nil
	}
	currentList := c.SubCommands
	var found *SubCommand
	for _, segment := range path {
		found = nil
		for _, sc := range currentList {
			if strings.EqualFold(sc.SubCommandName, segment) {
				found = sc
				break
			}
			for _, alias := range sc.Aliases {
				if strings.EqualFold(alias, segment) {
					found = sc
					break
				}
			}
		}
		if found == nil {
			return nil
		}
		currentList = found.SubCommands
	}
	return found
}

// IsAppRoot reports whether sc is the synthetic application-root usage wrapper
// rather than a subcommand on a command path (even if that subcommand is named "root").
func (sc *SubCommand) IsAppRoot() bool {
	return sc != nil && sc.Parent == nil && sc.SubCommandName == ""
}

// CommandPath returns the explicit subcommand path segments relative to the application root.
func (sc *SubCommand) CommandPath() []string {
	if sc == nil || sc.IsAppRoot() {
		return nil
	}
	if sc.Parent == nil || sc.Parent.IsAppRoot() {
		return []string{sc.SubCommandName}
	}
	parentPath := sc.Parent.CommandPath()
	path := make([]string, 0, len(parentPath)+1)
	path = append(path, parentPath...)
	path = append(path, sc.SubCommandName)
	return path
}

func (sc *SubCommand) SubCommandSequence() string {
	return strings.Join(sc.CommandPath(), " ")
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
	if sc == nil {
		return ""
	}
	prefix := ""
	if sc.Command != nil && sc.MainCmdName != "" {
		prefix = sc.MainCmdName + " "
	}
	return prefix + sc.SubCommandSequence()
}

func (sc *SubCommand) MaxFlagLength() int {
	max := 0
	for _, p := range sc.AllParameters() {
		if p.HasGenerator() {
			continue
		}
		l := len(p.FlagString())
		if l > max {
			max = l
		}
	}
	return max
}

func requiredFlagPresent(params []*FunctionParameter) bool {
	for _, p := range params {
		if p.Required && !p.Inherited && !p.IsPositional && !p.HasGenerator() {
			return true
		}
	}
	return false
}

func (cmd *Command) HasRequiredFlags() bool {
	return requiredFlagPresent(cmd.Parameters)
}

func (sc *SubCommand) HasRequiredFlags() bool {
	return requiredFlagPresent(sc.Parameters)
}

func (sc *SubCommand) ResolveInheritance() {
	for _, p := range sc.Parameters {
		if p.DeclaredIn != sc.SubCommandName && p.DeclaredIn != "" {
			ancestor := sc.FindAncestor(p.DeclaredIn)
			if ancestor != nil {
				// Find matching parameter in ancestor
				var parentParam *FunctionParameter
				for _, pp := range ancestor.Parameters {
					if pp.Name == p.Name || pp.Name == p.InheritedFrom {
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
					if pp.Name == p.Name || pp.Name == p.InheritedFrom {
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
			key := p.Name
			if p.InheritedFrom != "" {
				key = p.InheritedFrom
			}
			if !seen[key] {
				seen[key] = true
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
// Used for displaying flags grouped by where they are defined.
type ParameterGroup struct {
	// CommandName is the name of the command that defines these parameters.
	CommandName string
	// Parameters is the list of parameters in this group.
	Parameters []*FunctionParameter
}

func (sc *SubCommand) ParameterGroups() []ParameterGroup {
	allParams := sc.AllParameters()
	grouped := make(map[string][]*FunctionParameter)
	for _, p := range allParams {
		if p.IsPositional || p.HasGenerator() {
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

func appendFlagsUsage(parts *[]string, params []*FunctionParameter) {
	hasFlags := false
	for _, p := range params {
		if !p.IsPositional && !p.HasGenerator() {
			hasFlags = true
			break
		}
	}
	if hasFlags {
		*parts = append(*parts, "[flags...]")
	}
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
		if cmd.SubCommandName == "" {
			continue
		}
		parts = append(parts, cmd.SubCommandName)
		appendFlagsUsage(&parts, cmd.Parameters)
	}

	// Add positional arguments for the LEAF command (sc)
	// We only show positionals for the command we are running.
	for _, p := range sc.Parameters {
		if p.IsPositional {
			if p.IsVarArg {
				parts = append(parts, fmt.Sprintf("[%s...]", p.Name))
			} else if p.HasDefaultValue {
				parts = append(parts, fmt.Sprintf("[%s]", p.Name))
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
		if p.IsPositional || p.HasGenerator() {
			continue
		}
		l := len(p.DefaultString())
		if l > max {
			max = l
		}
	}
	return max
}

// MaxUsageDefaultLength reports the display width of defaults after applying
// usage-specific stream descriptions.
func (sc *SubCommand) MaxUsageDefaultLength() int {
	max := 0
	for _, p := range sc.AllParameters() {
		if p.IsPositional || p.HasGenerator() {
			continue
		}
		l := len(p.UsageDefaultString())
		if l > max {
			max = l
		}
	}
	return max
}

func (p *FunctionParameter) HasGenerator() bool {
	return p.Generator.Type == SourceTypeGenerator
}

func (p *FunctionParameter) GeneratorCall() string {
	return p.GeneratorCallFor("", false)
}

// GeneratorCallFor renders a provider invocation using fields owned by receiver.
// Validation resolves and orders Inputs before templates use this method.
func (p *FunctionParameter) GeneratorCallFor(receiver string, fields bool) string {
	if p.Generator.Func != nil && p.Generator.Func.FunctionName != "" {
		args := make([]string, 0, len(p.Generator.Inputs))
		for _, input := range p.Generator.Inputs {
			if fields && receiver != "" {
				args = append(args, receiver+"."+input)
			} else {
				args = append(args, input)
			}
		}
		argumentList := strings.Join(args, ", ")
		if p.Generator.Func.CommandPackageName != "" {
			return fmt.Sprintf("%s.%s(%s)", p.Generator.Func.CommandPackageName, p.Generator.Func.FunctionName, argumentList)
		}
		return fmt.Sprintf("%s(%s)", p.Generator.Func.FunctionName, argumentList)
	}
	return ""
}

// OrderedGenerators returns provider-backed local parameters in deterministic
// dependency order. Command validation must be called first.
func OrderedGenerators(params []*FunctionParameter) []*FunctionParameter {
	byName := make(map[string]*FunctionParameter)
	for _, param := range params {
		if param.HasGenerator() && !param.Inherited {
			byName[param.Name] = param
		}
	}
	state := make(map[string]uint8)
	var ordered []*FunctionParameter
	var visit func(*FunctionParameter)
	visit = func(param *FunctionParameter) {
		if state[param.Name] == 2 {
			return
		}
		state[param.Name] = 1
		for _, input := range param.Generator.Inputs {
			if dependency := byName[input]; dependency != nil {
				visit(dependency)
			}
		}
		state[param.Name] = 2
		ordered = append(ordered, param)
	}
	for _, param := range params {
		if param.HasGenerator() && !param.Inherited {
			visit(param)
		}
	}
	return ordered
}
