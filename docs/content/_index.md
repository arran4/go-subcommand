---
title: "Go Subcommand"
type: docs
---

# Go Subcommand

**Go Subcommand** is a Go library and tool that generates a complete, dependency-less CLI subcommand system from your code comments.

<div class="buttons">
<a class="button" href="/go-subcommand/docs/introduction/">Get Started</a>
<a class="button" href="/go-subcommand/docs/reference/">Reference</a>
</div>

## Features

*   **Zero Dependencies**: Generated code uses only the standard library.
*   **Declarative**: Define commands and flags in comments.
*   **Type Safe**: Automatic argument parsing and type conversion.
*   **Nested Commands**: Supports arbitrarily deep command hierarchies.
*   **Man Pages**: Generates Unix man pages automatically.

## Example

```go
// Greet is a subcommand `app greet`
// Prints a greeting to the user.
func Greet(name string) {
    fmt.Printf("Hello, %s!\n", name)
}
```

Run `go generate`, then:

```bash
$ app greet --name Alice
Hello, Alice!
```
