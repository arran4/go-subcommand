package commentv1

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"regexp"
	"strings"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
)

var metadataCandidate = regexp.MustCompile(`(?m)^\w+ is a command(?:\s|$)`)
var metadataHeader = regexp.MustCompile("^\\w+ is a command\\s+`([^`]+)`(.*)$")
var commandSegment = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*$`)
var metadataManSection = regexp.MustCompile(`^[A-Z][A-Z0-9 ]*$`)

func declarationError(source parsers.CommandDeclaration, offset int, format string, args ...any) error {
	return fmt.Errorf("%s:%d: %s", source.Filename, source.Line+offset, fmt.Sprintf(format, args...))
}

func collectGoMetadata(fset *token.FileSet, file *ast.File, tree *CommandsTree) error {
	functionDocs := make(map[token.Pos]bool)
	for _, decl := range file.Decls {
		if function, ok := decl.(*ast.FuncDecl); ok && function.Doc != nil {
			functionDocs[function.Doc.Pos()] = true
		}
	}
	for _, group := range file.Comments {
		text := group.Text()
		if !metadataCandidate.MatchString(text) {
			continue
		}
		position := fset.Position(group.Pos())
		source := parsers.CommandDeclaration{Kind: "command", Filename: position.Filename, Line: position.Line}
		if functionDocs[group.Pos()] {
			// This spelling on functions was ignored by the legacy grammar.
			// Keep it non-executable, with a diagnostic rather than a breaking reinterpretation.
			log.Printf("Warning: %s:%d: functionless 'is a command' metadata on a function is ignored; use a package or standalone comment (actions use 'is a subcommand')", source.Filename, source.Line)
			continue
		}
		first, rest, _ := strings.Cut(text, "\n")
		match := metadataHeader.FindStringSubmatch(first)
		if match == nil {
			return declarationError(source, 0, "metadata must start its own comment group with a backtick-delimited path: 'Name is a command `full path`'")
		}
		if metadataCandidate.MatchString(rest) {
			return declarationError(source, 0, "use a separate comment group for each command declaration")
		}
		source.Path = match[1]
		// Normalize the declaration marker, then reuse the action comment parser.
		// Only the metadata model is populated; no function name is synthesized.
		source.Text = "Metadata is a subcommand `" + source.Path + "`" + match[2] + "\n" + rest
		tree.Declarations = append(tree.Declarations, source)
	}
	return nil
}

func validateDeclarationPath(source parsers.CommandDeclaration) ([]string, error) {
	parts := strings.Fields(source.Path)
	if len(parts) == 0 {
		return nil, declarationError(source, 0, "empty command path")
	}
	for _, part := range parts {
		if !commandSegment.MatchString(part) {
			return nil, declarationError(source, 0, "invalid command path component %q", part)
		}
	}
	return parts, nil
}

func parseMetadata(source parsers.CommandDeclaration) (*model.CommandMetadata, error) {
	parts, err := validateDeclarationPath(source)
	if err != nil {
		return nil, err
	}
	text := source.Text
	lineOffset := 1
	if !strings.HasPrefix(text, "Metadata is a subcommand `") {
		lineOffset = 0
		text = "Metadata is a subcommand `" + strings.Join(parts, " ") + "`\n" + text
	}
	lines := strings.Split(text, "\n")
	clean := []string{lines[0]}
	seen := make(map[string]bool)
	var short string
	var noCommand model.NoCommandPolicy
	for index, line := range lines[1:] {
		index += lineOffset
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if lower == "flags:" || strings.HasPrefix(lower, "flag ") || strings.HasPrefix(lower, "param ") {
			return nil, declarationError(source, index, "functionless metadata cannot declare flags or parameters; declare typed parameters on an executable Go function")
		}
		key, value, hasColon := strings.Cut(trimmed, ":")
		key = strings.ToLower(strings.TrimSpace(key))
		canonical := key
		switch key {
		case "alias":
			canonical = "aliases"
		case "default command", "default":
			canonical = "default route"
		}
		if hasColon && (canonical == "description" || canonical == "aliases" || canonical == "cli-parser" || canonical == "default route" || canonical == "no command") {
			if seen[canonical] {
				return nil, declarationError(source, index, "duplicate metadata directive %q", key)
			}
			seen[canonical] = true
			value = strings.TrimSpace(value)
			if value == "" {
				return nil, declarationError(source, index, "empty metadata directive %q", key)
			}
			if canonical == "description" {
				short = value
				continue
			}
			if canonical == "no command" {
				if len(parts) != 1 {
					return nil, declarationError(source, index, "No command policy is only supported on an application root")
				}
				if value != string(model.NoCommandError) {
					return nil, declarationError(source, index, "unsupported No command policy %q; expected error", value)
				}
				noCommand = model.NoCommandError
				continue
			}
			if canonical == "default route" {
				if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
					return nil, declarationError(source, index, "metadata defaults require an explicit Default route path")
				}
				// Routing directives are metadata, not rendered help prose.
				continue
			}
		}
		clean = append(clean, line)
	}
	_, _, description, help, aliases, params, _ := ParseSubCommandComments(strings.Join(clean, "\n"))
	if len(params) != 0 {
		return nil, declarationError(source, 0, "functionless metadata cannot declare parameters without Go types")
	}
	if short != "" {
		if description != "" {
			return nil, declarationError(source, 0, "description declared both inline and with Description:")
		}
		description = short
	}
	route, _ := parseDefaultDirective(text)
	return &model.CommandMetadata{Path: strings.Join(parts, " "), Description: description, ExtendedHelp: help, Aliases: aliases, CLIParser: parseCliParserDirective(text), DefaultRoute: route, NoCommand: noCommand, Filename: source.Filename, Line: source.Line}, nil
}

