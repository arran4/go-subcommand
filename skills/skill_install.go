package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SkillInstall is a subcommand `gosubc skill install` installs an AI agent skill.
// Installs an AI agent skill.
//
// Flags:
//
//	source: @1 The source to install the skill from (e.g. owner/repo, or path)
//	name: @2 (default: "") The name of the skill to install (if omitted, inferred from source)
//	scope: --scope (default: "user") The installation scope ('user' or 'project')
//	agent: --agent (default: "") Explicitly target a specific agent (e.g. 'codex', 'claude')
func SkillInstall(source string, name string, scope string, agent string) error {
	if source == "" {
		return fmt.Errorf("source is required")
	}

	if name == "" {
		// Infer name from source
		parts := strings.Split(source, "/")
		name = parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".git")
	}

	fmt.Printf("Fetching skill '%s' from '%s'...\n", name, source)
	tempDir, revision, isLocal, err := fetch(source)
	if err != nil {
		return fmt.Errorf("failed to fetch skill: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }() // Ensure cleanup

	skillMdPath := filepath.Join(tempDir, "SKILL.md")
	if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
		return fmt.Errorf("invalid skill: SKILL.md not found in source")
	}

	destPath, err := resolveSkillPath(agent, scope, name)
	if err != nil {
		return fmt.Errorf("failed to resolve installation path: %w", err)
	}

	// Check if already installed
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("skill '%s' is already installed at %s (use update instead)", name, destPath)
	}

	// Move temp directory to destination
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Simple rename might fail across devices, so we use copyDir
	if err := copyDir(tempDir, destPath); err != nil {
		return fmt.Errorf("failed to install skill to %s: %w", destPath, err)
	}

	// Write metadata
	meta := &SkillMetadata{
		Name:        name,
		Source:      source,
		Revision:    revision,
		Scope:       scope,
		Agent:       agent,
		InstalledAt: time.Now(),
		IsLocal:     isLocal,
	}
	if err := writeSkillMetadata(destPath, meta); err != nil {
		_ = os.RemoveAll(destPath) // Rollback
		return fmt.Errorf("failed to write skill metadata: %w", err)
	}

	fmt.Printf("Successfully installed skill '%s' to %s\n", name, destPath)
	return nil
}

// SkillUpdate is a subcommand `gosubc skill update` updates an AI agent skill.
// Updates an AI agent skill.
//
// Flags:
//
//	name: @1 (default: "") The name of the skill to update
//	all: --all (default: false) Update all installed skills
//	scope: --scope (default: "user") The installation scope ('user' or 'project')
//	agent: --agent (default: "") Explicitly target a specific agent (e.g. 'codex', 'claude')
//	force: --force (default: false) Force update even if local modifications exist
func SkillUpdate(name string, all bool, scope string, agent string, force bool) error {
	if !all && name == "" {
		return fmt.Errorf("must specify a skill name or use --all")
	}

	skillsToUpdate := []string{}
	if all {
		// Discover skills
		root, err := resolveSkillRoot(agent, scope)
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No skills installed.")
				return nil
			}
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				skillsToUpdate = append(skillsToUpdate, e.Name())
			}
		}
	} else {
		skillsToUpdate = append(skillsToUpdate, name)
	}

	for _, sName := range skillsToUpdate {
		destPath, err := resolveSkillPath(agent, scope, sName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving path for %s: %v\n", sName, err)
			continue
		}

		meta, err := readSkillMetadata(destPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: no source metadata available for '%s', cannot update automatically.\n", sName)
			continue
		}

		fmt.Printf("Checking for updates for '%s' from %s...\n", sName, meta.Source)

		tempDir, newRevision, _, err := fetch(meta.Source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch update for '%s': %v\n", sName, err)
			continue
		}

		if meta.Revision == newRevision && !force {
			fmt.Printf("Skill '%s' is already up to date (revision: %s).\n", sName, newRevision)
			_ = os.RemoveAll(tempDir)
			continue
		}

		fmt.Printf("Applying update for '%s' (%s -> %s)...\n", sName, meta.Revision, newRevision)

		// Remove old directory
		if err := os.RemoveAll(destPath); err != nil {
			err = fmt.Errorf("failed to remove old skill directory for '%s': %w", sName, err)
			if !all {
				_ = os.RemoveAll(tempDir)
				return err
			}
			fmt.Fprintln(os.Stderr, err)
			_ = os.RemoveAll(tempDir)
			continue
		}

		// Copy new directory
		if err := copyDir(tempDir, destPath); err != nil {
			err = fmt.Errorf("failed to copy updated skill '%s' to %s: %w", sName, destPath, err)
			if !all {
				_ = os.RemoveAll(tempDir)
				return err
			}
			fmt.Fprintln(os.Stderr, err)
			_ = os.RemoveAll(tempDir)
			continue
		}

		// Write new metadata
		meta.Revision = newRevision
		meta.InstalledAt = time.Now()
		if err := writeSkillMetadata(destPath, meta); err != nil {
			err = fmt.Errorf("failed to write updated skill metadata for '%s': %w", sName, err)
			if !all {
				_ = os.RemoveAll(tempDir)
				return err
			}
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Printf("Successfully updated skill '%s'\n", sName)
		}

		_ = os.RemoveAll(tempDir)
	}

	return nil
}

