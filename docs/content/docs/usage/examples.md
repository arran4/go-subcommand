---
title: "Examples"
weight: 4
---

# Examples

Each directory under [examples/](https://github.com/arran4/go-subcommand/tree/main/examples) is a standalone Go module. The linked sources and existing generated files are authoritative. Run these commands from the corresponding example directory. File examples need an input file; issue185 and issue455 are historical source reproductions rather than claims of new test execution.

| Example | Demonstrates | Invocation |
| --- | --- | --- |
| [basic1](https://github.com/arran4/go-subcommand/tree/main/examples/basic1) | Minimal action | `go run ./cmd/basic1 example1` |
| [complex](https://github.com/arran4/go-subcommand/tree/main/examples/complex) | Flags, defaults, nesting and duration | `go run ./cmd/complex toplevel -n test` |
| [returns](https://github.com/arran4/go-subcommand/tree/main/examples/returns) | Returning errors | `go run ./cmd/returns simple` |
| [defaultexpr](https://github.com/arran4/go-subcommand/tree/main/examples/defaultexpr) | Go expressions for defaults | `go run ./cmd/app` |
| [io_types](https://github.com/arran4/go-subcommand/tree/main/examples/io_types) | File arguments and borrowed streams | `go run ./cmd/io_types copy-file input.txt -` |
| [issue50](https://github.com/arran4/go-subcommand/tree/main/examples/issue50) | Explicit parent inheritance | `go run ./cmd/issue50 parent` |
| [go-flag-test](https://github.com/arran4/go-subcommand/tree/main/examples/go-flag-test) | Go-flag root, GNU child | `go run ./cmd/app -name value` |
| [gnu-root-goflag-child](https://github.com/arran4/go-subcommand/tree/main/examples/gnu-root-goflag-child) | Mixed dialects and inherited flags | `go run ./cmd/app child -go-flag value -b childpos` |
| [issue185](https://github.com/arran4/go-subcommand/tree/main/examples/issue185) | I/O flags and positional readers/writers | `go run ./cmd/app mycmd2 - -` |
| [issue455](https://github.com/arran4/go-subcommand/tree/main/examples/issue455) | Repeated I/O reader source example | `go run ./cmd/app --in input.txt` |

```sh
git clone https://github.com/arran4/go-subcommand.git
cd go-subcommand/examples/complex
go run ./cmd/complex toplevel -n test
go test ./...
```

For a clean project, use the [quickstart](../getting-started/). Each full source file below belongs in a separate empty module. Generate using:

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34 generate --emit-go-generate=false
```

## Error returns

```go
package returns

import (
	"errors"
	"fmt"
)

// SimpleError is a subcommand `returns simple`
// Returns a simple error if the flag is set.
// Flags:
//
//	fail: --fail -f (default: false) Make the command fail
func SimpleError(fail bool) error {
	if fail {
		return errors.New("simple error occurred")
	}
	fmt.Println("SimpleError success")
	return nil
}
```

```sh
go run ./cmd/returns simple
go run ./cmd/returns simple --fail
```

The first prints `SimpleError success`; the second returns an error and nonzero exit status.

## Default expressions

```go
package defaultexpr

import (
	"fmt"
	"math"
	"runtime"
)

var (
	_ = math.MaxInt32
	_ = runtime.NumCPU
)


// Root is a subcommand `app`
//
// Flags:
//
//	cores: (default: runtime.NumCPU()) Number of cores
//	limit: (default: math.MaxInt32) Max limit
func Root(cores int, limit int) {
	fmt.Printf("cores: %d, limit: %d\n", cores, limit)
}
```

Run `go run ./cmd/app`. The core count depends on the host; the limit is 2147483647.

## File and stream arguments

```go
package app

import (
	"fmt"
	"io"
)

// App is a subcommand `io_types`
func App() error {
	return nil
}

// RunCopyFile is a subcommand `io_types copy-file` that copies a file
func RunCopyFile(
	input io.Reader, // @1 The input file
	output io.Writer, // @2 The output file
) error {
	_, err := io.Copy(output, input)
	if err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}
	return nil
}

// RunInspectFile is a subcommand `io_types inspect-file` that inspects an io.Reader
func RunInspectFile(
	file io.Reader, // @1 The file to inspect
) error {
	b, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}
	fmt.Printf("File size: %d\n", len(b))
	return nil
}
```

```sh
printf 'Hello World!\n\n' > input.txt
go run ./cmd/io_types copy-file input.txt output.txt
go run ./cmd/io_types inspect-file input.txt
go run ./cmd/io_types copy-file input.txt -
printf 'from stdin\n' | go run ./cmd/io_types copy-file - -
```

Inspect reports 14 bytes for the sample input. Only `-` selects a borrowed stream, and generated code leaves borrowed streams open. Constructor-owned framework stream injection: **since version:** v0.0.33. Direct action-stream annotation is not available in v0.0.34.

## Explicit inheritance

```go
package issue50

import "fmt"

// Root is a subcommand `issue50`
func Root() error {
	return nil
}

// Parent is a subcommand `issue50 parent`
//
// Flags:
//
//	verbose: --verbose -v Verbosity level
func Parent(verbose bool) error {
	fmt.Printf("Parent verbose: %v\n", verbose)
	return nil
}

// Child is a subcommand `issue50 parent child`
//
// Flags:
//
//	verbose: (from parent)
func Child(verbose bool) error {
	fmt.Printf("Child verbose: %v\n", verbose)
	return nil
}
```

```sh
go run ./cmd/issue50 parent
go run ./cmd/issue50 parent --verbose child
```

The child uses the parent's parameter instead of registering a separate flag.

## More practical guides

- [Optional flag values and optional positionals](../recipes/) — distinguish omitted, bare and explicitly empty values.
- [Constructor context and invocation streams](../recipes/#constructor-context) — test generated commands with application-owned context and streams.
- [Scoped dependencies and lifecycle](../../advanced/scoped-lifecycle/) — typed requirements, authorization, lazy acquisition and cleanup.
- [Plus/minus parser](../../advanced/plus-minus-parser/) — polarity, clusters, parser boundaries and raw unmatched arguments.
- [Functionless metadata](../../advanced/command-metadata/) and [no-command policy](../../advanced/no-command-policy/) — application roots and help.
- [Generated errors](../../advanced/generated-errors/) — preserve error chains and exit status.

See [Feature versions](../feature-versions/) to choose a generator version.
