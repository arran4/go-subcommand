package parsers

import (
	"fmt"
	"io/fs"

	"github.com/arran4/go-subcommand/model"
)

type Parser interface {
	Parse(fsys fs.FS, root string) (*model.DataModel, error)
}

var parsers = make(map[string]Parser)

func Register(name string, p Parser) {
	parsers[name] = p
}

func Get(name string) (Parser, error) {
	p, ok := parsers[name]
	if !ok {
		return nil, fmt.Errorf("parser %q not found", name)
	}
	return p, nil
}
