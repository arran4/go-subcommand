---
title: "Installation"
weight: 2
---

# Installation

`gosubc` is a build-time generator, not a runtime library. The published v0.0.33 generator requires Go 1.26.0 ([go.mod](https://github.com/arran4/go-subcommand/blob/v0.0.33/go.mod)). Its generated application can target older Go versions; recent runtime fixtures are actually compiled with Go 1.22.12. Do not confuse the generator's toolchain with the generated application's toolchain. Test your chosen target compiler.

## Install a released version

```sh
go install github.com/arran4/go-subcommand/cmd/gosubc@v0.0.33
gosubc version
```

Ensure your Go binary directory (normally `$(go env GOPATH)/bin` unless GOBIN is set) is on PATH. Use `@latest` for exploration, or a pinned tag for reproducibility.

Without a separate installation, run the generator from your module directory:

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.33 generate --emit-go-generate=false
```

See [Getting started](../../usage/getting-started/) and [Feature versions](../../usage/feature-versions/).
