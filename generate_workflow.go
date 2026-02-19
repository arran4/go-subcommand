package go_subcommand

import (
	"fmt"
	"path/filepath"
)

// GenerateGithubWorkflow is a subcommand `gosubc generate-github-workflow`
//
// Flags:
//   dir: --dir (default: ".") Project root directory containing go.mod
func GenerateGithubWorkflow(dir string) error {
	writer := &OSFileWriter{}
	return GenerateGithubWorkflowWithWriter(writer, dir)
}

func GenerateGithubWorkflowWithWriter(writer FileWriter, dir string) error {
	if err := initTemplates(); err != nil {
		return err
	}

	workflowsDir := filepath.Join(dir, ".github", "workflows")
	if err := generateFile(writer, workflowsDir, "generate_verification.yml", "generate_verification.yml.gotmpl", nil, false); err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "generate_verification.yml"))
	return nil
}
