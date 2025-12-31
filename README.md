# Go Subcommand

**Go Subcommand** generates subcommand code for command-line interfaces (CLIs) in Go from source code comments. By leveraging specially formatted code comments, it automatically generates a dependency-less subcommand system, allowing you to focus on your application's core logic instead of boilerplate code.

**Status:** Pre-v1. The API and generated code structure may change.

## Key Features

- **Convention over Configuration:** Define your CLI structure with simple, intuitive code comments.
- **Zero Dependencies:** The generated code is self-contained and doesn't require any external libraries.
- **Automatic Code Generation:** `gosubc` parses your Go files and generates a complete, ready-to-use CLI.
- **Easy to Use:** Get started quickly with a simple `go generate` command.
- **Man Page Generation:** Automatically generate Unix man pages for your CLI.

## Installation

To install `gosubc`, use `go install`:

```bash
go install github.com/arran4/go-subcommand/cmd/gosubc@latest
```

## Getting Started

Using `go-subcommand` is as easy as adding a comment to your Go functions.

### 1. Define Your Commands

Create a Go file and define a function that will serve as your command. Add a comment above the function in the format `// MyFunction is a subcommand \`my-app my-command\``.

For example, create a file named `main.go`:

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

Create a file named `generate.go` in the same directory and add one of the following `go:generate` directives.

**Option 1: Simple Command**

This is the easiest option and is recommended for most use cases. It requires `gosubc` to be installed on your system.

```go
package main

//go:generate gosubc generate
```

**Option 2: Conditional Command**

This version is more robust and will use `gosubc` if it's in your `PATH`, otherwise it will fall back to using `go run`. This is useful for projects where contributors may not have `gosubc` installed.

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

#### Man Page Generation (Optional)

You can optionally generate unix man pages by providing the `--man-dir` flag:

```bash
gosubc generate --man-dir path/to/man/pages
```

Or in your `go:generate` directive:

```go
//go:generate gosubc generate --man-dir ./man
```

### 4. Run Your New CLI

You can now run your newly generated CLI:

```bash
go run ./cmd/my-app hello
```

You should see the output:

```
Hello, World!
```

## Advanced Usage

### Subcommands

You can create nested subcommands by extending the command path in the comment:

```go
// PrintUser is a subcommand `my-app users get`
// This command retrieves and prints a user's information.
func PrintUser(username string) {
    fmt.Printf("Fetching user: %s\n", username)
}
```

### Parameters

`go-subcommand` automatically maps function parameters to command-line arguments:

```go
// CreateUser is a subcommand `my-app users create`
// Creates a new user with the given username and email.
func CreateUser(username string, email string) {
    fmt.Printf("Creating user %s with email %s\n", username, email)
}
```

After running `go generate`, you can use the new command like this:

```bash
go run ./cmd/my-app users create --username "JohnDoe" --email "john.doe@example.com"
```

## CLI Reference

The `gosubc` tool provides several commands to help manage your CLI project.

### `generate`

Generates the CLI code based on the found subcommand comments.

```bash
gosubc generate [flags]
```

**Flags:**

- `--dir <path>`: The project root directory containing `go.mod`. Defaults to the current directory.
- `--man-dir <path>`: Directory to generate Unix man pages in. If omitted, man pages are not generated.

### `list`

Lists all identified subcommands in the project. This is useful for debugging or verifying that `gosubc` is correctly parsing your comments.

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

- [`examples/basic1`](examples/basic1): A simple example with basic subcommands.
- [`examples/complex`](examples/complex): A more complex example showing nested commands and parameters.

## Contributing

Contributions are welcome! If you find a bug or have a feature request, please (discuss features first) open an issue on our [GitHub repository](https://github.com/arran4/go-subcommand).

## License

This project is licensed under the **BSD 3-Clause License**. See the [LICENSE](LICENSE) file for details.
