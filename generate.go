package go_subcommand

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*.gotmpl
var templatesFS embed.FS

var templates *template.Template

func Generate(dir string) {
	var err error
	templates = template.New("").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	})
	templates, err = templates.ParseFS(templatesFS, "templates/*.gotmpl")
	if err != nil {
		log.Panicf("Error parsing templates: %s", err)
	}

	dataModel := parse(dir)
	for _, cmd := range dataModel.Commands {
		cmdOutDir := path.Join(dir, "cmd", cmd.MainCmdName)
		generateFile(cmdOutDir, "main.go", "main.go.gotmpl", cmd)
		generateFile(cmdOutDir, "root.go", "root.go.gotmpl", cmd)
		for _, subCmd := range cmd.SubCommands {
			generateFile(cmdOutDir, subCmd.SubCommandName+".go", "cmd.gotmpl", subCmd)
		}
	}
}

func parse(dir string) *DataModel {
	var files []io.Reader
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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
		files = append(files, f)
		return nil
	})
	dataModel, err := ParseGoFiles(dir, files...)
	if err != nil {
		panic(err)
	}
	return dataModel
}

func generateFile(dir, fileName, templateName string, data interface{}) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Panicf("failed to create directory %s: %s", dir, err)
	}
	filePath := path.Join(dir, fileName)
	f, err := os.Create(filePath)
	if err != nil {
		log.Panicf("failed to create file %s: %s", filePath, err)
	}
	defer f.Close()
	if err := templates.ExecuteTemplate(f, templateName, data); err != nil {
		log.Panicf("failed to execute template %s: %s", templateName, err)
	}
}
