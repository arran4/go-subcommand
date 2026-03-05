package commentv1

import (
	"embed"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

//go:embed testdata/basic/*
var basicTestData embed.FS

func TestParseGoFile(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		wantCmdName     string
		wantDescription string
		wantSubCommands []string
		wantMissing     bool
	}{
		{
			name:            "Implicit Command Name",
			filename:        "go_implicit_cmd.go",
			wantCmdName:     "parent",
			wantDescription: "Does work in a directory",
		},
		{
			name:            "Explicit Command Name",
			filename:        "go_explicit_cmd.go",
			wantCmdName:     "my-parent",
			wantDescription: "Does work explicitly",
		},
		{
			name:            "Implicit Subcommand of Implicit Parent",
			filename:        "go_implicit_subcmd.go",
			wantCmdName:     "parent",
			wantDescription: "Does work in a directory",
			wantSubCommands: []string{"child"},
		},
		{
			name:            "Implicit Command Name with Acronym",
			filename:        "go_implicit_acronym.go",
			wantCmdName:     "http-client",
			wantDescription: "does http things",
		},
		{
			name:        "Not a subcommand",
			filename:    "go_not_cmd.go",
			wantMissing: true,
		},
		{
			name:        "Subcommand with receiver (ignored)",
			filename:    "go_ignored_receiver.go",
			wantMissing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := basicTestData.ReadFile("testdata/basic/" + tt.filename)
			if err != nil {
				t.Fatalf("failed to read test file %s: %v", tt.filename, err)
			}

			fset := token.NewFileSet()
			cmdTree := &CommandsTree{
				Commands:    make(map[string]*CommandTree),
				PackagePath: "example.com/test",
			}

			err = ParseGoFile(fset, "test.go", "example.com/test", strings.NewReader(string(src)), cmdTree)
			if err != nil {
				t.Fatalf("ParseGoFile failed: %v", err)
			}

			if tt.wantMissing {
				if len(cmdTree.Commands) > 0 {
					t.Errorf("Expected no commands, but got keys: %v", getKeys(cmdTree.Commands))
				}
				return
			}

			if _, ok := cmdTree.Commands[tt.wantCmdName]; !ok {
				t.Errorf("Expected command '%s' to be created, but got keys: %v", tt.wantCmdName, getKeys(cmdTree.Commands))
			} else {
				ct := cmdTree.Commands[tt.wantCmdName]
				if ct.Description != tt.wantDescription {
					t.Errorf("Expected description '%s', got '%s'", tt.wantDescription, ct.Description)
				}

				if len(tt.wantSubCommands) > 0 {
					for _, subName := range tt.wantSubCommands {
						if _, ok := ct.SubCommands[subName]; !ok {
							t.Errorf("Expected subcommand '%s' in '%s', but not found", subName, tt.wantCmdName)
						}
					}
				}
			}
		})
	}
}

