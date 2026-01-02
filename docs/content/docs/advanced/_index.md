---
title: "Advanced"
weight: 5
---

# Advanced Topics

## Man Page Generation

`go-subcommand` can generate standard Unix man pages.

```bash
gosubc generate --man-dir ./man
```

This creates a `man1` directory inside `./man` containing gzipped man pages (e.g., `my-app.1.gz`, `my-app-subcmd.1.gz`).

The content is derived from:
*   The command description.
*   The extended help text in comments.
*   Flag descriptions and defaults.

## Architecture

The generator works by:
1.  **Parsing**: It scans `.go` files in your module (using `go/parser` and `go/ast`) to find functions with the magic `// ... is a subcommand ...` comment.
2.  **Modeling**: It builds a tree of commands, handling implicit parent commands and resolving parameter details.
3.  **Generating**: It uses Go templates (embedded in the tool) to generate:
    *   `root.go`: The entry point and root command logic.
    *   `cmd.go`: Helper interfaces and shared logic.
    *   Subcommand files: `subcommand_xyz.go` for each command node.
    *   Usage templates: Text files for usage output.

## Runtime Logic

The generated code uses a lightweight `Command` interface.
*   **Flags**: It uses the standard `flag` package.
*   **Dispatch**: Commands are organized in a map/tree structure. `Execute` methods dispatch to subcommands or run the bound function.
*   **Dependency-Free**: The output does not import `github.com/arran4/go-subcommand` at runtime.

## Conditional Generation

To ensure your project builds even if a contributor doesn't have `gosubc` installed, use this `go:generate` directive:

```go
//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate || go run github.com/arran4/go-subcommand/cmd/gosubc generate"
```

This attempts to use the installed binary first, falling back to `go run` (which compiles it from source) if missing.
