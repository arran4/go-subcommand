package go_subcommand

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/model"
	"golang.org/x/tools/txtar"
)

//go:embed templates/testdata/runtime/issue464.txtar
var plusMinusFixture []byte

//go:embed templates/testdata/runtime/issue534_command_boundary.txtar
var commandBoundaryFixture []byte

//go:embed templates/testdata/runtime/issue464_units.go.txt
var plusMinusUnitTests string

func TestPlusMinusGenerationAndRuntime(t *testing.T) {
	input, expected := splitInputExpected(txtar.Parse(plusMinusFixture))
	var invocations []struct {
		Args                       []string
		Stdout, Stderr, StderrFile string
		Status                     int
	}
	if err := json.Unmarshal(expected["invocations.json"].Data, &invocations); err != nil {
		t.Fatal(err)
	}
	if len(invocations) == 0 {
		t.Fatal("empty manifest")
	}
	var sourceFiles map[string][]byte
	for _, selection := range []string{"source", "default", "gnu-root"} {
		t.Run(selection, func(t *testing.T) {
			scenario := fstest.MapFS{}
			for name, file := range input {
				scenario[name] = file
			}
			backend := "gnu"
			if selection != "source" {
				if selection == "default" {
					backend = "plus-minus"
				}
				scenario["app.go"] = &fstest.MapFile{Data: bytes.Replace(input["app.go"].Data, []byte("// CLI-Parser: plus-minus\n"), nil, 1)}
			}
			generate := func() map[string][]byte {
				writer := NewCollectingFileWriter()
				if err := GenerateWithFS(scenario, writer, ".", "man", "commentv1", nil, false, false, nil, false, false, "", "", "", GenerateOptions{CLIParser: backend}); err != nil {
					t.Fatal(err)
				}
				return writer.Files
			}
			files := generate()
			if !reflect.DeepEqual(files, generate()) {
				t.Fatal("nondeterministic generation")
			}
			if selection == "source" {
				sourceFiles = files
				if os.Getenv("UPDATE_SNAPSHOTS") == "1" || os.Getenv("UPDATE_GOLDENS") == "1" {
					ar := txtar.Parse(plusMinusFixture)
					for i, f := range ar.Files {
						for _, name := range []string{"root.go", "plus_minus.go", "gnu_pm.go", "templates/app_usage.txt"} {
							if f.Name == "expected/"+name && files["cmd/app/"+name] != nil {
								ar.Files[i].Data = files["cmd/app/"+name]
							}
						}
						if f.Name == "expected/app.1" && files["man/app.1"] != nil {
							ar.Files[i].Data = files["man/app.1"]
						}
					}
					if err := os.WriteFile("templates/testdata/runtime/issue464.txtar", txtar.Format(ar), 0644); err != nil {
						t.Fatal(err)
					}
					plusMinusFixture = txtar.Format(ar)
					_, expected = splitInputExpected(ar)
				}
			} else if selection == "default" && !reflect.DeepEqual(files, sourceFiles) {
				t.Fatal("source/default selection differ")
			}
			for _, name := range []string{"root.go", "plus_minus.go", "gnu_pm.go", "templates/app_usage.txt"} {
				if selection == "gnu-root" && (name == "root.go" || name == "templates/app_usage.txt" || name == "gnu_pm.go") {
					continue
				}
				if expected[name] == nil {
					t.Fatalf("missing artifact %s", name)
				}
				if !bytes.Equal(files["cmd/app/"+name], expected[name].Data) {
					t.Fatalf("generated %s differs from source-comment golden", name)
				}
			}
			// The compiler and process boundary require materialized files.
			if selection != "gnu-root" && !bytes.Equal(files["man/app.1"], expected["app.1"].Data) {
				t.Fatal("generated manual differs from golden")
			}
			dir := t.TempDir()
			for name, file := range scenario {
				writeRuntimeFixture(t, filepath.Join(dir, name), string(file.Data))
			}
			for name, data := range files {
				writeRuntimeFixture(t, filepath.Join(dir, name), string(data))
			}
			writeRuntimeFixture(t, filepath.Join(dir, "cmd/app/plus_minus_test.go"), plusMinusUnitTests)
			writeRuntimeFixture(t, filepath.Join(dir, "cmd/app/invocations.json"), string(expected["invocations.json"].Data))
			unitPattern := "TestPlusMinus"
			if selection == "gnu-root" {
				unitPattern = "TestPlusMinusTokenizer|TestPlusMinusBoundariesAndAtomicClusters|TestPlusMinusConversionAndErrors"
			}
			command := exec.Command("go", "test", "./cmd/app", "-run", unitPattern)
			command.Dir = dir
			if out, err := command.CombinedOutput(); err != nil {
				t.Fatalf("generated unit/API tests: %v\n%s", err, out)
			}
			binary := filepath.Join(dir, "app")
			build := exec.Command("go", "build", "-o", binary, "./cmd/app")
			build.Dir = dir
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}
			for _, invocation := range invocations {
				if selection == "gnu-root" && (len(invocation.Args) == 0 || invocation.Args[0] != "gnu") {
					continue
				}
				command := exec.Command(binary, invocation.Args...)
				command.Dir = dir
				command.Env = append(os.Environ(), "COLUMNS=80")
				var stdout, stderr bytes.Buffer
				command.Stdout, command.Stderr = &stdout, &stderr
				err := command.Run()
				status := 0
				if err != nil {
					if exit, ok := err.(*exec.ExitError); ok {
						status = exit.ExitCode()
					} else {
						t.Fatal(err)
					}
				}
				want := invocation.Stderr
				if invocation.StderrFile != "" {
					want = string(expected[invocation.StderrFile].Data)
				}
				got := strings.ReplaceAll(stderr.String(), binary, "app")
				if status != invocation.Status || stdout.String() != invocation.Stdout || got != want {
					t.Errorf("%v: status=%d stdout=%q stderr=%q; want status=%d stdout=%q stderr=%q", invocation.Args, status, stdout.String(), got, invocation.Status, invocation.Stdout, want)
				}
			}
		})
	}
}

