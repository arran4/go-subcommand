package commentv1

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/parsers"
	"golang.org/x/tools/txtar"
)

func TestParseSubCommandComments(t *testing.T) {
	// Handler for "commentv1 parsing tests"
	runParsingTest := func(t *testing.T, archive *txtar.Archive) {
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
		if expectedOutput == nil {
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

		// Normalize newlines for comparison
		actualStr := strings.TrimSpace(string(actualJson))
		expectedStr := strings.TrimSpace(string(expectedOutput))

		if actualStr != expectedStr {
			t.Errorf("Mismatch in %s:\nExpected:\n%s\nGot:\n%s", t.Name(), expectedStr, actualStr)
		}
	}

	// Handler for "commentv1 circular parsing tests"
	runCircularTest := func(t *testing.T, archive *txtar.Archive) {
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
	}

	parsers.RunTxtarTests(t, os.DirFS("."), "testdata", map[string]func(*testing.T, *txtar.Archive){
		"commentv1 parsing tests":          runParsingTest,
		"commentv1 circular parsing tests": runCircularTest,
	}, runParsingTest)
}
