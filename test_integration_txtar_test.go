package go_subcommand

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestIntegration_Txtar(t *testing.T) {
	// Read all .txtar files in testdata/runtime
	files, err := os.ReadDir("testdata/runtime")
	if err != nil {
		t.Fatalf("failed to read testdata/runtime directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".txtar") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", "runtime", file.Name()))
			if err != nil {
				t.Fatalf("failed to read %s: %v", file.Name(), err)
			}

			archive := txtar.Parse(content)

			// Setup temporary directory for the test
			tempDir := t.TempDir()

			// Extract files
			for _, f := range archive.Files {
				if strings.HasSuffix(f.Name, ".args") || strings.HasSuffix(f.Name, ".out") {
					continue
				}
				path := filepath.Join(tempDir, f.Name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatalf("failed to create directory for %s: %v", f.Name, err)
				}
				if err := os.WriteFile(path, f.Data, 0644); err != nil {
					t.Fatalf("failed to write %s: %v", f.Name, err)
				}
			}

			// Generate code
			writer := &OSFileWriter{}
			// Use os.DirFS to provide the FS rooted at tempDir
			if err := GenerateWithFS(os.DirFS(tempDir), writer, tempDir, "", "commentv1"); err != nil {
				t.Fatalf("GenerateWithFS failed: %v", err)
			}

			// Run go mod tidy
			cmd := exec.Command("go", "mod", "tidy")
			cmd.Dir = tempDir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go mod tidy failed: %v\nOutput:\n%s", err, out)
			}

			// Find app name
			cmdDir := filepath.Join(tempDir, "cmd")
			entries, err := os.ReadDir(cmdDir)
			if err != nil || len(entries) == 0 {
				t.Fatalf("cmd directory not found or empty in %s", tempDir)
			}
			var appName string
			for _, entry := range entries {
				if entry.IsDir() {
					appName = entry.Name()
					break
				}
			}
			if appName == "" {
				// Fallback or fail?
				// agents.md and errors.go are in cmd/.
				// Subcommand dirs are in cmd/<name>.
				t.Fatalf("No app directory found in %s/cmd", tempDir)
			}

			binPath := filepath.Join(tempDir, appName)

			// Build
			buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/"+appName)
			buildCmd.Dir = tempDir
			if out, err := buildCmd.CombinedOutput(); err != nil {
				t.Fatalf("Build failed: %v\nOutput:\n%s", err, out)
			}

			// Run test cases defined in txtar
			for _, f := range archive.Files {
				if strings.HasSuffix(f.Name, ".args") {
					testCaseName := strings.TrimSuffix(f.Name, ".args")
					expectedOutFile := testCaseName + ".out"

					var expectedOut []byte
					found := false
					for _, ef := range archive.Files {
						if ef.Name == expectedOutFile {
							expectedOut = bytes.TrimSpace(ef.Data)
							found = true
							break
						}
					}
					if !found {
						continue
					}

					rawArgs := string(f.Data)
					argsLines := strings.Split(rawArgs, "\n")
					var args []string
					for _, line := range argsLines {
						line = strings.TrimSpace(line)
						if line != "" {
							args = append(args, line)
						}
					}

					t.Run(testCaseName, func(t *testing.T) {
						runCmd := exec.Command(binPath, args...)
						runCmd.Dir = tempDir
						output, _ := runCmd.CombinedOutput()

						trimmedOutput := bytes.TrimSpace(output)
						if !bytes.Contains(trimmedOutput, expectedOut) {
							t.Errorf("Output mismatch for %s.\nExpected to contain:\n%s\nGot:\n%s", testCaseName, string(expectedOut), string(trimmedOutput))
						}
					})
				}
			}
		})
	}
}
