package parsers

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

// GetTestTypes extracts the list of test types from the "tests.txt" file in the archive.
// Returns a slice of strings, or nil if the file is not found.
func GetTestTypes(archive *txtar.Archive) []string {
	for _, f := range archive.Files {
		if f.Name == "tests.txt" {
			lines := strings.Split(string(f.Data), "\n")
			var types []string
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					types = append(types, trimmed)
				}
			}
			return types
		}
	}
	return nil
}

// GetOptions parses options.json from the archive if present.
func GetOptions(archive *txtar.Archive) (*ParseOptions, error) {
	for _, f := range archive.Files {
		if f.Name == "options.json" {
			var opts ParseOptions
			if err := json.Unmarshal(f.Data, &opts); err != nil {
				return nil, err
			}
			return &opts, nil
		}
	}
	return nil, nil
}

// RunTxtarTests iterates over .txtar files in a directory and runs tests based on their declared type in tests.txt.
// fsys: The file system to read from.
// dir: The directory within fsys to scan.
// handlers: A map where keys are test types (lines in tests.txt) and values are test functions.
// defaultHandler: An optional handler to run if tests.txt is missing. If nil, files without tests.txt are skipped.
func RunTxtarTests(t *testing.T, fsys fs.FS, dir string, handlers map[string]func(*testing.T, *txtar.Archive), defaultHandler func(*testing.T, *txtar.Archive)) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		t.Fatalf("failed to read directory %s: %v", dir, err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(dir, entry.Name())
			content, err := fs.ReadFile(fsys, path)
			if err != nil {
				t.Fatalf("failed to read %s: %v", path, err)
			}

			archive := txtar.Parse(content)
			testTypes := GetTestTypes(archive)

			if len(testTypes) == 0 {
				if defaultHandler != nil {
					defaultHandler(t, archive)
				}
				return
			}

			ranAny := false
			for _, testType := range testTypes {
				if handler, ok := handlers[testType]; ok {
					ranAny = true
					t.Run(testType, func(t *testing.T) {
						handler(t, archive)
					})
				} else {
					t.Errorf("Unknown test type: %s", testType)
				}
			}

			if !ranAny {
				t.Logf("Warning: %s declares tests types %v but none matched registered handlers", entry.Name(), testTypes)
			}
		})
	}
}
