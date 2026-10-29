---
title: "Project generation configuration"
weight: 11
---

`gosubc generate --config gosubc.mdc` reads optional generator settings.
Both `.mdc` and `.mdconf` are supported candidates; neither filename is required.
Generated applications do not read this file or depend on go-subcommand at runtime.

```text
format-version: 1
cli-parser: gnu
recursive: true
emit-go-generate: false
paths:
- commands
```

Explicit flags override file values, including `--recursive=false` and
`--prov-version=`. File settings override built-in defaults. Settings belong at
the top, without a `# Configuration` heading. Generator settings are `dir`,
`man-dir`, `parser-name`, `cli-parser`, `paths`, `recursive`,
`force`, `clean`, `replace-templates`, `project-provenance` (alias `project`),
`timestamp`, `emit-go-generate`, `prov-version`, `prov-commit`, and `prov-date`.
Keys ignore case, hyphens and underscores. `version` is an alias for
`format-version`, and one version declaration is required.

Lists accept comma-separated values, JSON string arrays, or Markdown entries
after an empty setting. Strings may use double or single quotes. Boolean values
accept true/false, yes/no and 1/0. Invalid values, duplicate settings (including
conflicting aliases), malformed lists, unknown keys, and unsupported headings
are errors with a filename and line number. Strict JSON with `"version": 1` and
lower-camel-case field names is also supported; duplicate keys and null values
are rejected.

Native files may append `# Commands`, `# Documentation` and `# Man content`
sections with typed fences. See [Functionless command metadata](../command-metadata/)
for source/config equivalence, declarations, external file references, precedence
and examples. The JSON reader remains limited to generator settings.

The wider design remains in [#63](https://github.com/arran4/go-subcommand/issues/63).
Minimum generator versions, typed template variables, multiple targets and setup
questions are deferred. Unsupported keys/headings are rejected rather than
accepted without effect.

Programmatic generation uses `GenerateCLIConfiguredWithOptions` with explicit
`ExplicitFlags` metadata. The compatible `GenerateCLIConfiguredCommand` wrapper
uses file values over its fallback arguments. Neither reads process argv.
Generated commands can request `(flag presence)` on a `map[string]bool` parameter
to receive their parser's per-invocation presence map. GNU and Go-flag backends
normalize aliases to Go parameter names. Presence is independent of value and
does not reparse command-line text.
