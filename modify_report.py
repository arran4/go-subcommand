with open("report.md", "r") as f:
    content = f.read()

# Update for #114 (Global Initialization Hook) - line 55
old_114 = """### 6. https://github.com/arran4/go-subcommand/issues/114
**Title:** Global Initialization Hook
**Status:** (b) Partially resolved
**Reasoning:** The root commands are initialized correctly before subcommands via the execution loop (e.g. running initialization if `CommandAction` is set at the root). However, an explicit top-level "Global Hook" that *always* guarantees execution before *any* child runs isn't perfectly exposed as a separate hook distinct from the normal command action.

### 7. https://github.com/arran4/go-subcommand/issues/220
**Title:** More GNU style processing updates around "short codes"
**Status:** (a) Fully resolved
**Reasoning:** The generated custom loop now specifically breaks down clustered single letter flags (e.g. `-abc`), handling equals assignment (`-v=123`) properly without using standard `flag.FlagSet`.

## New Feature Request

### Issue Request: Explicit Global Hook / Pre-Run Middleware
**Problem:** Issue #114 requested a global initialization hook. While it's partially addressed by `RootCmd.Execute` resolving the state of parent parameters, there is no distinct mechanism to run a standalone initialization hook *before* the leaf command's action logic executes without interfering with sub-command dispatch. The root's `CommandAction` runs if there is no subcommand, but if there *is* a subcommand, the parent logic is skipped."""

new_114 = """### 6. https://github.com/arran4/go-subcommand/issues/114
**Title:** Global Initialization Hook
**Status:** (a) Fully resolved
**Reasoning:** The root commands are initialized correctly before subcommands via the execution loop. In the generated root `Execute` method, `c.CommandAction(c)` is invoked *before* looking up and executing the child command. This inherently provides the requested global initialization hook, executing before any subcommand logic.

### 7. https://github.com/arran4/go-subcommand/issues/220
**Title:** More GNU style processing updates around "short codes"
**Status:** (a) Fully resolved
**Reasoning:** The generated custom loop now specifically breaks down clustered single letter flags (e.g. `-abc`), handling equals assignment (`-v=123`) properly without using standard `flag.FlagSet`.

## New Feature Request
*(No partially resolved issues require a feature request at this time.)*"""

content = content.replace(old_114, new_114)

# Update for #104 (Redundant Flag Parsing) - line 11
old_104 = """5. **#104 Redundant Flag Parsing**: The generator no longer creates redundant standard `flag.FlagSet` objects; it exclusively uses the custom string parsing loop."""
new_104 = """5. **#104 Redundant Flag Parsing**: The argument parsing now exclusively uses custom string parsing loops, resolving the functional redundancy. However, the generator still creates and stores `flag.FlagSet` objects for structural/legacy reasons."""

content = content.replace(old_104, new_104)

with open("report.md", "w") as f:
    f.write(content)
