package go_subcommand

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGenerateConfigStrictAndApplyPrecedence(t *testing.T) {
	path := "gosubc.json"
	content := `{
  "version": 1,
  "dir": "configured",
  "cliParser": "go-flag",
  "recursive": false,
  "paths": ["commands"],
  "emitGoGenerate": false
}`
	config, err := loadGenerateConfigJSON(path, []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	values := generateCLIValues{dir: ".", cliParser: "gnu", recursive: true, emitGoGenerate: true}
	values.applyConfig(config, map[string]bool{"cli-parser": true})
	if values.dir != "configured" || values.cliParser != "gnu" || values.recursive || values.emitGoGenerate || len(values.paths) != 1 || values.paths[0] != "commands" {
		t.Fatalf("merged values = %#v", values)
	}
}

func TestLoadGenerateConfigMarkdown(t *testing.T) {
	// Test .mdconf extension
	mdconfPath := "gosubc.mdconf"
	mdconfContent := `format-version: 1
dir: configured_md
cli-parser: go-flag
recursive: false
clean: true
paths: cmd, pkg

`
	config, err := loadGenerateConfigMarkdown(mdconfPath, []byte(mdconfContent))
	if err != nil {
		t.Fatalf("LoadGenerateConfig(.mdconf) failed: %v", err)
	}
	if config.Dir == nil || *config.Dir != "configured_md" {
		t.Errorf("expected dir 'configured_md', got %v", config.Dir)
	}
	if config.CLIParser == nil || *config.CLIParser != "go-flag" {
		t.Errorf("expected cliParser 'go-flag', got %v", config.CLIParser)
	}
	if config.Recursive == nil || *config.Recursive != false {
		t.Errorf("expected recursive false, got %v", config.Recursive)
	}
	if config.Clean == nil || *config.Clean != true {
		t.Errorf("expected clean true, got %v", config.Clean)
	}
	if config.Paths == nil || len(*config.Paths) != 2 || (*config.Paths)[0] != "cmd" || (*config.Paths)[1] != "pkg" {
		t.Errorf("expected paths [cmd, pkg], got %v", config.Paths)
	}

	// Test .mdc extension with list items
	mdcPath := "gosubc.mdc"
	mdcContent := `version: 1
dir: mdc_dir
paths:
- alpha
- beta
force: true
`
	config2, err := loadGenerateConfigMarkdown(mdcPath, []byte(mdcContent))
	if err != nil {
		t.Fatalf("LoadGenerateConfig(.mdc) failed: %v", err)
	}
	if config2.Dir == nil || *config2.Dir != "mdc_dir" {
		t.Errorf("expected dir 'mdc_dir', got %v", config2.Dir)
	}
	if config2.Force == nil || *config2.Force != true {
		t.Errorf("expected force true, got %v", config2.Force)
	}
	if config2.Paths == nil || len(*config2.Paths) != 2 || (*config2.Paths)[0] != "alpha" || (*config2.Paths)[1] != "beta" {
		t.Errorf("expected paths [alpha, beta], got %v", config2.Paths)
	}
}

func TestLoadGenerateConfigRejectsUnknownAndUnsupported(t *testing.T) {
	for name, content := range map[string]string{
		"unknown.json":   `{"version":1,"typo":true}`,
		"version.json":   `{"version":2}`,
		"unknown.mdconf": "version: 1\ntypo: true\n",
		"version.mdconf": "version: 2\ndir: .\n",
	} {
		var err error
		if strings.HasSuffix(name, ".json") {
			_, err = loadGenerateConfigJSON(name, []byte(content))
		} else {
			_, err = loadGenerateConfigMarkdown(name, []byte(content))
		}
		if err == nil {
			t.Fatalf("LoadGenerateConfig(%s) unexpectedly succeeded", name)
		}
	}
}

func TestExplicitGenerateFlags(t *testing.T) {
	// After "generate" subcommand
	explicit := ExplicitGenerateFlags([]string{"generate", "--dir=explicit", "--clean", "--path", "commands"})
	for _, name := range []string{"dir", "clean", "path"} {
		if !explicit[name] {
			t.Errorf("%q not recorded with generate prefix", name)
		}
	}

	// Directly passed args without "generate" prefix
	direct := ExplicitGenerateFlags([]string{"--clean", "--dir=foo"})
	for _, name := range []string{"clean", "dir"} {
		if !direct[name] {
			t.Errorf("%q not recorded in direct args", name)
		}
	}

	// Test runner flags (no leading --)
	if explicit := ExplicitGenerateFlags([]string{"-test.v", "-run", "TestSomething"}); len(explicit) != 0 {
		t.Errorf("expected empty map for test runner arguments, got %#v", explicit)
	}
}

func TestGenerateConfigStrictDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		line              int
	}{
		{"boolean", "format-version: 1\nrecursive: banana\n", "invalid boolean", 2},
		{"empty boolean", "format-version: 1\nclean:\n", "invalid boolean", 2},
		{"duplicate", "format-version: 1\nclean: true\nclean: false\n", "duplicate/conflicting", 3},
		{"alias conflict", "format-version: 1\nproject: true\nproject-provenance: false\n", "duplicate/conflicting", 3},
		{"version alias conflict", "format-version: 1\nversion: 1\n", "duplicate/conflicting", 2},
		{"unknown", "format-version: 1\ntypo: true\n", "unknown key", 2},
		{"unknown heading", "format-version: 1\n# Bananas\n", "unsupported heading", 2},
		{"redundant heading", "# Configuration\nformat-version: 1\n", "unsupported heading", 1},
		{"variables deferred", "format-version: 1\n# Variables\n", "unsupported heading", 2},
		{"empty commands", "format-version: 1\n# Commands\n", "empty section", 2},
		{"minimum deferred", "format-version: 1\nmin-gosubc: 1\n", "unknown key", 2},
		{"missing version", "clean: false\n", "missing required", 1},
		{"invalid version", "format-version: banana\n", "invalid format version", 1},
		{"unsupported version", "format-version: 2\n", "unsupported format version", 1},
		{"orphan list", "format-version: 1\n- item\n", "list entry", 2},
		{"empty list item", "format-version: 1\npaths:\n-\n", "list entry", 3},
		{"empty quoted item", "format-version: 1\npaths:\n- \"\"\n", "invalid list entry", 3},
		{"mixed list", "format-version: 1\npaths: a\n- b\n", "list entry", 3},
		{"malformed list", "format-version: 1\npaths: a,,b\n", "invalid list entry", 2},
		{"unclosed quote", "format-version: 1\ndir: \"oops\n", "invalid syntax", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadGenerateConfigMarkdown("project.mdc", []byte(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "project.mdc") || !strings.Contains(err.Error(), fmt.Sprintf("line %d", tc.line)) {
				t.Fatalf("error = %v; want file, line %d and %q", err, tc.line, tc.want)
			}
		})
	}
	for _, input := range []string{
		`{"version":1,"version":1}`,
		`{"version":1,"clean":true,"Clean":false}`,
		`{"version":1,"clean":null}`,
		`{"version":1,"variables":{"name":"unused"}}`,
		`{"version":1,"clean":"banana"}`,
		`{"version":1} {"version":1}`,
		`[]`,
	} {
		if _, err := loadGenerateConfigJSON("project.json", []byte(input)); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}

