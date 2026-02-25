# Go Subcommand

**Go Subcommand** generates subcommand code for command-line interfaces (CLIs) in Go from source code comments. By leveraging specially formatted code comments, it automatically generates a dependency-less subcommand system, allowing you to focus on your application's core logic instead of boilerplate code.

**Status:** Pre-v1. The API and generated code structure may change.

## Key Features

- **Convention over Configuration:** Define your CLI structure with simple, intuitive code comments.
- **Zero Dependencies:** The generated code is self-contained and doesn't require any external libraries.
- **Automatic Code Generation:** `gosubc` parses your Go files and generates a complete, ready-to-use CLI.
- **Parameter Auto-Mapping:** Automatically maps CLI flags, positional arguments, and variadic arguments to function parameters.
- **Rich Syntax Support:** Supports custom flag names, default values, and description overrides via comments.
- **Man Page Generation:** Automatically generate Unix man pages for your CLI.

## Installation

To install `gosubc`, use `go install`:

```bash
go install github.com/arran4/go-subcommand/cmd/gosubc@latest
```

## Getting Started

### 1. Define Your Commands

Create a Go file and define a function that will serve as your command. Add a comment above the function in the format `// FunctionName is a subcommand 'root-command sub-command...'`.

Example `main.go`:

```go
package main

import "fmt"

// PrintHelloWorld is a subcommand `my-app hello`
// This command prints "Hello, World!" to the console.
func PrintHelloWorld() {
    fmt.Println("Hello, World!")
}
```

### 2. Add a `generate.go` File

Create a file named `generate.go` in the same directory (package `main`). This robust version checks if `gosubc` is installed; if not, it uses `go run` to fetch and run it. This ensures it works for everyone without manual installation steps.

```go
package main

//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate || go run github.com/arran4/go-subcommand/cmd/gosubc generate"
```

### 3. Generate the CLI

Run `go generate` in your terminal:

```bash
go generate
```

This will create a `cmd/my-app` directory containing the generated CLI code.

### 4. Run Your New CLI

You can now run your newly generated CLI:

```bash
go run ./cmd/my-app hello
```

Output:
```
Hello, World!
```

## Comment Syntax Guide

`go-subcommand` uses a specific comment syntax to configure your CLI.

### Subcommand Definition

The primary directive defines where the command lives in the CLI hierarchy:

```go
// FuncName is a subcommand `root-cmd parent child`
```

### Subcommand Aliases

You can define aliases for a subcommand using the `Aliases:` or `Alias:` directive.

```go
// MyFunc is a subcommand `app cmd`
// Aliases: c, command
func MyFunc() { ... }
```

You can also use an inline syntax:

```go
// MyFunc is a subcommand `app cmd` (aka: c)
func MyFunc() { ... }
```

### Description & Extended Help

*   **Short Description:** The text immediately following the subcommand definition (or prefixed with `that ` or `-- `) becomes the short description used in usage lists.
*   **Extended Help:** Any subsequent lines that do not look like parameter definitions are treated as extended help text, displayed when the user requests help for that specific command.

```go
// MyFunc is a subcommand `app cmd` -- Does something cool
//
// This is the extended help text. It can span multiple lines
// and provide detailed usage examples or explanations.
```

### Parameter Configuration

Function parameters are automatically mapped to CLI flags. You can customize them using comments. `go-subcommand` looks for configuration in three places, in this priority order (highest to lowest):

1.  **`Flags:` Block:** A dedicated block in the main function documentation.
2.  **Inline Comments:** Comments on the same line as the parameter definition.
3.  **Preceding Comments:** Comments on the line immediately before the parameter.

#### The `Flags:` Block

This is the cleanest way to define multiple parameters. It must be an indented block following a line containing just `Flags:`.

```go
// MyFunc is a subcommand `app cmd`
//
// Flags:
//
//   username: --username -u (default: "guest") The user to greet
//   count:    --count -c    (default: 1)       Number of times
func MyFunc(username string, count int) { ... }
```

#### Syntax Reference

Inside a `Flags:` block or inline/preceding comments, you can use the following syntax tokens to configure a parameter:

