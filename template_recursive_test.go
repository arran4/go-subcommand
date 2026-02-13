package go_subcommand

import (
	"testing"
	"testing/fstest"
)

func TestRecursiveTemplateParsing(t *testing.T) {
	// Create a mock FS with templates in subdirectories
	mockFS := fstest.MapFS{
		"templates/root.gotmpl":          {Data: []byte("root template")},
		"templates/subdir/nested.gotmpl": {Data: []byte("nested template")},
		"templates/subdir/ignore.txt":    {Data: []byte("should be ignored")},
	}

	// Call the parsing function (exported for test or using a helper if private)
	// Since parseTemplates will be private in generate.go (part of go_subcommand package),
	// we can access it here if this test is in the same package.

	// Note: We haven't implemented parseTemplates yet, so this test will fail to compile
	// if we run it now. But we are preparing it.

	tmpl, err := ParseTemplates(mockFS)
	if err != nil {
		t.Fatalf("ParseTemplates failed: %v", err)
	}

	// Verify root template is present
	if t1 := tmpl.Lookup("root.gotmpl"); t1 == nil {
		t.Error("root.gotmpl not found")
	}

	// Verify nested template is present
	// Note: template name is basename by default when using ParseFS/ParseGlob
	if t2 := tmpl.Lookup("nested.gotmpl"); t2 == nil {
		t.Error("nested.gotmpl not found")
	}

	// Verify we can execute them (optional, just checking existence is enough for parser)
}
