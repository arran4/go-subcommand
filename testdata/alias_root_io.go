package app

import stream "io"

// App is a subcommand `aliasroot`.
func App(
	reader stream.Reader, // @1 input reader
	writer stream.Writer, // @2 output writer
) {
}
