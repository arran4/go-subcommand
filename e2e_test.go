package go_subcommand

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/tools/txtar"
)

//go:embed templates/testdata/e2e/*.txtar
var e2eTemplatesFS embed.FS

// TestE2E_Generation runs the full parser and generator pipeline on input files
// defined in templates/testdata/e2e/*.txtar.
// Each .txtar file must contain "input/go.mod" and "input/main.go" (or other source files).
// It verifies that code generation matches the expected files in "expected/".
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
			expectedFiles := make(map[string][]byte)
			hasTestsTxt := false

			for _, f := range archive.Files {
				if f.Name == "tests.txt" {
					hasTestsTxt = true
					if strings.TrimSpace(string(f.Data)) != "This is a full go e2e test" {
						t.Errorf("tests.txt content mismatch. Got: %q, Want: %q", string(f.Data), "This is a full go e2e test")
					}
					continue
				}

				if strings.HasPrefix(f.Name, "input/") {
					name := strings.TrimPrefix(f.Name, "input/")
					inputFS[name] = &fstest.MapFile{Data: f.Data}
				} else if strings.HasPrefix(f.Name, "expected/") {
					name := strings.TrimPrefix(f.Name, "expected/")
					expectedFiles[name] = f.Data
				}
			}

			if !hasTestsTxt {
				t.Errorf("tests.txt missing in %s", entry.Name())
			}

			if len(inputFS) == 0 {
				// If we have no input files, we can't run the generator.
				// However, maybe the test is just checking structure?
				// But GenerateWithFS will fail if no commands found.
				// Let's assume input is required.
				// For now, fail if input is missing to prompt migration.
				t.Fatalf("No input files found in %s (must be under input/)", entry.Name())
			}

			// Run Generator
			writer := NewCollectingFileWriter()
			// Use "." as root since files are at root of MapFS
			if err := GenerateWithFS(inputFS, writer, ".", "", "commentv1", nil, false); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Verify generated content matches expected content
			var missingFiles []string
			var mismatchFiles []string

			// Check expected files
			for path, expectedContent := range expectedFiles {
				generatedContent, ok := writer.Files[path]
				if !ok {
					missingFiles = append(missingFiles, path)
					continue
				}
				if !bytes.Equal(generatedContent, expectedContent) {
					mismatchFiles = append(mismatchFiles, path)
				}
			}

			if len(missingFiles) > 0 || len(mismatchFiles) > 0 || len(expectedFiles) == 0 {
				t.Errorf("Verification failed for %s", entry.Name())
				if len(missingFiles) > 0 {
					t.Errorf("Missing files: %v", missingFiles)
				}
				if len(mismatchFiles) > 0 {
					t.Errorf("Content mismatch in files: %v", mismatchFiles)
				}
				if len(expectedFiles) == 0 {
					t.Errorf("No expected files defined")
				}

				// Write suggested txtar update to file
				suggestedContent := new(bytes.Buffer)
				for path, content := range writer.Files {
					fmt.Fprintf(suggestedContent, "-- expected/%s --\n", path)
					suggestedContent.Write(content)
					if len(content) > 0 && content[len(content)-1] != '\n' {
						suggestedContent.WriteByte('\n')
					}
				}
				if err := os.WriteFile("SUGGESTED_"+entry.Name(), suggestedContent.Bytes(), 0644); err != nil {
					t.Logf("Failed to write suggested content: %v", err)
				}
			}
		})
	}
}
