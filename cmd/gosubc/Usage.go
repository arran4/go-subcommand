package main

import (
	"flag"
	"fmt"
	"os"
)

var _ Cmd = (*UsageCmd)(nil)

type UsageCmd struct {
	*RootCmd
	Flags *flag.FlagSet

	SubCommands map[string]Cmd
}

func (c *UsageCmd) Usage() {
	err := executeUsage(os.Stderr, "usage_usage.txt", c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating usage: %s\n", err)
	}
}

func (c *UsageCmd) Execute(args []string) error {
	if len(args) > 0 {
		if cmd, ok := c.SubCommands[args[0]]; ok {
			return cmd.Execute(args[1:])
		}
	}
	err := c.Flags.Parse(args)
	if err != nil {
		return NewUserError(err, fmt.Sprintf("flag parse error %s", err.Error()))
	}

	c.RootCmd.Usage()

	return nil
}

func (c *RootCmd) NewUsageCmd() *UsageCmd {
	set := flag.NewFlagSet("usage", flag.ContinueOnError)
	v := &UsageCmd{
		RootCmd:     c,
		Flags:       set,
		SubCommands: make(map[string]Cmd),
	}

	set.Usage = v.Usage

	return v
}
