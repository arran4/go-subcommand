---
title: "Functionless command metadata"
weight: 12
---

Use a package or standalone comment to describe a root or child without
registering an executable action:

```go
// App is a command `app` -- Manage jobs.
// More application help.
package commands

// Generate is a subcommand `app generate` -- Generate jobs.
func Generate() { fmt.Println("generate") }
```

`app` prints described help on stderr and exits 0. `app generate` prints
`generate` on stdout. `app unknown` returns an unknown-command error and status 1.
`app help`, `app usage`, `app --help`, recursive help and version keep normal
dispatch behaviour and do not acquire business providers.

An implicit root, inferred from `app generate`, has the same no-action behaviour
without explicit root help metadata. A real `is a subcommand` function at `app`
declares an executable root action. A literal `app root` remains a child, including
its normal aliases. The README places source, command trees and representative
invocations for all three root forms side by side.

Backticks are required around Go declaration paths. Each comment group holds one
declaration. The form applies to any canonical command path, not a separate
application metadata language. For compatibility, this spelling on function
comments remains ignored with a warning; real actions use `is a subcommand`.

The model records explicit metadata separately from the action function name.
One metadata declaration per canonical path is allowed across Go and config.
Metadata may coexist with a real source action: nonempty short descriptions
override source short descriptions, long help is appended, and aliases are
combined then validated by existing routing rules. Conflicting CLI-Parser or
Default route directives are errors. Root aliases require a real action; alias
paths cannot be used as canonical metadata targets.

Functionless flags, positionals and providers are currently unsupported and
explicitly rejected. Go types and binding must come from a real typed source,
rather than being guessed from prose. Root flags without a root action remain
follow-up work under #63/#522. No dummy action, global registry, manual root
construction or main-template replacement is required for metadata.

## Native declarations

Both candidate extensions, `.mdc` and `.mdconf`, accept generator settings first,
then typed appendices:

~~~text
format-version: 1

# Commands

```command: app
Description: Manage jobs.
More application help.
```

```command: app config
Description: Inspect configuration.
Aliases: c
```

# Documentation

```markdown: app file=docs/overview.md
```

```text: app config
Additional configuration help.
```

# Man content

```man: app config EXAMPLES file=docs/config-examples.txt
```
~~~

Command blocks normalize to the same commentv1 tokens as Go declarations.
Description, aliases, CLI-Parser and Default route directives are command-scoped.
Empty metadata bodies are allowed. Metadata-only roots may select existing
default routes such as `config show` or `help config show`, reusing ordinary
dispatch. Appending metadata to an independently declared real root does not
remove its action or create a second one.

Documentation targets must already exist as source actions, metadata declarations
or implicit parents. A metadata-only child can serve as a documented topic: bare
invocation prints help, and unknown child names fail. Text and Markdown append
literal help to the target, including indentation and template-like braces.
They are not interpreted as code or recursively included configuration.

Man sections use uppercase names; underscores stand for spaces. Appendices cannot
replace NAME, SYNOPSIS, DESCRIPTION, OPTIONS or SEE_ALSO. Generation writes the
root and child `.1` files under `--man-dir`, including `man/app.1`.

Only one of each top-level section is allowed, and each section needs at least
one typed fenced block. Unknown headings, wrong block kinds, unclosed/mismatched
fences and generator settings after appendices are errors. A longer backtick
fence may contain shorter literal fences in its body. Minimum-version checks,
arbitrary typed template variables and multiple targets remain separate #63 work.

External references are relative to the configuration directory. Markdown accepts
`.md`/`.markdown`; text and man fragments accept `.txt`/`.text`. Quote paths with
spaces. References cannot be absolute or escape the config directory, including
through external symlinks in the OS reader. Missing files, invalid UTF-8/NUL bytes
and combining inline content with a file reference are errors with source
filename/line diagnostics. JSON remains a generator-settings compatibility reader.

Programmatic readers can use `LoadGenerateConfigFS` and
`GenerateOptions.CommandDeclarations` with an in-memory filesystem. Metadata
parsing, validation and rendering precede output cleanup/writes. Invalid
declarations preserve existing generated files even when `--clean` is selected.

The inspectable fixture `templates/testdata/issue516.go.txtar` contains real Go
and native sources, external documentation, representative invocations, help
snapshots and an empty-root generated-source golden. `TestGenerateCommandMetadata`
compiles and runs complete applications on GNU and Go-flag, compares Go/config
output equivalence and checks repeated generation. The CLI integration test also
verifies metadata aliases, complete regeneration and preservation on an invalid
clean request.