func TestGenerateConfigExplicitEmptyAndAbsent(t *testing.T) {
	config, err := loadGenerateConfigMarkdown("project.mdconf", []byte("format-version: 1\nrecursive: true\nprov-version: release\npaths: commands\n"))
	if err != nil {
		t.Fatal(err)
	}
	values := generateCLIValues{}
	values.applyConfig(config, map[string]bool{"recursive": true, "provVersion": true, "paths": true})
	if values.recursive || values.provVersion != "" || len(values.paths) != 0 {
		t.Fatalf("explicit zero values overwritten: %#v", values)
	}
	values.applyConfig(config, nil)
	if !values.recursive || values.provVersion != "release" || len(values.paths) != 1 {
		t.Fatalf("absent values did not use config: %#v", values)
	}
}

func TestGenerateCLIConfiguredCommand_IgnoresAmbientOsArgs(t *testing.T) {
	// Programmatic callers must NOT be affected by ambient process arguments,
	// even when os.Args starts with host binary and contains generate / conflicting flags.
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"gosubc", "generate", "--recursive=false", "--dir=/nonexistent/unrelated/path", "--clean"}

	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/ambient\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "app.go"), []byte("package app\n\n// Root is a subcommand `app`\nfunc Root() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(project, "gosubc.json")
	config := `{"version":1,"dir":"` + strings.ReplaceAll(project, `\`, `\\`) + `","clean":false}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	// Call GenerateCLIConfiguredCommand directly as an exported library function.
	err := GenerateCLIConfiguredCommand(
		"/nonexistent/fallback", "", "commentv1", "gnu", nil, true, false, false, nil, true, true, true, "", "", "", configPath,
	)
	if err != nil {
		t.Fatalf("GenerateCLIConfiguredCommand failed: %v", err)
	}

	// Also verify with GenerateCLIConfiguredWithOptions.
	err = GenerateCLIConfiguredWithOptions(GenerateCLIOptions{
		Dir:        "/nonexistent/fallback",
		ParserName: "commentv1",
		CLIParser:  "gnu",
		Recursive:  true,
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("GenerateCLIConfiguredWithOptions failed: %v", err)
	}
}

func TestGenerateCLIConfiguredCommand_IgnoresAmbientOsArgsStartingWithGenerate(t *testing.T) {
	// Verify that even if os.Args[0] == "generate", exported library calls do not inspect os.Args.
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"generate", "--clean", "--recursive=false", "--dir=/wrong/path"}

	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/ambient2\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "app.go"), []byte("package app\n\n// Root is a subcommand `app`\nfunc Root() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(project, "gosubc.mdconf")
	config := "version: 1\ndir: " + project + "\nclean: false\nrecursive: true\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	err := GenerateCLIConfiguredCommand(
		project, "", "commentv1", "gnu", nil, true, false, false, nil, true, true, true, "", "", "", configPath,
	)
	if err != nil {
		t.Fatalf("GenerateCLIConfiguredCommand failed: %v", err)
	}
}

