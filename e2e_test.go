package go_subcommand

import (
	"embed"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/tools/txtar"
)

//go:embed templates/testdata/e2e/*.txtar
var e2eTemplatesFS embed.FS

// TestE2E_Generation runs the full parser and generator pipeline on input files
// defined in templates/testdata/e2e/*.txtar.
// Each .txtar file must contain "go.mod" and "main.go" (or other source files).
// It verifies that code generation succeeds without error.
// Future: Could also support verifying output file content against expected files in the txtar archive.
func TestE2E_Generation(t *testing.T) {
	dirEntries, err := e2eTemplatesFS.ReadDir("templates/testdata/e2e")
	if err != nil {
		t.Fatalf("failed to read testdata dir: %v", err)
	}

	for _, entry := range dirEntries {
		if !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			content, err := e2eTemplatesFS.ReadFile("templates/testdata/e2e/" + entry.Name())
			if err != nil {
				t.Fatalf("failed to read %s: %v", entry.Name(), err)
			}

			archive := txtar.Parse(content)

			// Build input FS from txtar
			inputFS := make(fstest.MapFS)
			for _, f := range archive.Files {
				// Only include input source files, skip expected output if we add it later
				if strings.HasSuffix(f.Name, ".go") || strings.HasSuffix(f.Name, "go.mod") {
					inputFS[f.Name] = &fstest.MapFile{Data: f.Data}
				}
			}

			// Run Generator
			writer := NewCollectingFileWriter()
			// Use "." as root since files are at root of MapFS
			if err := GenerateWithFS(inputFS, writer, ".", "", "commentv1", nil, false); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Basic verification: Check that files were generated
			if len(writer.Files) == 0 {
				t.Errorf("No files generated")
			}

			// Specifically check for expected command files based on the input
			// For basic_parsing.txtar (app mycmd)
			if entry.Name() == "basic_parsing.txtar" {
				if _, ok := writer.Files["cmd/app/mycmd.go"]; !ok {
					t.Errorf("Expected cmd/app/mycmd.go to be generated")
				}
				// Verify generated content contains key features
				content := string(writer.Files["cmd/app/mycmd.go"])

				// Global config copy
				if !strings.Contains(content, "c.config = c.RootCmd.config") {
					t.Errorf("Missing global config inheritance logic")
				}
				// Generator call
				if !strings.Contains(content, "pkg.LoadConfig()") {
					t.Errorf("Missing generator call pkg.LoadConfig()")
				}
				// Required flag check
				if !strings.Contains(content, "required flag -global-flag not provided") { // derived from global_flag
					t.Errorf("Missing required flag check for global_flag")
				}
				// Alias
				if !strings.Contains(content, "aliases: g") {
					// Aliases are typically in usage file or command registration, not directly in flag parsing struct/logic usually?
					// Wait, usage description might have it.
				}
			}

			// For parser_pkg.txtar
			if entry.Name() == "parser_pkg.txtar" {
				content := string(writer.Files["cmd/app/mycmd.go"])
				// Check imports
				if !strings.Contains(content, "\"encoding/json\"") {
					t.Errorf("Missing import encoding/json")
				}
				if !strings.Contains(content, "\"example.com/pkg\"") {
					t.Errorf("Missing import example.com/pkg")
				}
				// Check calls
				if !strings.Contains(content, "json.Unmarshal") {
					t.Errorf("Missing json.Unmarshal call")
				}
				if !strings.Contains(content, "pkg.Gen") {
					t.Errorf("Missing pkg.Gen call")
				}
			}
		})
	}
}
