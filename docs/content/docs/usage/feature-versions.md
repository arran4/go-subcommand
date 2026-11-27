---
title: "Feature versions"
weight: 6
---

# Feature versions

Versions refer to gosubc, not the target application's Go compiler. The introduction versions below are based on the published [v0.0.32](https://github.com/arran4/go-subcommand/releases/tag/v0.0.32) and [v0.0.33](https://github.com/arran4/go-subcommand/releases/tag/v0.0.33) changelogs. Older features are not assigned speculative introduction versions.

| Feature | Since version | Implementation |
| --- | --- | --- |
| Native project config | v0.0.32 | [#510](https://github.com/arran4/go-subcommand/pull/510) |
| GNU optional flag values | v0.0.32 | [#511](https://github.com/arran4/go-subcommand/pull/511) |
| Typed root context | v0.0.32 | [#501](https://github.com/arran4/go-subcommand/pull/501) |
| Local provider DI and ordering | v0.0.32 | [#512](https://github.com/arran4/go-subcommand/pull/512) |
| Typed usage errors and generated process boundary | v0.0.32 | [#526](https://github.com/arran4/go-subcommand/pull/526) |
| Functionless command metadata | v0.0.32 | [#527](https://github.com/arran4/go-subcommand/pull/527) |
| No-command policy | v0.0.32 | [#528](https://github.com/arran4/go-subcommand/pull/528) |
| Plus-minus backend | v0.0.32 | [#505](https://github.com/arran4/go-subcommand/pull/505) |
| Leaf and late help fixes | v0.0.33 | [#551](https://github.com/arran4/go-subcommand/pull/551) |
| Explicit optional root positionals | v0.0.33 | [#552](https://github.com/arran4/go-subcommand/pull/552) |
| Constructor-owned framework streams | v0.0.33 | [#554](https://github.com/arran4/go-subcommand/pull/554) |
| Scoped mandatory guards and lazy shared providers | v0.0.33 | [#555](https://github.com/arran4/go-subcommand/pull/555) |
| Unmatched argv (plus-minus only) | v0.0.33 | [#556](https://github.com/arran4/go-subcommand/pull/556) |
| Unified Requires, typed inference and unified cleanup | Unreleased (proposed v0.0.34) | [#567](https://github.com/arran4/go-subcommand/pull/567) |

Update the unreleased row to the actual published tag once available. Direct action-stream injection (#559), mixed typed variadic arguments (#562) and typed finite choices (#565) remain separate work. See [all releases](https://github.com/arran4/go-subcommand/releases).
