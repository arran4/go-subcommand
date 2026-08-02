package go_subcommand

import (
	"bytes"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/tools/txtar"
)

// ArchiveToMapFS converts a txtar archive into a fstest.MapFS
func ArchiveToMapFS(ar *txtar.Archive) fstest.MapFS {
	out := fstest.MapFS{}
	for _, f := range ar.Files {
		name := path.Clean(strings.TrimPrefix(f.Name, "/"))
		if name == "." {
			continue
		}
		out[name] = &fstest.MapFile{Data: append([]byte(nil), f.Data...)}
	}
	return out
}

func TestList(t *testing.T) {
	fixture := `
-- go.mod --
module example.com/test

go 1.22
-- main.go --
package main
// Root is a subcommand ` + "`app`" + `
func Root() {}

// Sub is a subcommand ` + "`app sub`" + `
func Sub() {}
`
	ar := txtar.Parse([]byte(fixture))
	fsys := ArchiveToMapFS(ar)

	var buf bytes.Buffer
	err := List(".", "commentv1", []string{}, true, fsys, &buf)

	if err != nil {
		t.Fatalf("List returned unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Command: app") {
		t.Errorf("Expected output to contain 'Command: app', got: %s", output)
	}
	if !strings.Contains(output, "Subcommand: sub") {
		t.Errorf("Expected output to contain 'Subcommand: sub', got: %s", output)
	}
}

func TestValidate(t *testing.T) {
	fixture := `
-- go.mod --
module example.com/test

go 1.22
-- main.go --
package main
// Root is a subcommand ` + "`app`" + `
func Root() {}
`
	ar := txtar.Parse([]byte(fixture))
	fsys := ArchiveToMapFS(ar)

	var buf bytes.Buffer
	err := Validate(".", "commentv1", []string{}, true, fsys, &buf)

	if err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Validation successful.") {
		t.Errorf("Expected output to contain 'Validation successful.', got: %s", output)
	}
}

func TestList_Error(t *testing.T) {
	var buf bytes.Buffer
	err := List(".", "invalidparser", []string{}, true, &buf)
	if err == nil {
		t.Error("Expected error for invalid parser, got nil")
	}
}

func TestValidate_Error(t *testing.T) {
	var buf bytes.Buffer
	err := Validate(".", "invalidparser", []string{}, true, &buf)
	if err == nil {
		t.Error("Expected error for invalid parser, got nil")
	}
}
