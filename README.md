# Go Subcommand

**Go Subcommand** (`gosubc`) is a command-line code generation tool that creates subcommand code for command-line interfaces (CLIs) in Go from source code comments. **It is designed to be installed and run as a standalone executable during your build process (e.g., via `go generate`), not imported as a library dependency in your Go code.**

By leveraging specially formatted code comments, it automatically generates a dependency-less subcommand system, allowing you to focus on your application's core logic instead of boilerplate code.

**Status:** Pre-v1. The API and generated code structure may change.

## Key Features

- **Convention over Configuration:** Define your CLI structure with simple, intuitive code comments.
- **Zero Dependencies:** The generated code is self-contained and doesn't require any external libraries.
- **Automatic Code Generation:** `gosubc` parses your Go files and generates a complete, ready-to-use CLI.
- **Parameter Auto-Mapping:** Automatically maps CLI flags, positional arguments, and variadic arguments to function parameters.
- **Rich Syntax Support:** Supports custom flag names, default values, and description overrides via comments.
- **Man Page Generation:** Automatically generate Unix man pages for your CLI.

## Installation

To install `gosubc`, use `go install`:

```bash
go install github.com/arran4/go-subcommand/cmd/gosubc@latest
```

## Getting Started

### 1. Define Your Commands

Create a Go file and define a function that will serve as your command. Add a comment above the function in the format `// FunctionName is a subcommand 'root-command sub-command...'`.

Example `main.go`:

```go
package main

import "fmt"

// PrintHelloWorld is a subcommand `my-app hello`
// This command prints "Hello, World!" to the console.
func PrintHelloWorld() {
    fmt.Println("Hello, World!")
}
```

### 2. Add a `generate.go` File

Create a file named `generate.go` in the same directory (package `main`). This robust version checks if `gosubc` is installed; if not, it uses `go run` to fetch and run it. This ensures it works for everyone without manual installation steps.

```go
package main

//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate || go run github.com/arran4/go-subcommand/cmd/gosubc generate"
```

### 3. Generate the CLI

Run `go generate` in your terminal:

```bash
go generate
```

This will create a `cmd/my-app` directory containing the generated CLI code.

### 4. Run Your New CLI

You can now run your newly generated CLI:

```bash
go run ./cmd/my-app hello
```

Output:
```
Hello, World!
```

## Comment Syntax Guide

`go-subcommand` uses a specific comment syntax to configure your CLI.

### Subcommand Definition

The primary directive defines where the command lives in the CLI hierarchy:

```go
// FuncName is a subcommand `root-cmd parent child`
```

### Subcommand Aliases & Command Routing

`go-subcommand` provides expressive alias and routing capabilities across child commands, nested hierarchies, and the application root.

#### Child Subcommand Aliases

You can define aliases for a subcommand using the `Aliases:` or `Alias:` directive, or inline `(alias: ...)` / `(aka: ...)`:

```go
// SvcAction is a subcommand `app svc` Manage services.
// Aliases: service, s
func SvcAction() error { ... }

// SvcListAction is a subcommand `app svc list` List active services.
// (aka: ls)
func SvcListAction(all bool) error { ... }
```

Aliases resolve at intermediate and leaf positions across execution and help traversal:

```console
$ app svc list --all
$ app service ls --all
$ app s ls --all
$ app help s ls
```

#### Multiple Invocations for a Single Action (Multi-Level Path Aliasing)

An application root action can be made reachable through both bare invocation and an explicit nested command path several levels deep without duplicating the action implementation:

```go
// RunProg is a subcommand `prog` Root application action.
// Aliases: tools docs simple, tools docs s
//
// Flags:
//   config: --config (default: "default.cfg") Configuration path
func RunProg(config string) error { ... }

// ToolsAction is a subcommand `prog tools` Developer tools.
// Aliases: t
func ToolsAction() error { ... }

// DocsAction is a subcommand `prog tools docs` Documentation tools.
// Aliases: d
func DocsAction() error { ... }
```

Both bare invocation and the canonical or aliased nested path select the exact same underlying action:

```console
$ prog --config custom.cfg
# Executes RunProg(config="custom.cfg")

$ prog tools docs simple --config custom.cfg
# Executes RunProg(config="custom.cfg")

$ prog t d s --config custom.cfg
# Executes RunProg(config="custom.cfg")

$ prog help tools docs simple
# Displays help for the nested path without executing RunProg or providers
```

