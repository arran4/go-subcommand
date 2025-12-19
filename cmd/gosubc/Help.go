package main

import (
	"flag"
	"fmt"
	"os"
)

var _ Cmd = (*HelpCmd)(nil)

type HelpCmd struct {
	*RootCmd
	Flags *flag.FlagSet

	SubCommands map[string]Cmd
}

func (c *HelpCmd) Usage() {
	err := executeUsage(os.Stderr, "help_usage.txt", c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating usage: %s\n", err)
	}
}

func (c *HelpCmd) Execute(args []string) error {
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

func (c *RootCmd) NewHelpCmd() *HelpCmd {
	set := flag.NewFlagSet("help", flag.ContinueOnError)
	v := &HelpCmd{
		RootCmd:     c,
		Flags:       set,
		SubCommands: make(map[string]Cmd),
	}

	set.Usage = v.Usage

	return v
}