*   **Flags:** `-f`, `--flag`. One or more flag aliases.
*   **Default Value:** `default: value` or `default: "value"`.
*   **Required:** `(required)`. Marks the flag as required.
*   **Global:** `(global)`. Marks the flag as persistent (available to subcommands).
*   **Inherited:** `(from parent)`. Indicates the parameter is inherited from a parent command.
*   **Custom Parser:** `(parser: FunctionName)` or `(parser: "pkg".Function)`.
*   **Generator:** `(generator: FunctionName)` or `(generator: "pkg".Function)`. Used for dependency injection or complex initialization.
*   **Positional Argument:** `@N` (e.g., `@1`, `@2`). Maps the Nth positional argument (1-based) to this parameter.
*   **Variadic Arguments:** `min...max` (e.g., `1...3`) or `...`. Maps remaining arguments to a slice.
*   **Description:** Any remaining text is treated as the parameter description.

### Supported Types

The following Go types are supported for function parameters:

*   `string`: (Default)
*   `int`: Parsed as an integer.
*   `bool`: Parsed as a boolean flag (no value required, e.g., `--verbose`).
*   `time.Duration`: Parsed using `time.ParseDuration` (e.g., `10s`, `1h`).
*   `error`: (Return value only) Your function can return an `error`, which will be propagated to the CLI exit code.

## Advanced Usage

### Positional Arguments

To accept positional arguments instead of flags, use the `@N` syntax.

```go
// Greet is a subcommand `app greet`
//
// Flags:
//
//   name: @1 The name to greet
func Greet(name string) {
    fmt.Printf("Hello, %s!\n", name)
}
```
Usage: `app greet John`

### Variadic Arguments

To accept a variable number of arguments, use a slice parameter and mark it with `...`.

```go
// ProcessFiles is a subcommand `app process`
//
// Flags:
//
//   files: ... List of files to process
func ProcessFiles(files ...string) {
    for _, file := range files {
        fmt.Println("Processing", file)
    }
}
```
Usage: `app process file1.txt file2.txt file3.txt`

### Custom Flags

You can define custom short and long flags.

```go
// Serve is a subcommand `app serve`
//
// Flags:
//
//   port: -p --port (default: 8080) Port to listen on
func Serve(port int) { ... }
```

### Nesting Commands

Nesting is implicit based on the command path string.

```go
// Root command: `app`
// Child: `app users`
// Grandchild: `app users create`

// CreateUser is a subcommand `app users create`
func CreateUser(...) { ... }

// ListUsers is a subcommand `app users list`
func ListUsers(...) { ... }
```

### Global Flags (Inheritance)

Subcommands can inherit flags from their parent command. This allows the child command to access the parent's state (like global verbosity or configuration).

In the child command, you can either:
1.  **Implicitly inherit:** If the child command doesn't declare the parameter, it can still access the parent's state via the generated struct (if custom logic is used).
2.  **Explicitly inherit:** Declare the parameter in the child command and mark it with `(from parent)` to tell `gosubc` to map it to the parent's flag instead of creating a new one.

```go
// Parent is a subcommand `app parent`
// Flags:
//
//   verbose: -v --verbose
func Parent(verbose bool) { ... }

// Child is a subcommand `app parent child`
// Flags:
//
//   verbose: (from parent)
func Child(verbose bool) {
    // verbose here refers to the same variable as Parent's verbose
}
```

### Man Page Generation

To generate man pages, pass the `--man-dir` flag to `gosubc`.

```bash
gosubc generate --man-dir ./man
```

This will generate standard Unix man pages in the specified directory, using the descriptions and extended help text from your comments.

## CLI Reference

### `gosubc generate`

Generates the Go code for your CLI.

*   `--dir <path>`: Root directory containing `go.mod`. Defaults to current directory.
*   `--man-dir <path>`: Directory to write man pages to.

### `gosubc list`

Lists all detected subcommands.

*   `--dir <path>`: Root directory.

### `gosubc validate`

Validates subcommand definitions for errors or conflicts.

*   `--dir <path>`: Root directory.

### `gosubc goreleaser`

Generates release configuration.

*   `--dir <path>`: Root directory containing `go.mod`. Defaults to current directory.
*   `--go-releaser-github-workflow`: Generate GitHub Action workflow for GoReleaser.

## Contributing

Contributions are welcome! If you find a bug or have a feature request, please open an issue on our [GitHub repository](https://github.com/arran4/go-subcommand).

## License

This project is licensed under the **BSD 3-Clause License**. See the [LICENSE](LICENSE) file for details.
