package model

import (
	"fmt"
	"sort"
	"strings"
)

type CommentOption func(*CommentConfig)

type CommentConfig struct {
	IncludeInherited bool
}

func WithInherited(include bool) CommentOption {
	return func(c *CommentConfig) {
		c.IncludeInherited = include
	}
}

func (sc *SubCommand) Comment(opts ...CommentOption) string {
	config := CommentConfig{}
	for _, opt := range opts {
		opt(&config)
	}

	var sb strings.Builder
	sb.WriteString("// ")
	sb.WriteString(sc.SubCommandFunctionName)
	sb.WriteString(" is a subcommand `")

	// Full command sequence
	fullSequence := sc.MainCmdName
	if seq := sc.SubCommandSequence(); seq != "" {
		fullSequence += " " + seq
	}
	sb.WriteString(fullSequence)
	sb.WriteString("`")

	if sc.SubCommandDescription != "" {
		sb.WriteString(" that ")
		sb.WriteString(sc.SubCommandDescription)
	}
	sb.WriteString("\n//")

	if sc.SubCommandExtendedHelp != "" {
		sb.WriteString("\n// ")
		sb.WriteString(strings.ReplaceAll(sc.SubCommandExtendedHelp, "\n", "\n// "))
		sb.WriteString("\n//")
	}

	if len(sc.Aliases) > 0 {
		sb.WriteString("\n// aliases: ")
		// Copy aliases to avoid modifying original struct
		aliases := make([]string, len(sc.Aliases))
		copy(aliases, sc.Aliases)
		sort.Strings(aliases)
		sb.WriteString(strings.Join(aliases, ", "))
	}

	if len(sc.Parameters) > 0 {
		var relevantParams []*FunctionParameter
		for _, p := range sc.Parameters {
			// If inherited, only include if config.IncludeInherited is true
			if !config.IncludeInherited && p.DeclaredIn != "" && p.DeclaredIn != sc.SubCommandName {
				continue
			}
			relevantParams = append(relevantParams, p)
		}

		if len(relevantParams) > 0 {
			// Sort parameters by name for deterministic output
			sort.Slice(relevantParams, func(i, j int) bool {
				return relevantParams[i].Name < relevantParams[j].Name
			})

			sb.WriteString("\n//\n// Flags:\n//")
			for _, p := range relevantParams {
				sb.WriteString("\n//\t")
				sb.WriteString(p.Name)
				sb.WriteString(": ")
				sb.WriteString(p.Comment())
			}
		}
	}

	return sb.String()
}

func (p *FunctionParameter) Comment() string {
	var parts []string

	if p.Description != "" {
		parts = append(parts, p.Description)
	}

	if p.IsRequired {
		parts = append(parts, "(required)")
	}

	if p.Default != "" {
		def := p.Default
		if p.Type == "string" && !strings.HasPrefix(def, "\"") {
			def = fmt.Sprintf("%q", def)
		}
		parts = append(parts, fmt.Sprintf("(default: %s)", def))
	}

	var explicitAliases []string
	// FlagAliases usually contains the parameter name itself if implicitly derived, or explicitly defined aliases.
	// We check if FlagAliases contains items that are not just the kebab-case of Name.
	// However, simple reconstruction: just list all aliases except if only one exists and it matches Name (or kebab-case Name).
	// The parser logic uses flag regex `-[\w-]+`.
	// We'll output all aliases prefixed with - or --.

	for _, a := range p.FlagAliases {
		prefix := "-"
		if len(a) > 1 {
			prefix = "--"
		}
		explicitAliases = append(explicitAliases, prefix+a)
	}
	// Sort for stability
	sort.Strings(explicitAliases)

	if len(explicitAliases) > 0 {
		// If explicit aliases are present, list them.
		// However, the parser detects any -flag in the description as an alias.
		// We should be careful not to double count if the name is used as flag.
		// But usually FlagAliases is populated.
		parts = append(parts, strings.Join(explicitAliases, ", "))
	}

	if p.IsPositional {
		parts = append(parts, fmt.Sprintf("@%d", p.PositionalArgIndex))
	}

	if p.IsVarArg {
		if p.VarArgMin != 0 || p.VarArgMax != 0 {
			parts = append(parts, fmt.Sprintf("%d...%d", p.VarArgMin, p.VarArgMax))
		} else {
			parts = append(parts, "...")
		}
	}

	if p.Parser.Type == ParserTypeCustom && p.Parser.Func != nil {
		fn := p.Parser.Func.FunctionName
		if p.Parser.Func.ImportPath != "" {
			fn = fmt.Sprintf("%q.%s", p.Parser.Func.ImportPath, fn)
		}
		parts = append(parts, fmt.Sprintf("(parser: %s)", fn))
	}

	if p.Generator.Type == SourceTypeGenerator && p.Generator.Func != nil {
		fn := p.Generator.Func.FunctionName
		if p.Generator.Func.ImportPath != "" {
			fn = fmt.Sprintf("%q.%s", p.Generator.Func.ImportPath, fn)
		}
		parts = append(parts, fmt.Sprintf("(generator: %s)", fn))
	}

	return strings.Join(parts, " ")
}
