package go_subcommand_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	go_subcommand "github.com/arran4/go-subcommand"
)

func TestIntegration_DashArgument(t *testing.T) {
	// Create a temporary directory for the test project
	tempDir, err := os.MkdirTemp("", "gosubc-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Define the test project content
	moduleName := "example.com/integration"
	goModContent := fmt.Sprintf("module %s\n\ngo 1.25\n", moduleName)

	// Create pkg directory for the logic
	pkgDir := filepath.Join(tempDir, "pkg")
	if err := os.Mkdir(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	pkgGoContent := `package pkg

import "fmt"

// Run is a subcommand ` + "`testapp run`" + `
//
// Flags:
//	output: -o, --output string Output file (default: "default")
//
// input: @1
func Run(output string, input string) {
	fmt.Printf("Output: %q, Input: %q\n", output, input)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "lib.go"), []byte(pkgGoContent), 0644); err != nil {
		t.Fatalf("failed to write lib.go: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Also need a dummy main.go or similar in root so ParseGoFiles finds something?
	// ParseGoFiles walks from root.
	// It finds .go files.
	// It parses them.
	// If pkg/lib.go is there, it should find it.
	// But it expects a "root command" logic usually.
	// If no root command logic is found, maybe it fails?
	// "no commands found in ..." error earlier.
	// In my repro, I had `repro/repro.go`. I ran `gosubc generate --dir repro`.
	// Here `pkg/lib.go` defines `testapp run`. `testapp` is main cmd. `run` is sub.
	// `testapp` itself doesn't have a function. That's fine.
	// But `ParseGoFiles` needs to find the module path.

	// Run gosubc Generate
	if err := go_subcommand.Generate(tempDir, ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Build the generated app
	exePath := filepath.Join(tempDir, "testapp")
	// The generated main is in cmd/testapp
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = tempDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	cmd = exec.Command("go", "build", "-o", exePath, "./cmd/testapp")
	cmd.Dir = tempDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Run the app with `-` argument
	// ./testapp run -
	runCmd := exec.Command(exePath, "run", "-")
	runCmd.Dir = tempDir
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("execution failed: %v\nOutput: %s", err, out)
	}

	expected := `Output: "default", Input: "-"`
	if !strings.Contains(string(out), expected) {
		t.Errorf("Unexpected output. Want substring %q, got:\n%s", expected, out)
	}

	// Test with --output
	runCmd = exec.Command(exePath, "run", "--output", "out.txt", "-")
	runCmd.Dir = tempDir
	out, err = runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("execution failed: %v\nOutput: %s", err, out)
	}
	expected = `Output: "out.txt", Input: "-"`
	if !strings.Contains(string(out), expected) {
		t.Errorf("Unexpected output. Want substring %q, got:\n%s", expected, out)
	}
}
