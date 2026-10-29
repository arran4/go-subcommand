package go_subcommand

import (
	"bufio"
	"bytes"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/arran4/go-subcommand/parsers"
)

var configFence = regexp.MustCompile("^(`{3,})(command|markdown|text|man):[ \\t]*(.*)$")
var manSectionName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type configBlock struct {
	declaration parsers.CommandDeclaration
	reference   string
	headerLine  int
}

func loadGenerateConfigMarkdownFS(filename string, content []byte, fsys fs.FS, configPath string) (GenerateConfig, error) {
	settings, blocks, err := splitConfigSections(filename, content)
	if err != nil {
		return GenerateConfig{}, err
	}
	config, err := loadGenerateSettingsMarkdown(filename, settings)
	if err != nil {
		return GenerateConfig{}, err
	}
	for _, block := range blocks {
		declaration := block.declaration
		if block.reference != "" {
			if strings.TrimSpace(declaration.Text) != "" {
				return GenerateConfig{}, configLineError(filename, block.headerLine, "cannot combine inline content and file reference")
			}
			if !fs.ValidPath(block.reference) || strings.Contains(block.reference, "\\") {
				return GenerateConfig{}, configLineError(filename, block.headerLine, "external reference must be a safe relative path: %q", block.reference)
			}
			extension := strings.ToLower(path.Ext(block.reference))
			valid := (declaration.Kind == "markdown" && (extension == ".md" || extension == ".markdown")) || ((declaration.Kind == "text" || declaration.Kind == "man") && (extension == ".txt" || extension == ".text"))
			if !valid {
				return GenerateConfig{}, configLineError(filename, block.headerLine, "unsupported external %s file %q", declaration.Kind, block.reference)
			}
			if fsys == nil {
				return GenerateConfig{}, configLineError(filename, block.headerLine, "external references require LoadGenerateConfig or LoadGenerateConfigFS")
			}
			data, err := fs.ReadFile(fsys, path.Join(path.Dir(configPath), block.reference))
			if err != nil {
				return GenerateConfig{}, configLineError(filename, block.headerLine, "read external reference %q: %v", block.reference, err)
			}
			if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
				return GenerateConfig{}, configLineError(filename, block.headerLine, "external reference %q must be UTF-8 text without NUL bytes", block.reference)
			}
			declaration.Text = string(data)
		}
		config.CommandDeclarations = append(config.CommandDeclarations, declaration)
	}
	return config, nil
}

func splitConfigSections(filename string, content []byte) ([]byte, []configBlock, error) {
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return nil, nil, configLineError(filename, 1, "configuration must be UTF-8 text without NUL bytes")
	}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 4096), 8*1024*1024)
	var settings bytes.Buffer
	var blocks []configBlock
	section, fence := "", ""
	seenSections := make(map[string]bool)
	sectionLine, sectionBlocks, line := 0, 0, 0
	var block configBlock
	var body strings.Builder
	for scanner.Scan() {
		line++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if fence != "" {
			if trimmed == fence {
				block.declaration.Text = body.String()
				blocks = append(blocks, block)
				sectionBlocks++
				fence = ""
				body.Reset()
			} else {
				body.WriteString(raw)
				body.WriteByte('\n')
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if section != "" && sectionBlocks == 0 {
				return nil, nil, configLineError(filename, sectionLine, "empty section %q requires a fenced block", section)
			}
			heading := strings.ToLower(trimmed)
			switch heading {
			case "# commands", "# documentation", "# man content":
			default:
				return nil, nil, configLineError(filename, line, "unsupported heading %q", trimmed)
			}
			if seenSections[heading] {
				return nil, nil, configLineError(filename, line, "duplicate section %q", trimmed)
			}
			seenSections[heading] = true
			section, sectionLine, sectionBlocks = heading, line, 0
			continue
		}
		if section == "" {
			settings.WriteString(raw)
			settings.WriteByte('\n')
			continue
		}
		if trimmed == "" {
			continue
		}
		match := configFence.FindStringSubmatch(trimmed)
		if match == nil {
			return nil, nil, configLineError(filename, line, "expected a typed fenced block in %s", section)
		}
		kind := match[2]
		if (section == "# commands" && kind != "command") || (section == "# documentation" && kind != "text" && kind != "markdown") || (section == "# man content" && kind != "man") {
			return nil, nil, configLineError(filename, line, "unsupported %s block in %s", kind, section)
		}
		block = configBlock{declaration: parsers.CommandDeclaration{Kind: kind, Filename: filename, Line: line + 1}, headerLine: line}
		header := strings.TrimSpace(match[3])
		if index := strings.Index(header, " file="); index >= 0 {
			if kind == "command" {
				return nil, nil, configLineError(filename, line, "command metadata must be inline; file references are documentation only")
			}
			reference := strings.TrimSpace(header[index+6:])
			if strings.HasPrefix(reference, "\"") {
				var err error
				reference, err = strconv.Unquote(reference)
				if err != nil {
					return nil, nil, configLineError(filename, line, "invalid quoted file reference: %v", err)
				}
			} else if strings.ContainsAny(reference, " \t") {
				return nil, nil, configLineError(filename, line, "file references containing spaces must be quoted")
			}
			if reference == "" {
				return nil, nil, configLineError(filename, line, "empty file reference")
			}
			block.reference = reference
			header = strings.TrimSpace(header[:index])
		}
		if kind == "man" {
			parts := strings.Fields(header)
			if len(parts) < 2 || !manSectionName.MatchString(parts[len(parts)-1]) {
				return nil, nil, configLineError(filename, line, "man block requires a command path and uppercase SECTION")
			}
			block.declaration.Section = strings.ReplaceAll(parts[len(parts)-1], "_", " ")
			switch block.declaration.Section {
			case "NAME", "SYNOPSIS", "DESCRIPTION", "OPTIONS", "SEE ALSO":
				return nil, nil, configLineError(filename, line, "man appendix cannot replace built-in section %q", block.declaration.Section)
			}
			header = strings.Join(parts[:len(parts)-1], " ")
		}
		if header == "" {
			return nil, nil, configLineError(filename, line, "empty command path in fenced block")
		}
		block.declaration.Path = header
		fence = match[1]
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, configLineError(filename, line+1, "%v", err)
	}
	if fence != "" {
		return nil, nil, configLineError(filename, block.headerLine, "unclosed %s fenced block", block.declaration.Kind)
	}
	if section != "" && sectionBlocks == 0 {
		return nil, nil, configLineError(filename, sectionLine, "empty section %q requires a fenced block", section)
	}
	return settings.Bytes(), blocks, nil
}
