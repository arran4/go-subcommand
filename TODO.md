# TODO

All requested issues have been addressed in this PR.

*   #220: GNU style short codes (impl in `cmd.go.gotmpl` and `root.go.gotmpl`)
*   #107: Pointers for nullable (verified existing support + checks)
*   #82: Repeatable flags for slices (verified `append` logic)
*   #331: Required vs Default/optional (impl `IsRequired` logic with runtime validation and updated usage)
*   #114: Global Initialization Hook (impl `Global` flag and execution logic)
*   #330: Parent flags usage grouping (verified `ParameterGroups` logic works)
*   #49: Custom flag parsers (impl `Parser` logic)

## Completed Improvements
*   Implement usage template for Root command to support grouping and required indicators consistent with subcommands.
*   Support expanding flags in usage string (e.g. `[--flag <val>]`) for small flag counts.
