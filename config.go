package go_subcommand

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GenerateConfig is the optional, versioned configuration accepted by
// gosubc generate. It represents configuration loaded from format-neutral
// readers (including native .mdconf / .mdc and .json). Pointer fields
// deliberately distinguish an omitted value from an explicit false, empty string,
// or empty list.
type GenerateConfig struct {
	Version           int       `json:"version"`
	Dir               *string   `json:"dir,omitempty"`
	ManDir            *string   `json:"manDir,omitempty"`
	ParserName        *string   `json:"parserName,omitempty"`
	CLIParser         *string   `json:"cliParser,omitempty"`
	Paths             *[]string `json:"paths,omitempty"`
	Recursive         *bool     `json:"recursive,omitempty"`
	Force             *bool     `json:"force,omitempty"`
	Clean             *bool     `json:"clean,omitempty"`
	ReplaceTemplates  *[]string `json:"replaceTemplates,omitempty"`
	ProjectProvenance *bool     `json:"projectProvenance,omitempty"`
	Timestamp         *bool     `json:"timestamp,omitempty"`
	EmitGoGenerate    *bool     `json:"emitGoGenerate,omitempty"`
	ProvVersion       *string   `json:"provVersion,omitempty"`
	ProvCommit        *string   `json:"provCommit,omitempty"`
	ProvDate          *string   `json:"provDate,omitempty"`
}

// LoadGenerateConfig reads a configuration file, dispatching by extension.
// It supports native Markdown-readable configuration (.mdconf, .mdc) and strict version-1 JSON (.json).
func LoadGenerateConfig(filename string) (GenerateConfig, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return GenerateConfig{}, fmt.Errorf("read generation config %q: %w", filename, err)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		return loadGenerateConfigJSON(filename, content)
	case ".mdconf", ".mdc":
		return loadGenerateConfigMarkdown(filename, content)
	default:
		return GenerateConfig{}, fmt.Errorf("unsupported configuration format %q for %q; supported formats are .json, .mdconf, .mdc", ext, filename)
	}
}

