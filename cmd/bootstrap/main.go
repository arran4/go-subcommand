package main

import (
	"flag"
	"log"
	"os"
	"path"

	go_subcommand "github.com/arran4/go-subcommand"
)

func main() {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	var dir string
	var outDir string
	fs.StringVar(&dir, "dir", ".", "directory to look for subcommands (where the source is)")
	fs.StringVar(&outDir, "outDir", ".", "directory to output command to (where the cmd dir is)")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		log.Panicf("Error parsing flags: %s", err)
		return
	}
	cmds, err := go_subcommand.Parse(os.DirFS(dir))
	if err != nil {
		panic(err)
	}
	for _, cmd := range cmds {
		cmdOutDir := path.Join(outDir, "cmd", cmd.ProgName)

	}
}
