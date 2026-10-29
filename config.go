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
	Version           int            `json:"version"`
	Dir               *string        `json:"dir,omitempty"`
	ManDir            *string        `json:"manDir,omitempty"`
	ParserName        *string        `json:"parserName,omitempty"`
	CLIParser         *string        `json:"cliParser,omitempty"`
	Paths             *[]string      `json:"paths,omitempty"`
	Recursive         *bool          `json:"recursive,omitempty"`
	Force             *bool          `json:"force,omitempty"`
	Clean             *bool          `json:"clean,omitempty"`
	ReplaceTemplates  *[]string      `json:"replaceTemplates,omitempty"`
	ProjectProvenance *bool          `json:"projectProvenance,omitempty"`
	Timestamp         *bool          `json:"timestamp,omitempty"`
	EmitGoGenerate    *bool          `json:"emitGoGenerate,omitempty"`
	ProvVersion       *string        `json:"provVersion,omitempty"`
	ProvCommit        *string        `json:"provCommit,omitempty"`
	ProvDate          *string        `json:"provDate,omitempty"`
	TemplateVariables map[string]any `json:"variables,omitempty"`
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
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var config GenerateConfig
	if err := decoder.Decode(&config); err != nil {
		return GenerateConfig{}, fmt.Errorf("parse generation config %q: %w", filename, err)
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
	config := GenerateConfig{
		Version:           1,
		TemplateVariables: make(map[string]any),
	}
	currentSection := ""
	lineNum := 0
	var lastListKey string

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
			currentSection = strings.ToLower(heading)
			lastListKey = ""
			continue
		}

		if strings.HasPrefix(line, "- ") && lastListKey != "" && currentSection == "" {
			item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			item = strings.Trim(item, "\"'")
			appendListField(&config, lastListKey, item)
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return GenerateConfig{}, fmt.Errorf("parse generation config %q (line %d): expected 'key: value'", filename, lineNum)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"'")

		if currentSection == "variables" {
			config.TemplateVariables[key] = val
			continue
		}

		lastListKey = ""
		switch normalizeKey(key) {
		case "version":
			v, err := strconv.Atoi(val)
			if err != nil {
				return GenerateConfig{}, fmt.Errorf("parse generation config %q (line %d): invalid version: %w", filename, lineNum, err)
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
			items := parseStringSlice(val)
			config.Paths = &items
			lastListKey = "paths"
		case "recursive":
			b := parseBool(val)
			config.Recursive = &b
		case "force":
			b := parseBool(val)
			config.Force = &b
		case "clean":
			b := parseBool(val)
			config.Clean = &b
		case "replacetemplates":
			items := parseStringSlice(val)
			config.ReplaceTemplates = &items
			lastListKey = "replacetemplates"
		case "projectprovenance", "project":
			b := parseBool(val)
			config.ProjectProvenance = &b
		case "timestamp":
			b := parseBool(val)
			config.Timestamp = &b
		case "emitgogenerate":
			b := parseBool(val)
			config.EmitGoGenerate = &b
		case "provversion":
			config.ProvVersion = ptr(val)
		case "provcommit":
			config.ProvCommit = ptr(val)
		case "provdate":
			config.ProvDate = ptr(val)
		default:
			return GenerateConfig{}, fmt.Errorf("parse generation config %q (line %d): unknown key %q", filename, lineNum, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return GenerateConfig{}, fmt.Errorf("read generation config %q: %w", filename, err)
	}
	if config.Version != 1 {
		return GenerateConfig{}, fmt.Errorf("generation config %q has version %d; supported version is 1", filename, config.Version)
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

func parseBool(val string) bool {
	lower := strings.ToLower(val)
	return lower == "true" || lower == "1" || lower == "yes"
}

func parseStringSlice(val string) []string {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		trimmed = strings.Trim(trimmed, "\"'")
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
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

