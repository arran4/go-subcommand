package templates

import (
	"bytes"
	"embed"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"text/template"

	gosubctemplates "github.com/arran4/go-subcommand/cmd/gosubc/templates"
	"github.com/arran4/go-subcommand/model"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/tools/txtar"
)

//go:embed cmd/templates/usage.txt.gotmpl cmd/templates/templates.go.gotmpl testdata/*.txtar
var templatesFS embed.FS

var usageTemplateDelimsReplacer = strings.NewReplacer(
	"{{", `{{"{{"}}`,
	"}}", `{{"}}"}}`,
)

func escapeUsageText(s string) string {
	return usageTemplateDelimsReplacer.Replace(s)
}

func renderRuntimeUsageTemplateAtWidth(t *testing.T, generatedTemplate string, recursive bool, width int) string {
	t.Helper()
	t.Setenv("COLUMNS", strconv.Itoa(width))
	cloned, err := gosubctemplates.GetTemplates().Clone()
	if err != nil {
		t.Fatalf("failed to clone runtime templates: %v", err)
	}
	tmpl, err := cloned.New("generated_usage.txt").Parse(generatedTemplate)
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
		t.Fatalf("Failed to execute generated runtime usage template (recursive=%v, width=%d): %v\nContent:\n%s", recursive, width, err, generatedTemplate)
	}
	return out.String()
}

func renderRuntimeUsageTemplate(t *testing.T, generatedTemplate string, recursive bool) string {
	t.Helper()
	return renderRuntimeUsageTemplateAtWidth(t, generatedTemplate, recursive, 80)
}

func stage1UsageFuncs() template.FuncMap {
	return template.FuncMap{
		"lower":           strings.ToLower,
		"title":           func(s string) string { return cases.Title(language.Und, cases.NoLower).String(s) },
		"upper":           strings.ToUpper,
		"replace":         strings.ReplaceAll,
		"escapeUsageText": escapeUsageText,
		"add":             func(a, b int) int { return a + b },
		"list":            func(args ...interface{}) []interface{} { return args },
	}
}

