package parsers

// CommandDeclaration is a functionless source block supplied by a project file.
// Kind is command, text, markdown or man. Line is the first content line.
// Executable actions are always discovered from Go functions, never these blocks.
type CommandDeclaration struct {
	Kind     string
	Path     string
	Text     string
	Section  string
	Filename string
	Line     int
}

type ParseOptions struct {
	CommandDeclarations []CommandDeclaration
	SearchPaths         []string
	Recursive           bool
	// GOOS and GOARCH select the target source set. Empty values retain the
	// generator host defaults. They affect source discovery only; callers must
	// compile generated output for the same target.
	GOOS   string
	GOARCH string
	// BuildTags are additional build constraints satisfied while discovering
	// source files. Their order has no semantic effect.
	BuildTags []string
}
