package go_subcommand

import (
	"strings"
	"testing"
)

func TestIssueAliasParsing_Semicolon(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
// Aliases: a, b; c
func MyCmd() {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	path := "cmd/app/root.go"
	content, ok := writer.Files[path]
	if !ok {
		// Try cmd/root.go if generated there
		path = "cmd/root.go"
		content, ok = writer.Files[path]
		if !ok {
			t.Fatalf("Root file not found")
		}
	}

	code := string(content)
	if !strings.Contains(code, `"a"] = subCmd`) {
		t.Errorf("Alias 'a' not found in %s. Content:\n%s", path, code)
	}
	if !strings.Contains(code, `"b"] = subCmd`) {
		t.Errorf("Alias 'b' not found in %s", path)
	}
	if !strings.Contains(code, `"c"] = subCmd`) {
		t.Errorf("Alias 'c' not found in %s (semicolon support)", path)
	}
}
