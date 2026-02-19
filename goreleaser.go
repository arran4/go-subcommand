package go_subcommand

import (
	"fmt"
	"path/filepath"
)

// Goreleaser is a subcommand `gosubc goreleaser` generates goreleaser configuration and workflows
//
// Flags:
//
//	dir:			--dir				(default: ".")		The project root directory
//	githubWorkflow:		--go-releaser-github-workflow	(default: false)	Generate GitHub Actions release workflow
//	verificationWorkflow:	--verification-workflow		(default: false)	Generate verification workflow
//	prCreationWorkflow:	--pr-creation-workflow		(default: false)	Generate PR creation workflow
func Goreleaser(dir string, githubWorkflow bool, verificationWorkflow bool, prCreationWorkflow bool) error {
	writer := &OSFileWriter{}
	return GoreleaserWithWriter(writer, dir, githubWorkflow, verificationWorkflow, prCreationWorkflow)
}

func GoreleaserWithWriter(writer FileWriter, dir string, githubWorkflow bool, verificationWorkflow bool, prCreationWorkflow bool) error {
	if err := initTemplates(); err != nil {
		return err
	}

	if err := generateFile(writer, dir, ".goreleaser.yml", "goreleaser.yml.gotmpl", nil, false); err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", filepath.Join(dir, ".goreleaser.yml"))

	if githubWorkflow {
		workflowsDir := filepath.Join(dir, ".github", "workflows")
		if err := generateFile(writer, workflowsDir, "release.yml", "release.yml.gotmpl", nil, false); err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "release.yml"))
	}
	if verificationWorkflow {
		workflowsDir := filepath.Join(dir, ".github", "workflows")
		if err := generateFile(writer, workflowsDir, "generate_verification.yml", "generate_verification.yml.gotmpl", nil, false); err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "generate_verification.yml"))
	}
	if prCreationWorkflow {
		workflowsDir := filepath.Join(dir, ".github", "workflows")
		if err := generateFile(writer, workflowsDir, "generate_pr.yml", "generate_pr.yml.gotmpl", nil, false); err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "generate_pr.yml"))
	}
	return nil
}
