package go_subcommand

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// GenerateConfig is the optional, versioned JSON configuration accepted by
// gosubc generate. Pointer fields deliberately distinguish an omitted value
// from an explicit false, empty string, or empty list.
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

// LoadGenerateConfig reads a strict version-1 JSON configuration file.
func LoadGenerateConfig(filename string) (GenerateConfig, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return GenerateConfig{}, fmt.Errorf("read generation config %q: %w", filename, err)
	}
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
	if config.Dir != nil && !explicit["dir"] {
		v.dir = *config.Dir
	}
	if config.ManDir != nil && !explicit["man-dir"] {
		v.manDir = *config.ManDir
	}
	if config.ParserName != nil && !explicit["parser-name"] {
		v.parserName = *config.ParserName
	}
	if config.CLIParser != nil && !explicit["cli-parser"] {
		v.cliParser = *config.CLIParser
	}
	if config.Paths != nil && !explicit["path"] {
		v.paths = append([]string(nil), (*config.Paths)...)
	}
	if config.Recursive != nil && !explicit["recursive"] {
		v.recursive = *config.Recursive
	}
	if config.Force != nil && !explicit["force"] {
		v.force = *config.Force
	}
	if config.Clean != nil && !explicit["clean"] {
		v.clean = *config.Clean
	}
	if config.ReplaceTemplates != nil && !explicit["replace-template"] {
		v.replaceTemplates = append([]string(nil), (*config.ReplaceTemplates)...)
	}
	if config.ProjectProvenance != nil && !explicit["project-provenance"] && !explicit["project"] {
		v.projectProvenance = *config.ProjectProvenance
	}
	if config.Timestamp != nil && !explicit["timestamp"] {
		v.timestamp = *config.Timestamp
	}
	if config.EmitGoGenerate != nil && !explicit["emit-go-generate"] {
		v.emitGoGenerate = *config.EmitGoGenerate
	}
	if config.ProvVersion != nil && !explicit["prov-version"] {
		v.provVersion = *config.ProvVersion
	}
	if config.ProvCommit != nil && !explicit["prov-commit"] {
		v.provCommit = *config.ProvCommit
	}
	if config.ProvDate != nil && !explicit["prov-date"] {
		v.provDate = *config.ProvDate
	}
}

func explicitGenerateFlags(args []string) map[string]bool {
	explicit := make(map[string]bool)
	for _, arg := range args {
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
