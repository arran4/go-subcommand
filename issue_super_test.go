package go_subcommand

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestMergedPR350PR388GeneratedRequirements(t *testing.T) {
	src := `package main

// App is a subcommand ` + "`app`" + `.
//
// Flags:
//
//	config: (required) --config -c Config path
func App(config string) {}

// Parent is a subcommand ` + "`app parent`" + ` that Does work in a directory
//
// Flags:
//
//	dir: --dir (default: ".") The directory
func Parent(dir string) {}

// Child is a subcommand ` + "`app parent child`" + ` that Does work in a directory
//
// Flags:
//
//	dir: --dir (from parent)
//	i: --i (default: 0) A random int
//	values: -v Repeatable values
//	ptr: --ptr Nullable pointer
//	parsed: (parser: "example.com/test/pkg".ParseThing) Parsed value
//	generated: (generator: "example.com/test/pkg".GenThing) Generated value
func Child(dir string, i int, values []string, ptr *int, parsed string, generated string) {}
`

	input := fstest.MapFS{
		"go.mod":     &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go":    &fstest.MapFile{Data: []byte(src)},
		"pkg/pkg.go": &fstest.MapFile{Data: []byte("package pkg\n\nfunc ParseThing(s string) (string, error) { return s, nil }\nfunc GenThing() (string, error) { return \"generated\", nil }\n")},
	}
	writer := runGenerateInMemory(t, input)

	rootUsage := string(mustGeneratedFile(t, writer, "cmd/app/templates/app_usage.txt"))
	if !strings.Contains(rootUsage, "Usage: app [flags...]") {
		t.Fatalf("root usage template was not generated from usage.txt.gotmpl:\n%s", rootUsage)
	}

	rootGo := string(mustGeneratedFile(t, writer, "cmd/app/root.go"))
	assertContains(t, rootGo, `executeUsage(os.Stderr, "app_usage.txt"`, "root usage should use embedded usage template")
	assertContains(t, rootGo, `seenFlags := make(map[string]bool)`, "required root flag should create seenFlags")
	assertContains(t, rootGo, `required flag --config not provided`, "required root flag should be validated")
	if action := strings.Index(rootGo, "if c.CommandAction != nil"); action == -1 {
		t.Fatal("root command action missing")
	} else if dispatch := strings.Index(rootGo, "if len(remainingArgs) > 0"); dispatch == -1 {
		t.Fatal("root command dispatch missing")
	} else if action > dispatch {
		t.Fatalf("root command action should run before subcommand dispatch\nroot.go:\n%s", rootGo)
	}

	childGo := string(mustGeneratedFile(t, writer, "cmd/app/parent_child.go"))
	assertContains(t, childGo, `"example.com/test/pkg"`, "external parser/generator package should be imported")
	assertContains(t, childGo, `pkg.ParseThing(value)`, "custom parser should be called")
	assertContains(t, childGo, `pkg.GenThing()`, "generator should be called")
	assertNotContains(t, childGo, `case "generated":`, "generator-backed parameter should not be exposed as a flag")
	assertContains(t, childGo, `c.values = append(c.values, value)`, "slice flags should be repeatable")
	assertContains(t, childGo, `c.ptr = &val`, "pointer flag should preserve omitted-vs-zero state")
	assertContains(t, childGo, `value = shorts[j+1:]`, "short value flags should consume the rest of a GNU-style short cluster")

	childUsage := string(mustGeneratedFile(t, writer, "cmd/app/templates/child_usage.txt"))
	assertContains(t, childUsage, "`parent` Flags:", "from-parent flag should be grouped under parent")
	assertContains(t, childUsage, "`child` Flags:", "child-local flags should be grouped under child")
	assertNotContains(t, childUsage, "generated", "generator-backed parameter should not appear in usage flags")
}

func mustGeneratedFile(t *testing.T, writer *CollectingFileWriter, path string) []byte {
	t.Helper()
	content, ok := writer.Files[path]
	if !ok {
		t.Fatalf("generated file %q missing; generated files: %v", path, writer.Files)
	}
	return content
}

func assertContains(t *testing.T, s, substr, msg string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("%s: missing %q in:\n%s", msg, substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr, msg string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("%s: found unexpected %q in:\n%s", msg, substr, s)
	}
}
