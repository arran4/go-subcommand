package go_subcommand

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGenerateConfigStrictAndApplyPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gosubc.json")
	content := `{
  "version": 1,
  "dir": "configured",
  "cliParser": "go-flag",
  "recursive": false,
  "paths": ["commands"],
  "emitGoGenerate": false
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadGenerateConfig(path)
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
	dir := t.TempDir()

	// Test .mdconf extension
	mdconfPath := filepath.Join(dir, "gosubc.mdconf")
	mdconfContent := `version: 1
dir: configured_md
cli-parser: go-flag
recursive: false
clean: true
paths: cmd, pkg

# Variables
appName: mytool
env: test
`
	if err := os.WriteFile(mdconfPath, []byte(mdconfContent), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadGenerateConfig(mdconfPath)
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
	if config.TemplateVariables == nil || config.TemplateVariables["appName"] != "mytool" || config.TemplateVariables["env"] != "test" {
		t.Errorf("expected variables appName: mytool, env: test, got %v", config.TemplateVariables)
	}

	// Test .mdc extension with list items
	mdcPath := filepath.Join(dir, "gosubc.mdc")
	mdcContent := `version: 1
dir: mdc_dir
paths:
- alpha
- beta
force: true
`
	if err := os.WriteFile(mdcPath, []byte(mdcContent), 0o600); err != nil {
		t.Fatal(err)
	}
	config2, err := LoadGenerateConfig(mdcPath)
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
	dir := t.TempDir()
	for name, content := range map[string]string{
		"unknown.json":   `{"version":1,"typo":true}`,
		"version.json":   `{"version":2}`,
		"unknown.mdconf": "version: 1\ntypo: true\n",
		"version.mdconf": "version: 2\ndir: .\n",
		"badext.xyz":     "version: 1\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadGenerateConfig(path); err == nil {
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

func TestGenerateCLIConfiguredCommand_IgnoresAmbientOsArgs(t *testing.T) {
	// Programmatic callers must NOT be affected by ambient process arguments,
	// even when os.Args starts with host binary and contains generate / conflicting flags.
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"host", "generate", "--recursive=false", "--dir=/nonexistent/unrelated/path", "--clean"}

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
		project, "", "commentv1", "gnu", nil, true, false, false, nil, true, true, true, "", "", "", configPath,
	)
	if err != nil {
		t.Fatalf("GenerateCLIConfiguredCommand failed: %v", err)
	}

	// Also verify with GenerateCLIConfiguredWithOptions.
	err = GenerateCLIConfiguredWithOptions(GenerateCLIOptions{
		Dir:        project,
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
	root, err := os.ReadFile(filepath.Join(project, "cmd", "app", "root.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(root), "fs.Parse(args)") || strings.Contains(string(root), "//go:generate") {
		t.Fatalf("configuration was not applied to generated root:\n%s", root)
	}

	command = exec.Command("go", "run", "./cmd/gosubc", "generate", "--config", configPath, "--cli-parser", "gnu", "--emit-go-generate=true")
	command.Dir = "."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("explicit configured generation failed: %v\n%s", err, output)
	}
	root, err = os.ReadFile(filepath.Join(project, "cmd", "app", "root.go"))
	if err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile(filepath.Join(project, "cmd", "app", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(root), "fs.Parse(args)") || !strings.Contains(string(main), "//go:generate") {
		t.Fatalf("explicit flags did not override config:\nroot:\n%s\nmain:\n%s", root, main)
	}
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