#### Default Routes to Built-in Help Destinations

A bare invocation can be explicitly configured to forward to an existing help route using the `Default route:` directive (or inline `(default route: ...)`):

```go
// SimpleAction is a subcommand `prog simple` Simple demonstration command.
// Default route: help simple
//
// Flags:
//   name: -n --name (default: "world") Target name
func SimpleAction(name string) error { ... }

// OtherAction is a subcommand `prog other` Other command.
func OtherAction() error { ... }
```

In this configuration, bare invocation and `./prog help simple` produce identical output and exit status 0, while guaranteeing zero business action or provider execution:

```console
$ prog
Usage: prog simple [flags...]

Simple demonstration command.

Flags:
    -n, --name string (default: "world") Target name

$ prog help simple
Usage: prog simple [flags...]

Simple demonstration command.

Flags:
    -n, --name string (default: "world") Target name

$ prog help
Usage: prog <subcommand>

Subcommands:
    other      Other command.
    simple     Simple demonstration command.
```

Built-in commands (`help`, `usage`, `version`) remain side-effect-free and reserved, distinct from user actions.

#### Root Action vs Literal `root` Subcommand

An application root and an ordinary subcommand literally named `root` can coexist cleanly:

```go
// Downloadsd is a subcommand `downloadsd` Manage downloads and services.
func Downloadsd(config string) error { ... }

// Root is a subcommand `downloadsd root` Run the optional privileged supervisor.
// (aka: supervisor, r)
func Root(config string, serve bool) error { ... }
```

*   `downloadsd`: Executes the application-level `Downloadsd` action.
*   `downloadsd root` (or `downloadsd supervisor`, `downloadsd r`): Dispatches only to the explicit supervisor `Root` action.
*   `downloadsd help` or `downloadsd -h`: Shows application help without starting the supervisor or executing actions.
*   `downloadsd root --help` or `downloadsd help root`: Shows supervisor child help.

### Description & Extended Help

*   **Short Description:** The text immediately following the subcommand definition (or prefixed with `that ` or `-- `) becomes the short description used in usage lists.
*   **Extended Help:** Any subsequent lines that do not look like parameter definitions are treated as extended help text, displayed when the user requests help for that specific command.

```go
// MyFunc is a subcommand `app cmd` -- Does something cool
//
// This is the extended help text. It can span multiple lines
// and provide detailed usage examples or explanations.
```

### Parameter Configuration

Function parameters are automatically mapped to CLI flags. You can customize them using comments. `go-subcommand` looks for configuration in three places, in this priority order (highest to lowest):

1.  **`Flags:` Block:** A dedicated block in the main function documentation.
2.  **Inline Comments:** Comments on the same line as the parameter definition.
3.  **Preceding Comments:** Comments on the line immediately before the parameter.

#### The `Flags:` Block

This is the cleanest way to define multiple parameters. It must be an indented block following a line containing just `Flags:`.

```go
// MyFunc is a subcommand `app cmd`
//
// Flags:
//
//   username: --username -u (default: "guest") The user to greet
//   count:    --count -c    (default: 1)       Number of times
func MyFunc(username string, count int) { ... }
```

#### Syntax Reference

Inside a `Flags:` block or inline/preceding comments, you can use the following syntax tokens to configure a parameter:

*   **Flags:** `-f`, `--flag`. One or more flag aliases.
*   **Default Value:** `default: value` or `default: "value"`.
*   **Required:** `required`. Marks a flag as required; generated execution returns an error if it is omitted.
*   **From Parent:** `from parent`. Maps a child parameter to a flag declared on an ancestor command.
*   **Custom Parser:** `parser: Func` or `parser: "import/path".Func`. Uses a custom string parser for the parameter.
*   **Generator:** `generator: Func` or `generator: "import/path".Func`. Populates the parameter from code instead of exposing it as a CLI flag. Providers may name parameter dependencies, for example `generator: BuildClient(endpoint, token)`. Dependencies are bound first, providers run once per command execution in deterministic dependency order, and unknown dependencies or cycles fail generation with diagnostics.
*   **Positional Argument:** `@N` (e.g., `@1`, `@2`). Maps the Nth positional argument (1-based) to this parameter.
*   **Variadic Arguments:** `min...max` (e.g., `1...3`) or `...`. Maps remaining arguments to a slice.
*   **Description:** Any remaining text is treated as the parameter description.

