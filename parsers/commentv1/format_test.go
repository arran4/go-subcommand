package commentv1

import (
	"strings"
	"testing"

	"github.com/arran4/go-subcommand/model"
)

func TestFormatSubCommand(t *testing.T) {
	parser := &CommentParser{}

	sc := &model.SubCommand{
		Command: &model.Command{
			MainCmdName: "app",
		},
		SubCommandName:         "mycmd",
		SubCommandFunctionName: "MyCmd",
		SubCommandDescription:  "does something",
		Aliases:                []string{"foo", "bar"},
		Parameters: []*model.FunctionParameter{
			{
				Name:         "flag",
				FlagAliases:  []string{"f", "flag"},
				Description:  "a flag",
				Type:         "string",
				Default:      "default",
				IsRequired:   true,
				DeclaredIn:   "mycmd",
				Generator:    model.GeneratorConfig{Type: model.SourceTypeFlag},
				Parser:       model.ParserConfig{Type: model.ParserTypeImplicit},
			},
		},
	}

	formatted, err := parser.Format(sc)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	expected := `// MyCmd is a subcommand ` + "`" + `app mycmd` + "`" + ` that does something
//
// aliases: bar, foo
//
// Flags:
//
//	flag: a flag (required) (default: "default") --flag, -f`

	if strings.TrimSpace(formatted) != strings.TrimSpace(expected) {
		t.Errorf("Format mismatch:\nGot:\n%s\nWant:\n%s", formatted, expected)
	}
}
