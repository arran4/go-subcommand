package go_subcommand

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/txtar"
)

// Template is a subcommand `gosubc template` -- Manage generation templates
func Template() {}

// TemplateExport exports built-in templates using the legacy public API.
func TemplateExport(output string, asTxtar bool) error {
	return templateExportWithOptions(output, asTxtar, false)
}

// TemplateExportCommand is a subcommand `gosubc template export` -- Exports the built-in templates
//
// Exports the built-in templates to a specified directory or txtar file.
//
// Flags:
//
//	output: --output -o (default: "templates") The destination directory or file.
//	asTxtar: --as-txtar (default: false) Export as a txtar archive instead of a directory.
//	force: --force (default: false) Replace existing exported template files.
func TemplateExportCommand(output string, asTxtar bool, force bool) error {
	return templateExportWithOptions(output, asTxtar, force)
}

type exportedTemplateFile struct {
	name string
	data []byte
}

func templateExportWithOptions(output string, asTxtar bool, force bool) error {
	files, err := collectExportedTemplateFiles(TemplatesFS)
	if err != nil {
		return fmt.Errorf("gather templates for export: %w", err)
	}
	if asTxtar {
		return exportTxtar(output, files, force)
	}
	return exportDir(output, files, force)
}

func collectExportedTemplateFiles(templates fs.FS) ([]exportedTemplateFile, error) {
	var files []exportedTemplateFile
	err := fs.WalkDir(templates, "templates", func(templatePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(templatePath, "templates/")
		if rel == "." || rel == "" || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("invalid embedded template path %q", templatePath)
		}
		content, err := fs.ReadFile(templates, templatePath)
		if err != nil {
			return err
		}
		files = append(files, exportedTemplateFile{name: filepath.ToSlash(rel), data: content})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

func exportDir(output string, files []exportedTemplateFile, force bool) error {
	if err := preflightDirectoryExport(output, files, force); err != nil {
		return err
	}
	for _, file := range files {
		targetPath := filepath.Join(output, filepath.FromSlash(file.name))
		if err := ensureDirectory(filepath.Dir(targetPath)); err != nil {
			return fmt.Errorf("create template directory for %s: %w", file.name, err)
		}
		if err := writeFileAtomically(targetPath, file.data); err != nil {
			return fmt.Errorf("write template %s: %w", file.name, err)
		}
	}
	fmt.Printf("Templates exported successfully to directory %s\n", output)
	return nil
}

func exportTxtar(output string, files []exportedTemplateFile, force bool) error {
	archive := &txtar.Archive{}
	for _, file := range files {
		archive.Files = append(archive.Files, txtar.File{
			Name: file.name,
			Data: file.data,
		})
	}
	content := txtar.Format(archive)
	if err := preflightArchiveExport(output, force); err != nil {
		return err
	}
	if err := ensureDirectory(filepath.Dir(output)); err != nil {
		return fmt.Errorf("create txtar parent directory: %w", err)
	}
	if err := writeFileAtomically(output, content); err != nil {
		return fmt.Errorf("write txtar file %s: %w", output, err)
	}
	fmt.Printf("Templates exported successfully to txtar %s\n", output)
	return nil
}

func preflightDirectoryExport(output string, files []exportedTemplateFile, force bool) error {
	if err := rejectSymlinkAncestors(output); err != nil {
		return err
	}
	if info, err := os.Lstat(output); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("template export destination %s is not a directory", output)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect template export destination %s: %w", output, err)
	}

	for _, file := range files {
		target := filepath.Join(output, filepath.FromSlash(file.name))
		if err := rejectSymlinkAncestors(target); err != nil {
			return err
		}
		info, err := os.Lstat(target)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect template export target %s: %w", target, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing template export through symlink %s", target)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("template export target %s is not a regular file", target)
		}
		if !force {
			return fmt.Errorf("template export target %s already exists (use --force to replace it)", target)
		}
	}
	return nil
}

func preflightArchiveExport(output string, force bool) error {
	if err := rejectSymlinkAncestors(output); err != nil {
		return err
	}
	info, err := os.Lstat(output)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect txtar export destination %s: %w", output, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing txtar export through symlink %s", output)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("txtar export destination %s is not a regular file", output)
	}
	if !force {
		return fmt.Errorf("txtar export destination %s already exists (use --force to replace it)", output)
	}
	return nil
}

func rejectSymlinkAncestors(target string) error {
	for current := filepath.Clean(target); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing template export through symlink %s", current)
		}
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect template export path %s: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

func ensureDirectory(dir string) error {
	if err := rejectSymlinkAncestors(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a safe directory", dir)
	}
	return nil
}

func writeFileAtomically(destination string, content []byte) (err error) {
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".gosubc-template-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	closed := false
	defer func() {
		if closeErr := func() error {
			if closed {
				return nil
			}
			return temporary.Close()
		}(); err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0644); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	closed = true
	return os.Rename(temporaryName, destination)
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