Multiple parenthesized attributes can be combined with semicolons, for example `(required; parser: ParseThing)`.

Generated files plan import identifiers for the complete file before rendering. Source import aliases and qualified default expressions are therefore preserved when possible and deterministically renamed when they collide with generated dependencies such as `flag`, `fmt`, or `os`.

### Supported Types

The following Go types are supported for function parameters:

*   `string`: (Default)
*   `int`: Parsed as an integer.
*   `bool`: Parsed as a boolean flag (no value required, e.g., `--verbose`).
*   `time.Duration`: Parsed using `time.ParseDuration` (e.g., `10s`, `1h`).
*   `io.Reader` and `io.ReadCloser`: `-` borrows standard input; any other value is opened as an input file.
*   `io.Writer` and `io.WriteCloser`: `-` borrows standard output; any other value is opened with create, truncate, and write access.
*   Pointers such as `*int`: preserve the difference between omitted and explicitly provided zero values.
*   Slices such as `[]string`: support repeatable flags.
*   `error`: (Return value only) Your function can return an `error`, which will be propagated to the CLI exit code.

For I/O parameters, the literal values `stdin` and `stdout` are ordinary file paths; only `-` selects a standard stream. Files opened by generated code are closed on every exit path, while borrowed standard streams are never closed. Bare `*os.File` parameters are rejected for implicit CLI binding because their intended access mode is ambiguous; use the appropriate reader/writer interface or configure an explicit provider.

## Advanced Usage

### Positional Arguments

To accept positional arguments instead of flags, use the `@N` syntax.

```go
// Greet is a subcommand `app greet`
//
// Flags:
//
//   name: @1 The name to greet
func Greet(name string) {
    fmt.Printf("Hello, %s!\n", name)
}
```
Usage: `app greet John`

### Variadic Arguments

To accept a variable number of arguments, use a slice parameter and mark it with `...`.

```go
// ProcessFiles is a subcommand `app process`
//
// Flags:
//
//   files: ... List of files to process
func ProcessFiles(files ...string) {
    for _, file := range files {
        fmt.Println("Processing", file)
    }
}
```
Usage: `app process file1.txt file2.txt file3.txt`

### Custom Flags

You can define custom short and long flags.

```go
// Serve is a subcommand `app serve`
//
// Flags:
//
//   port: -p --port (default: 8080) Port to listen on
func Serve(port int) { ... }
```


### GNU-Style Flag Grouping & Assignments

The generated custom argument parser natively supports GNU-style single-letter short codes grouped together (e.g., `-abc` is parsed as `-a -b -c`). It also correctly handles equals assignment (e.g., `-v=123` or `--flag="value"`).

### GNU-Style `--` Termination (Parser Architecture)

The CLI uses a custom parser generated by `gosubc` rather than delegating entirely to the standard `flag` package (which is only used to manage underlying struct bindings). This ensures deterministic and bounded behaviour out-of-the-box.
*   **Termination Detection:** The first standalone `--` token encountered halts all flag parsing and subcommand recognition for the current command.
*   **Positional Passthrough:** All tokens following `--` (including subsequent `--` tokens, unknown flags, or subcommand names) are treated strictly as positional arguments and passed through untouched.
*   **Command Scope:** The termination is contextual to the command level where it is encountered; an `app -- subcommand` passes `subcommand` as an argument to `app`, while `app subcommand -- child` passes `child` as an argument to `subcommand`.


### Required vs Optional Parameters

To enforce that a specific flag must be provided at runtime, mark the parameter with the `required` keyword inside parentheses (e.g., `(required)`). The execution will fail gracefully if a user omits the required parameter. For optional parameters, omitting the flag relies on Go zero-values unless overridden by `default:`.

### Stream Parameters

For `io.Reader` and `io.ReadCloser` parameters, pass `-` to select standard
input. For `io.Writer` and `io.WriteCloser`, pass `-` to select standard
output. Generated usage repeats this direction-specific meaning, including for
`default: -`. Names such as `stdin` and `stdout` remain ordinary file paths.

