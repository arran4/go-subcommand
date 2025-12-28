# Go Subcommand (`gosubc`)

`go-subcommand` is a tool that generates a dependency-less CLI subcommand system for Go applications. It uses code comments to define the CLI structure, allowing you to keep your command definitions right next to the code that implements them.

**Status:** Pre-v1. The API and generated code structure may change.

## Features

- **Zero Dependencies:** The generated code relies only on the Go standard library.
- **Declarative:** Define your CLI commands using Go comments.
- **Go Generate Friendly:** Integrate easily with `go generate` for automatic updates.
- **Man Page Generation:** Automatically generate man pages for your CLI.

## Installation

Install the `gosubc` tool using `go install`:

```bash
go install github.com/arran4/go-subcommand/cmd/gosubc@latest
```

## Usage

### 1. Define Commands

Add a comment to any exported function in your package that you want to be a subcommand. The format is:

```go
// FunctionName is a subcommand `root-command [sub-command]...`
```

Example:

```go
package main

import "fmt"

// Hello is a subcommand `myapp hello`
func Hello() {
	fmt.Println("Hello world")
}

// AddUser is a subcommand `myapp users add`
func AddUser(name string, age int) {
	fmt.Printf("Adding user %s, age %d\n", name, age)
}
```

Function arguments are automatically parsed as command-line flags. Supported types include `string`, `int`, `bool`, etc.

### 2. Configure `go:generate`

Add a `generate.go` file (or add to an existing one) in your project root or `cmd` directory.

**Recommended (Robust):**
This version uses the installed `gosubc` if available, or falls back to `go run` to ensure the tool is available.

```go
//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate || go run github.com/arran4/go-subcommand/cmd/gosubc generate"
```

**Simple:**

```go
//go:generate gosubc generate
```

### 3. Generate Code

Run `go generate` in your project root:

```bash
go generate ./...
```

This will generate a `cmd` directory (or update it) with the necessary entry points and subcommand logic.

### 4. Build and Run

Build your application using the generated main file:

```bash
go build -o myapp ./cmd/myapp
./myapp hello
./myapp users add --name "Alice" --age 30
```

## CLI Reference

The `gosubc` tool has three main commands: `generate`, `list`, and `validate`.

### `generate`

Generates the CLI code based on the found subcommand comments.

```bash
gosubc generate [flags]
```

**Flags:**

- `--dir <path>`: The project root directory containing `go.mod`. Defaults to the current directory.
- `--man-dir <path>`: Directory to generate Unix man pages in. If omitted, man pages are not generated.

### `list`

Lists all identified subcommands in the project.

```bash
gosubc list [flags]
```

**Flags:**

- `--dir <path>`: The project root directory containing `go.mod`. Defaults to the current directory.

### `validate`

Validates the subcommand definitions to ensure there are no conflicts or errors.

```bash
gosubc validate [flags]
```

**Flags:**

- `--dir <path>`: The project root directory containing `go.mod`. Defaults to the current directory.

## Examples

Check the `examples/` directory for working examples:

- `examples/basic1`: A simple example with basic subcommands.
- `examples/complex`: A more complex example showing nested commands and parameters.

## Contributing

Contributions are welcome! Please open an issue to discuss any changes before submitting a pull request.

## License

This project is licensed under the BSD 3-Clause License. See the [LICENSE](LICENSE) file for details.
