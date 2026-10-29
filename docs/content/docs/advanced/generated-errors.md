---
title: "Generated errors and process boundary"
weight: 12
---

Generated `cmd` packages expose `ErrUsage` and `*UsageError`. These are generated
application types; applications do not import go-subcommand at runtime.

```go
var usage *cmd.UsageError
if errors.Is(err, cmd.ErrUsage) && errors.As(err, &usage) {
    fmt.Printf("%s: kind=%s flag=%s\n", usage.CommandPath, usage.Kind, usage.Flag)
}
```

The stable kinds are `UsageUnknownCommand`, `UsageUnknownFlag`,
`UsageMissingFlagValue`, `UsageInvalidValue`, `UsageInvalidArguments`, and
`UsageMissingRequiredFlag`. The error holds a canonical command path, a flag name
without leading dashes where applicable, an optional underlying cause, and a
presentation message. Classification never depends on matching that message.
Causes remain available through `errors.Is` and `errors.As`. Application-provided
usage errors and explicit exit errors retain their wrappers when returned from
dispatch.

Generated main calls a testable runner in `process.go`. Its error renderer and
status selector are separate helpers; only main calls `os.Exit`. Explicit
`ErrExitCode` overrides take precedence through any ordinary wrapping, followed
by category defaults and finally the unclassified fallback. Usage errors and
unclassified failures retain status 1. Success retains status 0.

Codes 0, 1 and custom nonzero codes are all explicit overrides. A nil inner cause
prints nothing, even when the exit error is wrapped. A nonnil inner cause prints
`Error: <cause>` to stderr, including for code 0. Explicit exit status also takes
precedence over help sentinel causes. Without an explicit exit override,
`ErrPrintHelp` and `ErrHelp` retain their normal behaviour.

GNU parsing and the Go-flag adapter attach types where errors occur. The latter
uses the standard flag registry, `flag.Value` conversion and `FlagSet.Set` to
retain grammar, conversion and visitation semantics while avoiding error-string
classification and duplicate error output. I/O acquisition failures remain
ordinary errors rather than becoming usage failures merely because they occurred
during argument binding.

`templates/testdata/issue523.go.txtar` bundles real Go annotations, the generated
error contract, and representative stdout/stderr/status expectations.
`TestGenerateTypedErrors` generates standalone applications in memory, verifies
repeatability, compiles their complete source, checks error identities and
causes, compares Go-flag behaviour with the standard library, and executes the
recorded invocations. Help cases verify that business providers do not run.

This feature enables gogenmux to replace parser-message scanning. The
[no-command policy](../no-command-policy/) also supplies typed bare-invocation errors.
Complete custom-main removal still needs version/build metadata and version output,
propagation of `builtBy`, and its
validation=2/internal=3 default classification and rendering contract under
[#479](https://github.com/arran4/go-subcommand/issues/479). Explicit exit errors
can already carry those statuses, but arbitrary application error types are not
automatically assigned a new default. No mutable global process-policy table is
introduced.
