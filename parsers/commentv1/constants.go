package commentv1

// Parsing directives found in comments to define commands and parameters.
const (
	// DirectiveFlags is the marker for the start of a flags definition block.
	// Example:
	//   Flags:
	//     ...
	DirectiveFlags = "Flags:"

	// DirectiveIsSubcommand is the marker to indicate that a function is a subcommand.
	// Example:
	//   // MyCommand is a subcommand of ParentCommand ...
	DirectiveIsSubcommand = "is a subcommand"

	// DirectiveAliasPrefix is the prefix for defining aliases for a command.
	// Example:
	//   alias: mycmd, cmd
	DirectiveAliasPrefix = "alias:"

	// DirectiveAliasesPrefix is the prefix for defining aliases for a command (plural).
	// Example:
	//   aliases: mycmd, cmd
	DirectiveAliasesPrefix = "aliases:"
)

// Prefixes used to identify parameter definitions in comments.
const (
	// PrefixFlag is the prefix for an explicit flag definition.
	// Example:
	//   flag myflag: ...
	PrefixFlag = "flag "

	// PrefixParam is the prefix for an explicit parameter definition.
	// Example:
	//   param myparam: ...
	PrefixParam = "param "
)

// Attributes used in parameter definitions to configure behavior.
const (
	// AttributeRequired indicates that the parameter is mandatory.
	// Usage: (required)
	AttributeRequired = "required"

	// AttributeGlobal indicates that the parameter is global (inherited from parent).
	// usage: (global)
	// Note: This is mapped to Inherited in the model.
	AttributeGlobal = "global"

	// AttributeGenerator specifies a generator function for the parameter.
	// Usage: (generator: MyGeneratorFunc)
	AttributeGenerator = "generator"

	// AttributeParser specifies a custom parser function for the parameter.
	// Usage: (parser: MyParserFunc) or (parser: "pkg".MyParserFunc)
	AttributeParser = "parser"

	// AttributeDefault specifies the default value for the parameter.
	// Usage: (default: "value")
	AttributeDefault = "default"

	// AttributeInherited indicates that the parameter is inherited from a parent command.
	// Usage: (inherited)
	AttributeInherited = "inherited"

	// AttributeFromParent is an alias for AttributeInherited.
	// Usage: (from parent)
	AttributeFromParent = "from parent"

	// AttributeAlias is used to define flag aliases for a parameter.
	// Usage: (alias: f)
	AttributeAlias = "alias"

	// AttributeAliases is used to define flag aliases for a parameter (plural).
	// Usage: (aliases: f, flag)
	AttributeAliases = "aliases"

	// AttributeAka is an alias for AttributeAliases.
	// Usage: (aka: f)
	AttributeAka = "aka"
)
