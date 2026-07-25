package model

import (
	"reflect"
	"testing"
)

func TestFunctionParameterHelpers(t *testing.T) {
	tests := []struct {
		typ        string
		isSlice    bool
		isPointer  bool
		baseType   string
		isBool     bool
		isString   bool
		isDuration bool
	}{
		{"int", false, false, "int", false, false, false},
		{"*int", false, true, "int", false, false, false},
		{"[]int", true, false, "int", false, false, false},
		{"[]*int", true, true, "int", false, false, false},
		{"string", false, false, "string", false, true, false},
		{"*string", false, true, "string", false, true, false},
		{"[]string", true, false, "string", false, true, false},
		{"bool", false, false, "bool", true, false, false},
		{"*bool", false, true, "bool", true, false, false},
		{"time.Duration", false, false, "time.Duration", false, false, true},
		{"*time.Duration", false, true, "time.Duration", false, false, true},
		{"[]time.Duration", true, false, "time.Duration", false, false, true},
		{"[]*time.Duration", true, true, "time.Duration", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			p := &FunctionParameter{Type: tt.typ}

			if got := p.IsSlice(); got != tt.isSlice {
				t.Errorf("IsSlice() = %v, want %v", got, tt.isSlice)
			}
			if got := p.HasPointer(); got != tt.isPointer {
				t.Errorf("HasPointer() = %v, want %v", got, tt.isPointer)
			}
			if got := p.BaseType(); got != tt.baseType {
				t.Errorf("BaseType() = %v, want %v", got, tt.baseType)
			}
			if got := p.IsBool(); got != tt.isBool {
				t.Errorf("IsBool() = %v, want %v", got, tt.isBool)
			}
			if got := p.IsString(); got != tt.isString {
				t.Errorf("IsString() = %v, want %v", got, tt.isString)
			}
			if got := p.IsDuration(); got != tt.isDuration {
				t.Errorf("IsDuration() = %v, want %v", got, tt.isDuration)
			}
		})
	}
}

func TestFunctionParameterGenerationHelpers(t *testing.T) {
	tests := []struct {
		name        string
		parameter   FunctionParameter
		parserCall  string
		castCode    string
		description string
		generator   string
	}{
		{
			name:        "built in int",
			parameter:   FunctionParameter{Type: "int", Default: "3"},
			parserCall:  "strconv.Atoi(value)",
			castCode:    "value",
			description: "integer",
		},
		{
			name:        "unsigned width",
			parameter:   FunctionParameter{Type: "uint32"},
			parserCall:  "strconv.ParseUint(value, 10, 32)",
			castCode:    "uint32(value)",
			description: "uint32",
		},
		{
			name:        "custom parser and generator in imported package",
			parameter:   FunctionParameter{Type: "*Widget", Parser: ParserConfig{Type: ParserTypeCustom, Func: &FuncRef{CommandPackageName: "widgets", FunctionName: "Parse"}}, Generator: GeneratorConfig{Type: SourceTypeGenerator, Func: &FuncRef{CommandPackageName: "widgets", FunctionName: "Default"}}},
			parserCall:  "widgets.Parse(value)",
			castCode:    "Widget(value)",
			description: "Widget",
			generator:   "widgets.Default()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.parameter.ParserCall("value"); got != tt.parserCall {
				t.Errorf("ParserCall() = %q, want %q", got, tt.parserCall)
			}
			if got := tt.parameter.CastCode("value"); got != tt.castCode {
				t.Errorf("CastCode() = %q, want %q", got, tt.castCode)
			}
			if got := tt.parameter.TypeDescription(); got != tt.description {
				t.Errorf("TypeDescription() = %q, want %q", got, tt.description)
			}
			if got := tt.parameter.GeneratorCall(); got != tt.generator {
				t.Errorf("GeneratorCall() = %q, want %q", got, tt.generator)
			}
		})
	}

	p := FunctionParameter{Name: "childDir", InheritedFrom: "dir", Type: "string", Default: "tmp"}
	if got := p.ValueFieldName(); got != "dir" {
		t.Errorf("ValueFieldName() = %q, want dir", got)
	}
	if got := p.DefaultString(); got != `(default: "tmp")` {
		t.Errorf("DefaultString() = %q", got)
	}
	p.Required = true
	if got := p.DefaultString(); got != "(required)" {
		t.Errorf("DefaultString() = %q", got)
	}
}

