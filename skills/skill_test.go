package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillInstall(t *testing.T) {
	// Create a dummy skill
	tempSource := t.TempDir()
	skillMd := filepath.Join(tempSource, "SKILL.md")
	_ = os.WriteFile(skillMd, []byte("# Test Skill"), 0644)

	// Override standard directories for testing using env var or similar?
	// We can't easily mock user home or wd in the current implementation without refactoring.
	// We'll run it and check if it errors out for now.
	// Actually, we can temporarily change the working directory.

	tempDest := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tempDest)
	defer func() { _ = os.Chdir(originalWd) }()

	err := SkillInstall(tempSource, "test_skill", "project", "")
	if err != nil {
		t.Fatalf("SkillInstall failed: %v", err)
	}

	// Verify installation
	expectedPath := filepath.Join(tempDest, ".agents", "skills", "test_skill")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Skill was not installed to expected path: %s", expectedPath)
	}

	// Verify SKILL.md
	if _, err := os.Stat(filepath.Join(expectedPath, "SKILL.md")); os.IsNotExist(err) {
		t.Fatalf("SKILL.md was not copied")
	}

	// Verify metadata
	meta, err := readSkillMetadata(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}
	if meta.Name != "test_skill" {
		t.Errorf("Expected name 'test_skill', got '%s'", meta.Name)
	}
	if meta.Scope != "project" {
		t.Errorf("Expected scope 'project', got '%s'", meta.Scope)
	}
}

func TestSkillInstall_PathTraversal(t *testing.T) {
	tempSource := t.TempDir()
	skillMd := filepath.Join(tempSource, "SKILL.md")
	_ = os.WriteFile(skillMd, []byte("# Test Skill"), 0644)

	// Attempt to install to an unsafe name (path traversal)
	tempDest := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tempDest)
	defer func() { _ = os.Chdir(originalWd) }()

	err := SkillInstall(tempSource, "../malicious_skill", "project", "")
	expectedEscapedPath := filepath.Join(tempDest, ".agents", "malicious_skill")
	if err == nil {
		t.Fatal("SkillInstall accepted a path traversal skill name")
	}
	if _, statErr := os.Stat(expectedEscapedPath); !os.IsNotExist(statErr) {
		t.Fatalf("path traversal installed a skill outside the skills root: %s", expectedEscapedPath)
	}
}

func TestResolveSkillPath(t *testing.T) {
	path, err := resolveSkillPath("", "project", "nested/skill")
	if err != nil {
		t.Fatalf("resolveSkillPath() error = %v", err)
	}
	if filepath.Base(path) != "skill" {
		t.Errorf("resolveSkillPath() = %q, want path ending in skill", path)
	}
	if _, err := resolveSkillPath("", "project", "../../escape"); err == nil {
		t.Error("resolveSkillPath accepted a traversal path")
	}
	if _, err := resolveSkillPath("", "invalid", "skill"); err == nil {
		t.Error("resolveSkillPath accepted an invalid scope")
	}
	root, err := resolveSkillRoot("", "project")
	if err != nil {
		t.Fatalf("resolveSkillRoot() error = %v", err)
	}
	if filepath.Base(root) != "skills" {
		t.Errorf("resolveSkillRoot() = %q, want path ending in skills", root)
	}
}

func TestSkillListAndInspectValidation(t *testing.T) {
	if err := SkillList("invalid", ""); err == nil {
		t.Error("SkillList accepted an invalid scope")
	}
	if err := SkillInspect("", "project", ""); err == nil {
		t.Error("SkillInspect accepted an empty skill name")
	}
	if err := SkillInspect("missing", "project", ""); err == nil {
		t.Error("SkillInspect accepted a missing skill")
	}
}

func TestValidateSafePath(t *testing.T) {
	root := "/target/root"
	tests := []struct {
		unsafePath string
		expectErr  bool
	}{
		{"/target/root/safe/path", false},
		{"/target/root/safe/../path", false}, // cleans to /target/root/path
		{"/target/root/../escape", true},
		{"/escape", true},
		{"../escape", true},
	}

	for _, tc := range tests {
		_, err := validateSafePath(root, tc.unsafePath)
		if (err != nil) != tc.expectErr {
			t.Errorf("validateSafePath(%q, %q) expectErr=%v got err=%v", root, tc.unsafePath, tc.expectErr, err)
		}
	}
}

func TestSkillUpdate_Success(t *testing.T) {
	tempDest := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tempDest)
	defer func() { _ = os.Chdir(originalWd) }()

	// Setup initial skill
	tempSource := t.TempDir()
	skillMd := filepath.Join(tempSource, "SKILL.md")
	_ = os.WriteFile(skillMd, []byte("# Version 1"), 0644)

	err := SkillInstall(tempSource, "update_skill", "project", "")
	if err != nil {
		t.Fatalf("Initial install failed: %v", err)
	}

	// Wait a moment so the revision (timestamp based for local) changes
	// Alternatively, we can just modify the file to trigger a different timestamp
	// Local fetch revision is determined by `SKILL.md` mtime. Since filesystems sometimes have low resolution,
	// we use force=true for this test, or we can spoof the mtime. Let's use --force.
	_ = os.WriteFile(skillMd, []byte("# Version 2"), 0644)

	// Run update
	err = SkillUpdate("update_skill", false, "project", "", true)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	expectedPath := filepath.Join(tempDest, ".agents", "skills", "update_skill")
	content, _ := os.ReadFile(filepath.Join(expectedPath, "SKILL.md"))
	if string(content) != "# Version 2" {
		t.Fatalf("Skill was not updated, content: %s", string(content))
	}
}

func TestSkillUpdate_NoOp(t *testing.T) {
	tempDest := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tempDest)
	defer func() { _ = os.Chdir(originalWd) }()

	tempSource := t.TempDir()
	skillMd := filepath.Join(tempSource, "SKILL.md")
	_ = os.WriteFile(skillMd, []byte("# Version 1"), 0644)

	_ = SkillInstall(tempSource, "noop_skill", "project", "")

	// Run update without modifying source
	err := SkillUpdate("noop_skill", false, "project", "", false)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	expectedPath := filepath.Join(tempDest, ".agents", "skills", "noop_skill")
	content, _ := os.ReadFile(filepath.Join(expectedPath, "SKILL.md"))
	if string(content) != "# Version 1" {
		t.Fatalf("Skill should not have been updated, content: %s", string(content))
	}
}

func TestSkillRemove(t *testing.T) {
	tempDest := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tempDest)
	defer func() { _ = os.Chdir(originalWd) }()

	tempSource := t.TempDir()
	skillMd := filepath.Join(tempSource, "SKILL.md")
	_ = os.WriteFile(skillMd, []byte("# To be removed"), 0644)

	_ = SkillInstall(tempSource, "remove_skill", "project", "")

	expectedPath := filepath.Join(tempDest, ".agents", "skills", "remove_skill")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Setup failed, skill not installed.")
	}

	err := SkillRemove("remove_skill", "project", "")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Fatalf("Skill directory was not removed.")
	}
}
