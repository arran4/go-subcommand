package commentv1

import (
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/model"
)

func TestIssue406_ConstructorMethodName(t *testing.T) {
	src := `package testdata
// AppMain is a command ` + "`app`" + `
// description: App
func AppMain() {}

// Foo1 is a subcommand ` + "`app foo`" + `
// description: sub
func foo1() {}

// Foo2 is a subcommand ` + "`app bar foo`" + `
// description: sub
func foo2() {}

// Foo3 is a subcommand ` + "`app baz foo`" + `
// description: sub
func foo3() {}

// Foo4 is a subcommand ` + "`app deep nested foo`" + `
// description: sub
func foo4() {}

// FooBar1 is a subcommand ` + "`app foo-bar`" + `
// description: sub
func foo_bar_1() {}

// FooBar2 is a subcommand ` + "`app foo_bar`" + `
// description: sub
func foo_bar_2() {}

// FooBar3 is a subcommand ` + "`app FooBar`" + `
// description: sub
func foo_bar_3() {}

// NewUserError is a subcommand ` + "`app user error`" + `
// description: sub
func new_user_error() {}

// NewLazyCommand is a subcommand ` + "`app lazy command`" + `
// description: sub
func new_lazy_command() {}
`

	fsys := fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module testdata\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(src)},
	}

	p := &CommentParser{}
	d, err := p.Parse(fsys, ".", nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(d.Commands) == 0 {
		t.Fatalf("No commands parsed")
	}

	root := d.Commands[0]

	seenConstructors := make(map[string]bool)
	var checkConstructors func(subs []*model.SubCommand)
	checkConstructors = func(subs []*model.SubCommand) {
		for _, sub := range subs {
			name := sub.ConstructorMethodName
			if name == "" {
				t.Errorf("ConstructorMethodName is empty for subcommand %s", sub.SubCommandName)
			}
			if seenConstructors[name] {
				t.Errorf("Duplicate ConstructorMethodName found: %s", name)
			}
			seenConstructors[name] = true
			if len(sub.SubCommands) > 0 {
				checkConstructors(sub.SubCommands)
			}
		}
	}

	checkConstructors(root.SubCommands)

	if len(seenConstructors) < 11 {
		t.Errorf("Expected at least 11 constructors, got %d", len(seenConstructors))
	}
}
