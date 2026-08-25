package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	files := []string{
		"templates/cmd/cmd.go.gotmpl",
		"templates/cmd/errors.go.gotmpl",
		"templates/cmd/flag_helpers.go.gotmpl",
		"templates/cmd/main.go.gotmpl",
		"templates/cmd/root.go.gotmpl",
		"templates/cmd/versioncmd.gotmpl",
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			panic(err)
		}
		s := string(b)
		if strings.Contains(s, "package {{.MainCmdName}}") {
			s = strings.Replace(s, "package {{.MainCmdName}}", "package {{if .SubCommandPackageName}}{{.SubCommandPackageName}}{{else if .CommandPackageName}}{{.CommandPackageName}}{{else}}main{{end}}", 1)
			os.WriteFile(f, []byte(s), 0644)
			fmt.Println("Fixed", f)
		} else if strings.Contains(s, "package cmd") {
			s = strings.Replace(s, "package cmd", "package {{if .SubCommandPackageName}}{{.SubCommandPackageName}}{{else if .CommandPackageName}}{{.CommandPackageName}}{{else}}main{{end}}", 1)
			os.WriteFile(f, []byte(s), 0644)
			fmt.Println("Fixed", f)
		}
	}
}
