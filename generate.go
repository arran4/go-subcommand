package go_subcommand

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
	_ "github.com/arran4/go-subcommand/parsers/commentv1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:embed templates
var TemplatesFS embed.FS

var templates *template.Template

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

// Generate is a subcommand `gosubc generate` generates the subcommand code
//
// Flags:
//   dir:        --dir         (default: ".")         Project root directory containing go.mod
//   manDir:     --man-dir                            Directory to generate man pages in optional
//   parserName: --parser-name (default: "commentv1") Name of the parser to use
func Generate(dir string, manDir string, parserName string) error {
	return GenerateWithFS(os.DirFS(dir), &OSFileWriter{}, dir, manDir, parserName)
}

// GenerateWithFS generates code using provided FS and Writer
func GenerateWithFS(inputFS fs.FS, writer FileWriter, dir string, manDir string, parserName string) error {
	if err := initTemplates(); err != nil {
		return err
	}

	p, err := parsers.Get(parserName)
	if err != nil {
		return err
	}

	// inputFS is already rooted at the source directory, so we parse from "."
	dataModel, err := p.Parse(inputFS, ".")
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
		if err := generateFile(writer, path.Join(dir, "cmd"), "errors.go", "errors.go.gotmpl", cmd, true); err != nil {
			return err
		}
		if err := generateFile(writer, path.Join(dir, "cmd"), "agents.md", "agents.md.gotmpl", cmd, false); err != nil {
			return err
		}
		if err := generateFile(writer, cmdOutDir, "root_test.go", "root_test.go.gotmpl", cmd, true); err != nil {
			return err
		}
		cmdTemplatesDir := path.Join(cmdOutDir, "templates")
		if err := generateFile(writer, cmdTemplatesDir, "templates.go", "templates.go.gotmpl", cmd, true); err != nil {
			return err
		}
		for _, subCmd := range cmd.SubCommands {
			assignUsageFileNames(cmd.SubCommands)
			if err := generateSubCommandFiles(writer, cmdOutDir, cmdTemplatesDir, manDir, subCmd); err != nil {
				return err
			}
		}
	}
	return nil
}
func assignUsageFileNames(subCommands []*model.SubCommand) {
	seen := make(map[string]int)
	for _, sc := range subCommands {
		lower := strings.ToLower(sc.SubCommandName)
		count := seen[lower]
		seen[lower]++
		suffix := ""
		if count > 0 {
			suffix = fmt.Sprintf("_%d", count)
		}
		sc.UsageFileName = fmt.Sprintf("%s%s_usage.txt", lower, suffix)

		if len(sc.SubCommands) > 0 {
			assignUsageFileNames(sc.SubCommands)
		}
	}
}
func generateSubCommandFiles(writer FileWriter, cmdOutDir, cmdTemplatesDir, manDir string, subCmd *model.SubCommand) error {
	if err := generateFile(writer, cmdOutDir, subCmd.SubCommandName+".go", "cmd.go.gotmpl", subCmd, true); err != nil {
		return err
	}
	if err := generateFile(writer, cmdOutDir, subCmd.SubCommandName+"_test.go", "cmd_test.go.gotmpl", subCmd, true); err != nil {
		return err
	}
	if err := generateFile(writer, cmdTemplatesDir, subCmd.UsageFileName, "usage.txt.gotmpl", subCmd, false); err != nil {
		return err
	}
	if manDir != "" {
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

// Helper to bridge legacy parse calls
func parse(dir string, parserName string) (*model.DataModel, error) {
	if dir == "" {
		dir = "."
	}
	p, err := parsers.Get(parserName)
	if err != nil {
		return nil, err
	}
	return p.Parse(os.DirFS(dir), ".")
}
func initTemplates() error {
	var err error
	templates, err = ParseTemplates(TemplatesFS)
	return err
}

func ParseTemplates(fsys fs.FS) (*template.Template, error) {
	tmpl := template.New("").Funcs(template.FuncMap{
		"lower":   strings.ToLower,
		"title":   func(s string) string { return cases.Title(language.Und, cases.NoLower).String(s) },
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
		"slice": func(args ...interface{}) []interface{} {
			return args
		},
	})

	var patterns []string
	err := fs.WalkDir(fsys, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".gotmpl") {
			patterns = append(patterns, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking templates: %w", err)
	}

	if len(patterns) == 0 {
		return tmpl, nil
	}

	tmpl, err = tmpl.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, fmt.Errorf("error parsing templates: %w", err)
	}
	return tmpl, nil
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

	// Use filepath.Join for file paths as it is OS-aware, which is appropriate for FileWriter
	filePath := filepath.Join(dir, fileName)

	if err := writer.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	return nil
}
