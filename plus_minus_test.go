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
			} else if selection == "default" && !reflect.DeepEqual(files, sourceFiles) {
				t.Fatal("source/default selection differ")
			}
			for _, name := range []string{"root.go", "plus_minus.go", "gnu_pm.go", "templates/app_usage.txt"} {
				if selection == "gnu-root" && (name == "root.go" || name == "templates/app_usage.txt") {
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
		{"short value", "count: --count", "count: -n --count", `command app: plus-minus short name "n" requires a scalar boolean and an ASCII letter other than h`},
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
		{"value short", []*model.FunctionParameter{{Name: "count", Type: "int", FlagAliases: []string{"n"}}}, false},
		{"unicode", []*model.FunctionParameter{{Name: "alpha", Type: "bool", FlagAliases: []string{"é"}}}, false},
		{"help", []*model.FunctionParameter{{Name: "help", Type: "bool"}}, false},
		{"short help", []*model.FunctionParameter{{Name: "alpha", Type: "bool", FlagAliases: []string{"h"}}}, false},
		{"boolean slice", []*model.FunctionParameter{{Name: "toggles", Type: "[]bool"}}, false},
		{"custom boolean", []*model.FunctionParameter{{Name: "toggle", Type: "bool", Parser: model.ParserConfig{Type: model.ParserTypeCustom, Func: &model.FuncRef{FunctionName: "Parse"}}}}, false},
		{"long value", []*model.FunctionParameter{{Name: "count", Type: "int"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validatePlusMinusParameters("app", tc.parameters); (err == nil) != tc.valid {
				t.Fatalf("error=%v valid=%t", err, tc.valid)
			}
		})
	}
}
