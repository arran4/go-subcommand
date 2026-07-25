# Issue Resolution Report

## Open Issues Now Resolved

Based on recent updates to the codebase (such as the parameter parser refactor and the move away from the standard `flag` package to custom parsing loops), the following open issues appear to be fully resolved in the codebase:

1. **#213 subcommand aliases**: The `Alias:` / `Aliases:` directive and the inline `(aka: ...)` syntax are fully implemented and documented in the README, and generation logic correctly registers aliases in the command tree.
2. **#268 parent-flag Directive Not Implemented**: The `(from parent)` directive is now fully implemented. Inherited parameters correctly trace from parent to child commands via the `DeclaredIn` and `Inherited` properties.
3. **#202 Lack of Custom Flag Parsing (`flag.Func`)**: The `parser: path.Func` directive was added to handle custom logic, satisfying the need for custom parsing.
4. **#287 Rewrite of flag parser to be "better"**: The `flag.FlagSet` usage has been entirely replaced by a more robust custom parsing loop in the generated code that properly handles short codes, assignments, and slice types.
5. **#104 Redundant Flag Parsing**: The generator no longer creates redundant standard `flag.FlagSet` objects; it exclusively uses the custom string parsing loop.
6. **#12 Strict Flag Validation for Zero Values**: Addressed by the support for pointer types (e.g., `*int`, `*string`), which preserves the difference between omitted values (nil) and explicit zero values.
7. **#50 Need to be able to specify parent subcommand flags and access them from the children**: Solved via the parent parameter inheritance mechanisms (`from parent`).

## Closed Issues Resolution Status

### 1. https://github.com/arran4/go-subcommand/issues/331
**Title:** Required vs Default/optional
**Status:** (a) Fully resolved
**Reasoning:** Support for pointers (like `*int`, `*string`) was added, allowing distinction between omitted and set values. Required flags are checked and enforced via `(required)`. Slices are also supported as optional repeatable parameters.

### 2. https://github.com/arran4/go-subcommand/issues/330
**Title:** Parent flags aren't properly reported in help/usage documentation
**Status:** (a) Fully resolved
**Reasoning:** The `appendFlagsUsage` and `FullUsageString` in `model/model.go` recursively walk up the tree and aggregate flags and positional arguments accurately. Usage strings include `<parent> <child> <flags>`.

### 3. https://github.com/arran4/go-subcommand/issues/49
**Title:** Need custom "flag parsers" for custom types
**Status:** (a) Fully resolved
**Reasoning:** The code parser extracts the `(parser: path.Func)` comment annotations into a structured `ParserConfig` and uses it to parse flag arguments, allowing user-defined parsing functionality.

### 4. https://github.com/arran4/go-subcommand/issues/82
**Title:** support repeatable flags for slices
**Status:** (a) Fully resolved
**Reasoning:** Slice parameter handling is robustly generated in `root.go.gotmpl`. Slice types correctly append values for every flag usage rather than overwriting.

### 5. https://github.com/arran4/go-subcommand/issues/107
**Title:** Document and support pointers for nullable (and/or sql like null classes
**Status:** (a) Fully resolved
**Reasoning:** Pointer types are fully generated in the parameter switch block (e.g., `*int`, `*string`). The `README.md` documents: "Pointers such as `*int`: preserve the difference between omitted and explicitly provided zero values."

### 6. https://github.com/arran4/go-subcommand/issues/114
**Title:** Global Initialization Hook
**Status:** (b) Partially resolved
**Reasoning:** The root commands are initialized correctly before subcommands via the execution loop (e.g. running initialization if `CommandAction` is set at the root). However, an explicit top-level "Global Hook" that *always* guarantees execution before *any* child runs isn't perfectly exposed as a separate hook distinct from the normal command action.

### 7. https://github.com/arran4/go-subcommand/issues/220
**Title:** More GNU style processing updates around "short codes"
**Status:** (a) Fully resolved
**Reasoning:** The generated custom loop now specifically breaks down clustered single letter flags (e.g. `-abc`), handling equals assignment (`-v=123`) properly without using standard `flag.FlagSet`.

## New Feature Request

### Issue Request: Explicit Global Hook / Pre-Run Middleware
**Problem:** Issue #114 requested a global initialization hook. While it's partially addressed by `RootCmd.Execute` resolving the state of parent parameters, there is no distinct mechanism to run a standalone initialization hook *before* the leaf command's action logic executes without interfering with sub-command dispatch. The root's `CommandAction` runs if there is no subcommand, but if there *is* a subcommand, the parent logic is skipped.
**Request:** Add a `PreRun` or `InitHook` capability in the comments (e.g. `// global-hook: mypkg.InitFunc`), which would insert a call right before `cmd().Execute(...)` executes the subcommand.

## Next Suggested Set of Open Issues to Resolve

Based on the current open issues list, the following set groups related configuration and formatting logic that would greatly polish the CLI output and usability:
1. **#402 Configurable usage-line folding and width**: Ties heavily into how usage strings are generated (`FullUsageString` and `usage.txt.gotmpl`).
2. **#361 Stop defaulting to `"TODO: Add usage text"`**: Directly addresses the default documentation fallbacks when parameters are missing descriptions.
3. **#197 Needs to provide a warning if there is no description for a function**: Another documentation consistency issue that perfectly aligns with #361.
4. **#111 Customizable usage templates or extra details such as examples**: Allows passing custom templates or overriding sections, fixing the rigidity of the current `usage.txt.gotmpl`.
5. **#14 Custom Usage Templates**: Duplicates #111.

**Theme:** "Documentation and Usage Formatting Customization". Addressing these together is highly efficient as they all require changes in `usage.txt.gotmpl`, parser model description handling, and command-line parsing of custom generator options.
