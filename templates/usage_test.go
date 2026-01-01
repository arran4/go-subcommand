package templates

import (
	"bytes"
	"embed"
	"encoding/json"
	"strings"
	"testing"
	"text/template"

	"github.com/arran4/go-subcommand"
	"golang.org/x/tools/txtar"
)

//go:embed usage.txt.gotmpl testdata/*.txtar
var templatesFS embed.FS

func TestUsageTemplate(t *testing.T) {
	// Parse the template
	tmplContent, err := templatesFS.ReadFile("usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("failed to read usage.txt.gotmpl: %v", err)
	}

	funcs := template.FuncMap{
		"lower":   strings.ToLower,
		"title":   strings.Title,
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
		if !strings.HasSuffix(entry.Name(), ".txtar") {
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
				if f.Name == "input.json" {
					inputData = f.Data
				} else if f.Name == "output.txt" {
					expectedOutput = f.Data
				}
			}

			if inputData == nil {
				t.Fatalf("input.json not found in %s", entry.Name())
			}

			var subCmd go_subcommand.SubCommand
			if err := json.Unmarshal(inputData, &subCmd); err != nil {
				t.Fatalf("failed to unmarshal input.json: %v", err)
			}

			// When unmarshalling, the embedded pointer *Command might be created but empty if no fields for it were in JSON?
			// But we saw MainCmdName in JSON.
			// Let's verify `ProgName()` uses it correctly.

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, &subCmd); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			if !bytes.Equal(buf.Bytes(), expectedOutput) {
				t.Errorf("Output mismatch for %s:\nExpected:\n%q\nGot:\n%q", entry.Name(), string(expectedOutput), buf.String())
			}
		})
	}
}
