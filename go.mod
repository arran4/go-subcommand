module github.com/arran4/go-subcommand

go 1.25.3

require (
	github.com/arran4/go-subcommand/examples/basic1 v0.0.0-00010101000000-000000000000
	github.com/arran4/go-subcommand/examples/complex v0.0.0-00010101000000-000000000000
	golang.org/x/mod v0.31.0
)

replace github.com/arran4/go-subcommand/examples/basic1 => ./examples/basic1

replace github.com/arran4/go-subcommand/examples/complex => ./examples/complex
