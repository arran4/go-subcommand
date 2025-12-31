package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/arran4/go-subcommand/examples/complex"
)

var _ Cmd = (*toplevelCmd)(nil)

type toplevelCmd struct {
	*RootCmd
	Flags *flag.FlagSet

	name string

	SubCommands map[string]Cmd
}

func (c *toplevelCmd) Usage() {
	err := executeUsage(os.Stderr, "toplevel_usage.txt", c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating usage: %s\n", err)
	}
}

func (c *toplevelCmd) Execute(args []string) error {
	if len(args) > 0 {
		if cmd, ok := c.SubCommands[args[0]]; ok {
			return cmd.Execute(args[1:])
		}
	}
	err := c.Flags.Parse(args)
	if err != nil {
		return NewUserError(err, fmt.Sprintf("flag parse error %s", err.Error()))
	}

	complex.TopLevel(c.name)

	return nil
}

func (c *RootCmd) NewtoplevelCmd() *toplevelCmd {
	set := flag.NewFlagSet("toplevel", flag.ContinueOnError)
	v := &toplevelCmd{
		RootCmd:     c,
		Flags:       set,
		SubCommands: make(map[string]Cmd),
	}

	set.StringVar(&v.name, "name", "world", "The name to greet")
	set.StringVar(&v.name, "n", "world", "The name to greet")

	set.Usage = v.Usage

	v.SubCommands["nested"] = v.NewnestedCmd()

	return v
}
