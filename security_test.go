package go_subcommand

import (
	"testing"
)

func TestPathTraversalInManPageGeneration(t *testing.T) {
	// 1. Source code with malicious MainCmdName
	src := `package main

// Evil is a subcommand ` + "`../evil foo`" + `
func Evil() {}
`
	fs := setupProject(t, src)
	writer := NewCollectingFileWriter()

	// 2. Run Generate with manDir set
	// We use "manpages" as the target directory.
	err := GenerateWithFS(fs, writer, ".", "manpages", "commentv1", nil, false)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 3. Check for escaped file
	// If traversal works, filepath.Join("manpages", "../evil-foo.1") -> "evil-foo.1"
	// So we look for "evil-foo.1" in writer.Files
	// If it was safe, it should be in "manpages/..."

	escapedFile := "evil-foo.1"
	if _, ok := writer.Files[escapedFile]; ok {
		t.Errorf("Security Vulnerability: Generated file escaped manDir! Found %s", escapedFile)
	}

	expectedSafeFile := "manpages/evil-foo.1"
	if _, ok := writer.Files[expectedSafeFile]; !ok {
		t.Errorf("Expected sanitized file %s to exist, but it does not", expectedSafeFile)
	}
}
