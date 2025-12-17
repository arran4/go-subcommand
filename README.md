# Go Subcommand

**Go Subcommand** generates subcommand code for command-line interfaces (CLIs) in Go from source code comments. By leveraging specially formatted code comments, it automatically generates a dependency-less subcommand system, allowing you to focus on your application's core logic instead of boilerplate code.

**Note:** API still under development. 

## Key Features

- **Convention over Configuration:** Define your CLI structure with simple, intuitive code comments.
- **Zero Dependencies:** The generated code is self-contained and doesn't require any external libraries.
- **Automatic Code Generation:** `gosubc` parses your Go files and generates a complete, ready-to-use CLI.
- **Easy to Use:** Get started quickly with a simple `go generate` command.

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

## Contributing

Contributions are welcome! If you find a bug or have a feature request, please open an issue on our [GitHub repository](https://github.com/arran4/go-subcommand).

## License

This project is licensed under the **MIT License**. See the [LICENSE](LICENSE) file for details.
