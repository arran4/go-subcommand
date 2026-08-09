package go_subcommand

import (
	"testing"
)

func TestSanitizePathPart(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal", "foo", "foo"},
		{"path traversal", "../evil", "evil"},
		{"deep traversal", "../../etc/passwd", "passwd"},
		{"absolute", "/etc/passwd", "passwd"},
		{"root", "/", ""},
		{"empty", "", ""},
		{"dot", ".", ""},
		{"dotdot", "..", ""},
		{"windows absolute", "C:\\Windows\\System32", "System32"},
		{"complex", "foo/bar/baz", "baz"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := sanitizePathPart(tc.input)
			if actual != tc.expected {
				t.Errorf("sanitizePathPart(%q) = %q, expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestSanitizeManFileName(t *testing.T) {
	tests := []struct {
		name       string
		mainCmd    string
		subCmdSeq  string
		expected   string
	}{
		{"normal", "mycmd", "sub1 sub2", "mycmd-sub1-sub2.1"},
		{"traversal main", "../evil", "sub1", "evil-sub1.1"},
		{"traversal sub", "mycmd", "../evil sub2", "mycmd-evil-sub2.1"},
		{"all traversal", "../foo", "../bar ../baz", "foo-bar-baz.1"},
		{"empty sub", "mycmd", "", "mycmd.1"},
		{"slashes", "/bin/mycmd", "/sub1 /sub2", "mycmd-sub1-sub2.1"},
		{"only root", "/", "/", "unnamed.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := sanitizeManFileName(tc.mainCmd, tc.subCmdSeq)
			if actual != tc.expected {
				t.Errorf("sanitizeManFileName(%q, %q) = %q, expected %q", tc.mainCmd, tc.subCmdSeq, actual, tc.expected)
			}
		})
	}
}
