package commentv1

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

var update = flag.Bool("update", false, "update golden files")

func TestParseSubCommandComments(t *testing.T) {
	// Find all .txtar files in testdata
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join("testdata", entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}
			archive := txtar.Parse(data)

			var inputComment string
			var expectedOutput []byte

			for _, f := range archive.Files {
				switch f.Name {
				case "input.comment":
					inputComment = string(f.Data)
				case "expected.json":
					expectedOutput = f.Data
				}
			}

			if inputComment == "" {
				t.Fatal("input.comment not found")
			}
			if expectedOutput == nil && !*update {
				t.Fatal("expected.json not found")
			}

			cmdName, subCommandSequence, description, extendedHelp, aliases, params, ok := ParseSubCommandComments(inputComment)

			result := struct {
				CmdName            string
				SubCommandSequence []string
				Description        string
				ExtendedHelp       string
				Aliases            []string
				Params             map[string]ParsedParam
				Ok                 bool
			}{
				CmdName:            cmdName,
				SubCommandSequence: subCommandSequence,
				Description:        description,
				ExtendedHelp:       extendedHelp,
				Aliases:            aliases,
				Params:             params,
				Ok:                 ok,
			}

			actualJson, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal result: %v", err)
			}

			if *update {
				found := false
				for i := range archive.Files {
					if archive.Files[i].Name == "expected.json" {
						archive.Files[i].Data = actualJson
						found = true
						break
					}
				}
				if !found {
					archive.Files = append(archive.Files, txtar.File{
						Name: "expected.json",
						Data: actualJson,
					})
				}
				if err := os.WriteFile(path, txtar.Format(archive), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
				return
			}

			// Normalize newlines for comparison
			actualStr := strings.TrimSpace(string(actualJson))
			expectedStr := strings.TrimSpace(string(expectedOutput))

			if actualStr != expectedStr {
				t.Errorf("Mismatch in %s:\nExpected:\n%s\nGot:\n%s", entry.Name(), expectedStr, actualStr)
			}
		})
	}
}
