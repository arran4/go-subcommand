package commentv1

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/parsers"
	"golang.org/x/tools/txtar"
)

func TestCircularParsing(t *testing.T) {
	// Find all .txtar files in testdata
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}

		data, err := os.ReadFile(filepath.Join("testdata", entry.Name()))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		archive := txtar.Parse(data)

		if !parsers.ShouldRunTestStrict(archive, "commentv1 circular parsing tests") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			// Build input FS from txtar
			inputFS := make(fstest.MapFS)
			sourceFileCount := 0

			for _, f := range archive.Files {
				if f.Name == "tests.txt" || f.Name == "options.json" || f.Name == "expected.json" {
					continue
				}
				inputFS[f.Name] = &fstest.MapFile{Data: f.Data}
				if strings.HasSuffix(f.Name, ".go") {
					sourceFileCount++
				}
			}

			if sourceFileCount != 1 {
				t.Errorf("Expected exactly one .go input file, got %d", sourceFileCount)
			}

			options, err := parsers.GetOptions(archive)
			if err != nil {
				t.Fatalf("Failed to parse options.json: %v", err)
			}

			p := &CommentParser{}
			_, err = p.Parse(inputFS, ".", options)
			if err != nil {
				t.Errorf("Parse failed: %v", err)
			}
		})
	}
}
