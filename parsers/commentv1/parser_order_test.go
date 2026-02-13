package commentv1

import (
	"embed"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/model"
	"golang.org/x/tools/txtar"
)

//go:embed testdata/parser_order/*.txtar
var parserOrderTestData embed.FS

func TestParserOrder(t *testing.T) {
	entries, err := parserOrderTestData.ReadDir("testdata/parser_order")
	if err != nil {
		t.Fatalf("failed to read testdata dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			content, err := parserOrderTestData.ReadFile("testdata/parser_order/" + entry.Name())
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			archive := txtar.Parse(content)
			fsys := make(fstest.MapFS)
			var expectedAliases []string
			var expectedSubCommands []string

			for _, f := range archive.Files {
				name := strings.TrimSpace(f.Name)
				switch name {
				case "expected_aliases.txt":
					lines := strings.Split(strings.TrimSpace(string(f.Data)), "\n")
					for _, l := range lines {
						if strings.TrimSpace(l) != "" {
							expectedAliases = append(expectedAliases, strings.TrimSpace(l))
						}
					}
				case "expected_subcommands.txt":
					lines := strings.Split(strings.TrimSpace(string(f.Data)), "\n")
					for _, l := range lines {
						if strings.TrimSpace(l) != "" {
							expectedSubCommands = append(expectedSubCommands, strings.TrimSpace(l))
						}
					}
				default:
					fsys[name] = &fstest.MapFile{Data: f.Data}
				}
			}

			// Run ParseGoFiles
			model, err := ParseGoFiles(fsys, ".")
			if err != nil {
				t.Fatalf("ParseGoFiles failed: %v", err)
			}

			if len(expectedAliases) > 0 {
				if len(model.Commands) == 0 {
					t.Fatalf("Model has no commands")
				}
				if len(model.Commands[0].SubCommands) == 0 {
					t.Fatalf("Model has no subcommands")
				}
				if len(model.Commands[0].SubCommands[0].Parameters) == 0 {
					t.Fatalf("Subcommand has no parameters")
				}

				// Assumption: The first parameter of the first subcommand is what we are testing for aliases
				got := model.Commands[0].SubCommands[0].Parameters[0].FlagAliases
				if !slicesEqual(got, expectedAliases) {
					t.Errorf("Aliases mismatch. Got: %v, Expected: %v", got, expectedAliases)
				}
			}

			if len(expectedSubCommands) > 0 {
				if len(model.Commands) == 0 {
					t.Fatalf("Model has no commands")
				}
				got := getSubCommandNames(model.Commands[0].SubCommands)
				if !slicesEqual(got, expectedSubCommands) {
					t.Errorf("Subcommands mismatch. Got: %v, Expected: %v", got, expectedSubCommands)
				}
			}
		})
	}
}

func getSubCommandNames(subs []*model.SubCommand) []string {
	var names []string
	for _, s := range subs {
		names = append(names, s.SubCommandName)
	}
	return names
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
