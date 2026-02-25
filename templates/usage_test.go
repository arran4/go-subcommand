package templates

import (
	"bytes"
	"embed"
	"encoding/json"
	"strings"
	"testing"
	"text/template"

	"github.com/arran4/go-subcommand/model"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/tools/txtar"
)

//go:embed cmd/templates/usage.txt.gotmpl testdata/*.txtar
var templatesFS embed.FS

func TestUsageTemplate(t *testing.T) {
	// Parse the template
	tmplContent, err := templatesFS.ReadFile("cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("failed to read usage.txt.gotmpl: %v", err)
	}

	funcs := template.FuncMap{
		"lower":   strings.ToLower,
		"title":   func(s string) string { return cases.Title(language.Und, cases.NoLower).String(s) },
		"upper":   strings.ToUpper,
		"replace": strings.ReplaceAll,
		"add":     func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("usage").Funcs(funcs).Parse(string(tmplContent))
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	// Iterate over txtar files
	dirEntries, err := templatesFS.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to read testdata dir: %v", err)
	}

	for _, entry := range dirEntries {
		if !strings.HasSuffix(entry.Name(), ".txtar") || strings.HasSuffix(entry.Name(), ".go.txtar") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			content, err := templatesFS.ReadFile("testdata/" + entry.Name())
			if err != nil {
				t.Fatalf("failed to read %s: %v", entry.Name(), err)
			}

			archive := txtar.Parse(content)
			var inputData []byte
			var expectedOutput []byte

			for _, f := range archive.Files {
				switch f.Name {
				case "input.json":
					inputData = f.Data
				case "output.txt":
					expectedOutput = f.Data
				}
			}

			if inputData == nil {
				t.Fatalf("input.json not found in %s", entry.Name())
			}

			var input struct {
				model.SubCommand
				Recursive bool
			}
			if err := json.Unmarshal(inputData, &input); err != nil {
				t.Fatalf("failed to unmarshal input.json: %v", err)
			}

			// Wrapper for template
			data := struct {
				*model.SubCommand
				Recursive bool
			}{
				SubCommand: &input.SubCommand,
				Recursive:  input.Recursive,
			}

			populateParentsUsage(data.SubCommand, nil)
			ApplyDefaultsToSubCommand(data.SubCommand)

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			// Verify the generated content is a valid template
			if _, err := template.New("generated").Parse(buf.String()); err != nil {
				t.Errorf("Generated content is not a valid template: %v\nContent:\n%s", err, buf.String())
			}

			if !bytes.Equal(buf.Bytes(), expectedOutput) {
				t.Errorf("Output mismatch for %s:\nExpected:\n%q\nGot:\n%q", entry.Name(), string(expectedOutput), buf.String())
			}
		})
	}
}

func populateParentsUsage(sc *model.SubCommand, parent *model.SubCommand) {
	sc.Parent = parent
	for _, child := range sc.SubCommands {
		populateParentsUsage(child, sc)
	}
}
