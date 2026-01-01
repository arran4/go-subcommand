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

// Generate is a subcommand `gosubc generate` that generates the subcommand code
// param dir (default: ".") Directory to generate code for
// param manDir (--man-dir) Directory to generate man pages in (optional)
func Generate(dir string, manDir string) error {
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

	dataModel, err := parse(dir)
	if err != nil {
		return err
	}
	if len(dataModel.Commands) == 0 {
		return fmt.Errorf("no commands found in %s", dir)
	}
	for _, cmd := range dataModel.Commands {
		cmdOutDir := path.Join(dir, "cmd", cmd.MainCmdName)
		if err := generateFile(cmdOutDir, "main.go", "main.go.gotmpl", cmd, true); err != nil {
			return err
		}
		if err := generateFile(cmdOutDir, "root.go", "root.go.gotmpl", cmd, true); err != nil {
			return err
		}
		cmdTemplatesDir := path.Join(cmdOutDir, "templates")
		if err := generateFile(cmdTemplatesDir, "templates.go", "templates.go.gotmpl", cmd, true); err != nil {
			return err
		}
		for _, subCmd := range cmd.SubCommands {
			if err := generateSubCommandFiles(cmdOutDir, cmdTemplatesDir, manDir, subCmd); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateSubCommandFiles(cmdOutDir, cmdTemplatesDir, manDir string, subCmd *SubCommand) error {
	if err := generateFile(cmdOutDir, subCmd.SubCommandName+".go", "cmd.gotmpl", subCmd, true); err != nil {
		return err
	}
	if err := generateFile(cmdTemplatesDir, subCmd.SubCommandName+"_usage.txt", "usage.txt.gotmpl", subCmd, false); err != nil {
		return err
	}
	if manDir != "" {
		manFileName := fmt.Sprintf("%s-%s.1", subCmd.MainCmdName, strings.ReplaceAll(subCmd.SubCommandSequence(), " ", "-"))
		if err := generateFile(manDir, manFileName, "man.gotmpl", subCmd, false); err != nil {
			return err
		}
	}
	for _, s := range subCmd.SubCommands {
		if err := generateSubCommandFiles(cmdOutDir, cmdTemplatesDir, manDir, s); err != nil {
			return err
		}
	}
	return nil
}

func parse(dir string) (*DataModel, error) {
	if dir == "" {
		dir = "."
	}
	var files []File
	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			c.Close()
		}
	}()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		files = append(files, File{
			Path:   path,
			Reader: f,
		})
		closers = append(closers, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ParseGoFiles(dir, files...)
}

func generateFile(dir, fileName, templateName string, data interface{}, formatCode bool) error {
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

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	filePath := path.Join(dir, fileName)
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer f.Close()

	_, err = f.Write(content)
	return err
}
