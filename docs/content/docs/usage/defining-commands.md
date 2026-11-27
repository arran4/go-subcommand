---
title: "Defining Commands"
weight: 2
---

# Defining commands

Declare a command by putting its path in a comment above an ordinary Go function. Root and child actions live in importable application packages, separate from generated executable code.

Save this complete file as `commands.go` in a fresh Go module:

```go
package commands

import "fmt"

// Root is a subcommand `app` -- Application entry point.
func Root() { fmt.Println("root") }

// Users is a command `app users` -- User commands.

// UserCreate is a subcommand `app users create` -- Create a user.
//
// Flags:
//
//	name: @1 User name.
func UserCreate(name string) { fmt.Println("create", name) }

// UserDelete is a subcommand `app users delete` -- Delete a user.
//
// Flags:
//
//	name: @1 User name.
func UserDelete(name string) { fmt.Println("delete", name) }
```

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34 generate --emit-go-generate=false
go run ./cmd/app users create alice
go run ./cmd/app help users delete
```

The first invocation prints `create alice`. The blank line after the metadata-only Users comment is intentional. Intermediate commands can also be inferred from child paths.

**Since version:** v0.0.32 supports explicit functionless [command metadata](../../advanced/command-metadata/) and [no-command policy](../../advanced/no-command-policy/). Leaf help after positionals was corrected in v0.0.33.

Short descriptions follow the command path; later prose provides extended help. Built-in help, usage and version should not execute business actions. See the [examples catalogue](../examples/).