func parseRenderedGoldenName(name string) (width int, recursive bool, ok bool) {
	if !strings.HasPrefix(name, "rendered-") || !strings.HasSuffix(name, ".txt") {
		return 0, false, false
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(name, "rendered-"), ".txt")
	parts := strings.Split(trimmed, "-")
	if len(parts) != 2 {
		return 0, false, false
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil || w <= 0 {
		return 0, false, false
	}
	switch parts[1] {
	case "shallow":
		return w, false, true
	case "recursive":
		return w, true, true
	default:
		return 0, false, false
	}
}

func TestGeneratedTemplatesGoMatchesGotmpl(t *testing.T) {
	gotmplBytes, err := templatesFS.ReadFile("cmd/templates/templates.go.gotmpl")
	if err != nil {
		t.Fatalf("failed to read cmd/templates/templates.go.gotmpl: %v", err)
	}
	generatedBytes, err := os.ReadFile("../cmd/gosubc/templates/templates.go")
	if err != nil {
		t.Fatalf("failed to read ../cmd/gosubc/templates/templates.go: %v", err)
	}
	if !bytes.Equal(gotmplBytes, generatedBytes) {
		t.Errorf("cmd/gosubc/templates/templates.go drifted from templates/cmd/templates/templates.go.gotmpl")
	}
}

func TestUsageTemplate(t *testing.T) {
	// Parse the template
	tmplContent, err := templatesFS.ReadFile("cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("failed to read usage.txt.gotmpl: %v", err)
	}

	tmpl, err := template.New("usage").Funcs(stage1UsageFuncs()).Parse(string(tmplContent))
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
			renderedSpecCount := 0

			for _, f := range archive.Files {
				switch f.Name {
				case "input.json":
					inputData = f.Data
				case "output.txt":
					expectedOutput = f.Data
				default:
					if strings.HasPrefix(f.Name, "rendered-") {
						if _, _, ok := parseRenderedGoldenName(f.Name); !ok {
							t.Fatalf("invalid rendered golden filename %q in %s (want rendered-<width>-<shallow|recursive>.txt)", f.Name, entry.Name())
						}
						renderedSpecCount++
					}
				}
			}

			if inputData == nil {
				t.Fatalf("input.json not found in %s", entry.Name())
			}
			if renderedSpecCount == 0 {
				t.Fatalf("no rendered-<width>-<shallow|recursive>.txt golden found in %s", entry.Name())
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
			if !data.HasDistinctRecursiveSubcommands() {
				if strings.Contains(buf.String(), ".Recursive") {
					t.Errorf("Expected no .Recursive branch in %s when shallow and recursive subcommand listings are identical:\n%s", entry.Name(), buf.String())
				}
				if shallowRendered != recursiveRendered {
					t.Errorf("Expected identical shallow and recursive output when shallow and recursive listings match:\nshallow:\n%q\nrecursive:\n%q", shallowRendered, recursiveRendered)
				}
			}

			if *updateGolden {
				found := false
				for i := range archive.Files {
					if archive.Files[i].Name == "output.txt" {
						archive.Files[i].Data = buf.Bytes()
						found = true
					} else if w, rec, ok := parseRenderedGoldenName(archive.Files[i].Name); ok {
						rendered := renderRuntimeUsageTemplateAtWidth(t, buf.String(), rec, w)
						archive.Files[i].Data = []byte(rendered)
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
			} else {
				if !bytes.Equal(buf.Bytes(), expectedOutput) {
					t.Errorf("Output mismatch for %s (output.txt):\nExpected:\n%q\nGot:\n%q", entry.Name(), string(expectedOutput), buf.String())
				}
				for _, f := range archive.Files {
					if w, rec, ok := parseRenderedGoldenName(f.Name); ok {
						gotRendered := renderRuntimeUsageTemplateAtWidth(t, buf.String(), rec, w)
						if gotRendered != string(f.Data) {
							t.Errorf("Rendered output mismatch for %s (%s):\nExpected:\n%q\nGot:\n%q", entry.Name(), f.Name, string(f.Data), gotRendered)
						}
					}
				}
			}
		})
	}
}

func TestUsageTemplateIOStreams(t *testing.T) {
	tmplContent, err := templatesFS.ReadFile("cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("read usage template: %v", err)
	}
	tmpl, err := template.New("usage").Funcs(stage1UsageFuncs()).Parse(string(tmplContent))
	if err != nil {
		t.Fatalf("parse usage template: %v", err)
	}

	var input struct {
		model.SubCommand
		Recursive bool
	}
	if err := json.Unmarshal(usageIOStreamsFixture, &input); err != nil {
		t.Fatalf("parse IO streams fixture: %v", err)
	}
	data := struct {
		*model.SubCommand
		Recursive bool
	}{SubCommand: &input.SubCommand, Recursive: input.Recursive}
	populateParentsUsage(data.SubCommand, nil)

	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		t.Fatalf("execute usage template: %v", err)
	}
	for _, want := range []string{
		"--reader io.Reader",
		"(default: stdin via '-')",
		"Input reader; use '-' for stdin",
		"--read-closer io.ReadCloser",
		"Closable input; use '-' for stdin",
		"--writer io.Writer",
		"(default: stdout via '-')",
		"Output writer; use '-' for stdout",
		"--write-closer io.WriteCloser",
		"Closable output; use '-' for stdout",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("generated usage missing %q:\n%s", want, output.String())
		}
	}
}

func populateParentsUsage(sc *model.SubCommand, parent *model.SubCommand) {
	sc.Parent = parent
	childParent := sc
	if sc.SubCommandName == "" && parent == nil {
		childParent = nil
	}
	for _, child := range sc.SubCommands {
		populateParentsUsage(child, childParent)
	}
}