func appendHelp(existing, extra string) string {
	if existing == "" {
		return strings.TrimRight(extra, "\n")
	}
	if extra == "" {
		return existing
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + strings.TrimRight(extra, "\n")
}

func applyDeclarations(tree *CommandsTree) error {
	for _, source := range tree.Declarations {
		if source.Kind != "command" {
			continue
		}
		metadata, err := parseMetadata(source)
		if err != nil {
			return err
		}
		parts := strings.Fields(metadata.Path)
		root := tree.Commands[parts[0]]
		if root == nil {
			root = &CommandTree{CommandName: parts[0], SubCommandTree: NewSubCommandTree(nil)}
			tree.Commands[parts[0]] = root
		}
		var previous *model.CommandMetadata
		if len(parts) == 1 {
			previous = root.Metadata
		} else {
			node := root.SubCommandTree
			for _, part := range parts[1:] {
				if node.SubCommands[part] == nil {
					node.SubCommands[part] = NewSubCommandTree(nil)
				}
				node = node.SubCommands[part]
			}
			if node.SubCommand == nil {
				node.SubCommand = &model.SubCommand{SubCommandName: parts[len(parts)-1]}
			}
			previous = node.Metadata
		}
		if previous != nil {
			return declarationError(source, 0, "duplicate command metadata %q (first at %s:%d)", metadata.Path, previous.Filename, previous.Line)
		}
		if len(parts) == 1 {
			if root.CLIParser != "" && metadata.CLIParser != "" && root.CLIParser != metadata.CLIParser {
				return declarationError(source, 0, "conflicting CLI-Parser for %q", metadata.Path)
			}
			if root.DefaultRoute != "" && metadata.DefaultRoute != "" && root.DefaultRoute != metadata.DefaultRoute {
				return declarationError(source, 0, "conflicting Default route for %q", metadata.Path)
			}
			root.Metadata = metadata
			root.NoCommand = metadata.NoCommand
			if metadata.Description != "" {
				root.Description = metadata.Description
			}
			root.ExtendedHelp = appendHelp(root.ExtendedHelp, metadata.ExtendedHelp)
			root.Aliases = append(root.Aliases, metadata.Aliases...)
			if metadata.CLIParser != "" {
				root.CLIParser = metadata.CLIParser
			}
			if metadata.DefaultRoute != "" {
				root.DefaultRoute = metadata.DefaultRoute
			}
			if root.DefinitionFile == "" {
				root.DefinitionFile = metadata.Filename
			}
		} else {
			node := root.Find(parts[1:]).SubCommand
			if node.CLIParser != "" && metadata.CLIParser != "" && node.CLIParser != metadata.CLIParser {
				return declarationError(source, 0, "conflicting CLI-Parser for %q", metadata.Path)
			}
			if node.DefaultRoute != "" && metadata.DefaultRoute != "" && node.DefaultRoute != metadata.DefaultRoute {
				return declarationError(source, 0, "conflicting Default route for %q", metadata.Path)
			}
			node.Metadata = metadata
			if metadata.Description != "" {
				node.SubCommandDescription = metadata.Description
			}
			node.SubCommandExtendedHelp = appendHelp(node.SubCommandExtendedHelp, metadata.ExtendedHelp)
			node.Aliases = append(node.Aliases, metadata.Aliases...)
			if metadata.CLIParser != "" {
				node.CLIParser = metadata.CLIParser
			}
			if metadata.DefaultRoute != "" {
				node.DefaultRoute = metadata.DefaultRoute
			}
			if node.DefinitionFile == "" {
				node.DefinitionFile = metadata.Filename
			}
		}
	}
	for _, source := range tree.Declarations {
		if source.Kind == "command" {
			continue
		}
		parts, err := validateDeclarationPath(source)
		if err != nil {
			return err
		}
		root := tree.Commands[parts[0]]
		if root == nil {
			return declarationError(source, 0, "documentation target %q does not exist", source.Path)
		}
		var help *string
		var sections *[]model.ManSection
		if len(parts) == 1 {
			help, sections = &root.ExtendedHelp, &root.ManSections
		} else {
			node := root.Find(parts[1:])
			if node == nil {
				return declarationError(source, 0, "documentation target %q does not exist", source.Path)
			}
			// An implicit parent is already a real structural node; documentation
			// makes its metadata explicit, without creating an executable action.
			if node.SubCommand == nil {
				node.SubCommand = &model.SubCommand{SubCommandName: parts[len(parts)-1]}
			}
			help, sections = &node.SubCommandExtendedHelp, &node.ManSections
		}
		switch source.Kind {
		case "text", "markdown":
			*help = appendHelp(*help, source.Text)
		case "man":
			if !metadataManSection.MatchString(source.Section) {
				return declarationError(source, 0, "invalid man section %q", source.Section)
			}
			switch source.Section {
			case "NAME", "SYNOPSIS", "DESCRIPTION", "OPTIONS", "SEE ALSO":
				return declarationError(source, 0, "man appendix cannot replace built-in section %q", source.Section)
			}
			for _, section := range *sections {
				if section.Name == source.Section {
					return declarationError(source, 0, "duplicate man section %q for %q", source.Section, source.Path)
				}
			}
			*sections = append(*sections, model.ManSection{Name: source.Section, Text: source.Text})
		default:
			return declarationError(source, 0, "unsupported declaration kind %q", source.Kind)
		}
	}
	return nil
}
