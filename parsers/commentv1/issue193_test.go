package commentv1

import (
	"testing"
	"testing/fstest"
)

func TestIssue193_RootCommandInSubPackage(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/test\n\ngo 1.21\n")},
		"cli/cmd.go": {Data: []byte(`package cli

// MyCommand is a subcommand ` + "`my-app`" + `
func MyCommand() error { return nil }
`)},
	}

	model, err := ParseGoFiles(fsys, ".")
	if err != nil {
		t.Fatalf("ParseGoFiles failed: %v", err)
	}

	if len(model.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(model.Commands))
	}

	cmd := model.Commands[0]
	if cmd.MainCmdName != "my-app" {
		t.Errorf("Expected MainCmdName 'my-app', got '%s'", cmd.MainCmdName)
	}

	expectedPackageName := "cli"
	if cmd.CommandPackageName != expectedPackageName {
		t.Errorf("Expected CommandPackageName '%s', got '%s'", expectedPackageName, cmd.CommandPackageName)
	}

	expectedImportPath := "example.com/test/cli"
	if cmd.ImportPath != expectedImportPath {
		t.Errorf("Expected ImportPath '%s', got '%s'", expectedImportPath, cmd.ImportPath)
	}
}
