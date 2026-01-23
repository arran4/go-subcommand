package go_subcommand

import (
	"fmt"
	"path/filepath"
)

// Goreleaser is a subcommand `gosubc goreleaser`
//
// Flags:
//   dir:            --dir string                  (default: ".")
//   githubWorkflow: --go-releaser-github-workflow
func Goreleaser(dir string, githubWorkflow bool) error {
	writer := &OSFileWriter{}
	return GoreleaserWithWriter(writer, dir, githubWorkflow)
}

func GoreleaserWithWriter(writer FileWriter, dir string, githubWorkflow bool) error {
	if err := initTemplates(); err != nil {
		return err
	}

	if err := generateFile(writer, dir, ".goreleaser.yml", "goreleaser.yml.gotmpl", nil, false); err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", filepath.Join(dir, ".goreleaser.yml"))

	if githubWorkflow {
		workflowsDir := filepath.Join(dir, ".github", "workflows")
		if err := generateFile(writer, workflowsDir, "release.yml", "github_release.yml.gotmpl", nil, false); err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "release.yml"))
	}
	return nil
}
