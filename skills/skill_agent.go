package skills

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveSkillPath determines where a skill should be installed based on agent and scope.
// If scope is "project", it installs in the current working directory's .agents/skills/.
// If scope is "user", it installs in the user's home directory under .agents/skills/ (or an agent-specific path).
// Getwd is a wrapper around os.Getwd to allow for testing overrides.
var Getwd = os.Getwd

// UserHomeDir is a wrapper around os.UserHomeDir to allow for testing overrides.
var UserHomeDir = os.UserHomeDir

func resolveSkillPath(agentName, scope, skillName string) (string, error) {
	root, err := resolveSkillRoot(agentName, scope)
	if err != nil {
		return "", err
	}
	return validateSafePath(root, filepath.Join(root, skillName))
}

// resolveSkillRoot returns the root directory for all skills for a given agent and scope.
func resolveSkillRoot(agentName, scope string) (string, error) {
	var baseDir string

	switch scope {
	case "project":
		wd, err := Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		baseDir = filepath.Join(wd, ".agents")
	case "user":
		home, err := UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		// Generic fallback path unless agent specifically overrides
		baseDir = filepath.Join(home, ".agents")

		// Some agents might have specific non-standard paths, this can be expanded later.
		switch agentName {
		case "cursor":
			// cursor actually uses .cursor/rules in newer versions, but .agents is a common standard being adopted
			baseDir = filepath.Join(home, ".cursor", "skills")
		case "copilot":
			baseDir = filepath.Join(home, ".github", "copilot", "skills")
		}
	default:
		return "", fmt.Errorf("invalid scope: %s (must be 'user' or 'project')", scope)
	}

	return filepath.Join(baseDir, "skills"), nil
}
