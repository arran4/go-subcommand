package go_subcommand

import (
	"strings"
	"testing"
)

func TestIssueParserPkg_Parsing(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
// myflag: --json-data (parser: "encoding/json".Unmarshal)
func MyCmd(myflag string) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	path := "cmd/app/mycmd.go"
	content, ok := writer.Files[path]
	if !ok {
		t.Fatalf("File not found: %s", path)
	}

	code := string(content)

	// Check if import is present
	if !strings.Contains(code, `"encoding/json"`) {
		t.Errorf("Import 'encoding/json' not found in %s. Content:\n%s", path, code)
	}

	// Check usage: json.Unmarshal(...)
	// The variable name is `value` (hardcoded in cmd.go.gotmpl for parser calls)
	if !strings.Contains(code, `json.Unmarshal(value)`) {
		t.Errorf("Parser call 'json.Unmarshal' not found in %s", path)
	}
}

func TestIssueParserPkg_LocalFunc(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
// myflag: --data (parser: ParseMyType)
func MyCmd(myflag string) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	path := "cmd/app/mycmd.go"
	content, ok := writer.Files[path]
	if !ok {
		t.Fatalf("File not found: %s", path)
	}

	code := string(content)

	// Check usage: ParseMyType(...)
	if !strings.Contains(code, `ParseMyType(value)`) {
		t.Errorf("Parser call 'ParseMyType' not found in %s. Content:\n%s", path, code)
	}

	// Ensure no extra import was added (e.g. empty string)
	if strings.Contains(code, `import ""`) {
		t.Errorf("Found empty import in %s", path)
	}
}
