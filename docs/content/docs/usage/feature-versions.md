---
title: "Feature versions"
weight: 6
---

# Feature versions

Versions refer to gosubc, not the target application's Go compiler. The introduction versions below are based on the published [v0.0.32](https://github.com/arran4/go-subcommand/releases/tag/v0.0.32), [v0.0.33](https://github.com/arran4/go-subcommand/releases/tag/v0.0.33) and [v0.0.34](https://github.com/arran4/go-subcommand/releases/tag/v0.0.34) changelogs. Older features are not assigned speculative introduction versions.

| Feature | Since version | Guide |
| --- | --- | --- |
| Native project config | v0.0.32 | [Guide](../../advanced/project-configuration/) |
| GNU optional flag values | v0.0.32 | [Guide](../recipes/#optional-flag-values) |
| Typed root context | v0.0.32 | [Guide](../recipes/#constructor-context) |
| Local provider DI and ordering | v0.0.32 | [Guide](../../advanced/scoped-lifecycle/) |
| Typed usage errors and generated process boundary | v0.0.32 | [Guide](../../advanced/generated-errors/) |
| Functionless command metadata | v0.0.32 | [Guide](../../advanced/command-metadata/) |
| No-command policy | v0.0.32 | [Guide](../../advanced/no-command-policy/) |
| Plus-minus backend | v0.0.32 | [Guide](../../advanced/plus-minus-parser/) |
| Leaf and late help fixes | v0.0.33 | [Guide](../defining-commands/) |
| Explicit optional root positionals | v0.0.33 | [Guide](../recipes/#optional-positionals) |
| Constructor-owned framework streams | v0.0.33 | [Guide](../recipes/#invocation-streams) |
| Scoped mandatory guards and lazy shared providers | v0.0.33 | [Guide](../../advanced/scoped-lifecycle/) |
| Unmatched argv (plus-minus only) | v0.0.33 | [Guide](../../advanced/plus-minus-parser/#opt-in-unmatched-argv-filter-plus-minus-only) |
| Unified Requires, typed inference and unified cleanup | v0.0.34 | [Guide](../../advanced/scoped-lifecycle/) |

See [Compatibility](../../reference/compatibility/) for Go compiler requirements and [releases](https://github.com/arran4/go-subcommand/releases) for release changes.