func TestUsageTemplateRuntimeFormattingAndEscaping(t *testing.T) {
	tmplContent, err := templatesFS.ReadFile("cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("failed to read usage.txt.gotmpl: %v", err)
	}

	tmpl, err := template.New("usage").Funcs(stage1UsageFuncs()).Parse(string(tmplContent))
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
			`{{formatHelpRow 38 "service up" "Start service"}}`,
			`{{formatHelpRow 38 "service silent-group" ""}}`,
			`{{formatHelpRow 38 "service silent-group deep-leaf-command" "Nested leaf action"}}`,
			`{{formatHelpRow 12 "up" "Start service"}}`,
			`{{formatHelpRow 12 "silent-group" ""}}`,
		} {
			if !strings.Contains(stage1, requiredExpr) {
				t.Errorf("stage 1 output missing inline runtime formatting call %q:\n%s", requiredExpr, stage1)
			}
		}
		for _, forbidden := range []string{"subcommands", "subcommandWidth", "range $commands", "%-40s", "%-10s"} {
			if strings.Contains(stage1, forbidden) {
				t.Errorf("stage 1 output must not contain %q:\n%s", forbidden, stage1)
			}
		}

		shallow := renderRuntimeUsageTemplate(t, stage1, false)
		wantShallow := strings.Join([]string{
			"Usage: app service <subcommand>",
			"",
			"Manage services",
			"",
			"Subcommands:",
			"",
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
			"",
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

	t.Run("hanging_indent_and_narrow_fallback_across_terminal_widths", func(t *testing.T) {
		root := &model.Command{MainCmdName: "app"}
		skill := &model.SubCommand{
			Command:               root,
			SubCommandName:        "skill",
			SubCommandDescription: "Manage skills",
			Parameters: []*model.FunctionParameter{
				{
					Name:        "config",
					Type:        "string",
					FlagAliases: []string{"c", "cfg", "config"},
					Default:     "config.json",
					Description: "Configuration file path to load before executing skill actions.\n\nSecond paragraph explaining override precedence.",
					DeclaredIn:  "skill",
				},
				{
					Name:        "url",
					Type:        "string",
					Description: "Registry endpoint https://registry.example.com/very/long/path/that/exceeds/narrow/columns for fetching.",
					DeclaredIn:  "skill",
				},
				{
					Name:        "quiet",
					Type:        "bool",
					Default:     "false",
					Description: "",
					DeclaredIn:  "skill",
				},
			},
			SubCommands: []*model.SubCommand{
				{
					Command:               root,
					SubCommandName:        "install",
					SubCommandDescription: "Installs an AI agent skill and configures it for the selected environment.",
				},
				{
					Command:               root,
					SubCommandName:        "café",
					SubCommandDescription: "Unicode subcommand description that wraps cleanly at narrow terminal widths.",
				},
			},
		}
		populateParentsUsage(skill, nil)

		var generated bytes.Buffer
		if err := tmpl.Execute(&generated, skill); err != nil {
			t.Fatalf("stage 1 execute failed: %v", err)
		}
		stage1 := generated.String()

		for _, required := range []string{
			`{{formatHelpRow 13 "skill install"`,
			`{{formatHelpRow 7 "install"`,
			`{{formatHelpRow 7 "café"`,
			`{{formatFlagRow 28 26 "-c, --cfg, --config string" "(default: \"config.json\")"`,
			`{{formatFlagRow 28 26 "--url string" ""`,
			`{{formatFlagRow 28 26 "--quiet" "(default: false)" ""}}`,
		} {
			if !strings.Contains(stage1, required) {
				t.Errorf("stage 1 output missing %q:\n%s", required, stage1)
			}
		}

		// Render recursive at width 44:
		// indent for recursive commands is 4 + 13 + 1 = 18 spaces (44 - 18 = 26 >= 16 -> aligned hanging indent).
		narrowRecursive := renderRuntimeUsageTemplateAtWidth(t, stage1, true, 44)
		wantInstallWrapped := strings.Join([]string{
			"    skill install Installs an AI agent skill",
			"                  and configures it for the",
			"                  selected environment.",
		}, "\n")
		if !strings.Contains(narrowRecursive, wantInstallWrapped) {
			t.Errorf("narrow recursive output did not wrap command with hanging indent:\nwant substring:\n%s\ngot:\n%s", wantInstallWrapped, narrowRecursive)
		}

		// Render shallow at width 40:
		// commands have indent 4 + 7 + 1 = 12 (40 - 12 = 28 >= 16 -> aligned),
		// while flags have indent 4 + 28 + 1 + 26 + 1 = 60 > 40 -> compact fallback layout!
		narrow40 := renderRuntimeUsageTemplateAtWidth(t, stage1, false, 40)
		wantCompactConfigFlag := strings.Join([]string{
			`    -c, --cfg, --config string`,
			`        (default: "config.json")`,
			`        Configuration file path to load`,
			`        before executing skill actions.`,
			``,
			`        Second paragraph explaining`,
			`        override precedence.`,
		}, "\n")
		if !strings.Contains(narrow40, wantCompactConfigFlag) {
			t.Errorf("narrow width=40 output did not switch wide flag to compact layout within 40 columns:\nwant substring:\n%s\ngot:\n%s", wantCompactConfigFlag, narrow40)
		}
		for _, line := range strings.Split(narrow40, "\n") {
			if strings.Contains(line, "https://registry.example.com/") {
				continue // unbreakable URL token is preserved intact
			}
			if len([]rune(line)) > 40 {
				t.Errorf("width=40 rendered line exceeded 40 columns (len=%d): %q", len([]rune(line)), line)
			}
		}

		// Render at width 80 to verify aligned flag wrapping, paragraph breaks, long words, and empty description flags:
		// flag indent is 4 + 28 + 1 + 26 + 1 = 60 spaces (80 - 60 = 20 >= 16 -> aligned).
		flagWrapped := renderRuntimeUsageTemplateAtWidth(t, stage1, false, 80)
		wantConfigFlag := strings.Join([]string{
			`    -c, --cfg, --config string   (default: "config.json")   Configuration file`,
			`                                                            path to load before`,
			`                                                            executing skill`,
			`                                                            actions.`,
			``,
			`                                                            Second paragraph`,
			`                                                            explaining override`,
			`                                                            precedence.`,
		}, "\n")
		if !strings.Contains(flagWrapped, wantConfigFlag) {
			t.Errorf("flag wrapping with paragraph break mismatch:\nwant substring:\n%s\ngot:\n%s", wantConfigFlag, flagWrapped)
		}

		if !strings.Contains(flagWrapped, "https://registry.example.com/very/long/path/that/exceeds/narrow/columns") {
			t.Errorf("long word in flag description was truncated:\n%s", flagWrapped)
		}
		if !strings.Contains(flagWrapped, "\n    --quiet                      (default: false)\n") {
			t.Errorf("empty-description flag should omit trailing spaces:\n%q", flagWrapped)
		}

		for _, line := range strings.Split(flagWrapped, "\n") {
			if strings.HasSuffix(line, " ") {
				t.Errorf("rendered line has trailing space: %q", line)
			}
		}

		// Legacy wrapFlag adapter contract via runtime template:
		legacyTemplate := `{{wrapFlag 17 26 "--config string" "(default: \"config.json\")" "Configuration file path to load before executing skill actions."}}`
		legacyWrapped := renderRuntimeUsageTemplateAtWidth(t, legacyTemplate, false, 68)
		wantLegacyWrapped := strings.Join([]string{
			"Configuration file",
			"                                                 path to load before",
			"                                                 executing skill",
			"                                                 actions.",
		}, "\n")
		if legacyWrapped != wantLegacyWrapped {
			t.Errorf("legacy wrapFlag adapter mismatch:\nwant:\n%q\ngot:\n%q", wantLegacyWrapped, legacyWrapped)
		}
	})

	t.Run("identical_shallow_and_recursive_entries_omit_recursive_conditional", func(t *testing.T) {
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
		populateParentsUsage(rootUsage, nil)

		var generated bytes.Buffer
		if err := tmpl.Execute(&generated, rootUsage); err != nil {
			t.Fatalf("stage 1 execute failed: %v", err)
		}
		if strings.Contains(generated.String(), ".Recursive") {
			t.Errorf("expected no .Recursive branch when shallow and recursive entries are identical:\n%s", generated.String())
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
			rendered := renderRuntimeUsageTemplateAtWidth(t, generated.String(), recursive, 100)
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
