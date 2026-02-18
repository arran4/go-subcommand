package commentv1

import (
	"reflect"
	"testing"
)

func TestParseParamDetails_Issue268(t *testing.T) {
	tests := []struct {
		text     string
		expected ParsedParam
	}{
		{
			text: "(from: parent) Description",
			expected: ParsedParam{
				FromParent:  true,
				Description: "Description",
			},
		},
		{
			text: "(default from environment MY_ENV fallback: \"bob\") Description",
			expected: ParsedParam{
				EnvVar:         "MY_ENV",
				EnvVarFallback: "bob",
				Description:    "Description",
			},
		},
		{
			text: "(default from environment MY_ENV) Description",
			expected: ParsedParam{
				EnvVar:      "MY_ENV",
				Description: "Description",
			},
		},
	}

	for _, test := range tests {
		got := parseParamDetails(test.text)
		if !reflect.DeepEqual(got, test.expected) {
			t.Errorf("parseParamDetails(%q) = %+v, want %+v", test.text, got, test.expected)
		}
	}
}
