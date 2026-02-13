package stringdiff

import (
	"fmt"
	"os"
)

//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate || go run github.com/arran4/go-subcommand/cmd/gosubc generate"

// RunDiff is a subcommand `stringdiff diff`
// Compares two strings or files side by side.
//
// Usage: stringdiff diff [flags] <file1> <file2>
//
// Flags:
//   maxLines: -m --max-lines (default: 1000) Max lines to lookahead for alignment
//   term: -t --term (default: false) Enable terminal colors
//   interactive: -i --interactive (default: false) Enable interactive mode (todo)
//   file1: @1 Source file or string
//   file2: @2 Target file or string
func RunDiff(file1, file2 string, maxLines int, term bool, interactive bool) error {
	c1, err := readFileOrString(file1)
	if err != nil {
		return fmt.Errorf("failed to read input 1: %w", err)
	}
	c2, err := readFileOrString(file2)
	if err != nil {
		return fmt.Errorf("failed to read input 2: %w", err)
	}

	out := Diff(c1, c2, MaxLines(maxLines), Term(term), Interactive(interactive))
	fmt.Println(out)
	return nil
}

func readFileOrString(s string) (string, error) {
	// Try to read file
	if _, err := os.Stat(s); err == nil {
		b, err := os.ReadFile(s)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// Fallback to treating s as content
	return s, nil
}