### Nesting Commands

Nesting is implicit based on the command path string.

```go
// Root command: `app`
// Child: `app users`
// Grandchild: `app users create`

// CreateUser is a subcommand `app users create`
func CreateUser(...) { ... }

// ListUsers is a subcommand `app users list`
func ListUsers(...) { ... }
```

Each generated command owns a distinct usage resource. Commands with the same
leaf name in different paths (for example, `app serve` and
`app scenario serve`) retain separate help text and usage syntax.

Subcommand and flag rows remain inline in each generated `*_usage.txt` template
and are formatted at runtime with hanging-indent word wrapping to the active
terminal width (`COLUMNS`). Recursive help displays full command paths while
shallow help displays direct child names, aligning descriptions from each
branch's displayed labels without fixed padding widths.

### Root Actions and Subcommand Dispatch

When an application defines both a root action and subcommands, a recognized
subcommand (including the built-in `version`, `help`, and `usage` commands, or
an explicitly named child command such as `app root`) is dispatched without
running the application root action. Application-root identity (`RootCmd` and
`NewRoot` in `root.go`) is distinct from command-path subcommands, so an
explicit `app root` child retains its own generated struct, constructor,
usage resource, and tests without colliding with the application root.

Built-in `help` and `usage` commands also traverse nested subcommand paths
(for example, `app help services list`) without invoking business actions or
initializing generator-backed providers. Unrecognized command tokens on a
non-positional parent fail with an `unknown command` error rather than falling
through to the parent action. `--` preserves positional-argument semantics so a
token after it is not treated as a subcommand.

An error returned by a durable command action is annotated once with the
command name (for example, `app failed: ...`). The generated wrapper uses `%w`,
so callers can still use `errors.Is` and `errors.As` with the underlying error.

### Implicit and explicit command trees

The command path inside backticks determines the generated executable and its
nested command hierarchy. All of the examples below produce **one**
`cmd/gogenmux` executable, not a separate executable per function.

**Example A — implicit root and implicit intermediate command.**
Only the leaf commands are annotated; `gogenmux` and `config` are created
automatically as dispatch-only parents:

```go
// Generate is a subcommand `gogenmux generate` -- Generate source.
func Generate() {}

// Inspect is a subcommand `gogenmux inspect` -- Inspect the model.
func Inspect() {}

// Show is a subcommand `gogenmux config show` -- Show configuration.
func Show() {}
```

The generated command tree is:

```text
gogenmux                  [implicit root: no action function]
├── generate              [Generate()]
├── inspect               [Inspect()]
└── config                [implicit parent: no action function]
    └── show              [Show()]
```

**Example B — explicit root with a default action and explicit intermediate command.**
An annotated application-root function is not merely a structural declaration:
it becomes the **default action invoked by bare `gogenmux`**. Likewise, an
explicitly annotated `config` function handles bare `gogenmux config`.
The hierarchy still produces one executable:

```go
// Gogenmux is a subcommand `gogenmux` -- Multiplexer tools.
func Gogenmux() {}

// Generate is a subcommand `gogenmux generate` -- Generate source.
func Generate() {}

// Config is a subcommand `gogenmux config` -- Manage configuration.
func Config() {}

// Show is a subcommand `gogenmux config show` -- Show configuration.
func Show() {}
```

```text
gogenmux                  [explicit root + default action: Gogenmux()]
├── generate              [Generate()]
└── config                [explicit command action: Config()]
    └── show              [Show()]
```

For Example B, invocation selects the corresponding action:

```text
gogenmux                  → Gogenmux()       (default/root action)
gogenmux generate         → Generate()       (not Gogenmux())
gogenmux config           → Config()         (intermediate action)
gogenmux config show      → Show()           (not Config() or Gogenmux())
```

An **explicit root action is optional at definition time**, but when declared
it is also the application's default action. There is currently no separate
`commentv1` annotation for an *explicit, metadata-only root* that avoids
registering such a default action. For a dispatch-only root, use the implicit
root form in Example A rather than adding an empty root function solely to
declare the executable name. Similarly, omit the explicit `config` function
when that intermediate node needs no action.

