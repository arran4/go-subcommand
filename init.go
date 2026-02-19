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
	if err := InitWithWriter(writer, dir, goreleaser, ghRelease, ghVerification); err != nil {
		return err
	}
	return nil
}

func InitWithWriter(writer FileWriter, dir string, goreleaser bool, ghRelease bool, ghVerification bool) error {
	if goreleaser {
		if err := GenerateGoreleaserWithWriter(writer, dir); err != nil {
			return err
		}
	}
	if ghRelease {
		if err := GenerateGhReleaseWithWriter(writer, dir); err != nil {
			return err
		}
	}
	if ghVerification {
		if err := GenerateGhVerificationWithWriter(writer, dir); err != nil {
			return err
		}
	}
	return nil
}

// GenerateGoreleaser is a subcommand `gosubc generate goreleaser`
//
// Flags:
//   dir: --dir (default: ".") Project root directory
func GenerateGoreleaser(dir string) error {
	return GenerateGoreleaserWithWriter(&OSFileWriter{}, dir)
}

func GenerateGoreleaserWithWriter(writer FileWriter, dir string) error {
	if err := initTemplates(); err != nil {
		return err
	}
	if err := generateFile(writer, dir, ".goreleaser.yml", "goreleaser.yml.gotmpl", nil, false); err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", filepath.Join(dir, ".goreleaser.yml"))
	return nil
}

// GenerateGhRelease is a subcommand `gosubc generate gh-release`
//
// Flags:
//   dir: --dir (default: ".") Project root directory
func GenerateGhRelease(dir string) error {
	return GenerateGhReleaseWithWriter(&OSFileWriter{}, dir)
}

func GenerateGhReleaseWithWriter(writer FileWriter, dir string) error {
	if err := initTemplates(); err != nil {
		return err
	}
	workflowsDir := filepath.Join(dir, ".github", "workflows")
	if err := generateFile(writer, workflowsDir, "release.yml", "release.yml.gotmpl", nil, false); err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", filepath.Join(workflowsDir, "release.yml"))
	return nil
}

// GenerateGhVerification is a subcommand `gosubc generate gh-verification`
//
// Flags:
//   dir: --dir (default: ".") Project root directory
func GenerateGhVerification(dir string) error {
	return GenerateGhVerificationWithWriter(&OSFileWriter{}, dir)
}

func GenerateGhVerificationWithWriter(writer FileWriter, dir string) error {
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
