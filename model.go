package go_subcommand

type DataModel struct {
	PackageName string
	Commands    []*Command
}

type Command struct {
	*DataModel
	MainCmdName string
	SubCommands []*SubCommand
}

type SubCommand struct {
	*Command
	Parent                 *SubCommand
	SubCommandName         string
	SubCommandFunctionName string
	SubCommandDescription  string
}

func (sc *SubCommand) SubCommandSequence() string {
	if sc.Parent == nil {
		return sc.SubCommandName
	}
	return sc.Parent.SubCommandSequence() + " " + sc.SubCommandName
}

func (sc *SubCommand) ParentCmdName() string {
	if sc.Parent != nil {
		return sc.Parent.SubCommandName
	}
	return ""
}

func (sc *SubCommand) HasSubcommands() bool {
	return len(sc.SubCommands) > 0
}
