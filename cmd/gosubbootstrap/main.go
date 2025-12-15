package main

import (
	"embed"
	"log"
	"os"
	"path"
	"text/template"

	go_subcommand "github.com/arran4/go-subcommand/pkg"
)

//go:embed templates/*.gotmpl
var templatesFS embed.FS

var templates *template.Template

func main() {
	var err error
	templates, err = template.ParseFS(templatesFS, "templates/*.gotmpl")
	if err != nil {
		log.Panicf("Error parsing templates: %s", err)
	}
	dataModel, err := go_subcommand.ParseGoFiles("pkg", "pkg/parser.go", "pkg/model.go")
	if err != nil {
		panic(err)
	}
	cmdOutDir := path.Join("cmd", "go-subcommand")
	if err := os.MkdirAll(cmdOutDir, 0755); err != nil {
		log.Panicf("failed to create directory %s: %s", cmdOutDir, err)
	}
	generateFile(cmdOutDir, "main.go", "main.gotmpl", dataModel.Commands[0])
}

func generateFile(dir, fileName, templateName string, data interface{}) {
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
