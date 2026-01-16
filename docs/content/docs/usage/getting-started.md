---
title: "Getting Started"
weight: 1
---

# Getting Started

Let's build a simple CLI application named `greet`.

## 1. Create a Go Module

```bash
mkdir greet
cd greet
go mod init example.com/greet
```

## 2. Define a Command

Create `main.go` and define your command function.

```go
package main

import "fmt"

// Hello is a subcommand `greet hello`
// Prints a greeting.
func Hello(name string) {
    fmt.Printf("Hello, %s!\n", name)
}

func main() {
    // This will be replaced by the generated code usage,
    // but for now you can leave it empty or use it to run the generated cmd.
}
```

## 3. Configure Generation

Create a `generate.go` file to instruct `go generate`.

```go
package main

//go:generate gosubc generate
```

## 4. Generate Code

Run the generator:

```bash
go generate
```

This will create a `cmd/greet` directory with the generated code.

## 5. Run Your CLI

You can now run your CLI using `go run`:

```bash
go run ./cmd/greet hello --name World
```

Output:
```
Hello, World!
```
