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

type FunctionParameter struct {
	Name string
	Type string
}

type SubCommand struct {
	*Command
	Parent                 *SubCommand
	SubCommandName         string
	SubCommandFunctionName string
	SubCommandDescription  string
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
	// This seems incorrect, it should check if any other subcommand has this one as a parent.
	// This will be fixed later if needed.
	return false
}
