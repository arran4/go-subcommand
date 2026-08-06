package commentv1

import (
	_ "embed"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/model"
)

//go:embed testdata/issue406_collision.go
var issue406CollisionSource string

func TestIssue406_ConstructorMethodName(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module testdata\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(issue406CollisionSource)},
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
