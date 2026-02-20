package go_subcommand

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed testdata/issue330.go
var issue330Src string

func TestIssue330_ParentFlagsReporting(t *testing.T) {
	fs := setupProject(t, issue330Src)
	writer := runGenerateInMemory(t, fs)

	// Check Child usage
	usagePath := "cmd/app/templates/child_usage.txt"
	content, ok := writer.Files[usagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", usagePath)
	}

	usageText := string(content)
	t.Logf("Generated usage (Child):\n%s", usageText)

	if !strings.Contains(usageText, "`child` Flags:") {
		t.Error("Missing '`child` Flags:' section in usage")
	}

	if !strings.Contains(usageText, "`parent` Flags:") {
		t.Error("Missing '`parent` Flags:' section in usage")
	}

	// Check that --dir is under parent Flags
	parentIndex := strings.Index(usageText, "`parent` Flags:")
	childIndex := strings.Index(usageText, "`child` Flags:")
	dirIndex := strings.Index(usageText, "--dir")

	if parentIndex != -1 && childIndex != -1 && dirIndex != -1 {
		if dirIndex < parentIndex {
			t.Error("--dir flag appears before `parent` Flags: header")
		}
		if dirIndex > childIndex {
			t.Error("--dir flag appears after `child` Flags: header (should be under parent)")
		}
	}

	// Check GrandChild usage
	gcUsagePath := "cmd/app/templates/grandchild_usage.txt"
	gcContent, ok := writer.Files[gcUsagePath]
	if !ok {
		t.Fatalf("Usage file not found: %s", gcUsagePath)
	}
	gcUsageText := string(gcContent)

	if !strings.Contains(gcUsageText, "`parent` Flags:") {
		t.Error("Missing '`parent` Flags:' section in GrandChild usage")
	}
	if !strings.Contains(gcUsageText, "--dir") {
		t.Error("GrandChild usage missing --dir (inherited)")
	}
	if !strings.Contains(gcUsageText, "The directory") {
		t.Error("GrandChild usage missing inherited description for --dir")
	}
}
