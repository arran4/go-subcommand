package templates

import (
	"bytes"
	"embed"
	"encoding/json"
	"go/format"
	"strings"
	"testing"

	go_subcommand "github.com/arran4/go-subcommand"
	"github.com/arran4/go-subcommand/model"
	"golang.org/x/tools/txtar"
)

//go:embed testdata/*.go.txtar
var goTemplatesFS embed.FS

func TestGoTemplates(t *testing.T) {
	// Parse all templates
	tmpl, err := go_subcommand.ParseTemplates(go_subcommand.TemplatesFS)
	if err != nil {
		t.Fatalf("failed to parse templates: %v", err)
	}

	// Iterate over txtar files
	dirEntries, err := goTemplatesFS.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to read testdata dir: %v", err)
	}

	for _, entry := range dirEntries {
		if !strings.HasSuffix(entry.Name(), ".go.txtar") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			content, err := goTemplatesFS.ReadFile("testdata/" + entry.Name())
			if err != nil {
				t.Fatalf("failed to read %s: %v", entry.Name(), err)
			}

			archive := txtar.Parse(content)
			var inputData []byte
			var expectedOutput []byte
			var templateName string

			// Look for specific files in the archive
			for _, f := range archive.Files {
				switch f.Name {
				case "input.json":
					inputData = f.Data
				case "output.go":
					expectedOutput = f.Data
				case "template_name":
					templateName = strings.TrimSpace(string(f.Data))
				}
			}

			if inputData == nil {
				t.Fatalf("input.json not found in %s", entry.Name())
			}
			if templateName == "" {
				t.Fatalf("template_name not found in %s", entry.Name())
			}

			var data interface{}

			if templateName == "cmd.go.gotmpl" || templateName == "cmd_test.go.gotmpl" {
				var sc model.SubCommand
				if err := json.Unmarshal(inputData, &sc); err != nil {
					t.Fatalf("failed to unmarshal input.json into SubCommand: %v", err)
				}
				populateParents(&sc, nil)
				data = &sc
			} else {
				// root.go.gotmpl and main.go.gotmpl use *Command
				var cmd model.Command
				if err := json.Unmarshal(inputData, &cmd); err != nil {
					t.Fatalf("failed to unmarshal input.json into Command: %v", err)
				}
				for _, sc := range cmd.SubCommands {
					populateParents(sc, nil)
				}
				data = &cmd
			}

			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
				t.Fatalf("failed to execute template %s: %v", templateName, err)
			}

			// Verify Go formatting
			formatted, err := format.Source(buf.Bytes())
			if err != nil {
				t.Errorf("Generated code is not valid Go: %v\nCode:\n%s", err, buf.String())
			} else {
				if !bytes.Equal(formatted, expectedOutput) {
					t.Errorf("Output mismatch for %s:\nExpected:\n%s\nGot:\n%s", entry.Name(), string(expectedOutput), string(formatted))
				}
			}
		})
	}
}

func populateParents(sc *model.SubCommand, parent *model.SubCommand) {
	sc.Parent = parent
	for _, child := range sc.SubCommands {
		populateParents(child, sc)
	}
}