func loadGenerateConfigJSON(filename string, content []byte) (GenerateConfig, error) {
	// Decode entries individually so duplicate settings cannot silently win.
	keys := json.NewDecoder(bytes.NewReader(content))
	if tok, err := keys.Token(); err != nil || tok != json.Delim('{') {
		return GenerateConfig{}, configLineError(filename, 1, "expected a JSON object")
	}
	seen := make(map[string]bool)
	for keys.More() {
		tok, err := keys.Token()
		if err != nil {
			return GenerateConfig{}, configLineError(filename, configLine(content, keys.InputOffset()), "%v", err)
		}
		key, ok := tok.(string)
		if !ok {
			return GenerateConfig{}, configLineError(filename, configLine(content, keys.InputOffset()), "expected a setting name")
		}
		if seen[normalizeKey(key)] {
			return GenerateConfig{}, configLineError(filename, configLine(content, keys.InputOffset()), "duplicate key %q", key)
		}
		seen[normalizeKey(key)] = true
		var value json.RawMessage
		if err := keys.Decode(&value); err != nil {
			return GenerateConfig{}, configLineError(filename, configLine(content, keys.InputOffset()), "%v", err)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return GenerateConfig{}, configLineError(filename, configLine(content, keys.InputOffset()), "null is not a value for %q; omit the setting instead", key)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var config GenerateConfig
	if err := decoder.Decode(&config); err != nil {
		return GenerateConfig{}, configLineError(filename, configLine(content, decoder.InputOffset()), "%v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return GenerateConfig{}, fmt.Errorf("parse generation config %q: multiple JSON values", filename)
		}
		return GenerateConfig{}, fmt.Errorf("parse generation config %q: %w", filename, err)
	}
	if config.Version != 1 {
		return GenerateConfig{}, fmt.Errorf("generation config %q has version %d; supported version is 1", filename, config.Version)
	}
	return config, nil
}

func loadGenerateConfigMarkdown(filename string, content []byte) (GenerateConfig, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var config GenerateConfig
	seen := make(map[string]int)
	lineNum := 0
	var lastListKey string

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			return GenerateConfig{}, configLineError(filename, lineNum, "unsupported heading %q; this version supports top-level generator settings only", line)
		}

		if line == "-" || strings.HasPrefix(line, "- ") {
			if lastListKey == "" || line == "-" {
				return GenerateConfig{}, configLineError(filename, lineNum, "list entry must follow an empty paths or replace-templates setting and contain a value")
			}
			item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			item, err := parseConfigString(item)
			if err != nil || item == "" {
				return GenerateConfig{}, configLineError(filename, lineNum, "invalid list entry %q", line)
			}
			appendListField(&config, lastListKey, item)
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return GenerateConfig{}, configLineError(filename, lineNum, "expected 'key: value'")
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		normalized := normalizeKey(key)
		if normalized == "formatversion" {
			normalized = "version"
		}
		if normalized == "project" {
			normalized = "projectprovenance"
		}
		if previous, ok := seen[normalized]; ok {
			return GenerateConfig{}, configLineError(filename, lineNum, "duplicate/conflicting key %q (first declared on line %d)", key, previous)
		}
		seen[normalized] = lineNum
		var err error
		val, err = parseConfigString(val)
		if err != nil {
			return GenerateConfig{}, configLineError(filename, lineNum, "%v", err)
		}

		lastListKey = ""
		switch normalized {
		case "version":
			v, err := strconv.Atoi(val)
			if err != nil {
				return GenerateConfig{}, configLineError(filename, lineNum, "invalid format version: %v", err)
			}
			if v != 1 {
				return GenerateConfig{}, configLineError(filename, lineNum, "unsupported format version %d; supported version is 1", v)
			}
			config.Version = v
		case "dir":
			config.Dir = ptr(val)
		case "mandir":
			config.ManDir = ptr(val)
		case "parsername":
			config.ParserName = ptr(val)
		case "cliparser":
			config.CLIParser = ptr(val)
		case "paths":
			items, err := parseStringSlice(val)
			if err != nil {
				return GenerateConfig{}, configLineError(filename, lineNum, "%v", err)
			}
			config.Paths = &items
			if val == "" {
				lastListKey = "paths"
			}
		case "replacetemplates":
			items, err := parseStringSlice(val)
			if err != nil {
				return GenerateConfig{}, configLineError(filename, lineNum, "%v", err)
			}
			config.ReplaceTemplates = &items
			if val == "" {
				lastListKey = "replacetemplates"
			}
		case "recursive", "force", "clean", "projectprovenance", "timestamp", "emitgogenerate":
			b, err := parseBool(val)
			if err != nil {
				return GenerateConfig{}, configLineError(filename, lineNum, "invalid boolean for %s: %v", key, err)
			}
			switch normalized {
			case "recursive":
				config.Recursive = &b
			case "force":
				config.Force = &b
			case "clean":
				config.Clean = &b
			case "projectprovenance":
				config.ProjectProvenance = &b
			case "timestamp":
				config.Timestamp = &b
			case "emitgogenerate":
				config.EmitGoGenerate = &b
			}
		case "provversion":
			config.ProvVersion = ptr(val)
		case "provcommit":
			config.ProvCommit = ptr(val)
		case "provdate":
			config.ProvDate = ptr(val)
		default:
			return GenerateConfig{}, configLineError(filename, lineNum, "unknown key %q", key)
		}
	}
	if err := scanner.Err(); err != nil {
		return GenerateConfig{}, configLineError(filename, lineNum+1, "%v", err)
	}
	if config.Version != 1 {
		return GenerateConfig{}, configLineError(filename, 1, "missing required format-version: 1")
	}
	return config, nil
}

func normalizeKey(k string) string {
	k = strings.ReplaceAll(k, "-", "")
	k = strings.ReplaceAll(k, "_", "")
	return strings.ToLower(k)
}

func ptr[T any](v T) *T {
	return &v
}

func configLineError(filename string, line int, format string, args ...any) error {
	return fmt.Errorf("parse generation config %q (line %d): %s", filename, line, fmt.Sprintf(format, args...))
}

func configLine(content []byte, offset int64) int {
	return bytes.Count(content[:offset], []byte("\n")) + 1
}

func parseConfigString(val string) (string, error) {
	if strings.HasPrefix(val, "\"") {
		return strconv.Unquote(val)
	}
	if strings.HasPrefix(val, "'") {
		if len(val) < 2 || !strings.HasSuffix(val, "'") || strings.Contains(val[1:len(val)-1], "'") {
			return "", fmt.Errorf("malformed quoted value %q", val)
		}
		return val[1 : len(val)-1], nil
	}
	return val, nil
}

func parseBool(val string) (bool, error) {
	switch strings.ToLower(val) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%q (expected true/false, yes/no or 1/0)", val)
	}
}

