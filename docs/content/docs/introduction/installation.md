---
title: "Installation"
weight: 2
---

# Installation

To use **Go Subcommand**, you need to install the `gosubc` CLI tool.

## Prerequisites

*   **Go**: Version 1.18 or later.

## Installing `gosubc`

Run the following command to install the latest version:

```bash
go install github.com/arran4/go-subcommand/cmd/gosubc@latest
```

Verify the installation:

```bash
gosubc --help
```

## Adding to Your Project

You don't need to import `go-subcommand` in your application logic files unless you are using specific helper types. The generated code will reside in a separate package (usually `cmd/appname`).