func TestPlusMinusRejectsAmbiguousSource(t *testing.T) {
	input, _ := splitInputExpected(txtar.Parse(plusMinusFixture))
	for _, tc := range []struct{ name, from, to, want string }{
		{"short collision", "beta: -b --beta", "beta: -a --beta", `command app: plus-minus short name "a" collides between alpha and beta`},
		{"long collision", "beta: -b --beta", "beta: -b --beta --enable", `command app: plus-minus long name "enable" collides between alpha and beta`},
		{"invalid short char", "count: --count", "count: -1 --count", `command app: plus-minus short name "1" requires an ASCII letter other than h`},
		{"custom boolean", "alpha: -a --alpha", "alpha: (parser: ParseCustom) -a --alpha", "command app: plus-minus requires scalar booleans without custom parsers (alpha)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario := fstest.MapFS{}
			for name, file := range input {
				scenario[name] = file
			}
			scenario["app.go"] = &fstest.MapFile{Data: bytes.Replace(input["app.go"].Data, []byte(tc.from), []byte(tc.to), 1)}
			writer := NewCollectingFileWriter()
			err := GenerateWithFS(scenario, writer, ".", "", "commentv1", nil, false, false, nil, false, false, "", "", "")
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error=%v want %q", err, tc.want)
			}
			if len(writer.Files) != 0 || len(writer.Dirs) != 0 {
				t.Fatal("invalid source wrote output")
			}
		})
	}
}

