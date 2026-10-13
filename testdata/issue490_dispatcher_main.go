package main

import (
	"os"

	"example.com/contextapp/buildinfo"
	generated "example.com/contextapp/cmd/contextapp"
)

// run is deliberately owned by the consumer. Applications can retain their
// process setup and choose how Execute errors affect the process.
func run(args []string) error {
	root, err := generated.NewRootWithContext("contextapp", "v1", "", "", buildinfo.BuildInfo{Version: "v1"})
	if err != nil {
		return err
	}
	return root.Execute(args)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		panic(err)
	}
}
