package go_subcommand

import (
	"testing"
)

func TestParseSubCommandCommentsGofmt(t *testing.T) {
	// Note: The blank line after Flags: is what gofmt adds.
	// Also note the indentation of the parameter line.
	text := `MyFunc is a subcommand ` + "`app cmd`" + `
Flags:

	username: --username -u (default: "guest") The user to greet`

	_, _, _, _, gotParams, _ := ParseSubCommandComments(text)

	if _, ok := gotParams["username"]; !ok {
		t.Errorf("Failed to parse username parameter with blank line after Flags:")
	}
}