func TestValidatePlusMinusParameters(t *testing.T) {
	for _, tc := range []struct {
		name       string
		parameters []*model.FunctionParameter
		valid      bool
	}{
		{"separate namespaces", []*model.FunctionParameter{{Name: "a", Type: "bool"}, {Name: "b", Type: "bool"}, {Name: "ab", Type: "bool"}}, true},
		{"short collision", []*model.FunctionParameter{{Name: "alpha", Type: "bool", FlagAliases: []string{"a"}}, {Name: "a", Type: "bool"}}, false},
		{"long collision", []*model.FunctionParameter{{Name: "alpha", Type: "bool", FlagAliases: []string{"both"}}, {Name: "both", Type: "bool"}}, false},
		{"value short", []*model.FunctionParameter{{Name: "count", Type: "int", FlagAliases: []string{"n"}}}, true},
		{"single char long", []*model.FunctionParameter{{Name: "alpha", Type: "bool", LongAliases: []string{"a"}}}, true},
		{"separate short and long same char", []*model.FunctionParameter{{Name: "alpha", Type: "bool", ShortAliases: []string{"a"}}, {Name: "beta", Type: "string", LongAliases: []string{"a"}}}, true},
		{"unicode", []*model.FunctionParameter{{Name: "alpha", Type: "bool", FlagAliases: []string{"é"}}}, false},
		{"help", []*model.FunctionParameter{{Name: "help", Type: "bool"}}, false},
		{"short help", []*model.FunctionParameter{{Name: "alpha", Type: "bool", FlagAliases: []string{"h"}}}, false},
		{"boolean slice", []*model.FunctionParameter{{Name: "toggles", Type: "[]bool"}}, false},
		{"custom boolean", []*model.FunctionParameter{{Name: "toggle", Type: "bool", Parser: model.ParserConfig{Type: model.ParserTypeCustom, Func: &model.FuncRef{FunctionName: "Parse"}}}}, false},
		{"long value", []*model.FunctionParameter{{Name: "count", Type: "int"}}, true},
		{"invalid plus-bare mode", []*model.FunctionParameter{{Name: "count", Type: "int", PlusBare: model.PlusOperationConfig{Mode: "negate"}}}, false},
		{"valid plus-bare reset", []*model.FunctionParameter{{Name: "count", Type: "int", PlusBare: model.PlusOperationConfig{Mode: "reset"}}}, true},
		{"valid plus-bare zero", []*model.FunctionParameter{{Name: "count", Type: "int", PlusBare: model.PlusOperationConfig{Mode: "zero"}}}, true},
		{"valid plus-bare func", []*model.FunctionParameter{{Name: "count", Type: "int", PlusBare: model.PlusOperationConfig{Mode: "func", Func: &model.FuncRef{FunctionName: "MyReset"}}}}, true},
		{"invalid plus-value mode", []*model.FunctionParameter{{Name: "count", Type: "int", PlusValue: model.PlusOperationConfig{Mode: "add"}}}, false},
		{"valid plus-value set", []*model.FunctionParameter{{Name: "count", Type: "int", PlusValue: model.PlusOperationConfig{Mode: "set"}}}, true},
		{"valid plus-value func", []*model.FunctionParameter{{Name: "count", Type: "int", PlusValue: model.PlusOperationConfig{Mode: "func", Func: &model.FuncRef{FunctionName: "MyAdd"}}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validatePlusMinusParameters("app", tc.parameters); (err == nil) != tc.valid {
				t.Fatalf("error=%v valid=%t", err, tc.valid)
			}
		})
	}
}

func TestCommandBoundaryAndMixedTrees(t *testing.T) {
	input, _ := splitInputExpected(txtar.Parse(commandBoundaryFixture))
	scenario := fstest.MapFS{}
	for name, file := range input {
		scenario[name] = file
	}
	generate := func() map[string][]byte {
		writer := NewCollectingFileWriter()
		if err := GenerateWithFS(scenario, writer, ".", "man", "commentv1", nil, false, false, nil, false, false, "", "", "", GenerateOptions{CLIParser: "plus-minus"}); err != nil {
			t.Fatal(err)
		}
		return writer.Files
	}
	files := generate()
	if !reflect.DeepEqual(files, generate()) {
		t.Fatal("nondeterministic generation")
	}

	dir := t.TempDir()
	for name, file := range scenario {
		writeRuntimeFixture(t, filepath.Join(dir, name), string(file.Data))
	}
	for name, data := range files {
		writeRuntimeFixture(t, filepath.Join(dir, name), string(data))
	}

	// 1. Run scope identity unit test verifying root preserves distinct value
	scopeUnitTest := `package main

import (
	"testing"
)

func TestScopeIdentityDistinctValues(t *testing.T) {
	root, err := NewRoot("app", "devel", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Execute([]string{"--token=root-distinct", "shadow", "--token=child-distinct"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if root.token != "root-distinct" {
		t.Errorf("root.token = %q, want root-distinct", root.token)
	}
}
`
	writeRuntimeFixture(t, filepath.Join(dir, "cmd/app/scope_test.go"), scopeUnitTest)
	testCmd := exec.Command("go", "test", "./cmd/app", "-run", "TestScopeIdentityDistinctValues")
	testCmd.Dir = dir
	if out, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("scope unit test failed: %v\n%s", err, out)
	}

	// 2. Build executable
	binary := filepath.Join(dir, "app")
	build := exec.Command("go", "build", "-o", binary, "./cmd/app")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build error: %v\n%s", err, out)
	}
	build122 := exec.Command("go", "build", "-o", binary+"-122", "./cmd/app")
	build122.Dir = dir
	build122.Env = append(os.Environ(), "GOTOOLCHAIN=go1.22.12")
	if out, err := build122.CombinedOutput(); err != nil {
		t.Fatalf("go 1.22.12 build error: %v\n%s", err, out)
	}

	for _, bin := range []string{binary, binary + "-122"} {
		runCLI := func(args ...string) (stdout string, stderr string, exitCode int) {
			cmd := exec.Command(bin, args...)
			cmd.Dir = dir
			var outBuf, errBuf bytes.Buffer
			cmd.Stdout = &outBuf
			cmd.Stderr = &errBuf
			err := cmd.Run()
			code := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				} else {
					code = -1
				}
			}
			return outBuf.String(), errBuf.String(), code
		}

		// 1. app --token=alpha child -> succeeds
		out, errOut, code := runCLI("--token=alpha", "child")
		if code != 0 || !strings.Contains(out, "child: token=alpha") {
			t.Fatalf("app --token=alpha child failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 2. app child --token=beta -> succeeds (inherited flag after child)
		out, errOut, code = runCLI("child", "--token=beta")
		if code != 0 || !strings.Contains(out, "child: token=beta") {
			t.Fatalf("app child --token=beta failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 3. app child (missing required token) -> fails with typed missing-required-flag
		out, errOut, code = runCLI("child")
		if code == 0 || !strings.Contains(errOut, "required flag --token not provided") {
			t.Fatalf("app child should fail with missing required flag: code=%d out=%q err=%q", code, out, errOut)
		}

		// 4. app --token=gamma gnu target1 (plus-minus root + GNU child) -> succeeds, observes parent token
		out, errOut, code = runCLI("--token=gamma", "gnu", "target1")
		if code != 0 || !strings.Contains(out, "gnu: token=gamma target=target1") {
			t.Fatalf("app --token=gamma gnu target1 failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 5. app --token=delta gnu (missing child positional argument) -> fails
		out, errOut, code = runCLI("--token=delta", "gnu")
		if code == 0 || !strings.Contains(errOut, "expected at least 1 positional arguments") {
			t.Fatalf("app --token=delta gnu should fail missing positional: code=%d out=%q err=%q", code, out, errOut)
		}

		// 6. app --token=delta --out=/nonexistent/path/out.txt gnu (missing child positional argument)
		// Must fail before any parent action or file write
		outPath := filepath.Join(dir, "never_created_dir", "out.txt")
		_, _, code = runCLI("--token=delta", "--out="+outPath, "gnu")
		if code == 0 {
			t.Fatalf("app with missing child positional must fail")
		}
		if _, err := os.Stat(outPath); !os.IsNotExist(err) {
			t.Fatalf("out.txt was created despite child positional failure: %v", err)
		}

		// 7. app --token=epsilon goflag item1 (plus-minus root + go-flag child) -> succeeds
		out, errOut, code = runCLI("--token=epsilon", "goflag", "item1")
		if code != 0 || !strings.Contains(out, "goflag: token=epsilon item=item1") {
			t.Fatalf("app --token=epsilon goflag item1 failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 8. app --token=epsilon goflag (missing child positional argument) -> fails
		out, errOut, code = runCLI("--token=epsilon", "goflag")
		if code == 0 || !strings.Contains(errOut, "expected at least 1 positional arguments") {
			t.Fatalf("app --token=epsilon goflag should fail missing positional: code=%d out=%q err=%q", code, out, errOut)
		}

		// 9. Root directly with token -> succeeds
		out, errOut, code = runCLI("--token=root-token")
		if code != 0 || !strings.Contains(out, "root: token=root-token") {
			t.Fatalf("app --token=root-token failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 10. Root directly without token -> fails with missing required flag
		out, errOut, code = runCLI()
		if code == 0 || !strings.Contains(errOut, "required flag --token not provided") {
			t.Fatalf("app without token should fail: code=%d out=%q err=%q", code, out, errOut)
		}

		// 11. Root help -> succeeds with status 0 without requiring token
		out, errOut, code = runCLI("--help")
		if code != 0 {
			t.Fatalf("app --help failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 12. Child help -> succeeds with status 0 without requiring token
		out, errOut, code = runCLI("child", "--help")
		if code != 0 {
			t.Fatalf("app child --help failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 13. Scope identity regression: app shadow --token=child (child-local optional token)
		// Must not satisfy root's required token!
		out, errOut, code = runCLI("shadow", "--token=child")
		if code == 0 || !strings.Contains(errOut, "required flag --token not provided") {
			t.Fatalf("app shadow --token=child should fail root required token: code=%d out=%q err=%q", code, out, errOut)
		}

		// 14. app --token=root shadow --token=child succeeds, preserving both distinct values
		out, errOut, code = runCLI("--token=root-val", "shadow", "--token=child-val")
		if code != 0 || !strings.Contains(out, "shadow: token=child-val") {
			t.Fatalf("app --token=root-val shadow --token=child-val failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 15. If both root and child require token: satisfying child only fails root
		out, errOut, code = runCLI("bothreq", "--token=child-val")
		if code == 0 || !strings.Contains(errOut, "required flag --token not provided") {
			t.Fatalf("app bothreq --token=child-val must fail root required token: code=%d out=%q err=%q", code, out, errOut)
		}

		// 16. If both root and child require token: satisfying root only fails child
		out, errOut, code = runCLI("--token=root-val", "bothreq")
		if code == 0 || !strings.Contains(errOut, "required flag --token not provided") {
			t.Fatalf("app --token=root-val bothreq must fail child required token: code=%d out=%q err=%q", code, out, errOut)
		}

		// 17. If both root and child require token: satisfying both succeeds
		out, errOut, code = runCLI("--token=root-val", "bothreq", "--token=child-val")
		if code != 0 || !strings.Contains(out, "bothreq: token=child-val") {
			t.Fatalf("app --token=root-val bothreq --token=child-val failed: code=%d out=%q err=%q", code, out, errOut)
		}

		// 18. Missing child required flag does not commit staged parent I/O or state
		unwritten1 := filepath.Join(dir, "never_created_both.txt")
		_, _, code = runCLI("--token=root-val", "--out="+unwritten1, "bothreq")
		if code == 0 {
			t.Fatalf("bothreq missing child token must fail")
		}
		if _, err := os.Stat(unwritten1); !os.IsNotExist(err) {
			t.Fatalf("file %s was created despite child required flag failure", unwritten1)
		}

		// 19. Missing root required flag does not commit staged child I/O or state
		unwritten2 := filepath.Join(dir, "never_created_shadow.txt")
		_, _, code = runCLI("--out="+unwritten2, "shadow", "--token=child-val")
		if code == 0 {
			t.Fatalf("shadow missing root token must fail")
		}
		if _, err := os.Stat(unwritten2); !os.IsNotExist(err) {
			t.Fatalf("file %s was created despite root required flag failure", unwritten2)
		}
	}
}
