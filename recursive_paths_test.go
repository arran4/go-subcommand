package go_subcommand

import (
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/parsers"
)

func TestGenerate_Recursive(t *testing.T) {
	fs := fstest.MapFS{
		"go.mod":      &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go":     &fstest.MapFile{Data: []byte(`package main
// Root is a subcommand ` + "`app`" + `
func Root() {}
`)},
		"sub/sub.go": &fstest.MapFile{Data: []byte(`package sub
// Sub is a subcommand ` + "`app sub`" + `
func Sub() {}
`)},
	}

	// Test recursive=true (default)
	writer := NewMockWriter()
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, ok := writer.Files["cmd/app/sub.go"]; !ok {
		t.Errorf("Expected sub.go to be generated with recursive=true")
	}

	// Test recursive=false
	writer = NewMockWriter()
	err = GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: false}, false)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, ok := writer.Files["cmd/app/sub.go"]; ok {
		t.Errorf("Expected sub.go NOT to be generated with recursive=false")
	}
}

func TestGenerate_Paths(t *testing.T) {
	fs := fstest.MapFS{
		"go.mod":      &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go":     &fstest.MapFile{Data: []byte(`package main
// Root is a subcommand ` + "`app`" + `
func Root() {}
`)},
		"pkg1/cmd.go": &fstest.MapFile{Data: []byte(`package pkg1
// Cmd1 is a subcommand ` + "`app cmd1`" + `
func Cmd1() {}
`)},
		"pkg2/cmd.go": &fstest.MapFile{Data: []byte(`package pkg2
// Cmd2 is a subcommand ` + "`app cmd2`" + `
func Cmd2() {}
`)},
	}

	// Test with specific path
	writer := NewMockWriter()
	err := GenerateWithFS(fs, writer, ".", "", "commentv1", &parsers.ParseOptions{
		SearchPaths: []string{"pkg1"},
		Recursive:   true,
	}, false)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, ok := writer.Files["cmd/app/cmd1.go"]; !ok {
		t.Errorf("Expected cmd1.go to be generated")
	}
	if _, ok := writer.Files["cmd/app/cmd2.go"]; ok {
		t.Errorf("Expected cmd2.go NOT to be generated")
	}
}
