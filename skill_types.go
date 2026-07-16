package go_subcommand

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SkillMetadata stores provenance and version information for an installed skill.
type SkillMetadata struct {
	Name           string    `json:"name"`
	Source         string    `json:"source"`
	Path           string    `json:"path,omitempty"`
	Revision       string    `json:"revision"` // Git commit, digest, or timestamp
	Scope          string    `json:"scope"`
	Agent          string    `json:"agent"`
	InstalledAt    time.Time `json:"installed_at"`
	UpdaterVersion string    `json:"updater_version,omitempty"`
	IsLocal        bool      `json:"is_local"`
}

// readSkillMetadata reads the metadata from a skill directory.
func readSkillMetadata(skillDir string) (*SkillMetadata, error) {
	metaPath := filepath.Join(skillDir, "skill_metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("metadata not found in %s", skillDir)
		}
		return nil, err
	}

	var meta SkillMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("invalid metadata in %s: %w", metaPath, err)
	}

	return &meta, nil
}

// writeSkillMetadata writes the metadata to a skill directory.
func writeSkillMetadata(skillDir string, meta *SkillMetadata) error {
	metaPath := filepath.Join(skillDir, "skill_metadata.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}
