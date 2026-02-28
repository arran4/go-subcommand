package go_subcommand

import (
	"embed"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/parsers"
	"golang.org/x/tools/txtar"
)

//go:embed templates/testdata/e2e/*.txtar
var e2eTemplatesFS embed.FS

// TestE2E_Generation runs the full parser and generator pipeline on input files
// defined in templates/testdata/e2e/*.txtar.
func TestE2E_Generation(t *testing.T) {
	// Handler for "e2e generation tests"
	runE2ETest := func(t *testing.T, archive *txtar.Archive) {
		// Build input FS from txtar
		inputFS := make(fstest.MapFS)
		for _, f := range archive.Files {
			// Only include input source files
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

		// Specific checks based on test name (kept from original)
		if strings.Contains(t.Name(), "basic_parsing.txtar") {
			if _, ok := writer.Files["cmd/app/mycmd.go"]; !ok {
				t.Errorf("Expected cmd/app/mycmd.go to be generated")
			}
			content := string(writer.Files["cmd/app/mycmd.go"])
			if !strings.Contains(content, "pkg.LoadConfig()") {
				t.Errorf("Missing generator call pkg.LoadConfig()")
			}
			if !strings.Contains(content, "required flag -global_flag not provided") {
				t.Errorf("Missing required flag check for global_flag")
			}
		}
		if strings.Contains(t.Name(), "parser_pkg.txtar") {
			if _, ok := writer.Files["cmd/app/mycmd.go"]; !ok {
				t.Fatalf("Expected cmd/app/mycmd.go to be generated")
			}
			content := string(writer.Files["cmd/app/mycmd.go"])
			if !strings.Contains(content, "\"encoding/json\"") {
				t.Errorf("Missing import encoding/json")
			}
			if !strings.Contains(content, "\"example.com/pkg\"") {
				t.Errorf("Missing import example.com/pkg")
			}
			if !strings.Contains(content, "json.Unmarshal") {
				t.Errorf("Missing json.Unmarshal call")
			}
			if !strings.Contains(content, "pkg.Gen") {
				t.Errorf("Missing pkg.Gen call")
			}
		}
	}

	parsers.RunTxtarTests(t, e2eTemplatesFS, "templates/testdata/e2e", map[string]func(*testing.T, *txtar.Archive){
		"e2e generation tests": runE2ETest,
	}, runE2ETest)
}
