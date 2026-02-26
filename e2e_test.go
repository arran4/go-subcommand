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
// Each .txtar file must contain input files prefixed with "input/" and expected output files
// prefixed with "expected/".
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

			inputFS := make(fstest.MapFS)
			expectedFiles := make(map[string]string)

			for _, f := range archive.Files {
				if strings.HasPrefix(f.Name, "input/") {
					name := strings.TrimPrefix(f.Name, "input/")
					inputFS[name] = &fstest.MapFile{Data: f.Data}
				} else if strings.HasPrefix(f.Name, "expected/") {
					name := strings.TrimPrefix(f.Name, "expected/")
					expectedFiles[name] = string(f.Data)
				}
			}

			// Run Generator
			writer := NewCollectingFileWriter()
			// Use "." as root since files are at root of MapFS
			if err := GenerateWithFS(inputFS, writer, ".", "", "commentv1", nil, false); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Verify generated files match expected files
			if len(expectedFiles) > 0 {
				for path, expectedContent := range expectedFiles {
					generatedContentBytes, ok := writer.Files[path]
					if !ok {
						t.Errorf("Expected file %s was not generated", path)
						continue
					}
					generatedContent := string(generatedContentBytes)
					if generatedContent != expectedContent {
						t.Errorf("File %s content mismatch", path)
						// For debugging, one could print the diff here.
						// t.Logf("Expected:\n%s\nGot:\n%s\n", expectedContent, generatedContent)
					}
				}

				// Check for unexpected files
				for path := range writer.Files {
					if _, ok := expectedFiles[path]; !ok {
						t.Errorf("Unexpected file generated: %s", path)
					}
				}
			} else {
				t.Errorf("No expected files found in %s. Please update the test file.", entry.Name())
			}
		})
	}
}
