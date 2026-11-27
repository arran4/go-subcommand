---
title: "No-command policy"
weight: 13
---

**Since version:** v0.0.32.

Use functionless root metadata to require a selected command:

```go
// App is a command `app` -- Run jobs.
// No command: error
package commands

import "fmt"

// Run is a subcommand `app run` -- Run a job.
func Run() { fmt.Println("run") }
```

Or place the same directive in native configuration:

~~~text
format-version: 1
# Commands
```command: app
Description: Run jobs.
No command: error
```
~~~

| Invocation | stdout | stderr | Status |
| --- | --- | --- | --- |
| `app` | empty | `Error: no command provided` | 1 |
| `app --` | empty | `Error: no command provided` | 1 |
| `app run` | `run` | empty | 0 |
| `app help`, `app usage`, `app --help` | empty | normal help | 0 |
| `app version` | normal version | empty | 0 |
| `app missing` | empty | usage and unknown-command error | 1 |
| `app -- run` | empty | usage and unknown-command error | 1 |

The directive is an execution policy, independent of executable routing. When
omitted, bare invocation preserves existing help and status 0. Only `error` is
accepted. Empty/unknown values, duplicate declarations and policies on children
are generation errors with source filename and line diagnostics. A real root
action or any default route, including a help route or a child marked default,
conflicts with the policy. Put it in root metadata rather than an action comment.

The dispatcher returns `*cmd.UsageError` unchanged, with kind
`cmd.UsageNoCommand`, canonical root path, empty flag and nil cause. Both direct
and wrapped errors support `errors.Is(err, cmd.ErrUsage)` and `errors.As`.
Unknown commands and parser failures retain their distinct kinds. Generated
process helpers render the error and choose default status 1 separately; wrapped
explicit `cmd.ErrExitCode` overrides remain authoritative, including status 0.
Only generated main calls `os.Exit`.

Explicit help, usage and version do not acquire business providers. The policy
does not change selected actions, nested aliases, inherited flags, positional
binding or parser-specific grammar. `--` stops routing as before; without any
following argument it leaves no command selected. Functionless root flags require typed source declarations. Scoped `Requires:` guards and providers are supported separately; see [Scoped lifecycle](../scoped-lifecycle/).

No custom main, dummy action or global callbacks are needed. See [Functionless metadata](../command-metadata/) for defining the root and [Generated errors](../generated-errors/) for error classification.
