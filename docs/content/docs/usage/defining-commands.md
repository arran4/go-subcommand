---
title: "Defining Commands"
weight: 2
---

# Defining Commands

Commands are defined by adding specific comments to your Go functions.

## Syntax

```go
// FunctionName is a subcommand `root-cmd sub-cmd...`
// Description text...
```

*   **FunctionName**: The name of the Go function to execute.
*   **root-cmd**: The name of your root application (e.g., `git`, `kubectl`).
*   **sub-cmd...**: A space-separated list of subcommands (e.g., `remote add`).

## Description

The text following the command definition or on subsequent lines is used as the command description in usage text.

```go
// Push is a subcommand `git push`
// Update remote refs along with associated objects.
func Push() { ... }
```

## Extended Help

Any comments that are not part of the command definition or parameter documentation are treated as **Extended Help**. This text is shown when the user requests help for a specific command (e.g., via `man` pages or detailed help output).

## Nested Commands

You can create deep hierarchies.

```go
// UserCreate is a subcommand `app users create`
func UserCreate() {}

// UserDelete is a subcommand `app users delete`
func UserDelete() {}
```

Intermediate commands (e.g., `users` in `app users create`) do not need to be explicitly defined if they don't have specific logic. `gosubc` will create "synthetic" commands for them. However, if you want to define behavior for the parent command, you can:

```go
// Users is a subcommand `app users`
func Users() { fmt.Println("Manage users") }
```
