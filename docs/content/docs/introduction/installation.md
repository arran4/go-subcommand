---
title: "Installation"
weight: 2
---

# Installation

`gosubc` is a build-time generator, not a runtime library. Install the current published release:

```sh
go install github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34
gosubc version
```

Ensure your Go binary directory (normally `$(go env GOPATH)/bin` unless GOBIN is set) is on PATH.

Alternatively, run a pinned version directly from your module:

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34 generate --emit-go-generate=false
```

Use `@latest` for exploration, or a release tag for repeatable builds.

For required Go versions, including older application targets, see [Compatibility](../../reference/compatibility/). Next: [Getting started](../../usage/getting-started/).
