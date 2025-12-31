package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/arran4/go-subcommand"
)

var _ Cmd = (*validateCmd)(nil)

type validateCmd struct {
	*RootCmd
	Flags *flag.FlagSet

	dir string

	SubCommands map[string]Cmd
}

func (c *validateCmd) Usage() {
	err := executeUsage(os.Stderr, "validate_usage.txt", c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating usage: %s\n", err)
	}
}

func (c *validateCmd) Execute(args []string) error {
	if len(args) > 0 {
		if cmd, ok := c.SubCommands[args[0]]; ok {
			return cmd.Execute(args[1:])
		}
	}
	err := c.Flags.Parse(args)
	if err != nil {
		return NewUserError(err, fmt.Sprintf("flag parse error %s", err.Error()))
	}
	go_subcommand.Validate(c.dir)
	return nil
}

func (c *RootCmd) NewvalidateCmd() *validateCmd {
	set := flag.NewFlagSet("validate", flag.ContinueOnError)
	v := &validateCmd{
		RootCmd:     c,
		Flags:       set,
		SubCommands: make(map[string]Cmd),
	}

	set.StringVar(&v.dir, "dir", ".", "Directory to validate")

	set.Usage = v.Usage

	return v
}
