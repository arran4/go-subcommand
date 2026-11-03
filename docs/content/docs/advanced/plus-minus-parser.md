---
title: "Plus/minus parser: proposed v1"
---

This optional backend remains under review in #464/#505. The grammar below is a
deliberate proposal, not an inference from shell or GNU conventions.

## Audit of the replaced implementation

At PR head 3133b2fa, `+ab` was rewritten to two canonical `--name=false`
assignments. `-ab` was GNU clustering (including value-taking shorts), and `--ab`
was one GNU long option. Aliases, defaults, repeated options, attached values,
help and unknown minus options consequently inherited GNU behavior. Unknown plus
characters used an unrelated untyped diagnostic. `-a=value` on a boolean enabled
`a` then failed at `=`; `+a=value` failed in the preprocessing loop. Single-letter
canonical names gained GNU `--a` recognition solely to support the rewrite.
When a canonical short name collided with another parameter's short alias, both
could be selected by the same toggle instead of reporting an ambiguous definition.

The preprocessing loop scanned the whole argv until a literal `--`, before GNU
could identify a value or stop at a positional/command. Thus `--label +a` could
corrupt the label, `word +a` could rewrite positional text, and `child +x` could
fail at a parent which did not declare x. A plus-looking value or a `--` consumed
as a value was also mistaken for parser syntax. Nested minus options were already
scoped by GNU's first-non-option boundary; plus options were not.

## Architecture decision (before implementation)

A scoped normalizer would need its own value arity, short/long lookup, boundary,
help and error logic before delegating to GNU. Proving equivalence would then
require specifying both grammars and would retain needless translation.

Choose B: a small dedicated tokenizer/parser with per-command option tables and
existing typed value-conversion calls. It stops at the first ordinary token and
returns the original remaining slice. It never scans a child's arguments. The
helper is generated only for trees using plus-minus. GNU and Go-flag templates
and conversion behavior stay unchanged. This does not require a parser framework;
option C (a separate infrastructure project) is unnecessary for this slice.

## Implemented grammar

