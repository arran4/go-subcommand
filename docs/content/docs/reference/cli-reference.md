---
title: "CLI Reference"
weight: 1
---

# CLI Reference

The `gosubc` tool has the following commands:

## `generate`

Generates the Go code for your CLI.

```bash
gosubc generate [flags]
```

**Flags:**

*   `--dir <path>`: The project root directory containing `go.mod`. Defaults to `.`.
*   `--man-dir <path>`: Directory to generate Unix man pages in. If omitted, no man pages are generated.

## `list`

Lists all identified subcommands in the project. Useful for debugging parsing.

```bash
gosubc list [flags]
```

**Flags:**

*   `--dir <path>`: The project root directory.

## `validate`

Validates subcommand definitions for conflicts or errors.

```bash
gosubc validate [flags]
```

**Flags:**

*   `--dir <path>`: The project root directory.
