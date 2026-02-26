package parsers

import (
	"encoding/json"
	"strings"

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

// ShouldRunTest checks if the given testType is present in the archive's tests.txt.
// If tests.txt is missing, it returns true (default behavior).
// If tests.txt is present, it returns true only if testType is listed.
func ShouldRunTest(archive *txtar.Archive, testType string) bool {
	types := GetTestTypes(archive)
	if types == nil {
		return true // Default behavior: run if no instructions
	}
	for _, t := range types {
		if t == testType {
			return true
		}
	}
	return false
}

// ShouldRunTestStrict checks if the given testType is present in the archive's tests.txt.
// If tests.txt is missing, it returns false.
func ShouldRunTestStrict(archive *txtar.Archive, testType string) bool {
	types := GetTestTypes(archive)
	if types == nil {
		return false
	}
	for _, t := range types {
		if t == testType {
			return true
		}
	}
	return false
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
