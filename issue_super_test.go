package go_subcommand

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed testdata/issue_super.go
var issueSuperSrc string

func TestIssueSuper(t *testing.T) {
	fs := setupProject(t, issueSuperSrc)
	writer := runGenerateInMemory(t, fs)

	// verify files are generated
	if _, ok := writer.Files["cmd/app/root.go"]; !ok {
		t.Fatal("cmd/app/root.go not generated")
	}
	// debug generated files
	var files []string
	for k := range writer.Files {
		files = append(files, k)
	}
	t.Logf("Generated files: %v", files)

	// Get the first subcommand file content
	var cmdContent string
	for k, v := range writer.Files {
		if strings.HasSuffix(k, "child.go") {
			cmdContent = string(v)
			t.Logf("Checking content of %s", k)
			break
		}
	}

	// #114 Global Init Hook
	rootContent := string(writer.Files["cmd/app/root.go"])
	if !strings.Contains(rootContent, "if c.CommandAction != nil {") {
		t.Error("Root command action check missing")
	}
	// Check if "global init failed" error message is present
	if !strings.Contains(rootContent, "global init failed") {
		t.Error("Global init error message missing in root.go (implies global hook logic missing)")
	}

	// #49 Custom Parser
	// Check if ParseCustom is called
	if !strings.Contains(rootContent, "ParseCustom(value)") {
		t.Error("Custom parser call missing in root.go")
	}

	// #220 GNU Style & #82 Repeatable Slices
	// Search for stricter substring to avoid whitespace issues
	if !strings.Contains(cmdContent, `HasPrefix(value, "=")`) {
		t.Errorf("Missing check for '=' prefix in short flag value (GNU style) in cmd content. Content:\n%s", cmdContent)
	}
	// Root command in this test has no short value flags, so the block is not generated.
	// if !strings.Contains(rootContent, `HasPrefix(value, "=")`) {
	// 	t.Errorf("Missing check for '=' prefix in short flag value (GNU style) in root content. Content:\n%s", rootContent)
	// }
	// Also check for append logic for repeatable slices
	// Note: Field name case depends on parameter name in function.
	// In issue_super.go: func Child(slice []string...) -> field slice
	if !strings.Contains(cmdContent, "append(c.slice,") {
		t.Errorf("Missing append for slice flags. Content:\n%s", cmdContent)
	}

	// #331 Required vs Optional
	// Check usage text for required/optional indicators
	// We check child_usage.txt because root usage.txt is not generated/used by default RootCmd
	childUsageContent := string(writer.Files["cmd/app/templates/child_usage.txt"])
	t.Logf("Child Usage Content:\n%s", childUsageContent)

	if strings.Contains(childUsageContent, "(required)") {
		t.Log("Found (required) indicating required parameter")
	} else {
		t.Error("Did not find (required) for required parameter in child usage")
	}

	// #330 Parent Flags
	// Check grouping in child usage
	if !strings.Contains(childUsageContent, "`app` Flags:") {
		t.Error("Missing '`app` Flags:' section in child usage")
	}
	if !strings.Contains(childUsageContent, "`child` Flags:") {
		t.Error("Missing '`child` Flags:' section in child usage")
	}
}
