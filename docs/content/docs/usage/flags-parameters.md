---
title: "Flags & Parameters"
weight: 3
---

# Flags & Parameters

`go-subcommand` automatically maps your function parameters to command-line flags and arguments.

## Automatic Mapping

Function parameters are converted to flags by default.

```go
func Server(port int, verbose bool)
```

Generates flags:
*   `--port <int>`
*   `--verbose` (boolean flag)

Flag names are automatically converted to kebab-case (e.g., `maxRetries` -> `--max-retries`).

## Customizing Flags

You can customize flags using comments. There are three ways to define them, in order of priority:

1.  **Flags Block**: A `Flags:` block in the function documentation.
2.  **Inline Comment**: Comment on the same line as the parameter.
3.  **Preceding Comment**: Comment on the line before the parameter.

### 1. Flags Block (Highest Priority)

```go
// MyCmd is a subcommand `app cmd`
// Flags:
//   port: default: 8080 description: Port to listen on
func MyCmd(port int) {}
```

### 2. Inline Comment

```go
func MyCmd(
    port int, // default: 8080 description: Port to listen on
) {}
```

### 3. Preceding Comment

```go
// default: 8080 description: Port to listen on
port int
```

## Syntax Reference

The parameter definition syntax supports several attributes:

*   **Flag Alias**: `flag: -p` or just `-p` to define aliases.
*   **Description**: `description: text` or just text (if unambiguous).
*   **Default Value**: `default: value`.
*   **Positional Argument**: `@N` (e.g., `@1`) marks the parameter as a positional argument at index N (1-based).
*   **Variadic**: `...` or `min...max` (e.g., `1...` or `1...3`) for variadic arguments.

### Examples

**Positional Argument**

```go
// Copy is a subcommand `cp`
func Copy(
    src string, // @1 source file
    dst string, // @2 destination file
)
```

**Custom Flag Name**

```go
func Serve(
    addr string, // flag: --address -a description: Bind address
)
```

**Required Variadic Arguments**

```go
func Echo(
    args []string, // @1 ... description: Strings to echo
)
```

## Supported Types

*   `int`, `int64`, etc.
*   `bool`
*   `string`
*   `time.Duration` (e.g., `10s`, `1m`)
*   `float64`
*   Structs (via custom selector handling, advanced)

## Implicit Parameters

You can also define parameters solely in comments without them strictly matching the function signature's parameter names if using advanced usage, but typically they map 1:1 to function arguments.
