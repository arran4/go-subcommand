package go_subcommand

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/tools/txtar"
)

// Template is a subcommand `gosubc template` -- Manage generation templates
func Template() {}

// TemplateExport is a subcommand `gosubc template export` -- Exports the built-in templates
//
// Exports the built-in templates to a specified directory or txtar file.
//
// Flags:
//
//	output: --output -o (default: "templates") The destination directory or file.
//	asTxtar: --as-txtar (default: false) Export as a txtar archive instead of a directory.
func TemplateExport(output string, asTxtar bool) error {
	if asTxtar {
		return exportTxtar(output)
	}
	return exportDir(output)
}

func exportDir(output string) error {
	err := fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel("templates", path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(output, rel)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		content, err := fs.ReadFile(TemplatesFS, path)
		if err != nil {
			return err
		}

		return os.WriteFile(targetPath, content, 0644)
	})

	if err != nil {
		return fmt.Errorf("failed to export templates: %w", err)
	}
	fmt.Printf("Templates exported successfully to directory %s\n", output)
	return nil
}

func exportTxtar(output string) error {
	archive := &txtar.Archive{}

	err := fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, err := filepath.Rel("templates", path)
		if err != nil {
			return err
		}

		content, err := fs.ReadFile(TemplatesFS, path)
		if err != nil {
			return err
		}

		archive.Files = append(archive.Files, txtar.File{
			Name: filepath.ToSlash(rel),
			Data: content,
		})

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to gather templates for txtar: %w", err)
	}

	content := txtar.Format(archive)
	if err := os.WriteFile(output, content, 0644); err != nil {
		return fmt.Errorf("failed to write txtar file %s: %w", output, err)
	}

	fmt.Printf("Templates exported successfully to txtar %s\n", output)
	return nil
}

// TemplateLayout is a subcommand `gosubc template layout` -- Displays the generation template layout
//
// Prints a tree-like structure of the templates and their descriptions.
func TemplateLayout() {
	fmt.Println("Template Layout:")
	fmt.Println("  templates/                      Base templates directory")
	fmt.Println("  ├── cmd/                        Code generation for the CLI structure")
	fmt.Println("  │   ├── root.go.gotmpl          The root CLI struct and execution loop")
	fmt.Println("  │   ├── cmd.go.gotmpl           The subcommand structs and execution loops")
	fmt.Println("  │   ├── main.go.gotmpl          The entry point (main.go) calling the RootCmd")
	fmt.Println("  │   ├── templates/              Embedded CLI usage templates")
	fmt.Println("  │   │   ├── usage.txt.gotmpl    The usage description for individual subcommands")
	fmt.Println("  │   │   ├── templates.go.gotmpl Loader for the generated usage text templates")
	fmt.Println("  │   │   └── man.gotmpl          Unix manual page template")
	fmt.Println("  ├── common/                     Common helper templates")
	fmt.Println("  │   └── common.gotmpl           Shared definitions and imports")
	fmt.Println("  ├── goreleaser.yml.gotmpl       Configuration template for goreleaser")
	fmt.Println("  └── github/                     CI/CD workflow templates")
	fmt.Println("      └── workflows/              GitHub Action workflows")
}
