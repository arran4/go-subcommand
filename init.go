package go_subcommand

import (
	"fmt"
	"path/filepath"
)

// Init is a subcommand `gosubc init`
//
// Flags:
//   dir: --dir (default: ".") Project root directory containing go.mod
//   goreleaser: --goreleaser (default: false) Generate .goreleaser.yml configuration file
//   ghRelease: --gh-release (default: false) Generate GitHub Action workflow for GoReleaser (release.yml)
//   ghVerification: --gh-verification (default: false) Generate GitHub Action workflow for verification (generate_verification.yml)
func Init(dir string, goreleaser bool, ghRelease bool, ghVerification bool) error {
	writer := &OSFileWriter{}
	return InitWithWriter(writer, dir, goreleaser, ghRelease, ghVerification)
}

func InitWithWriter(writer FileWriter, dir string, goreleaser bool, ghRelease bool, ghVerification bool) error {
	if err := initTemplates(); err != nil {
		return err
	}

	if goreleaser {
		if err := generateFile(writer, dir, ".goreleaser.yml", "goreleaser.yml.gotmpl", nil, false); err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", filepath.Join(dir, ".goreleaser.yml"))
	}

	if ghRelease {
		workflowsDir := filepath.Join(dir, ".github", "workflows")
		if err := generateFile(writer, workflowsDir, "release.yml", "release.yml.gotmpl", nil, false); err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "release.yml"))
	}

	if ghVerification {
		workflowsDir := filepath.Join(dir, ".github", "workflows")
		if err := generateFile(writer, workflowsDir, "generate_verification.yml", "generate_verification.yml.gotmpl", nil, false); err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "generate_verification.yml"))
	}

	return nil
}