| Form | Meaning |
| --- | --- |
| `-a`, `+a` | Enable/disable one declared short boolean. |
| `-ab`, `+ab` | Enable/disable each declared short boolean, in order. Validate the entire cluster before changing fields. |
| `--a`, `++a` | One-character long option named `a`, distinct from short namespace. |
| `--ab`, `++ab` | One long option named `ab`; never `a`/`b` clustering. A bare long boolean sets true. |
| `--enabled=false` | Explicit boolean value; standard Go `strconv.ParseBool` values are accepted. Short booleans do not take `=` assignments. |
| `-n 5`, `-n5`, `-n=5` | Value-taking short option. |
| `-abn5`, `-abn=5` | Clustered short toggles with trailing attached value. |
| `--count=3`, `--count 3` | Value-taking long option. Values resembling options require attached assignment (`--label=--verbose`). |
| `++count`, `++count=3` | Explicitly configured nonboolean plus operation (#531). |
| `--` | End parsing and routing in this scope; subsequent tokens are literal positionals. |
| `+`, `-` | Ordinary positional tokens. |
| `-h`, `--help` | Help for the current scope, when reached during option parsing. |
| `help`, `usage`, root `version` | Existing command dispatch; no business provider execution. |

Scalar toggles are last-wins, preserve declared defaults when absent, and record
explicit presence under Go parameter names for both polarities and all aliases.
Pointers use existing conversion/assignment rules. Nonboolean slices use repeated
long values. Inherited options are resolved unambiguously across scopes; child-declared
options shadow parent options without conflict (#534).

Interspersed options appearing after positional arguments are accepted within the same command scope
while preserving positional argument ordering. Subcommand boundaries yield unconsumed arguments
to the child command. Unambiguous parent options can be supplied before or after child commands;
child-declared options shadow parent options without conflict.
Required parent flags are validated across the selected command path: supplying the flag either
before or after child dispatch satisfies the requirement, while omitting it reports typed
`cmd.UsageMissingRequiredFlag` from the selected leaf command without premature mutation or acquisition.

In mixed parser trees (e.g. plus-minus root with GNU or Go-flag child commands), staged parent
option mutations are committed only after child option parsing, required-flag validation, and
positional argument validation all succeed. Any child failure or missing child positional argument
prevents parent commits and prevents parent I/O side effects.

Nonboolean plus operations have no implicit behavior and require explicit consumer configuration (#531):
- `(plus: reset)` or `(plus-bare: reset)`: resets to declared default.
- `(plus-bare: zero)`: sets to zero value.
- `(plus: FuncName)` or `(plus-bare: FuncName)`: calls consumer function `func() (T, error)`.
- `(plus-value: set)`: standard value assignment under `+`/`++` prefix.
- `(plus-value: FuncName)`: calls consumer function `func(string) (T, error)`.

Standalone `++` after `--` is intentionally undefined in this release (#532).
Generated Go code remains strictly compatible with Go 1.22 (#542).

Syntax errors reject assignments to boolean clusters (`-a=false`, `+ab=value`),
Unicode option names, numeric short names, and disabling help (`+h`) or embedding help in a cluster.
Short names are ASCII letters; long names have at least one ASCII letter/digit/underscore/hyphen,
starting with a letter or underscore. Collisions between parameters in the same namespace are generation errors.

Malformed syntax uses UsageInvalidArguments, unknown names UsageUnknownFlag,
missing long values UsageMissingFlagValue and conversion failures UsageInvalidValue.
Errors carry canonical command path, flag and conversion cause through #526's
ErrUsage/UsageError contract. I/O acquisition errors keep their ordinary causes.
Explicit exits retain existing precedence. Help stops when reached; earlier invalid
options still fail, and later argv is not pre-inspected merely to print help.

## Optional-value dependency (#511)

The optional-value work can target main independently: the feature commits are
38b2528/f3ff182, with pointer/slice tests fa1e95e and golden 1535be8. Its native
implementation belongs in GNU, not in this parser's v1 grammar. Restacking should
keep those model/comment/GNU changes, remove inherited #505 backend changes and
plus-minus optional-value tests, and reject selection of unsupported backends.
Retain bare/assigned-empty/attached-short/repeated pointer and slice contracts,
compiled GNU invocations and typed errors on current main. A forward-only merge
of main plus removal of the inherited backend delta can preserve branch history
without a force push; retarget the existing PR after validation. This assessment
does not modify #511 or assume exclusive ownership of its branch.

## Behavioral acceptance cases

The runtime archive records exact stdout, stderr and status for these cases.
Root a defaults true, b false and long ab false; presence keys use parameter names.

| Case | Expected result |
| --- | --- |
| `+ab` | a=false, b=false, ab=false; alpha/beta explicitly present. |
| `-ab` | a=true, b=true, ab=false; alpha/beta explicitly present. |
| `--ab`, `--both` | a=true, b=false, ab=true; only ab present. |
| `+a -a +a` | a=false; last toggle wins. |
| `+a --enable=true` | a=true through a long alias. |
| `--count=0 --label=` | Explicit zero/empty retained with presence. |
| `--label +a` | Label is literal +a; a remains at its default. |
| `--label -- +a` | Label is literal --; +a still disables a. |
| `-- -ab +ab --ab` | All three subsequent tokens are literal positionals. |
| `word +a` | Both tokens are positional; no option rewrite. |
| `+`, `-` | Ordinary positional boundaries. |
| `+az`, `-az` | Typed unknown z; no partial a assignment from that cluster. |
| `++a`, `+ab=value`, `-a=false` | Typed invalid syntax. |
| `+h`, `-ah` | Typed error; help is neither disabled nor clustered. |
| `--a` | Typed unknown long option, even for a canonical short a. |
| missing/bad count, bool, duration or custom value | Typed missing/invalid value with canonical path and conversion cause. |
| `gnu -ab`, `gnu --ab`, `gnu +ab` | Existing GNU cluster, long option, literal plus positional. |
| `legacy -ab`, `legacy --a`, `legacy +a` | Existing Go-flag exact-name behavior and literal plus positional. |
| `gnu pm +c` | Child-only toggle reaches the plus-minus child untouched. |
| `gnu pm deep +d` | Deep command inherits its nearest plus-minus ancestor. |
| `gnu pm old -ab` | A later Go-flag override interprets ab as one exact flag. |
| help, usage, version | Exact existing output/status; no business acquisition. |

Compiled variants cover explicit source selection, generator-default selection
and a GNU root with a plus-minus descendant. Generated-package unit tests check
tokenization, cluster atomicity, slice identity at scope boundaries, conversion,
I/O error identity and direct/wrapped ErrUsage classification. Complete root,
child, helper, usage and manual artifacts are compared with inspectable goldens.
