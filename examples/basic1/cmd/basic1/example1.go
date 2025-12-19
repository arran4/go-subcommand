package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/arran4/go-subcommand/examples/basic1"
)

var _ Cmd = (*example1Cmd)(nil)

type example1Cmd struct {
	*RootCmd
	Flags *flag.FlagSet

	SubCommands map[string]Cmd
}

func (c *example1Cmd) Usage() {
	err := executeUsage(os.Stderr, "example1_usage.txt", c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating usage: %s\n", err)
	}
}

func (c *example1Cmd) Execute(args []string) error {
	if len(args) > 0 {
		if cmd, ok := c.SubCommands[args[0]]; ok {
			return cmd.Execute(args[1:])
		}
	}
	err := c.Flags.Parse(args)
	if err != nil {
		return NewUserError(err, fmt.Sprintf("flag parse error %s", err.Error()))
	}
	basic1.ExampleCmd1()
	return nil
}

func (c *RootCmd) Newexample1Cmd() *example1Cmd {
	set := flag.NewFlagSet("example1", flag.ContinueOnError)
	v := &example1Cmd{
		RootCmd:     c,
		Flags:       set,
		SubCommands: make(map[string]Cmd),
	}

	set.Usage = v.Usage

	return v
}
