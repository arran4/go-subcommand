---
title: "Generator design principles"
weight: 10
---

# Generator design principles

> **go-subcommand owns command execution and process policy. User functions supply actions, initialization and dependencies. Templates remain an optional escape hatch.**

The preferred workflow is deliberately small: **write a Go function, document it with a Go comment, generate, and run**. A user should not need to understand the generated entry point, implement dispatch, manually register callbacks, or customize templates to get a usable CLI.

For example, the ordinary current workflow begins with:

```go
// Hello is a subcommand `example hello`.
// Greets the current user.
func Hello() error {
    fmt.Println("Hello")
    return nil
}
```

Generation discovers the function comment and creates the command. Generated files, including `main.go`, are implementation output, **not** part of the consumer extension API.

## Responsibilities and extension hierarchy

| Component | Ownership | Default |
| --- | --- | --- |
| Go function + comment | Application developer | Normal command action, parameters and documentation |
| Generated parser and dispatcher | go-subcommand | Resolve paths, aliases, flags, positionals, help and usage |
| Generated process entry | go-subcommand | Build root, execute, render errors and map exit status |
| Annotated command providers/initializers | Application developer, when required | None |
| Annotated startup function | Application developer, exceptionally | None |
| Scoped startup or output template override | Application developer, last resort | Built-in generated template |

**Choose the narrowest necessary customization.** Do not turn an exceptional consumer workaround into a mandatory root-constructor parameter, global policy registry, importable dispatcher, or full main-template fork. Generated applications must not require go-subcommand as a runtime dependency.

## Execution lifecycle

Normal invocation resolves the selected executable route and binds its arguments before business initialization. [#522 / PR #555](https://github.com/arran4/go-subcommand/pull/555) merged two lifecycle capabilities: mandatory selected-path error-only `Initializer` guards and invocation-scoped, demand-only typed `Shared provider` dependencies. See the [scoped lifecycle guide](../scoped-lifecycle/) for working source and native-config examples.

An unrelated sibling does not acquire an unused provider. Built-in help, usage and version should not run business initializers. Acquired `io.Closer` resources are released using the invocation's cleanup policy. Keep consumer packages independent of generated `package main` code.

[#561 / PR #567](https://github.com/arran4/go-subcommand/pull/567) implements the canonical `Requires:` directive, automatic unique-type binding, and unified dependency ordering. Existing `Initializer:`, `Shared provider:` and `Provider:` declarations remain supported. Existing parameter `generator:` dependencies and root context are documented separately in #403 and #483.

## Startup customization: exceptional, not a second flag parser

Most applications need **no startup hook**. Normal flags and command parameters belong in function comments and the generated command parser.

When process-level compatibility truly cannot be expressed through the normal command model, prefer an annotated Go startup function. Support the same hook selection through project configuration or an explicit generation-time flag, with consistent validation and no duplicated implementation. A startup function may receive raw arguments before normal dispatch, but should not ordinarily parse normal CLI flags.

Also permit a **small, overridable startup adapter template** through the existing template overlay mechanism. Avoid replacing `main.go` entirely. Generated main should stay tiny and internal, delegate to a directly testable runner, and be the only layer that calls `os.Exit`.

The startup/dispatcher redesign is tracked in [#490](https://github.com/arran4/go-subcommand/issues/490) / [PR #502](https://github.com/arran4/go-subcommand/pull/502). Separately importable dispatch remains an optional capability only when an actual package boundary requires it. Do not require it for ordinary applications.

## Process policy is declarative before imperative

Preserve backward-compatible default root dispatch and generated help/usage. An application may explicitly declare `No command: error` to treat bare invocation as a typed usage error; this must not be confused with an executable default route or an ordinary root action. It is tracked in [#524](https://github.com/arran4/go-subcommand/issues/524).

Prefer typed per-instance parser/usage errors with stable sentinels over matching message strings. An explicit `ErrExitCode` remains an override; detect it through `errors.As` even through wrapping. See [#523](https://github.com/arran4/go-subcommand/issues/523).

Build/version metadata should default to injected build values and be overridable through a typed generator/provider, rather than forcing replacement of the process entry template. The broader process policy redesign is in [#479](https://github.com/arran4/go-subcommand/issues/479) / [PR #503](https://github.com/arran4/go-subcommand/pull/503).

## Project configuration and documentation

A future native configuration file should primarily hold generator options and arbitrary typed template variables, with optional command-tree metadata, documentation and man-page appendices. It should **not** invent a second application-root concept or register executable actions without Go functions.

The configuration filename and extension are **not prescribed**. Candidates include `gosubc.mdconf` and `gosubc.mdc`; retain extension-based format dispatch for future migration. Generator settings belong at the top **without** a redundant `# Configuration` heading; Markdown headings denote sections, not comments. [Project generation configuration](../project-configuration/) supports both extensions. [Functionless command metadata](../command-metadata/) adds `# Commands`, `# Documentation` and `# Man content` appendices, reusing normalized commentv1 tokens for root and child metadata. Arbitrary typed variables, minimum-version checks and multiple targets remain **under design**; unsupported sections are rejected. See [#63](https://github.com/arran4/go-subcommand/issues/63) and [#516](https://github.com/arran4/go-subcommand/issues/516).

## Consumer acceptance cases

- **A4SH** ([arran4/shell#56](https://github.com/arran4/shell/issues/56)): first prove that normal command declarations and the smallest existing process compatibility boundary can work **without** startup injection. Only use the optional hook or importable dispatcher if executable `-c`/stdio/signal/argv/interactive tests prove they are required.
- **gogenmux** ([#479](https://github.com/arran4/go-subcommand/issues/479)): eliminate its full custom main template while preserving typed errors, exit categories, version/build output and the no-command policy.

## Reviewability and documentation

Each substantial feature needs inspectable source-to-generated output scenarios and compiled-executable tests, ideally via existing txtar infrastructure ([#521](https://github.com/arran4/go-subcommand/issues/521)). Include source comments, generated code, stdout/stderr/status, errors and deterministic regeneration.

Documentation should lead with the ordinary **function + comment + generate** path, followed by copy/paste recipes for specific advanced problems. Generator internals and template APIs belong in advanced documentation, not the getting-started flow.
