package go_subcommand

import (
	"strings"
	"testing"
)

func TestIssue11_RootLevelHelpUsageVersion(t *testing.T) {
	src := `package main

// Nested is a subcommand ` + "`app nested`" + `
func Nested() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	// Check root command code
	rootPath := "cmd/app/root.go"
	content, ok := writer.Files[rootPath]
	if !ok {
		t.Fatalf("Root file not found: %s", rootPath)
	}

	rootCode := string(content)

	// Requirement: Version only at top level.
	// We verify RootCmd supports Version.
	if !strings.Contains(rootCode, "Version  string") {
		t.Errorf("Root command should support Version")
	}

	// Requirement: Help and usage should work at every level (including Root).
	// We expect 'help' and 'usage' to be registered as commands in the RootCmd.
	// The current generator populates commands like: c.Commands["name"] = ...

	if !strings.Contains(rootCode, `c.Commands["help"]`) {
		t.Errorf("Issue 11: Root command should have 'help' command registered")
	}
	if !strings.Contains(rootCode, `c.Commands["usage"]`) {
		t.Errorf("Issue 11: Root command should have 'usage' command registered")
	}

	// Check if 'version' is registered as a command if it is explicitly listed in usage.
	// The prompt says "version only at top level".
	// If it is just a flag or handled in Execute, that might be fine, but if it appears in "Commands:" list
	// it should be in the map.
	// Current usage template lists "version".
	// So we expect it to be a command.
	if !strings.Contains(rootCode, `c.Commands["version"]`) {
		t.Errorf("Issue 11: Root command should have 'version' command registered")
	}
}

func TestIssue11_42_52_HelpUsageVersionVisibility(t *testing.T) {
	src := `package main

// Nested is a subcommand ` + "`app nested`" + `
func Nested() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	// Check nested command usage
	usagePath := "cmd/app/templates/nested_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	usageText := string(content)

	// Issue 11 & 42: Help and usage should work (be visible)
	missing := []string{}
	expected := []string{"help", "usage"}
	for _, exp := range expected {
		if !strings.Contains(usageText, exp) {
			missing = append(missing, exp)
		}
	}
	if len(missing) > 0 {
		t.Errorf("Issue 11/42: Expected nested usage text to contain %v, but they were missing.\nContent:\n%s", missing, usageText)
	}

	// Issue 41/52: Version should NOT be at nested level
	if strings.Contains(usageText, "version      Print version information") {
		t.Errorf("Issue 41/52: Nested usage text contains 'version' command which should only be at top level.\nContent:\n%s", usageText)
	}
}