func getKeys(m map[string]*CommandTree) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestParseSubCommandCommentsBasic(t *testing.T) {
	tests := []struct {
		name                   string
		text                   string
		filename               string
		wantCmdName            string
		wantSubCommandSequence []string
		wantDescription        string
		wantExtendedHelp       string
		wantAliases            []string
		wantParams             map[string]ParsedParam
		wantOk                 bool
	}{
		{
			name:                   "Example 1",
			text:                   "ExampleCmd1 is a subcommand `basic1 example1`\nDoes nothing practical",
			wantCmdName:            "basic1",
			wantSubCommandSequence: []string{"example1"},
			wantDescription:        "",
			wantExtendedHelp:       "Does nothing practical",
			wantOk:                 true,
		},
		{
			name:                   "With Aliases",
			text:                   "Cmd is a subcommand `app cmd` -- Description\nAliases: c, command",
			wantCmdName:            "app",
			wantSubCommandSequence: []string{"cmd"},
			wantDescription:        "Description",
			wantAliases:            []string{"c", "command"},
			wantOk:                 true,
		},
		{
			name:                   "With Inline Aliases",
			text:                   "Cmd is a subcommand `app cmd` -- Description (aka: c, command)",
			wantCmdName:            "app",
			wantSubCommandSequence: []string{"cmd"},
			wantDescription:        "Description",
			wantAliases:            []string{"c", "command"},
			wantOk:                 true,
		},
		{
			name:                   "With Lowercase Aliases Header",
			text:                   "Cmd is a subcommand `app cmd` -- Description\naliases: c, command",
			wantCmdName:            "app",
			wantSubCommandSequence: []string{"cmd"},
			wantDescription:        "Description",
			wantAliases:            []string{"c", "command"},
			wantOk:                 true,
		},
		{
			name:                   "Example 1.1",
			text:                   "ExampleCmd1 is a subcommand `basic1 example1`",
			wantCmdName:            "basic1",
			wantSubCommandSequence: []string{"example1"},
			wantDescription:        "",
			wantOk:                 true,
		},
		{
			name:                   "User Prototype 1",
			filename:               "comment_user_proto.txt",
			wantCmdName:            "my-app",
			wantSubCommandSequence: []string{"users", "get"},
			wantDescription:        "prints the users",
			wantExtendedHelp:       "PrintUser prints in x format\nwith x / y z",
			wantParams: map[string]ParsedParam{
				"username": {Flags: []string{"user-name", "u"}, Default: "guest", Description: "User name"},
				"file":     {Flags: nil, Default: "out.png", Description: "Input file"},
			},
			wantOk: true,
		},
		{
			name:                   "Param Style",
			filename:               "comment_param_style.txt",
			wantCmdName:            "app",
			wantSubCommandSequence: []string{"cmd"},
			wantDescription:        "runs cmd",
			wantParams: map[string]ParsedParam{
				"force": {Flags: []string{"f"}, Default: "false", Description: "Force it"},
			},
			wantOk: true,
		},
		{
			name:                   "Optional Separators",
			filename:               "comment_optional_separators.txt",
			wantCmdName:            "app",
			wantSubCommandSequence: []string{"cmd"},
			wantDescription:        "description",
			wantExtendedHelp:       "that can handle missing tokens",
			wantOk:                 true,
		},
		{
			name:                   "No Separator",
			text:                   `Cmd is a subcommand ` + "`app cmd`" + ` description`,
			wantCmdName:            "app",
			wantSubCommandSequence: []string{"cmd"},
			wantDescription:        "description",
			wantOk:                 true,
		},
		{
			name:            "Implicit Command Name",
			text:            "Parent is a subcommand that Does work in a directory",
			wantCmdName:     "",
			wantDescription: "Does work in a directory",
			wantOk:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := tt.text
			if tt.filename != "" {
				src, err := basicTestData.ReadFile("testdata/basic/" + tt.filename)
				if err != nil {
					t.Fatalf("failed to read test file %s: %v", tt.filename, err)
				}
				text = string(src)
			}

			gotCmdName, gotSubCommandSequence, gotDescription, gotExtendedHelp, gotAliases, gotParams, gotOk := ParseSubCommandComments(text)
			if gotCmdName != tt.wantCmdName {
				t.Errorf("gotCmdName = %v, want %v", gotCmdName, tt.wantCmdName)
			}
			if !reflect.DeepEqual(gotSubCommandSequence, tt.wantSubCommandSequence) {
				t.Errorf("gotSubCommandSequence = %v, want %v", gotSubCommandSequence, tt.wantSubCommandSequence)
			}
			if gotDescription != tt.wantDescription {
				t.Errorf("gotDescription = %q, want %q", gotDescription, tt.wantDescription)
			}
			if gotExtendedHelp != tt.wantExtendedHelp {
				t.Errorf("gotExtendedHelp = %q, want %q", gotExtendedHelp, tt.wantExtendedHelp)
			}
			if !reflect.DeepEqual(gotAliases, tt.wantAliases) {
				if len(gotAliases) != 0 || len(tt.wantAliases) != 0 {
					t.Errorf("gotAliases = %v, want %v", gotAliases, tt.wantAliases)
				}
			}
			// Check params map
			if len(gotParams) != len(tt.wantParams) {
				t.Errorf("gotParams len = %d, want %d", len(gotParams), len(tt.wantParams))
			}
			for k, v := range tt.wantParams {
				gotV, ok := gotParams[k]
				if !ok {
					t.Errorf("gotParams missing key %s", k)
					continue
				}
				if !reflect.DeepEqual(gotV.Flags, v.Flags) {
					// Handle nil vs empty slice
					if len(gotV.Flags) != 0 || len(v.Flags) != 0 {
						t.Errorf("param %s flags = %v, want %v", k, gotV.Flags, v.Flags)
					}
				}
				if gotV.Default != v.Default {
					t.Errorf("param %s default = %q, want %q", k, gotV.Default, v.Default)
				}
				if gotV.Description != v.Description {
					t.Errorf("param %s description = %q, want %q", k, gotV.Description, v.Description)
				}
			}

			if gotOk != tt.wantOk {
				t.Errorf("gotOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}