// SkillRemove is a subcommand `gosubc skill remove` removes an AI agent skill.
// Removes an AI agent skill.
//
// Flags:
//
//	name: @1 The name of the skill to remove
//	scope: --scope (default: "user") The installation scope ('user' or 'project')
//	agent: --agent (default: "") Explicitly target a specific agent (e.g. 'codex', 'claude')
func SkillRemove(name string, scope string, agent string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}

	destPath, err := resolveSkillPath(agent, scope, name)
	if err != nil {
		return fmt.Errorf("failed to resolve path for %s: %w", name, err)
	}

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return fmt.Errorf("skill '%s' is not installed", name)
	}

	meta, _ := readSkillMetadata(destPath)
	scopeReport := scope
	if meta != nil {
		scopeReport = meta.Scope
	}

	fmt.Printf("Removing skill '%s' (scope: %s)...\n", name, scopeReport)

	if err := os.RemoveAll(destPath); err != nil {
		return fmt.Errorf("failed to remove skill directory: %w", err)
	}

	fmt.Printf("Successfully removed skill '%s'\n", name)
	return nil
}

// SkillList is a subcommand `gosubc skill list` lists installed AI agent skills.
// Lists installed AI agent skills.
//
// Flags:
//
//	scope: --scope (default: "user") The installation scope ('user' or 'project')
//	agent: --agent (default: "") Explicitly target a specific agent (e.g. 'codex', 'claude')
func SkillList(scope string, agent string) error {
	root, err := resolveSkillRoot(agent, scope)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No skills installed.")
			return nil
		}
		return err
	}

	count := 0
	fmt.Printf("%-20s %-15s %-40s %s\n", "NAME", "SCOPE", "SOURCE", "REVISION")
	fmt.Println(strings.Repeat("-", 100))

	for _, e := range entries {
		if e.IsDir() {
			destPath := filepath.Join(root, e.Name())
			meta, err := readSkillMetadata(destPath)
			if err != nil {
				fmt.Printf("%-20s %-15s %-40s %s\n", e.Name(), scope, "(unknown)", "(no metadata)")
				continue
			}

			rev := meta.Revision
			if len(rev) > 8 {
				rev = rev[:8]
			}

			fmt.Printf("%-20s %-15s %-40s %s\n", meta.Name, meta.Scope, meta.Source, rev)
			count++
		}
	}

	if count == 0 {
		fmt.Println("No skills installed.")
	}

	return nil
}

// SkillInspect is a subcommand `gosubc skill inspect` inspects an AI agent skill.
// Inspects an AI agent skill.
//
// Flags:
//
//	name: @1 The name of the skill to inspect
//	scope: --scope (default: "user") The installation scope ('user' or 'project')
//	agent: --agent (default: "") Explicitly target a specific agent (e.g. 'codex', 'claude')
func SkillInspect(name string, scope string, agent string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}

	destPath, err := resolveSkillPath(agent, scope, name)
	if err != nil {
		return fmt.Errorf("failed to resolve path for %s: %w", name, err)
	}

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return fmt.Errorf("skill '%s' is not installed", name)
	}

	meta, err := readSkillMetadata(destPath)
	if err != nil {
		fmt.Printf("Skill '%s' is installed at %s, but no metadata is available.\n", name, destPath)
		return nil
	}

	fmt.Printf("Skill:       %s\n", meta.Name)
	fmt.Printf("Source:      %s\n", meta.Source)
	fmt.Printf("Revision:    %s\n", meta.Revision)
	fmt.Printf("Scope:       %s\n", meta.Scope)
	fmt.Printf("Agent:       %s\n", meta.Agent)
	fmt.Printf("Installed:   %s\n", meta.InstalledAt.Format(time.RFC1123))
	fmt.Printf("Local Path:  %s\n", destPath)

	return nil
}
