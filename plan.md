1. **Update `model/model.go` to store the positional algorithm**:
   - Add `PositionalAlgorithm string` to `model.Command` and `model.SubCommand`. It will default to empty (which implies `greedy`), but can be set to `"best-fit"`.

2. **Update `parsers/commentv1/constants.go`**:
   - Add a new directive constant like `DirectivePositionalAlgorithm = "positional-algorithm:"` or just use a regex in the parser to extract it (e.g. `(positional-algorithm: best-fit)` or `[positional-algorithm: best-fit]`).

3. **Update `parsers/commentv1/commentv1.go`**:
   - Update `ParseSubCommandComments` to detect and parse the `positional-algorithm` directive (e.g., matching `\(positional-algorithm:\s*([a-zA-Z_-]+)\)` or `\[positional-algorithm:\s*([a-zA-Z_-]+)\]`).
   - The returned values of `ParseSubCommandComments` will need to include `positionalAlgorithm string`. Update the signature and callers (both in `commentv1.go` and `parser_test.go`/`parser_regr_test.go`).
   - Store the extracted algorithm into `cmd.PositionalAlgorithm` in `collectSubCommands` and `cmdTree.Insert`.

4. **Update `templates/common/common.gotmpl` (`positional_args_parsing`)**:
   - If `.PositionalAlgorithm` is `best-fit`, apply the new best-fit parsing logic.
   - Wait, `positional_args_parsing` template currently only receives `.Parameters`. We need to pass the subcommand or command object to it, or pass `(list .PositionalAlgorithm .Parameters)` or something, so it knows which algorithm to use. Currently it's called with `{{- template "positional_args_parsing" .Parameters }}`. We will change this to `{{- template "positional_args_parsing" . }}` and inside use `.Parameters` and `.PositionalAlgorithm`.
   - Implement the `best-fit` matching algorithm inside `templates/common/common.gotmpl`:
     - Calculate minimum required positional arguments.
     - Count total optional positional arguments.
     - The logic for best-fit should ideally try to find a valid assignment for all provided arguments to the available positional parameters, ensuring that the custom parsers / type conversions succeed.
     - However, generating a full backtracking "best-fit" parser might be complex. Given the user's example: `string int string` with args `1 a`. Greedy matches `string` -> `1`, `int` -> fails because `a` is not int. Best-fit matches `string` -> `1`, `int` -> `skip`, `string` -> `a`. Actually, `a = 1` and `c = a`. Wait, `string int string` -> `[a] [b] [c]`. If `1 a`, `1` fits `[a]` (string), `a` fits `[c]` (string). So `[b]` (int) is skipped.
     - In code generation, we can implement a loop that tries to fill `remainingArgs` into positional arguments.

5. **Update documentation**:
   - Update `README.md` and `help_syntax.go` to explain `(positional-algorithm: best-fit)` and `[positional-algorithm: best-fit]`.

6. Complete pre commit steps to ensure proper testing, verification, review, and reflection are done.
