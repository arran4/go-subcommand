---
title: "Plus/minus parser"
weight: 15
---

# Plus/minus runtime parser

**Since version:** v0.0.32.

Set `--cli-parser plus-minus` or use `CLI-Parser: plus-minus` on a command.
This backend implements a dedicated command-scoped parser with atomic staging and validation (#464):

| Token | Meaning |
| --- | --- |
| `-a`, `+a` | Enable/disable short boolean `a`. |
| `-ab`, `+ab` | Enable/disable short booleans `a` and `b` in a cluster. |
| `--a`, `++a` | One-character long option named `a`, distinct from short namespace. |
| `--ab`, `++ab` | Long options named `ab`, independent of short clusters. |
| `-n 5`, `-n5`, `-n=5` | Value-taking short option. |
| `-abn5`, `-abn=5` | Short cluster with trailing attached value. |
| `--count=3`, `--count 3` | Value-taking long option. |
| `++count`, `++count=3` | Explicitly configured nonboolean plus operation (#531). |
| `--` | Delimiter terminating option scanning and routing; remaining tokens are literal. |

Scalar toggles are last-wins, preserve absent defaults, and track explicit
presence for both polarities and all aliases. A long boolean may take an explicit
Go boolean value such as `--enabled=false`. Short booleans reject `=` assignments.
When values resemble options, attached assignment is required (`--label=--verbose`).
Interspersed options after positional arguments are accepted within the same command scope
while preserving positional argument ordering (#534). Subcommand boundaries yield
unconsumed arguments to the child command. Unambiguous parent options can be supplied
before or after child commands; child-declared options shadow parent options without conflict.
Required parent flags are validated across the selected command path: supplying the flag either
before or after child dispatch satisfies the requirement, while omitting it reports typed
`cmd.UsageMissingRequiredFlag` from the selected leaf command without premature mutation or acquisition.

In mixed parser trees (e.g. plus-minus root with GNU or Go-flag child commands), staged parent
option mutations are committed only after child option parsing, required-flag validation, and
positional argument validation all succeed. Any child failure or missing child positional argument
prevents parent commits and prevents parent I/O side effects.

Nonboolean plus operations have no implicit behavior and require explicit consumer
configuration (#531):
- `(plus: reset)` or `(plus-bare: reset)`: resets to declared default.
- `(plus-bare: zero)`: sets to zero value.
- `(plus: FuncName)` or `(plus-bare: FuncName)`: calls consumer function `func() (T, error)`.
- `(plus-value: set)`: standard value assignment under `+`/`++` prefix.
- `(plus-value: FuncName)`: calls consumer function `func(string) (T, error)`.

Standalone `++` after `--` is intentionally undefined in this release (#532).
Generated Go code remains strictly compatible with Go 1.22 (#542).

`templates/testdata/runtime/issue464.txtar` records source comments, complete
generated artifacts/help and exact compiled invocations, including mixed-backend
subtrees. GNU and Go-flag keep their existing grammars. GNU optional-value flags are a separate implemented feature; they are not supported by this backend.

## Opt-in unmatched argv filter (`plus-minus` only)

**Since version:** v0.0.33.

Commands using the `plus-minus` parser backend can opt in to receiving otherwise-unrecognized command-line arguments (such as signed numeric values `-0.1`, `+0.1`, unknown flags, or arbitrary foreign options) directly in a `[]string` action parameter marked with `(unmatched)` (#550):

```go
package app

import "fmt"

// Volume is a subcommand `app volume`
// CLI-Parser: plus-minus
//
// Flags:
//
//	player: --player Target player.
//	args: (unmatched) Unmatched arguments collected verbatim.
func Volume(player string, args []string) error {
	fmt.Printf("player=%q args=%q\n", player, args)
	return nil
}
```

- **Doc comment syntax variants:** `(unmatched)`, `(unmatched argv)`, and `(unmatched-argv)`.
- **Backend requirement:** Supported exclusively with `CLI-Parser: plus-minus`. Non-`plus-minus` backends (`gnu`, `go-flag`) are rejected at generation time with clear error diagnostics.
- **Model constraints:** The parameter must have Go type `[]string`. It cannot be combined with positional parameters (`@1`), variadic tails (`...string`), `(optional value)`, defaults, providers, constructors, or flag aliases. At most one unmatched parameter is permitted per command.
- **Argv semantics:**
  - Known options, aliases, and recognized short clusters continue to be parsed normally.
  - Unmatched arguments are passed verbatim as `[]string` without splitting clusters or creating synthetic flags.
  - Any short cluster containing an unrecognized character (e.g. `-az` where `z` is unknown) is not partially committed; the whole cluster is preserved in `args`.
  - Once an unmatched argument or unknown token is encountered at a parent command, subcommand routing is frozen: subsequent arguments cannot dispatch child subcommands.
  - Trailing arguments after `--` are collected verbatim into `args`.
  - The collector parameter is internal to the action and never appears in usage, `--help`, or man pages.



Save the example as `app.go` in a fresh Go module, then generate and run:

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.33 generate --emit-go-generate=false
go run ./cmd/app volume --player=vlc -0.1
```

Expected stdout: `player="vlc" args=["-0.1"]`. The argument is preserved verbatim. See the [compiled parser fixture](https://github.com/arran4/go-subcommand/blob/main/templates/testdata/runtime/issue464.txtar) and [unmatched fixture](https://github.com/arran4/go-subcommand/blob/main/templates/testdata/runtime/issue550_unmatched.txtar) for real annotated input, generated output and invocation contracts. Historical design is preserved in [#464](https://github.com/arran4/go-subcommand/issues/464) and [#505](https://github.com/arran4/go-subcommand/pull/505).
