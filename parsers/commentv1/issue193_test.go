package commentv1

import (
	"testing"
	"testing/fstest"
)

func TestRootCommandImportPaths(t *testing.T) {
	tests := []struct {
		name                string
		files               fstest.MapFS
		expectedCmdName     string
		expectedPackageName string
		expectedImportPath  string
	}{
		{
			name: "Root Package (Regression)",
			files: fstest.MapFS{
				"go.mod": {Data: []byte("module example.com/test\n\ngo 1.21\n")},
				"main.go": {Data: []byte(`package main

// MyRoot is a subcommand ` + "`root-app`" + `
func MyRoot() error { return nil }
`)},
			},
			expectedCmdName:     "root-app",
			expectedPackageName: "main",
			expectedImportPath:  "example.com/test",
		},
		{
			name: "Subpackage (Issue #193)",
			files: fstest.MapFS{
				"go.mod": {Data: []byte("module example.com/test\n\ngo 1.21\n")},
				"cli/cmd.go": {Data: []byte(`package cli

// MyCommand is a subcommand ` + "`sub-app`" + `
func MyCommand() error { return nil }
`)},
			},
			expectedCmdName:     "sub-app",
			expectedPackageName: "cli",
			expectedImportPath:  "example.com/test/cli",
		},
		{
			name: "Deep Nested Subpackage",
			files: fstest.MapFS{
				"go.mod": {Data: []byte("module example.com/test\n\ngo 1.21\n")},
				"cmd/internal/core/root.go": {Data: []byte(`package core

// CoreCmd is a subcommand ` + "`core-app`" + `
func CoreCmd() error { return nil }
`)},
			},
			expectedCmdName:     "core-app",
			expectedPackageName: "core",
			expectedImportPath:  "example.com/test/cmd/internal/core",
		},
		{
			name: "Mismatched Package Name (e.g. main in subdir)",
			files: fstest.MapFS{
				"go.mod": {Data: []byte("module example.com/test\n\ngo 1.21\n")},
				"cmd/tool/main.go": {Data: []byte(`package main

// ToolCmd is a subcommand ` + "`tool-app`" + `
func ToolCmd() error { return nil }
`)},
			},
			expectedCmdName:     "tool-app",
			expectedPackageName: "main",
			expectedImportPath:  "example.com/test/cmd/tool",
		},
		{
			name: "Hyphenated Directory Name",
			files: fstest.MapFS{
				"go.mod": {Data: []byte("module example.com/test\n\ngo 1.21\n")},
				"my-lib/lib.go": {Data: []byte(`package mylib

// LibCmd is a subcommand ` + "`lib-app`" + `
func LibCmd() error { return nil }
`)},
			},
			expectedCmdName:     "lib-app",
			expectedPackageName: "mylib",
			expectedImportPath:  "example.com/test/my-lib",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model, err := ParseGoFiles(tc.files, ".")
			if err != nil {
				t.Fatalf("ParseGoFiles failed: %v", err)
			}

			if len(model.Commands) != 1 {
				t.Fatalf("Expected 1 command, got %d", len(model.Commands))
			}

			cmd := model.Commands[0]
			if cmd.MainCmdName != tc.expectedCmdName {
				t.Errorf("Expected MainCmdName '%s', got '%s'", tc.expectedCmdName, cmd.MainCmdName)
			}

			if cmd.CommandPackageName != tc.expectedPackageName {
				t.Errorf("Expected CommandPackageName '%s', got '%s'", tc.expectedPackageName, cmd.CommandPackageName)
			}

			if cmd.ImportPath != tc.expectedImportPath {
				t.Errorf("Expected ImportPath '%s', got '%s'", tc.expectedImportPath, cmd.ImportPath)
			}
		})
	}
}
