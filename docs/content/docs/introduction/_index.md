---
title: "Introduction"
weight: 1
---

# Introduction

**Go Subcommand** is a powerful, dependency-less library and tool for generating subcommand-based command-line interfaces (CLIs) in Go. It leverages **convention over configuration** by parsing your Go code comments to define the CLI structure, flags, and parameters.

## Why Go Subcommand?

*   **Zero Dependencies**: The generated code depends only on the Go standard library.
*   **Intuitive**: Define your CLI structure naturally using Go functions and comments.
*   **Automated**: Focus on your logic; let `gosubc` handle the boilerplate.
*   **Type Safe**: Automatically handles type conversions for `int`, `bool`, `time.Duration`, and more.
*   **Feature Rich**: Supports nested subcommands, aliases, default values, variadic arguments, and man page generation.

## Key Features

*   **Subcommand Trees**: Create deep hierarchies of commands (e.g., `app users create`).
*   **Argument Mapping**: Maps function parameters to CLI flags and positional arguments.
*   **Flexible Comments**: Define flags and descriptions directly in your Go source code.
*   **Generators**:
    *   **CLI Code**: Fully functional CLI entry points.
    *   **Man Pages**: Unix manual pages for your tool.
    *   **Usage**: Automatic usage text generation.

## Status

**Pre-v1**. The API and generated code structure may change. However, it is stable enough for use in production tools, including its own CLI.
