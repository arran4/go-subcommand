package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/arran4/go-subcommand"
)

var _ Cmd = (*generateCmd)(nil)

type generateCmd struct {
	*RootCmd
	Flags *flag.FlagSet

	dir string

	SubCommands map[string]Cmd
}

func (c *generateCmd) Usage() {
	err := executeUsage(os.Stderr, "generate_usage.txt", c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating usage: %s\n", err)
	}
}

func (c *generateCmd) Execute(args []string) error {
	if len(args) > 0 {
		if cmd, ok := c.SubCommands[args[0]]; ok {
			return cmd.Execute(args[1:])
		}
	}
	err := c.Flags.Parse(args)
	if err != nil {
		return NewUserError(err, fmt.Sprintf("flag parse error %s", err.Error()))
	}

	go_subcommand.Generate(c.dir)

	return nil
}

func (c *RootCmd) NewgenerateCmd() *generateCmd {
	set := flag.NewFlagSet("generate", flag.ContinueOnError)
	v := &generateCmd{
		RootCmd:     c,
		Flags:       set,
		SubCommands: make(map[string]Cmd),
	}

	set.StringVar(&v.dir, "dir", "", "TODO: Add usage text")

	set.Usage = v.Usage

	return v
}
