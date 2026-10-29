---
title: "No-command policy"
weight: 13
---

Use functionless root metadata to require a selected command:

```go
// App is a command `app` -- Run jobs.
// No command: error
package commands

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
following argument it leaves no command selected. Functionless root flags and
providers still need future typed binding support under #63/#522 and are rejected
rather than inferred from prose.

`templates/testdata/issue524.go.txtar` contains real Go/native sources, exact
process invocations and help snapshots, and an inspectable empty-root golden.
`TestGenerateNoCommandPolicy` compares equivalent Go/config output, repeats
generation, compiles empty and populated applications with GNU and Go-flag, and
runs direct typed-error/exit-override tests. Existing metadata scenarios also
exercise ordinary root actions and default routes. No custom main, dummy action,
global callbacks or runtime generator dependency is required.
