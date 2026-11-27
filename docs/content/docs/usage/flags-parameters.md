---
title: "Flags & Parameters"
weight: 3
---

# Flags and parameters

Annotated action parameters map to CLI flags by default. Prefer an indented `Flags:` block; inline parameter comments and preceding parameter comments are also supported, in that priority order.

## Copyable example

Save the following as `complex.go` in a fresh Go module. This is the action source from [examples/complex](https://github.com/arran4/go-subcommand/tree/v0.0.33/examples/complex), excluding its generation directive.

```go
package complex

import (
	"fmt"
	"time"
)

// TopLevel is a subcommand `complex toplevel` -- A top level command
//
// This command demonstrates a top level command with a string argument.
// It prints the name provided.
//
// Flags:
//
//	name: --name -n (default: "world") The name to greet
func TopLevel(name string) {
	fmt.Printf("TopLevel command executed with name: %s\n", name)
}

// Nested is a subcommand `complex toplevel nested` -- A nested command
//
// This command demonstrates a nested command under 'toplevel'.
// It takes an integer and a boolean.
//
// Flags:
//
//	count: --count -c (default: 1) Number of times to repeat
//	verbose: --verbose -v (default: false) Enable verbose output
func Nested(count int, verbose bool) {
	fmt.Printf("Nested command executed with count: %d, verbose: %v\n", count, verbose)
}

// Another is a subcommand `complex another` -- Another top level command
//
// This command demonstrates using a duration parameter.
//
// Flags:
//
//	wait: --wait -w (default: 1s) How long to wait
func Another(wait time.Duration) {
	fmt.Printf("Another command executed, waiting for %s\n", wait)
}
```

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.33 generate --emit-go-generate=false
go run ./cmd/complex toplevel -n test
go run ./cmd/complex toplevel nested -c 3 -v
go run ./cmd/complex another --wait=2s
```

The first invocation prints `TopLevel command executed with name: test`. The nested action accepts an integer and boolean; the last accepts a Go duration.

## Parameter declarations

| Form | Meaning | Availability |
| --- | --- | --- |
| `name: --name -n (default: "world") Name` | Named flag, alias, default and description | v0.0.33 |
| `src: @1 Source` | First positional | v0.0.33 |
| `src: @1 (optional) Source` | Explicitly omittable positional | Root support since version: v0.0.33 |
| `args: ... Remaining arguments` with `args ...string` | Variadic positional tail | v0.0.33 |
| `count: --count (required)` | Require explicit flag | v0.0.33 |
| `verbose: (from parent)` | Explicit inheritance | v0.0.33 |
| `seen: (flag presence)` on `map[string]bool` | Explicit flag presence | v0.0.32 |
| `raw: --raw (optional value)` | Optional flag value (GNU only) | Since version: v0.0.32 |
| `value: (generator: BuildValue)` | Local provider dependency | Provider DI since version: v0.0.32 |
| `config: (root context)` | Constructor-injected parameter | Since version: v0.0.32 |
| `args: (unmatched)` on `[]string` | Raw unmatched collector (plus-minus only) | Since version: v0.0.33 |

Use `(default: ...)` for explicit defaults. Do not assume a variadic declaration enforces a minimum count; validate application-specific limits in the action unless a documented generator contract applies.

Common types include strings, booleans, integers, floats, durations, pointers, repeated slices, and typed I/O interfaces. For `io.Reader` and `io.Writer`, only `-` borrows the corresponding invocation stream; other values are filenames. See [I/O examples](../examples/#file-and-stream-arguments). GNU is the default CLI parser; [plus/minus](../../advanced/plus-minus-parser/) and Go-flag have distinct token rules.

See [feature versions](../feature-versions/) and the [full syntax reference](https://github.com/arran4/go-subcommand/blob/main/README.md#comment-syntax-guide).
