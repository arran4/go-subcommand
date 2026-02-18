package go_subcommand

import (
	"fmt"
	"path/filepath"
)

// PrGenerate is a subcommand `gosubc pr-generate` -- Generate GitHub Action workflow for PR generation
//
// Flags:
//
//	dir:	--dir	(default: ".")	Project root directory
func PrGenerate(dir string) error {
	writer := &OSFileWriter{}
	return PrGenerateWithWriter(writer, dir)
}

func PrGenerateWithWriter(writer FileWriter, dir string) error {
	if err := initTemplates(); err != nil {
		return err
	}

	workflowsDir := filepath.Join(dir, ".github", "workflows")
	if err := generateFile(writer, workflowsDir, "pr_generate.yml", "pr_generate.yml.gotmpl", nil, false); err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "pr_generate.yml"))
	return nil
}
