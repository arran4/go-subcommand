1. Actually, for `best-fit`, we can create a specific set of tests that skip one argument to verify the best-fit logic really works in the test.
2. In `cmd_test.go.gotmpl`, when `.PositionalAlgorithm` is `best-fit`, we want to test best-fit explicitly. But how do we dynamically generate an argument list that skips one of the optional parameters to verify best-fit?
3. It might be easier to simply test the normal path in the main `Execute` test, and generate an *additional* test block `TestMyCmd_Execute_BestFit` for `best-fit` that provides an argument sequence skipping the middle optional if it exists.
4. Let's revert `cmd_test.go.gotmpl` and add `Test{{.SubCommandStructName}}_Execute_BestFit` if `{{- if eq .PositionalAlgorithm "best-fit" }}`
