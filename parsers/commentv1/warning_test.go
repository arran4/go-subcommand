package commentv1

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWarnings(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": &fstest.MapFile{
			Data: []byte("module example.com/test"),
		},
		"main.go": &fstest.MapFile{
			Data: []byte(`package main

// Root is a subcommand ` + "`root`" + `
// Flags:
// 	verbose: -v Verbose output
// 	dryrun: -n
func Root(verbose, dryrun bool) {}

// Sub is a subcommand ` + "`root sub`" + `
func Sub() {}
`),
		},
	}

	var buf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(originalOutput)

	_, err := ParseGoFiles(fsys, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles failed: %v", err)
	}

	output := buf.String()

	expectedWarnings := []string{
		"Warning: In command 'root' (function Root), the following parameters are missing descriptions while others have them: dryrun",
		"Warning: Subcommand 'root sub' (function Sub) is missing a short description.",
	}

	for _, warn := range expectedWarnings {
		if !strings.Contains(output, warn) {
			t.Errorf("Expected warning not found: %q\nGot output:\n%s", warn, output)
		}
	}
}
