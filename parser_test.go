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
		wantDescription        string
		wantSubCommandSequence []string
		wantOk                 bool
	}{
		{
			name:                   "Example 1",
			text:                   "ExampleCmd1 is a subcommand `basic1 example1`\nDoes nothing practical",
			wantCmdName:            "basic1",
			wantSubCommandSequence: []string{"example1"},
			wantDescription:        "Does nothing practical",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmdName, gotSubCommandSequence, gotDescription, gotOk := ParseSubCommandComments(tt.text)
			if gotCmdName != tt.wantCmdName {
				t.Errorf("ParseSubCommandComments() gotCmdName = %v, want %v", gotCmdName, tt.wantCmdName)
			}
			if gotDescription != tt.wantDescription {
				t.Errorf("ParseSubCommandComments() gotDescription = %v, want %v", gotDescription, tt.wantDescription)
			}
			if !reflect.DeepEqual(gotSubCommandSequence, tt.wantSubCommandSequence) {
				t.Errorf("ParseSubCommandComments() gotSubCommandSequence = %v, want %v", gotSubCommandSequence, tt.wantSubCommandSequence)
			}
			if gotOk != tt.wantOk {
				t.Errorf("ParseSubCommandComments() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}
