package go_subcommand

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// fetchLocal securely copies a local directory to a temporary destination.
// Returns the path to the temporary directory, and the revision (timestamp/hash).
func fetchLocal(source string) (string, string, error) {
	// Ensure source exists and is a directory
	info, err := os.Stat(source)
	if err != nil {
		return "", "", fmt.Errorf("local source error: %w", err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("local source must be a directory")
	}

	tempDir, err := os.MkdirTemp("", "gosubc-skill-fetch-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	err = copyDir(source, tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to copy local skill: %w", err)
	}

	// Use modification time of SKILL.md (or dir) as a pseudo-revision for local files
	revision := info.ModTime().Format("20060102150405")
	skillMdPath := filepath.Join(source, "SKILL.md")
	if mdInfo, err := os.Stat(skillMdPath); err == nil {
		revision = mdInfo.ModTime().Format("20060102150405")
	}

	return tempDir, revision, nil
}

// fetchRemote clones a remote git repository to a temporary directory.
func fetchRemote(source string) (string, string, error) {
	tempDir, err := os.MkdirTemp("", "gosubc-skill-fetch-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Basic git clone. We assume 'source' is a valid git URL or owner/repo format.
	// E.g., github.com/owner/repo or owner/repo (which we prefix with https://github.com/)
	repoURL := source
	if !strings.HasPrefix(repoURL, "http://") && !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "git@") {
		// Default to GitHub
		repoURL = "https://github.com/" + source
	}

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tempDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to clone remote repository %s: %s", repoURL, string(output))
	}

	// Get the commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = tempDir
	output, err := cmd.Output()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to get commit hash: %w", err)
	}
	revision := strings.TrimSpace(string(output))

	// Remove .git directory so it's not considered part of the skill payload
	os.RemoveAll(filepath.Join(tempDir, ".git"))

	return tempDir, revision, nil
}

// fetch retrieves a skill from a source (local or remote) to a temporary directory.
func fetch(source string) (string, string, bool, error) {
	info, err := os.Stat(source)
	if err == nil && info.IsDir() {
		tempDir, rev, err := fetchLocal(source)
		return tempDir, rev, true, err
	}

	// If it's not a local directory, try to fetch it as a remote git repository
	tempDir, rev, err := fetchRemote(source)
	return tempDir, rev, false, err
}

// validateSafePath ensures the destination path is safely within the target root directory.
func validateSafePath(targetRoot, unsafePath string) (string, error) {
	cleanRoot := filepath.Clean(targetRoot)
	cleanDest := filepath.Clean(unsafePath)

	if !strings.HasPrefix(cleanDest, cleanRoot+string(filepath.Separator)) && cleanDest != cleanRoot {
		return "", fmt.Errorf("unsafe path detected: %s escapes %s", unsafePath, targetRoot)
	}

	return cleanDest, nil
}

// copyDir recursively copies a directory tree.
func copyDir(src string, dst string) error {
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Skip symlinks for safety
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}

			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(srcFile, dstFile string) error {
	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	// Preserve permissions
	info, err := os.Stat(srcFile)
	if err == nil {
		_ = os.Chmod(dstFile, info.Mode())
	}

	return out.Close()
}
