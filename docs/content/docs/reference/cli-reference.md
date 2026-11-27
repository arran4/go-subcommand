---
title: "CLI Reference"
weight: 1
---

# CLI reference

This is a guide to the **generator executable** `gosubc`, not the CLI it generates for your application. Commands and options below are present in v0.0.33. The installed tool's own help (`gosubc help <command>`) is authoritative for the installed version.

## Command overview

| Command | Purpose |
| --- | --- |
| `generate` | Generate the application CLI and optionally man pages |
| `list` | List declared subcommands |
| `scan` | Inspect subcommands and flags |
| `validate` | Validate declarations without generating |
| `syntax` | Print the supported function-comment forms |
| `format` | Format subcommand documentation; use `--inplace` to modify files |
| `format-source-comments` | Format source comments in Go style |
| `template layout` | Inspect built-in template structure |
| `template export` | Export templates to a directory or txtar |
| `goreleaser` | Generate GoReleaser configuration and optional workflows |
| `skill install/list/inspect/update/remove` | Manage agent skills; see [Agent Skills](../../usage/skills/) |
| `help`, `usage`, `version` | Built-in help, usage and version |

## Generate

Run from your Go module:

```sh
gosubc validate
gosubc generate --man-dir man --emit-go-generate=false
```

| Option | Meaning / default |
| --- | --- |
| `--dir` | Module root, default `.` |
| `--man-dir` | Optional man-page output directory |
| `--parser-name` | Source parser, default `commentv1` |
| `--cli-parser` | Generated runtime dialect, default `gnu`; also `go-flag`, `plus-minus` |
| `--path` | Repeatable search paths relative to the module |
| `--recursive` | Recursively scan, default true |
| `--clean` | Remove generator-owned files before regeneration |
| `--force` | Allow overwriting files not owned by gosubc; use with care |
| `--replace-template` | Repeatable template alias/file, directory or txtar overlay |
| `--project-provenance` / `--project` | Include target Git metadata, default true |
| `--timestamp` | Include generation timestamp, default true |
| `--emit-go-generate` | Emit nested generation directive, default true |
| `--regenerate-command` | Record the project's canonical regeneration command in generated AGENTS.md |
| `--prov-version`, `--prov-commit`, `--prov-date` | Override provenance |
| `--config` | Optional versioned project configuration |

**Since version:** v0.0.32 for native project configuration. See [configuration](../../advanced/project-configuration/) for precedence. Use `--timestamp=false --project-provenance=false` when verifying deterministic generated output.

## Inspect and export templates

```sh
gosubc template layout
gosubc template export --output templates
gosubc template export --as-txtar --output templates.txtar
```

Export to an unused destination. Use `--force` only when deliberately replacing previously exported files. Prefer source declarations and generator options over maintaining local template copies; only override templates when necessary.

## Discover options

```sh
gosubc help generate
gosubc help template export
gosubc scan --dir .
gosubc list --dir .
gosubc validate --dir .
gosubc syntax
```

See [feature versions](../../usage/feature-versions/) before using capabilities described on current main.