func TestCommandAndSubCommandHelpers(t *testing.T) {
	rootDir := &FunctionParameter{Name: "rootDir", Type: "string", FlagAliases: []string{"root-dir"}, Default: "/tmp", Description: "root directory", DeclaredIn: "app", Required: true}
	root := &Command{
		MainCmdName:        "app",
		ImportPath:         "example.com/tools/app",
		CommandPackageName: "command",
		Parameters:         []*FunctionParameter{rootDir},
	}
	parentDir := &FunctionParameter{Name: "dir", Type: "string", FlagAliases: []string{"d", "dir"}, Default: ".", Description: "working directory", DeclaredIn: "parent"}
	parent := &SubCommand{Command: root, SubCommandName: "parent", Parameters: []*FunctionParameter{parentDir}}
	childDir := &FunctionParameter{Name: "d", Type: "string", DeclaredIn: "parent", InheritedFrom: "dir"}
	target := &FunctionParameter{Name: "target", Type: "string", IsPositional: true, PositionalArgIndex: 0, DeclaredIn: "child"}
	child := &SubCommand{
		Command:               root,
		Parent:                parent,
		SubCommandName:        "child",
		ImportPath:            "example.com/tools/app/child",
		SubCommandPackageName: "type",
		Parameters:            []*FunctionParameter{childDir, target},
	}
	parent.SubCommands = []*SubCommand{child}
	root.SubCommands = []*SubCommand{parent}

	root.ResolveInheritance()
	if childDir.Description != parentDir.Description || childDir.Default != parentDir.Default || !reflect.DeepEqual(childDir.FlagAliases, parentDir.FlagAliases) {
		t.Fatalf("inherited parameter = %#v, want values from %#v", childDir, parentDir)
	}
	if !root.HasRequiredFlags() || parent.HasRequiredFlags() {
		t.Fatal("required flag detection did not distinguish root and parent flags")
	}
	if got := root.ImportAlias(); got != "command" {
		t.Errorf("Command.ImportAlias() = %q, want command", got)
	}
	if got := child.ImportAlias(); got != "type" {
		t.Errorf("SubCommand.ImportAlias() = %q, want type", got)
	}
	if got := child.SubCommandSequence(); got != "parent child" {
		t.Errorf("SubCommandSequence() = %q", got)
	}
	if got := child.ParentCmdName(); got != "parent" {
		t.Errorf("ParentCmdName() = %q", got)
	}
	if !parent.HasSubcommands() || child.HasSubcommands() {
		t.Error("HasSubcommands() returned incorrect values")
	}
	if got := child.ProgName(); got != "app parent child" {
		t.Errorf("ProgName() = %q", got)
	}
	if got := child.FullUsageString(); got != "app [flags...] parent [flags...] child [flags...] <target>" {
		t.Errorf("FullUsageString() = %q", got)
	}

	all := child.AllParameters()
	if len(all) != 3 || all[0] != childDir || all[1] != target || all[2] != rootDir {
		t.Errorf("AllParameters() = %#v, want child inherited, target, root", all)
	}
	groups := child.ParameterGroups()
	gotGroups := make([]string, len(groups))
	for i, group := range groups {
		gotGroups[i] = group.CommandName
	}
	if want := []string{"app", "parent"}; !reflect.DeepEqual(gotGroups, want) {
		t.Errorf("ParameterGroups() = %v, want %v", gotGroups, want)
	}
	if child.MaxFlagLength() == 0 || child.MaxDefaultLength() == 0 {
		t.Error("expected flag and default widths for inherited parameters")
	}
}
