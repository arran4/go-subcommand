package parsers

type ParseOptions struct {
	SearchPaths []string
	Recursive   bool
	// GOOS and GOARCH select the target source set. Empty values retain the
	// generator host defaults. They affect source discovery only; callers must
	// compile generated output for the same target.
	GOOS   string
	GOARCH string
	// BuildTags are additional build constraints satisfied while discovering
	// source files. Their order has no semantic effect.
	BuildTags []string
}
