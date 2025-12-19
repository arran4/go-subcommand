package go_subcommand

type DataModel struct {
	PackageName string
	Commands    []*Command
}

type Command struct {
	*DataModel
	MainCmdName string
	SubCommands []*SubCommand
	PackagePath string
}

type FunctionParameter struct {
	Name string
	Type string
}

type SubCommand struct {
	*Command
	Parent                 *SubCommand
	SubCommands            []*SubCommand
	SubCommandName         string
	SubCommandFunctionName string
	SubCommandDescription  string
	RunCode                string
	ImportPath             string
	SubCommandPackageName  string
	Parameters             []*FunctionParameter
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

func (sc *SubCommand) ProgName() string {
	return sc.Command.MainCmdName + " " + sc.SubCommandSequence()
}
