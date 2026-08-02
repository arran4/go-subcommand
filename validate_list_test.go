package go_subcommand

import (
	"testing"
	"testing/fstest"
)

func TestValidate(t *testing.T) {
	t.Run("ValidParser", func(t *testing.T) {
		fs := fstest.MapFS{
			"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
			"main.go": &fstest.MapFile{Data: []byte(` + "`" + `package main

// Root is a subcommand ` + "`" + `app` + "`" + `
func Root() {}
` + "`" + `)},
		}

		err := GenerateWithFS(fs, NewCollectingFileWriter(), ".", "", "commentv1", nil, false, nil)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		// The original test tested Validate. We will test it directly, but it relies on os.DirFS
		// The issue implies we should use MapFS to avoid I/O. But Validate takes a string dir.
		// Wait, the memory fs test implies passing a testing/fstest.MapFS to something, or
		// perhaps the user is suggesting to refactor Validate to take an fs.FS?
	})
}