func TestGenerateCLIConfiguredCommandUsesFileThenExplicitFlags(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/configured\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "app.go"), []byte("package app\n\n// Root is a subcommand `app`\nfunc Root() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(project, "gosubc.json")
	config := `{"version":1,"dir":"` + strings.ReplaceAll(project, `\`, `\\`) + `","cliParser":"go-flag","emitGoGenerate":false}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command("go", "run", "./cmd/gosubc", "generate", "--config", configPath)
	command.Dir = "."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("configured generation failed: %v\n%s", err, output)
	}
	main, err := os.ReadFile(filepath.Join(project, "cmd", "app", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(main), "//go:generate") {
		t.Fatalf("configuration did not suppress nested generation:\n%s", main)
	}
	assertParser := func(want string) {
		t.Helper()
		binary := filepath.Join(project, "app")
		build := exec.Command("go", "build", "-o", binary, "./cmd/app")
		build.Dir = project
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build configured application: %v\n%s", err, output)
		}
		run := exec.Command(binary, "--unknown")
		run.Dir = project
		output, err := run.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 1 || !strings.Contains(string(output), want) {
			t.Fatalf("configured parser: err=%v output=%q; want status 1 and %q", err, output, want)
		}
	}
	assertParser("flag provided but not defined: -unknown")

	command = exec.Command("go", "run", "./cmd/gosubc", "generate", "--config", configPath, "--cli-parser", "gnu", "--emit-go-generate=true", "--timestamp=false", "--project=false", "--prov-version=")
	command.Dir = "."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("explicit configured generation failed: %v\n%s", err, output)
	}
	main, err = os.ReadFile(filepath.Join(project, "cmd", "app", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(main), "//go:generate") {
		t.Fatalf("explicit flags did not override config:\n%s", main)
	}
	assertParser("unknown flag: --unknown")
}

func TestGenerateRejectsUnintendedOneFlag(t *testing.T) {
	command := exec.Command("go", "run", "./cmd/gosubc", "generate", "--one", "gosubc.json")
	command.Dir = "."
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for unintended --one flag, got success: %s", output)
	}
	if !strings.Contains(string(output), "unknown flag") && !strings.Contains(string(output), "flag provided but not defined") {
		t.Fatalf("expected unknown flag error, got: %s", output)
	}
}