Durable functions stay in their original source package; gosubc generates
the dispatch wrappers and help resources. A function specifically named
`Root` is not required. For related distinctions involving a literal child
command named `root`, see [issue #513](https://github.com/arran4/go-subcommand/issues/513).

**Syntax matters:** use backticks around the entire command path:

```go
// Show is a subcommand `gogenmux config show`
```

A comment such as `// Show is a subcommand gogenmux config show` (without backticks)
is **not equivalent** in the current `commentv1` parser; it can fall back to
the Go function name and create an unintended executable tree. See
[issue #478](https://github.com/arran4/go-subcommand/issues/478) for
tracking of that ambiguity.

### Parent Flags (Inheritance)

Subcommands can explicitly map a parameter to a flag declared by an ancestor command using `(from parent)`.

```go
// Parent is a subcommand `app parent`
//
// Flags:
//
//   verbose: -v --verbose
func Parent(verbose bool) { ... }

// Child is a subcommand `app parent child`
//
// Flags:
//
//   verbose: (from parent)
func Child(verbose bool) {
    // verbose parameter here maps to Parent's verbose variable
}
```

### Template Customization & Line Wrapping

`gosubc` supports customizing the generated code and usage text templates using the `--replace-template` flag.

You can supply template overlays in three formats:
*   **Alias File Replacement:** `--replace-template usage=path/to/myusage.gotmpl` (available aliases: `usage`, `man`, `cmd`, `root`, `templates`).
*   **Folder Overlay:** `--replace-template path/to/templates_dir` overlays a directory containing custom `.gotmpl` files onto default templates.
*   **txtar Archive Overlay:** `--replace-template path/to/templates.txtar` overlays a `.txtar` archive containing custom template files.

Multiple overlays are applied in command-line order. Later definitions with the same template name replace earlier definitions, while unrelated named definitions from every layer remain available.

To view or export the built-in templates:
*   `gosubc template layout`: Displays the directory tree structure of built-in templates.
*   `gosubc template export [--output <dir>] [--as-txtar] [--force]`: Exports all built-in templates to a directory or a single `.txtar` archive file. Existing template files and archives are preserved unless `--force` is supplied; unrelated files in an existing directory are left untouched.

Generated CLI usage output automatically wraps parameter descriptions based on the terminal width specified by the `COLUMNS` environment variable (falling back to 80 columns).

### Man Page Generation

To generate man pages, pass the `--man-dir` flag to `gosubc`.

```bash
gosubc generate --man-dir ./man
```

This will generate standard Unix man pages in the specified directory, using the descriptions and extended help text from your comments.

When upgrading from a generator version affected by [#469](https://github.com/arran4/go-subcommand/issues/469), regenerating existing man pages will produce a one-time canonical diff that removes deterministic trailing whitespace. Commit the regenerated output from the fixed generator rather than preserving the old spaces.

## CLI Reference

### `gosubc generate`

Generates the Go code for your CLI.

*   `--dir <path>`: Root directory containing `go.mod`. Defaults to current directory.
*   `--man-dir <path>`: Directory to write man pages to.
*   `--replace-template <alias>=<file>|<dir>|<txtar>`: Overlays custom templates onto built-in generation templates.
*   `--emit-go-generate=false`: Do not add a nested `//go:generate` directive to the generated executable. The default is `true` for compatibility; use this when the project owns one canonical, pinned generation command.

### `gosubc template`

Manage generation templates.

*   `gosubc template export [--output <path>] [--as-txtar] [--force]`: Export built-in templates without replacing existing files by default. Use `--force` only to replace existing exported template files or archives.
*   `gosubc template layout`: Display template structure layout.

### `gosubc list`

Lists all detected subcommands.

*   `--dir <path>`: Root directory.

### `gosubc validate`

Validates subcommand definitions for errors or conflicts.

*   `--dir <path>`: Root directory.

### `gosubc goreleaser`

Generates release configuration.

*   `--dir <path>`: Root directory containing `go.mod`. Defaults to current directory.
*   `--go-releaser-github-workflow`: Generate GitHub Action workflow for GoReleaser.

### Parser Backend Configuration

### Generated application Go compatibility

The Go version needed to run `gosubc` is separate from the version supported by
the generated application. Generated command packages require **Go 1.16 or
newer** because they use standard-library embedding and `io/fs`.

The generator reads the target module's `go` directive and adapts its output:

| Target module version | Generated compatibility form |
| --- | --- |
| Go 1.16–1.17 | `interface{}` and local loops (no `slices`) |
| Go 1.18–1.20 | `any` and local loops |
| Go 1.21+ | `any` and `slices.Contains` |

Generation fails before writing output when a target module declares Go 1.15 or
older. The compatibility forms are regression-tested as generated source.
* `--parser-name` configures the source/comment parser behavior used to read your files. (Default: `commentv1`)
* `--cli-parser` configures the runtime argv parser behavior generated in your binary. (Default: `gnu`)
* parameter `parser:` specifies a custom value parser for reading your types out of flag/positional arguments.

### Target-aware source discovery

`commentv1` selects source files with Go build-constraint semantics. In
particular it excludes `_test.go` files and honours platform filename suffixes,
`//go:build`, legacy `// +build`, and explicit build tags. A generator invoked
through `GenerateWithFS` can select the intended output target with
`parsers.ParseOptions`:

```go
parseOptions := &parsers.ParseOptions{
	Recursive: true,
	GOOS:      "windows",
	GOARCH:    "amd64",
	BuildTags: []string{"enterprise"},
}
```

The generated command tree is valid for that selected target, not a universal
mixture of mutually exclusive platform implementations. Compile or package the
result using the same target values. Empty target fields retain the generator
host defaults.

#### Runtime Parser Backends
By default, the `gnu` backend is used, featuring clustered short flags (e.g., `-abc` is identical to `-a -b -c`).

You can override the backend applied either across the generator defaults, or individually by inheriting sub-commands to support mixing multiple CLI grammars. The parser backend for any given command resolves in this explicit fallback order:
1. The command's explicitly declared `CLI-Parser:` metadata.
2. The nearest explicit ancestor's declared backend.
3. The generation-wide default provided by `--cli-parser`.
4. The `gnu` backend.

```go
// Legacy is a subcommand `app legacy`
// CLI-Parser: go-flag
func Legacy() {}

// Nested override back to gnu inherits nearest explicit ancestor
// Child is a subcommand `app legacy child`
// CLI-Parser: gnu
func Child() {}
```

**Subtree Command Boundaries:**
A parent parsing its argv according to its own backend scopes the argv entirely per tree level. Once the parent identifies and dispatches a child subcommand, the remaining argv strictly belongs to the child's Execute scope, preventing parent-parsing bleed logic entirely without reinterpreting previously parsed flags from the caller level.

**Parser Specific Constraints:**
The `go-flag` backend integrates directly with the Go standard library `flag.FlagSet`, meaning it strictly ignores GNU-style clustered short options. A flag invoked as `-abc` evaluates specifically as the exact declared flag named `abc` if it exists. If `abc` is undeclared, it strictly returns an unknown flag error rather than being parsed as `-a -b -c`. It handles single dashes natively (so `-in README.md` is fully supported), and supports boolean toggles intuitively. Go-flag halts parsing explicitly on its first seen positional argument, exactly as defined by the flag standard library. Both `-name=value` and `--name=value` syntaxes are accepted.
All arguments preceding an explicit '--' are correctly resolved and parsed to correctly segregate positional offsets accurately across all backends natively supporting them. Unknown flags will properly log unrecognized syntax per the target backend structure. Standard `-h` and `--help` return FlagSet help behavior if undeclared, but explicitly declared `h` / `help` are treated as ordinary registered flags.

`plus-minus` is a reserved parser name for tracking future `+x`/`-x` style toggle parser additions (tracked via #464).


#### Replace CLI-Parser Backends
The runtime CLI parser behavior can be fully swapped by overriding the defined templates via the `--replace-template` option, allowing fallback syntax generation while replacing only targeted parser structures. The built-in backends are mapped exactly to the paths `cli-parsers/gnu.gotmpl` and `cli-parsers/go-flag.gotmpl`. Replacing one specific parser fragment entirely leaves the other unreplaced backends completely available and selectable for command generation:

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc generate --replace-template "cli-parsers/gnu.gotmpl=my_custom_parser.gotmpl"
```

## Contributing

Contributions are welcome! If you find a bug or have a feature request, please open an issue on our [GitHub repository](https://github.com/arran4/go-subcommand).

## License

This project is licensed under the **BSD 3-Clause License**. See the [LICENSE](LICENSE) file for details.
