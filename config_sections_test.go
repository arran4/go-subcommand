package go_subcommand

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestConfigSectionsRelativeReferences(t *testing.T) {
	content := "format-version: 1\n# Documentation\n```markdown: app file=docs/help.md\n```\n"
	help := "# Notes\n\n    keep indentation\nLiteral {{braces}} and `code`.\n"
	input := fstest.MapFS{"project/settings.mdc": {Data: []byte(content)}, "project/docs/help.md": {Data: []byte(help)}}
	config, err := LoadGenerateConfigFS(input, "project/settings.mdc")
	if err != nil {
		t.Fatal(err)
	}
	if len(config.CommandDeclarations) != 1 || config.CommandDeclarations[0].Text != help || config.CommandDeclarations[0].Path != "app" || config.CommandDeclarations[0].Filename != "project/settings.mdc" {
		t.Fatalf("config=%#v", config)
	}
	again, err := LoadGenerateConfigFS(input, "project/settings.mdc")
	if err != nil || !reflect.DeepEqual(config, again) {
		t.Fatal("configuration parsing is not deterministic")
	}
}

func TestConfigSectionsRejectMalformedInput(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		line              int
	}{
		{"unknown section", "# Unknown\n", "unsupported heading", 2},
		{"duplicate section", "# Commands\n```command: app\n```\n# Commands\n", "duplicate section", 5},
		{"unclosed", "# Commands\n```command: app\nDescription: App.\n", "unclosed", 3},
		{"settings after appendix", "# Commands\n```command: app\n```\nrecursive: false\n", "typed fenced block", 5},
		{"wrong block kind", "# Commands\n```markdown: app\n```\n", "unsupported markdown", 3},
		{"unsupported block", "# Commands\n```shell: app\n```\n", "typed fenced block", 3},
		{"inline and external", "# Documentation\n```markdown: app file=help.md\ninline\n```\n", "inline content", 3},
		{"path escape", "# Documentation\n```markdown: app file=../help.md\n```\n", "safe relative", 3},
		{"absolute path", "# Documentation\n```markdown: app file=/help.md\n```\n", "safe relative", 3},
		{"windows path", "# Documentation\n```markdown: app file=C:\\help.md\n```\n", "safe relative", 3},
		{"missing file", "# Documentation\n```markdown: app file=missing.md\n```\n", "read external reference", 3},
		{"wrong file type", "# Documentation\n```markdown: app file=help.txt\n```\n", "unsupported external", 3},
		{"command reference", "# Commands\n```command: app file=help.md\n```\n", "must be inline", 3},
		{"man section", "# Man content\n```man: app examples\ntext\n```\n", "uppercase SECTION", 3},
		{"reserved man section", "# Man content\n```man: app DESCRIPTION\ntext\n```\n", "built-in section", 3},
		{"empty section", "# Documentation\n", "empty section", 2},
		{"invalid UTF-8", "# Documentation\n```text: app\n\xff\n```\n", "UTF-8", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fstest.MapFS{"project.mdc": {Data: []byte("format-version: 1\n" + tc.input)}}
			_, err := LoadGenerateConfigFS(input, "project.mdc")
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "project.mdc") || !strings.Contains(err.Error(), fmt.Sprintf("line %d", tc.line)) {
				t.Fatalf("error=%v; want line %d and %q", err, tc.line, tc.want)
			}
		})
	}
}

func TestConfigSectionsInvalidExternalText(t *testing.T) {
	for _, data := range [][]byte{{0xff}, []byte("text\x00more")} {
		input := fstest.MapFS{
			"settings.mdconf": {Data: []byte("format-version: 1\n# Documentation\n```markdown: app file=docs/help.md\n```\n")},
			"docs/help.md":    {Data: data},
		}
		_, err := LoadGenerateConfigFS(input, "settings.mdconf")
		if err == nil || !strings.Contains(err.Error(), "UTF-8") || !strings.Contains(err.Error(), "docs/help.md") || !strings.Contains(err.Error(), "line 3") {
			t.Fatalf("external text error=%v", err)
		}
	}
}
