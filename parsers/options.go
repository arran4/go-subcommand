package parsers

type ParseOptions struct {
	SearchPaths []string
	Recursive   bool
}

type FormatConfig struct {
	IncludeInherited bool
}

type FormatOption func(*FormatConfig)

func WithInherited(include bool) FormatOption {
	return func(c *FormatConfig) {
		c.IncludeInherited = include
	}
}
