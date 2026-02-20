package go_subcommand

import (
	"strings"
	"testing"
)

func TestIssue330_ParentFlagsReporting(t *testing.T) {
	src := "package main\n" +
        "\n" +
        "// Parent is a subcommand `app parent` that Does work in a directory\n" +
        "//\n" +
        "// Flags:\n" +
        "//\n" +
        "//\tdir: --dir (default: \".\") The directory\n" +
        "func Parent(dir string) {}\n" +
        "\n" +
        "// Child is a subcommand `app parent child` that Does work in a directory\n" +
        "//\n" +
        "// Flags:\n" +
        "//\n" +
        "//\tdir: --dir (from parent)\n" +
        "//\ti: --i (default: 0) A random int\n" +
        "func Child(dir string, i int) {}\n" +
        "\n" +
        "// GrandChild is a subcommand `app parent child grandchild`\n" +
        "func GrandChild() {}\n"

	fs := setupProject(t, src)
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

    if parentIndex == -1 || childIndex == -1 || dirIndex == -1 {
        // Assertions above already cover missing sections
    } else {
        if dirIndex < parentIndex || (childIndex > parentIndex && dirIndex > childIndex) {
            // Wait, order: parent comes BEFORE child in usage usually if root->leaf?
            // My implementation iterates root->leaf. So Parent comes before Child.
            // So `parent` Flags: ... --dir ... `child` Flags: ...
            // So dirIndex > parentIndex AND dirIndex < childIndex.

            // Wait, iteration order in `FullUsageString` is root->leaf.
            // `ParameterGroups` also orders root->leaf.
            // So: `app` -> `parent` -> `child`.
            // Usage output:
            // `parent` Flags:
            // ...
            // `child` Flags:
            // ...

            if dirIndex < parentIndex {
                 t.Error("--dir flag appears before `parent` Flags: header")
            }
            if dirIndex > childIndex {
                 t.Error("--dir flag appears after `child` Flags: header (should be under parent)")
            }
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
