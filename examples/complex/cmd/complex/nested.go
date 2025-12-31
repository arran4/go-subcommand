package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/arran4/go-subcommand/examples/complex"
)

var _ Cmd = (*nestedCmd)(nil)

type nestedCmd struct {
	*toplevelCmd
	Flags *flag.FlagSet

	count int

	verbose bool

	SubCommands map[string]Cmd
}

func (c *nestedCmd) Usage() {
	err := executeUsage(os.Stderr, "nested_usage.txt", c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating usage: %s\n", err)
	}
}

func (c *nestedCmd) Execute(args []string) error {
	if len(args) > 0 {
		if cmd, ok := c.SubCommands[args[0]]; ok {
			return cmd.Execute(args[1:])
		}
	}
	err := c.Flags.Parse(args)
	if err != nil {
		return NewUserError(err, fmt.Sprintf("flag parse error %s", err.Error()))
	}

	complex.Nested(c.count, c.verbose)

	return nil
}

func (c *toplevelCmd) NewnestedCmd() *nestedCmd {
	set := flag.NewFlagSet("nested", flag.ContinueOnError)
	v := &nestedCmd{
		toplevelCmd: c,
		Flags:       set,
		SubCommands: make(map[string]Cmd),
	}

	set.IntVar(&v.count, "count", 1, "Number of times to repeat")
	set.IntVar(&v.count, "c", 1, "Number of times to repeat")

	set.BoolVar(&v.verbose, "verbose", false, "Enable verbose output")
	set.BoolVar(&v.verbose, "v", false, "Enable verbose output")

	set.Usage = v.Usage

	return v
}
