package commentv1

import (
	"bytes"
	"go/token"
	"testing"
)

func TestParseGoFile_Alias(t *testing.T) {
	src := `package mypkg
import stream "io"
import filesystem "os"

// Run is a subcommand ` + "`alias`" + `
func Run(r stream.Reader, w stream.Writer, rc stream.ReadCloser, wc stream.WriteCloser, f *filesystem.File) {}
`
	fset := token.NewFileSet()
	cmdTree := &CommandsTree{
		Commands:    map[string]*CommandTree{},
		PackagePath: "example.com/mypkg",
	}

	err := ParseGoFile(fset, "file.go", "example.com/mypkg", bytes.NewBufferString(src), cmdTree)
	if err != nil {
		t.Fatalf("ParseGoFile failed: %v", err)
	}

	tree, ok := cmdTree.Commands["alias"]
	if !ok {
		t.Fatalf("command 'alias' not found")
	}
	if len(tree.Parameters) != 5 {
		t.Fatalf("expected 5 parameters, got %d", len(tree.Parameters))
	}

	p := tree.Parameters[0]
	if p.TypeImportPath != "io" || p.TypeName != "Reader" || p.BaseType() != "io.Reader" {
		t.Errorf("r failed: %v %v %v", p.TypeImportPath, p.TypeName, p.BaseType())
	}

	p = tree.Parameters[1]
	if p.TypeImportPath != "io" || p.TypeName != "Writer" || p.BaseType() != "io.Writer" {
		t.Errorf("w failed: %v %v %v", p.TypeImportPath, p.TypeName, p.BaseType())
	}

	p = tree.Parameters[4]
	if p.TypeImportPath != "os" || p.TypeName != "File" || p.BaseType() != "os.File" {
		t.Errorf("f failed: %v %v %v", p.TypeImportPath, p.TypeName, p.BaseType())
	}
}
