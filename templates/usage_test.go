package templates

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/arran4/go-subcommand"
)

func TestUsageTemplate(t *testing.T) {
	tmplBytes, err := os.ReadFile("usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("failed to read template file: %v", err)
	}
	tmplStr := string(tmplBytes)

	funcs := template.FuncMap{
		"lower":   strings.ToLower,
		"title":   strings.Title,
		"upper":   strings.ToUpper,
		"replace": strings.ReplaceAll,
		"add":     func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("usage").Funcs(funcs).Parse(tmplStr)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	cmd := &go_subcommand.Command{
		MainCmdName: "myapp",
	}
	subCmd := &go_subcommand.SubCommand{
		Command:        cmd,
		SubCommandName: "foo",
		Parameters: []*go_subcommand.FunctionParameter{
			{Name: "verbose", Type: "bool", Description: "Enable verbose", Default: "false", FlagAliases: []string{"v"}},
			{Name: "count", Type: "int", Description: "Count items", Default: "0", FlagAliases: []string{"c"}},
		},
		SubCommands: []*go_subcommand.SubCommand{
			{SubCommandName: "bar", SubCommandDescription: "Bar command"},
			{SubCommandName: "baz", SubCommandDescription: "Baz command"},
		},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, subCmd); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	output := buf.String()

	// Check Subcommands section
	// Expecting indentation 4 spaces, no empty lines between items.
	// Empty line before version is acceptable/expected.
	expectedSubcommands := `Subcommands:
    bar        Bar command
    baz        Baz command`

	if !strings.Contains(output, expectedSubcommands) {
		t.Errorf("Expected subcommands section:\n%q\nGot:\n%q", expectedSubcommands, output)
	}

	expectedFlags := `Flags:
    -v       Enable verbose (default: false)
    -c int   Count items (default: 0)`

	if !strings.Contains(output, expectedFlags) {
		t.Errorf("Expected flags section:\n%q\nGot:\n%q", expectedFlags, output)
	}
}
