package stringdiff

import (
	"strings"
	"testing"
)

func TestDiff(t *testing.T) {
	left := `Hello
World
Foo
Bar`
	right := `Hello
World
Baz
Bar`

	out := Diff(left, right)
	t.Logf("Output:\n%s", out)

	if !strings.Contains(out, "Hello") {
		t.Errorf("Expected 'Hello' in output")
	}
	if !strings.Contains(out, "==") {
		t.Errorf("Expected '==' in output")
	}
}

func TestDiffAlignment(t *testing.T) {
	left := `A
B
C
D`
	right := `A
C
D`

	out := Diff(left, right)
	t.Logf("Output:\n%s", out)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}
}

func TestDiffColor(t *testing.T) {
	left := "A"
	right := "B"
	out := Diff(left, right, Term(true))
	if !strings.Contains(out, "\033[31mA") {
		t.Errorf("Expected red color code for left")
	}
}
