package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/arran4/go-subcommand"
)

var _ Cmd = (*listCmd)(nil)

type listCmd struct {
	*RootCmd
	Flags *flag.FlagSet

	dir string

	SubCommands map[string]Cmd
}

func (c *listCmd) Usage() {
	err := executeUsage(os.Stderr, "list_usage.txt", c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating usage: %s\n", err)
	}
}

func (c *listCmd) Execute(args []string) error {
	if len(args) > 0 {
		if cmd, ok := c.SubCommands[args[0]]; ok {
			return cmd.Execute(args[1:])
		}
	}
	err := c.Flags.Parse(args)
	if err != nil {
		return NewUserError(err, fmt.Sprintf("flag parse error %s", err.Error()))
	}

	go_subcommand.List(c.dir)

	return nil
}

func (c *RootCmd) NewlistCmd() *listCmd {
	set := flag.NewFlagSet("list", flag.ContinueOnError)
	v := &listCmd{
		RootCmd:     c,
		Flags:       set,
		SubCommands: make(map[string]Cmd),
	}

	set.StringVar(&v.dir, "dir", "", "TODO: Add usage text")

	set.Usage = v.Usage

	return v
}
