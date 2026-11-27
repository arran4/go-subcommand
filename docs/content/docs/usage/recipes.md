---
title: "Feature recipes"
weight: 7
---

# Feature recipes

Each section is a separate example module. Save the files at the paths specified, then run the pinned generator from that module:

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34 generate --emit-go-generate=false
```

## Optional flag values

**Since version:** v0.0.32 (GNU parser).

Create an empty module (for example `go mod init example.com/optionalvalue`) and save `app.go`:

```go
package app

import "fmt"

// Pointers is a subcommand `app pointers` - Pointer and double-pointer optional values.
//
// Flags:
//
//	ptr: (optional value) --ptr -p Optional string pointer.
//	num: (optional value) --num -n Optional int pointer.
//	tri: (optional value) --tri Optional double pointer.
func Pointers(ptr *string, num *int, tri **string) {
	ptrStr := "<nil>"
	if ptr != nil {
		ptrStr = fmt.Sprintf("%q", *ptr)
	}
	numStr := "<nil>"
	if num != nil {
		numStr = fmt.Sprintf("%d", *num)
	}
	triStr := "<nil>"
	if tri != nil {
		if *tri == nil {
			triStr = "<bare>"
		} else {
			triStr = fmt.Sprintf("%q", **tri)
		}
	}
	fmt.Printf("ptr=%s num=%s tri=%s\n", ptrStr, numStr, triStr)
}
```

Generate and run:

| Invocation | Expected stdout |
| --- | --- |
| `go run ./cmd/app pointers` | `ptr=<nil> num=<nil> tri=<nil>` |
| `go run ./cmd/app pointers --tri` | `ptr=<nil> num=<nil> tri=<bare>` |
| `go run ./cmd/app pointers --tri=` | `ptr=<nil> num=<nil> tri=""` |
| `go run ./cmd/app pointers --tri=hello --num=0` | `ptr=<nil> num=0 tri="hello"` |

Use a double pointer when omitted, bare and explicitly empty values must remain distinct. A following positional is not consumed as an optional flag value; use attached assignment.

## Optional positionals

**Since version:** v0.0.33 for explicitly optional root positionals.

Create a separate module (for example `go mod init example.com/optionalpos`) and save `app.go`:

```go
package app

import (
	"fmt"
)

// Root is a subcommand `app` -- Application with optional root positional.
//
// Flags:
//
//	url: @1 (optional) URL to extract
//	verbose: -v --verbose Verbose output
func Root(url string, verbose bool) error {
	if url == "" {
		fmt.Println("Usage policy: displaying root help")
		return nil
	}
	fmt.Printf("extracted: %s (verbose=%t)\n", url, verbose)
	return nil
}

// Sub is a subcommand `app sub` -- Explicit subcommand with required positional.
//
// Flags:
//
//	target: @1 (required) Required target
func Sub(target string) error {
	fmt.Printf("sub: %s\n", target)
	return nil
}

// Pair is a subcommand `app pair` -- Subcommand with required positional followed by optional positional.
//
// Flags:
//
//	first: @1 (required) First required target
//	second: @2 (optional) Second optional target
func Pair(first string, second string) error {
	fmt.Printf("pair: first=%s second=%s\n", first, second)
	return nil
}
```

After generation, `go run ./cmd/app` executes the root with an empty URL, while `go run ./cmd/app https://example.com` supplies the URL. `go run ./cmd/app pair arg1` reports `pair: first=arg1 second=`. Invoking `sub` without its required target returns a nonzero status. The root's example message is application behavior, not generated help.

## Constructor context

**Since version:** v0.0.32.

Context is supplied by the caller at root construction, not by argv. Flag presence is separate and records explicitly supplied values, including false, zero and empty strings.

Create the module `example.com/contextpresence` and a `buildinfo/` directory. Save `buildinfo/buildinfo.go`:

```go
package buildinfo

type BuildInfo struct {
	Version string
	Record func(string)
}
```

Save `app.go`:

