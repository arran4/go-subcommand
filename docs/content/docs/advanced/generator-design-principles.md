---
title: "How generation works"
weight: 10
---

# How generation works

The normal workflow is: write an ordinary Go function, annotate it with its command path and parameter documentation, generate, then run the executable. The application defines the behavior; generated code handles the command-line interface.

| Responsibility | Owner |
| --- | --- |
| Business logic, typed action parameters, provider functions | Application source |
| Command paths, aliases, descriptions and parameter declarations | Go comments or supported configuration |
| Flag parsing, routing, help and usage | Generated command tree |
| Root construction, error rendering and exit status | Generated process entry |
| Custom output templates | Optional project configuration |

Keep ordinary action functions in importable packages. The generated executable lives in a separate `cmd/<app>` package; edit your actions rather than modifying generated files. There is no generator runtime dependency.

## Execution

Routing and argument validation precede business initialization. Use [scoped requirements](../scoped-lifecycle/) for selected-path guards and demand-driven typed providers (**since version:** v0.0.34 for `Requires:`). Unused providers do not run; resources implementing `io.Closer` are released in reverse acquisition order. Help and version do not acquire business dependencies.

## Customization

[Functionless metadata](../command-metadata/) describes command roots without dummy actions. [No-command policy](../no-command-policy/) changes bare invocation behavior, and [generated errors](../generated-errors/) describe typed failure and explicit exit-status contracts.

Use [project configuration](../project-configuration/) to select generation options and template overlays. A custom template is an escape hatch, not a prerequisite. See the [CLI reference](../../reference/cli-reference/) for export and layout commands.

The default output is an executable package. A separately importable dispatcher and general pre-dispatch startup hook are not supported in v0.0.34.
