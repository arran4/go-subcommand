---
title: "Compatibility"
weight: 2
---

# Go and release compatibility

| gosubc release | Minimum Go for the generator's module | Supported generated Go floor |
| --- | --- | --- |
| [v0.0.32](https://github.com/arran4/go-subcommand/releases/tag/v0.0.32) | 1.26.0 | 1.22 |
| [v0.0.33](https://github.com/arran4/go-subcommand/releases/tag/v0.0.33) | 1.26.0 | 1.22 |
| [v0.0.34](https://github.com/arran4/go-subcommand/releases/tag/v0.0.34) | 1.26.0 | 1.22 |

The generator column is the Go directive of each tagged module. Go 1.22 is the supported generated-code floor, not a claim that every feature and every compiler combination has been tested. Representative generated applications in these release series are compiled and executed with Go 1.22.12. The generator also has older source-compatibility branches, but targets below Go 1.22 are outside the maintained floor.

## Keep an older application compiler

Run the generator independently rather than requiring it in the application module solely to pin tooling:

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34 generate --emit-go-generate=false
go test ./...
```

Use Go 1.26 or a compatible newer toolchain for the generator invocation; for the second command use your application's selected Go toolchain. Verify the minimum explicitly, for example with Go 1.22 installed and `GOTOOLCHAIN=local` so it cannot silently switch compilers.

Generated code does not import gosubc. A build-tagged tools import or a generator requirement in the same `go.mod` can still raise that module's minimum Go version; use a separate tools module or an independent pinned generator invocation if necessary.

See [Feature versions](../../usage/feature-versions/) for the release that introduced particular features.