```go
package app

import (
	"encoding/json"
	"fmt"

	"example.com/contextpresence/buildinfo"
)

// Root is a subcommand `app` -- Report root state.
//
// Flags:
//
//	info: (root context) Constructor-owned build information.
//	enabled: --enabled -e (default: true) Enable output.
//	count: --count -n (default: 7) Count.
//	label: --label -l (default: "default") Label.
//	seen: (flag presence) Explicit inputs.
func Root(info buildinfo.BuildInfo, enabled bool, count int, label string, seen map[string]bool) error {
	return report("root", info, enabled, count, label, seen)
}

// Child is a subcommand `app child` -- Report inherited context.
//
// Flags:
//
//	info: (from parent) Constructor-owned build information.
//	enabled: --enabled -e (default: true) Enable output.
//	count: --count -n (default: 7) Count.
//	label: --label -l (default: "default") Label.
//	seen: (flag presence) Explicit inputs.
func Child(info buildinfo.BuildInfo, enabled bool, count int, label string, seen map[string]bool) error {
	return report("child", info, enabled, count, label, seen)
}

// Grandchild is a subcommand `app child grandchild` -- Report deeply inherited context.
//
// Flags:
//
//	info: (from parent) Constructor-owned build information.
//	enabled: --enabled -e (default: true) Enable output.
//	count: --count -n (default: 7) Count.
//	label: --label -l (default: "default") Label.
//	seen: (flag presence) Explicit inputs.
func Grandchild(info buildinfo.BuildInfo, enabled bool, count int, label string, seen map[string]bool) error {
	return report("grandchild", info, enabled, count, label, seen)
}

func report(path string, info buildinfo.BuildInfo, enabled bool, count int, label string, seen map[string]bool) error {
	if info.Version == "reject" { return fmt.Errorf("context rejected") }
	flags, _ := json.Marshal(seen)
	line := fmt.Sprintf("%s version=%q enabled=%t count=%d label=%q seen=%s", path, info.Version, enabled, count, label, flags)
	if info.Record != nil { info.Record(line) } else { fmt.Println(line) }
	return nil
}
```

After generation, `go run ./cmd/app child grandchild --enabled=false -n 0 -l ""` prints an empty default build version, false, zero, empty label, and a presence map recording all three flags.

To exercise injected context, save this complete test as `cmd/app/context_example_test.go` **after** generating, or use it as the basis of an external harness. It demonstrates the constructor API; the ordinary generated main uses default context. Do not edit generator-owned Go files.

```go
package main

import (
	"testing"
	"example.com/contextpresence/buildinfo"
)

func TestContextExample(t *testing.T) {
	var got string
	info := buildinfo.BuildInfo{
		Version: "supplied",
		Record: func(s string) { got = s },
	}
	root, err := NewRootWithContext("app", "dev", "none", "unknown", info)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Execute([]string{"child"}); err != nil {
		t.Fatal(err)
	}
	const want = `child version="supplied" enabled=true count=7 label="default" seen={}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
```

Run `go test ./cmd/app -run TestContextExample`. Keep manually written test files separate from generated files; remove this demonstration test when finished.

## Invocation streams

**Since version:** v0.0.33 for constructor-owned framework streams.

The generated root can receive caller-owned streams through `RootOptions` and `CommandIO`. Framework help, usage, version, and `-` file-argument borrowing use these streams. Ordinary action functions that call `fmt.Println` directly still write to the process stdout; they do not automatically use the injected output.

For a module with a root named `app`, the following excerpt is a test in its generated executable package. The ellipses mark omitted package setup, imports and assertions.

```go
...
var stdout, stderr bytes.Buffer
root, err := NewRootWithOptions("app", "dev", "none", "unknown", RootOptions{
	IO: CommandIO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr},
})
if err != nil {
	t.Fatal(err)
}
if err := root.Execute([]string{"--help"}); err != nil {
	t.Fatal(err)
}
...
```

The caller retains ownership of supplied streams. Opened files are released by the generated lifecycle; borrowing a stream through a closer interface does not grant permission to close the underlying caller resource. See the [complete file and stream arguments example](../examples/#file-and-stream-arguments).
