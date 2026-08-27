package app

import stream "io"

// App is a subcommand `aliasio`.
func App() {}

// Run is a subcommand `aliasio run`.
func Run(
	reader stream.Reader, // @1 input reader
	writer stream.Writer, // @2 output writer
	readCloser stream.ReadCloser, // @3 input closer
	writeCloser stream.WriteCloser, // @4 output closer
) {
}
