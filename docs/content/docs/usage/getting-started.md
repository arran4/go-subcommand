---
title: "Getting Started"
weight: 1
---

# Getting started

Use ordinary importable Go packages for application logic. The generator creates a separate executable package under `cmd/<app>`; do not put actions in `package main` if the generated executable must import them.

## Create a module

The following complete POSIX shell example uses the action from [examples/basic1](https://github.com/arran4/go-subcommand/tree/v0.0.33/examples/basic1).

```sh
mkdir basic1
cd basic1
go mod init example.com/basic1
go mod edit -go=1.22
cat > examplecmd1.go <<'GO'
package basic1

import "fmt"

// ExampleCmd1 is a subcommand `basic1 example1`
// Does nothing practical
func ExampleCmd1() {
	fmt.Println("Caught me")
}
GO
```

## Generate and run

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34 generate --emit-go-generate=false
go run ./cmd/basic1 example1
```

Expected stdout:

```text
Caught me
```

```sh
go run ./cmd/basic1 --help
go run ./cmd/basic1 help example1
go build -o basic1 ./cmd/basic1
./basic1 example1
```

## Repeat generation

Save this as `generate.go` in the same module:

```go
package basic1

//go:generate go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34 generate --emit-go-generate=false
```

Run `go generate ./...` whenever action declarations change. **Since version:** v0.0.32 supports suppressing nested generated directives, allowing one canonical pinned generation command. Do not edit generated Go by hand.

Next: [Defining commands](../defining-commands/), [Flags and parameters](../flags-parameters/), [Examples](../examples/).
