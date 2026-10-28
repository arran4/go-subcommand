package templates

import (
	"bytes"
	"embed"
	"encoding/json"
	"os"
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

var usageTemplateDelimsReplacer = strings.NewReplacer(
	"{{", `{{"{{"}}`,
	"}}", `{{"}}"}}`,
)

func escapeUsageText(s string) string {
	return usageTemplateDelimsReplacer.Replace(s)
}

type runtimeUsageSubcommand struct {
	Label       string
	Description string
}

func runtimeUsageFuncs() template.FuncMap {
	return template.FuncMap{
		"subcommand": func(label, description string) runtimeUsageSubcommand {
			return runtimeUsageSubcommand{
				Label:       label,
				Description: description,
			}
		},
		"subcommands": func(items ...runtimeUsageSubcommand) []runtimeUsageSubcommand {
			return items
		},
		"subcommandWidth": func(items []runtimeUsageSubcommand) int {
			width := 0
			for _, item := range items {
				if len(item.Label) > width {
					width = len(item.Label)
				}
			}
			return width
		},
		"wrapFlag": func(maxFlag, maxDef int, flagStr, defStr, descStr string) string {
			return descStr
		},
	}
}

func renderRuntimeUsageTemplate(t *testing.T, generatedTemplate string, recursive bool) string {
	t.Helper()
	tmpl, err := template.New("generated").Funcs(runtimeUsageFuncs()).Parse(generatedTemplate)
	if err != nil {
		t.Fatalf("Generated content is not a valid runtime template: %v\nContent:\n%s", err, generatedTemplate)
	}
	var out bytes.Buffer
	err = tmpl.Execute(&out, struct {
		Recursive bool
		Injected  string
		Exploit   string
	}{
		Recursive: recursive,
		Injected:  "INJECTED_VALUE",
		Exploit:   "EXPLOITED_VALUE",
	})
	if err != nil {
		t.Fatalf("Failed to execute generated runtime usage template (recursive=%v): %v\nContent:\n%s", recursive, err, generatedTemplate)
	}
	return out.String()
}

func TestUsageTemplate(t *testing.T) {
	// Parse the template
	tmplContent, err := templatesFS.ReadFile("cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("failed to read usage.txt.gotmpl: %v", err)
	}

	funcs := template.FuncMap{
		"lower":           strings.ToLower,
		"title":           func(s string) string { return cases.Title(language.Und, cases.NoLower).String(s) },
		"upper":           strings.ToUpper,
		"replace":         strings.ReplaceAll,
		"escapeUsageText": escapeUsageText,
		"add":             func(a, b int) int { return a + b },
		"wrapFlag":        func(maxFlag, maxDef int, flagStr, defStr, descStr string) string { return descStr },
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

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			// Verify the generated content is a valid runtime template and can execute in both modes.
			shallowRendered := renderRuntimeUsageTemplate(t, buf.String(), false)
			recursiveRendered := renderRuntimeUsageTemplate(t, buf.String(), true)
			if len(data.SubCommands) == 0 {
				if shallowRendered != recursiveRendered {
					t.Errorf("Expected identical shallow and recursive output when no child subcommands exist:\nshallow:\n%q\nrecursive:\n%q", shallowRendered, recursiveRendered)
				}
			}

			if *updateGolden {
				found := false
				for i := range archive.Files {
					if archive.Files[i].Name == "output.txt" {
						archive.Files[i].Data = buf.Bytes()
						found = true
						break
					}
				}
				if !found {
					archive.Files = append(archive.Files, txtar.File{
						Name: "output.txt",
						Data: buf.Bytes(),
					})
				}
				if err := os.WriteFile("testdata/"+entry.Name(), txtar.Format(archive), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
			} else if !bytes.Equal(buf.Bytes(), expectedOutput) {
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

func TestUsageTemplateRuntimeFormattingAndEscaping(t *testing.T) {
	tmplContent, err := templatesFS.ReadFile("cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("failed to read usage.txt.gotmpl: %v", err)
	}

	funcs := template.FuncMap{
		"lower":           strings.ToLower,
		"title":           func(s string) string { return cases.Title(language.Und, cases.NoLower).String(s) },
		"upper":           strings.ToUpper,
		"replace":         strings.ReplaceAll,
		"escapeUsageText": escapeUsageText,
		"add":             func(a, b int) int { return a + b },
		"wrapFlag":        func(maxFlag, maxDef int, flagStr, defStr, descStr string) string { return descStr },
	}

	tmpl, err := template.New("usage").Funcs(funcs).Parse(string(tmplContent))
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	t.Run("recursive_and_shallow_runtime_alignment", func(t *testing.T) {
		root := &model.Command{MainCmdName: "app"}
		parent := &model.SubCommand{
			Command:                root,
			SubCommandName:         "service",
			SubCommandFunctionName: "Service",
			SubCommandDescription:  "Manage services",
			SubCommands: []*model.SubCommand{
				{
					Command:               root,
					SubCommandName:        "up",
					SubCommandDescription: "Start service",
				},
				{
					Command:               root,
					SubCommandName:        "silent-group",
					SubCommandDescription: "",
					SubCommands: []*model.SubCommand{
						{
							Command:               root,
							SubCommandName:        "deep-leaf-command",
							SubCommandDescription: "Nested leaf action",
						},
					},
				},
			},
		}
		populateParentsUsage(parent, nil)

		var generated bytes.Buffer
		if err := tmpl.Execute(&generated, parent); err != nil {
			t.Fatalf("stage 1 execute failed: %v", err)
		}
		stage1 := generated.String()

		for _, requiredExpr := range []string{
			"{{- $commands := subcommands -}}",
			"{{- $width := subcommandWidth $commands -}}",
			`{{printf "%-*s" $width .Label}} {{.Description}}`,
		} {
			if !strings.Contains(stage1, requiredExpr) {
				t.Errorf("stage 1 output missing runtime formatting expression %q:\n%s", requiredExpr, stage1)
			}
		}

		shallow := renderRuntimeUsageTemplate(t, stage1, false)
		wantShallow := strings.Join([]string{
			"Usage: app service <subcommand>",
			"",
			"Manage services",
			"",
			"Subcommands:",
			"    up           Start service",
			"    silent-group",
			"    help         Print this help message",
			"    usage        Print this usage message",
			"",
		}, "\n")
		if shallow != wantShallow {
			t.Errorf("shallow output mismatch:\nwant:\n%q\ngot:\n%q", wantShallow, shallow)
		}

		recursive := renderRuntimeUsageTemplate(t, stage1, true)
		wantRecursive := strings.Join([]string{
			"Usage: app service <subcommand>",
			"",
			"Manage services",
			"",
			"Subcommands:",
			"    service up                             Start service",
			"    service silent-group",
			"    service silent-group deep-leaf-command Nested leaf action",
			"    help                                   Print this help message",
			"    usage                                  Print this usage message",
			"",
		}, "\n")
		if recursive != wantRecursive {
			t.Errorf("recursive output mismatch:\nwant:\n%q\ngot:\n%q", wantRecursive, recursive)
		}

		for _, line := range strings.Split(recursive, "\n") {
			if strings.HasSuffix(line, " ") {
				t.Errorf("line has trailing space: %q", line)
			}
		}
	})

	t.Run("identical_shallow_and_recursive_entries_produce_byte_identical_output", func(t *testing.T) {
		root := &model.Command{MainCmdName: "app"}
		rootUsage := &model.SubCommand{
			Command:                root,
			SubCommandName:         "",
			SubCommandFunctionName: "Root",
			SubCommands: []*model.SubCommand{
				{
					Command:               root,
					SubCommandName:        "alpha",
					SubCommandDescription: "Alpha action",
				},
				{
					Command:               root,
					SubCommandName:        "very-long-direct-command",
					SubCommandDescription: "Long direct action",
				},
			},
		}
		// Direct children of root have Parent == nil so SubCommandSequence == SubCommandName.
		populateParentsUsage(rootUsage, nil)
		for _, child := range rootUsage.SubCommands {
			child.Parent = nil
		}

		var generated bytes.Buffer
		if err := tmpl.Execute(&generated, rootUsage); err != nil {
			t.Fatalf("stage 1 execute failed: %v", err)
		}
		shallow := renderRuntimeUsageTemplate(t, generated.String(), false)
		recursive := renderRuntimeUsageTemplate(t, generated.String(), true)
		if shallow != recursive {
			t.Errorf("expected byte-identical output when shallow and recursive entries match:\nshallow:\n%q\nrecursive:\n%q", shallow, recursive)
		}
	})

	t.Run("literal_braces_in_descriptions_cannot_inject_template_actions", func(t *testing.T) {
		root := &model.Command{MainCmdName: "app"}
		cmd := &model.SubCommand{
			Command:                root,
			SubCommandName:         "render",
			SubCommandFunctionName: "Render",
			SubCommandDescription:  "Render {{.Injected}} and unmatched {{ or }} safely",
			SubCommandExtendedHelp: "Extended {{printf \"%s\" .Exploit}} literal }} text {{",
			Parameters: []*model.FunctionParameter{
				{
					Name:        "format",
					Type:        "string",
					Default:     "{{.Injected}}",
					Description: "Format string like {{.Name}} or }}",
					DeclaredIn:  "render",
				},
				{
					Name:               "template-arg",
					Type:               "string",
					IsPositional:       true,
					PositionalArgIndex: 1,
					Description:        "Positional {{.Exploit}} value",
					DeclaredIn:         "render",
				},
			},
			SubCommands: []*model.SubCommand{
				{
					Command:               root,
					SubCommandName:        "preview",
					SubCommandDescription: "Preview {{.Injected}} and }} literal {{",
				},
			},
		}
		populateParentsUsage(cmd, nil)

		var generated bytes.Buffer
		if err := tmpl.Execute(&generated, cmd); err != nil {
			t.Fatalf("stage 1 execute failed: %v", err)
		}

		for _, recursive := range []bool{false, true} {
			rendered := renderRuntimeUsageTemplate(t, generated.String(), recursive)
			if strings.Contains(rendered, "INJECTED_VALUE") || strings.Contains(rendered, "EXPLOITED_VALUE") {
				t.Fatalf("template action was injected in rendered output (recursive=%v):\n%s", recursive, rendered)
			}
			for _, literal := range []string{
				"Render {{.Injected}} and unmatched {{ or }} safely",
				`Extended {{printf "%s" .Exploit}} literal }} text {{`,
				"Preview {{.Injected}} and }} literal {{",
				"Format string like {{.Name}} or }}",
				"Positional {{.Exploit}} value",
			} {
				if !strings.Contains(rendered, literal) {
					t.Errorf("rendered output (recursive=%v) missing literal %q:\n%s", recursive, literal, rendered)
				}
			}
		}
	})
}