func parseStringSlice(val string) ([]string, error) {
	if val == "" {
		return []string{}, nil
	}
	if strings.HasPrefix(val, "[") {
		var result []string
		if err := json.Unmarshal([]byte(val), &result); err != nil {
			return nil, fmt.Errorf("invalid string list: %w", err)
		}
		for _, item := range result {
			if item == "" {
				return nil, fmt.Errorf("empty list entry")
			}
		}
		return result, nil
	}
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		trimmed, err := parseConfigString(trimmed)
		if err != nil || trimmed == "" {
			return nil, fmt.Errorf("invalid list entry %q", p)
		}
		result = append(result, trimmed)
	}
	return result, nil
}

func appendListField(config *GenerateConfig, key, item string) {
	switch key {
	case "paths":
		if config.Paths == nil {
			config.Paths = &[]string{}
		}
		*config.Paths = append(*config.Paths, item)
	case "replacetemplates":
		if config.ReplaceTemplates == nil {
			config.ReplaceTemplates = &[]string{}
		}
		*config.ReplaceTemplates = append(*config.ReplaceTemplates, item)
	}
}

type generateCLIValues struct {
	dir, manDir, parserName, cliParser string
	paths                              []string
	recursive, force, clean            bool
	replaceTemplates                   []string
	projectProvenance, timestamp       bool
	emitGoGenerate                     bool
	provVersion, provCommit, provDate  string
}

func (v *generateCLIValues) applyConfig(config GenerateConfig, explicit map[string]bool) {
	if config.Dir != nil && !isExplicit(explicit, "dir") {
		v.dir = *config.Dir
	}
	if config.ManDir != nil && !isExplicit(explicit, "man-dir", "manDir") {
		v.manDir = *config.ManDir
	}
	if config.ParserName != nil && !isExplicit(explicit, "parser-name", "parserName") {
		v.parserName = *config.ParserName
	}
	if config.CLIParser != nil && !isExplicit(explicit, "cli-parser", "cliParser") {
		v.cliParser = *config.CLIParser
	}
	if config.Paths != nil && !isExplicit(explicit, "path", "paths") {
		v.paths = append([]string(nil), (*config.Paths)...)
	}
	if config.Recursive != nil && !isExplicit(explicit, "recursive") {
		v.recursive = *config.Recursive
	}
	if config.Force != nil && !isExplicit(explicit, "force") {
		v.force = *config.Force
	}
	if config.Clean != nil && !isExplicit(explicit, "clean") {
		v.clean = *config.Clean
	}
	if config.ReplaceTemplates != nil && !isExplicit(explicit, "replace-template", "replaceTemplates") {
		v.replaceTemplates = append([]string(nil), (*config.ReplaceTemplates)...)
	}
	if config.ProjectProvenance != nil && !isExplicit(explicit, "project-provenance", "projectProvenance", "project") {
		v.projectProvenance = *config.ProjectProvenance
	}
	if config.Timestamp != nil && !isExplicit(explicit, "timestamp") {
		v.timestamp = *config.Timestamp
	}
	if config.EmitGoGenerate != nil && !isExplicit(explicit, "emit-go-generate", "emitGoGenerate") {
		v.emitGoGenerate = *config.EmitGoGenerate
	}
	if config.ProvVersion != nil && !isExplicit(explicit, "prov-version", "provVersion") {
		v.provVersion = *config.ProvVersion
	}
	if config.ProvCommit != nil && !isExplicit(explicit, "prov-commit", "provCommit") {
		v.provCommit = *config.ProvCommit
	}
	if config.ProvDate != nil && !isExplicit(explicit, "prov-date", "provDate") {
		v.provDate = *config.ProvDate
	}
}

func isExplicit(explicit map[string]bool, names ...string) bool {
	if explicit == nil {
		return false
	}
	for _, name := range names {
		if explicit[name] {
			return true
		}
	}
	return false
}

// ExplicitGenerateFlags extracts explicitly passed CLI flag names from an argument slice.
// It stops parsing flags at "--" and handles both "--flag=value" and "--flag" forms.
func ExplicitGenerateFlags(args []string) map[string]bool {
	generateIdx := -1
	for i, arg := range args {
		if arg == "generate" {
			generateIdx = i
			break
		}
	}
	if generateIdx >= 0 {
		args = args[generateIdx+1:]
	}
	explicit := make(map[string]bool)
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		name := strings.TrimPrefix(arg, "--")
		if index := strings.IndexByte(name, '='); index >= 0 {
			name = name[:index]
		}
		explicit[name] = true
	}
	return explicit
}
