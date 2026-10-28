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

func TestLoadGenerateConfigRejectsUnknownAndUnsupported(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"unknown.json": `{"version":1,"typo":true}`,
		"version.json": `{"version":2}`,
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
	explicit := explicitGenerateFlags([]string{"generate", "--dir=explicit", "--clean", "--path", "commands"})
	for _, name := range []string{"dir", "clean", "path"} {
		if !explicit[name] {
			t.Errorf("%q not recorded", name)
		}
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
