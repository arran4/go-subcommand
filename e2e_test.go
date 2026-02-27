package go_subcommand

import (
	"bytes"
	"embed"
	"fmt"
	"os"
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
// Each .txtar file must contain input files prefixed with "input/" and expected output files
// prefixed with "expected/".
func TestE2E_Generation(t *testing.T) {
	dirEntries, err := e2eTemplatesFS.ReadDir("templates/testdata/e2e")
	if err != nil {
		t.Fatalf("failed to read e2e dir: %v", err)
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
			hasTestsTxt := false

			for _, f := range archive.Files {
				if f.Name == "tests.txt" {
					hasTestsTxt = true
					continue
				}

				if strings.HasPrefix(f.Name, "input/") {
					name := strings.TrimPrefix(f.Name, "input/")
					inputFS[name] = &fstest.MapFile{Data: f.Data}
				} else if strings.HasPrefix(f.Name, "expected/") {
					name := strings.TrimPrefix(f.Name, "expected/")
					expectedFiles[name] = string(f.Data)
				}
			}

			if !hasTestsTxt {
				// Fallback/Legacy: If no tests.txt, maybe assume it's just a test without specific instructions?
				// But our new standard requires it. For backward compatibility if any old tests exist,
				// we might want to skip or default. For now, let's just proceed.
			}

			if len(inputFS) == 0 {
				t.Fatalf("No input files found in %s (must be under input/)", entry.Name())
			}

			// Run Generator
			writer := NewCollectingFileWriter()
			// Use "." as root since files are at root of MapFS
			if err := GenerateWithFS(inputFS, writer, ".", "", nil); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Verify generated content matches expected content
			var missingFiles []string
			var mismatchFiles []string

			// Check expected files
			for path, expectedContent := range expectedFiles {
				generatedContentBytes, ok := writer.Files[path]
				if !ok {
					missingFiles = append(missingFiles, path)
					continue
				}
				generatedContent := string(generatedContentBytes)
				if generatedContent != expectedContent {
					mismatchFiles = append(mismatchFiles, path)
				}
			}

			if len(missingFiles) > 0 || len(mismatchFiles) > 0 {
				t.Errorf("Verification failed for %s", entry.Name())
				if len(missingFiles) > 0 {
					t.Errorf("Missing files: %v", missingFiles)
				}
				if len(mismatchFiles) > 0 {
					t.Errorf("Content mismatch in files: %v", mismatchFiles)
				}

				// Write suggested txtar update to file for easier debugging/updating
				suggestedContent := new(bytes.Buffer)

				// Reconstruct archive files list
				var newFiles []txtar.File

				// Keep input files and tests.txt
				for _, f := range archive.Files {
					if strings.HasPrefix(f.Name, "input/") || f.Name == "tests.txt" {
						newFiles = append(newFiles, f)
					}
				}

				// Add generated files as expected
				for path, content := range writer.Files {
					newFiles = append(newFiles, txtar.File{
						Name: "expected/" + path,
						Data: content,
					})
				}

				archive.Files = newFiles
				if err := os.WriteFile("SUGGESTED_"+entry.Name(), txtar.Format(archive), 0644); err != nil {
					t.Logf("Failed to write suggested content: %v", err)
				}
			}
		})
	}
}

// ShouldRunTest checks if a test should run based on the tests.txt file content.
// This allows selectively enabling tests, especially circular ones that might be flaky or wip.
func ShouldRunTest(testName string, testsConfig string) bool {
	// Simple implementation: check if line exists
	lines := strings.Split(testsConfig, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == testName {
			return true
		}
	}
	return false
}
