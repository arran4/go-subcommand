package commentv1

const (
	// Directives
	DirectiveSubcommand  = "is a subcommand"
	DirectiveFlags       = "Flags:"
	DirectiveAlias       = "alias:"
	DirectiveAliases     = "aliases:"
	DirectiveFlagPrefix  = "flag "
	DirectiveParamPrefix = "param "

	// Tags
	TagRequired      = "(required)"
	TagGlobal        = "(global)"
	TagParser        = "parser"
	TagGenerator     = "generator"
	TagFromParent    = "(from parent)"
	TagDefault       = "default:"
	TagAlias         = "alias:"   // For inline (alias: ...)
	TagAliases       = "aliases:" // For inline (aliases: ...)
	TagAka           = "aka:"     // For inline (aka: ...)
	TagPositional    = "@"
	TagVarArg        = "..."
	TagVarArgEllipsis = "..."
)
