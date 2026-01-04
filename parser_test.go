package go_subcommand

import (
	"reflect"
	"testing"
)

func TestParseSubCommandComments(t *testing.T) {
	tests := []struct {
		name                   string
		text                   string
		wantCmdName            string
		wantSubCommandSequence []string
		wantDescription        string
		wantExtendedHelp       string
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
			name:                   "Example 1.1",
			text:                   "ExampleCmd1 is a subcommand `basic1 example1`",
			wantCmdName:            "basic1",
			wantSubCommandSequence: []string{"example1"},
			wantDescription:        "",
			wantOk:                 true,
		},
		{
			name: "User Prototype 1",
			text: `PrintUser is a subcommand ` + "`my-app users get`" + ` that prints the users
Flags:
  username: --user-name -u -user-name (default: guest) User name
  file: (default: "out.png") Input file
PrintUser prints in x format
with x / y z`,
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
			name: "Param Style",
			text: `Cmd is a subcommand ` + "`app cmd`" + ` -- runs cmd
param force (-f, default: false) Force it`,
			wantCmdName:            "app",
			wantSubCommandSequence: []string{"cmd"},
			wantDescription:        "runs cmd",
			wantParams: map[string]ParsedParam{
				"force": {Flags: []string{"f"}, Default: "false", Description: "Force it"},
			},
			wantOk: true,
		},
		{
			name: "Optional Separators",
			text: `Cmd is a subcommand ` + "`app cmd`" + ` -- description
that can handle missing tokens`,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmdName, gotSubCommandSequence, gotDescription, gotExtendedHelp, gotParams, gotOk := ParseSubCommandComments(tt.text)
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
