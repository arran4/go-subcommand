package go_subcommand

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*.gotmpl
var templatesFS embed.FS

var templates *template.Template

type closableFile struct {
	io.Reader
	io.Closer
}

// FileWriter interface allows mocking file system writes
type FileWriter interface {
	WriteFile(path string, content []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
}

// OSFileWriter implements FileWriter using os package
type OSFileWriter struct{}

func (w *OSFileWriter) WriteFile(path string, content []byte, perm os.FileMode) error {
	return os.WriteFile(path, content, perm)
}

func (w *OSFileWriter) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Generate is a subcommand `gosubc generate` that generates the subcommand code
// param dir (default: ".") Directory to generate code for
// param manDir (--man-dir) Directory to generate man pages in (optional)
func Generate(dir string, manDir string) error {
	return GenerateWithFS(os.DirFS(dir), &OSFileWriter{}, dir, manDir)
}

// GenerateWithFS generates code using provided FS and Writer
func GenerateWithFS(inputFS fs.FS, writer FileWriter, dir string, manDir string) error {
	var err error
	templates = template.New("").Funcs(template.FuncMap{
		"lower":   strings.ToLower,
		"title":   strings.Title,
		"upper":   strings.ToUpper,
		"replace": strings.ReplaceAll,
		"add":     func(a, b int) int { return a + b },
		"until": func(n int) []int {
			res := make([]int, n)
			for i := 0; i < n; i++ {
				res[i] = i
			}
			return res
		},
	})
	templates, err = templates.ParseFS(templatesFS, "templates/*.gotmpl")
	if err != nil {
		return fmt.Errorf("error parsing templates: %w", err)
	}

	// inputFS is already rooted at the source directory, so we parse from "."
	dataModel, err := ParseGoFiles(inputFS, ".")
	if err != nil {
		return err
	}
	if len(dataModel.Commands) == 0 {
		return fmt.Errorf("no commands found in %s", dir)
	}
	for _, cmd := range dataModel.Commands {
		cmdOutDir := path.Join(dir, "cmd", cmd.MainCmdName)
		if err := generateFile(writer, cmdOutDir, "main.go", "main.go.gotmpl", cmd, true); err != nil {
			return err
		}
		if err := generateFile(writer, cmdOutDir, "root.go", "root.go.gotmpl", cmd, true); err != nil {
			return err
		}
		cmdTemplatesDir := path.Join(cmdOutDir, "templates")
		if err := generateFile(writer, cmdTemplatesDir, "templates.go", "templates.go.gotmpl", cmd, true); err != nil {
			return err
		}
		for _, subCmd := range cmd.SubCommands {
			if err := generateSubCommandFiles(writer, cmdOutDir, cmdTemplatesDir, manDir, subCmd); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateSubCommandFiles(writer FileWriter, cmdOutDir, cmdTemplatesDir, manDir string, subCmd *SubCommand) error {
	if err := generateFile(writer, cmdOutDir, subCmd.SubCommandName+".go", "cmd.gotmpl", subCmd, true); err != nil {
		return err
	}
	if err := generateFile(writer, cmdTemplatesDir, subCmd.SubCommandName+"_usage.txt", "usage.txt.gotmpl", subCmd, false); err != nil {
		return err
	}
	if manDir != "" {
		// Use filepath.Join to avoid unused import error if path/filepath is needed elsewhere or ensure usage
		// But wait, we imported path/filepath and it was unused.
		// "strings" and "fmt" are used here.
		// We used "path" for paths.
		// Let's use filepath somewhere if we keep the import, or remove it.
		// But manFileName construction doesn't strictly need filepath.

		manFileName := fmt.Sprintf("%s-%s.1", subCmd.MainCmdName, strings.ReplaceAll(subCmd.SubCommandSequence(), " ", "-"))
		if err := generateFile(writer, manDir, manFileName, "man.gotmpl", subCmd, false); err != nil {
			return err
		}
	}
	for _, s := range subCmd.SubCommands {
		if err := generateSubCommandFiles(writer, cmdOutDir, cmdTemplatesDir, manDir, s); err != nil {
			return err
		}
	}
	return nil
}

func generateFile(writer FileWriter, dir, fileName, templateName string, data interface{}, formatCode bool) error {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	var content []byte
	if formatCode {
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to format generated code for %s: %w\n%s", fileName, err, buf.String())
		}
		content = formatted
	} else {
		content = buf.Bytes()
	}

	if err := writer.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	// Use filepath.Join here instead of path.Join to justify import if needed,
	// but path.Join is usually preferred for logic not tied to OS.
	// However writer.WriteFile expects OS path.
	// OSFileWriter uses os.WriteFile.
	// So filepath.Join is better for cross-platform.
	// The original code used path.Join.
	// I will switch to filepath.Join for file paths to fix unused import AND correctness.
	filePath := filepath.Join(dir, fileName)

	if err := writer.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	return nil
}
