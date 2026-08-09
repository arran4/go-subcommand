1. The user points out that the testing block generated for `best-fit` might fail if there are custom types (since we blindly generate `args = append(args, "test")` etc., which would fail custom parsing if it's expecting an integer, enum, etc., or fail to compile if it can't map properly).
2. They say "We might have to drop the tested on generated sources requirement... Sorry".
3. I should revert the generated test for `best-fit` in `templates/cmd/cmd_test.go.gotmpl`. I will simply remove `Test{{.SubCommandStructName}}_Execute_BestFit` block from `cmd_test.go.gotmpl` and then also regenerate / update the testdata.
4. I will then submit the PR.
