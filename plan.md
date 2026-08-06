We must create an EXECUTABLE Go test using the actual `commentv1` parser to generate the code and execute the test, to fully comply with the code reviewer's feedback.

Wait, `templates/testdata/*.txtar` test generation templates, but they ONLY test the `templates` package. They take `input.json` and verify `output.go`.
They DO NOT test the parser (`parsers/commentv1`), nor do they execute the generated code!

To test the PARSER and the GENERATOR end-to-end AND EXECUTE the handler, we should probably add an integration test in `cmd/gosubc` or `parsers/commentv1` or `tests`. Wait, look at `issue112_repro_test.go` or similar? Let's check `ls` in the root!
