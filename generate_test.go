package go_subcommand

import (
	"bytes"
	_ "embed"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
)

//go:embed testdata/issue_runtime.go
var issueRuntimeSource string

//go:embed testdata/issue_runtime_parser.go
var issueRuntimeParserSource string

//go:embed testdata/issue_runtime_test.go
var issueRuntimeTestSource string

//go:embed testdata/alias_io.go
var aliasIOSource string

//go:embed testdata/alias_root_io.go
var aliasRootIOSource string

//go:embed testdata/issue478_nested_commands.go
var issue478NestedCommandsSource string

//go:embed testdata/issue475_root.go
var issue475RootSource string

//go:embed testdata/issue475_linux.go
var issue475LinuxSource string

//go:embed testdata/issue475_windows.go
var issue475WindowsSource string


//go:embed testdata/issue456_imports.go
var issue456ImportsSource string

//go:embed testdata/output_inventory.go
var outputInventorySource string

func TestGenerate_Recursive(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":     &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go":    &fstest.MapFile{Data: []byte("package main\n// Root is a subcommand `app`\nfunc Root() {}\n")},
		"sub/sub.go": &fstest.MapFile{Data: []byte("package sub\n// Sub is a subcommand `app sub`\nfunc Sub() {}\n")},
	}

	// Test recursive=true (default)
	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fsys, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, ok := writer.Files["cmd/app/sub.go"]; !ok {
		t.Errorf("Expected sub.go to be generated with recursive=true")
	}

	// Test recursive=false
	writer = NewCollectingFileWriter()
	err = GenerateWithFS(fsys, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: false}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, ok := writer.Files["cmd/app/sub.go"]; ok {
		t.Errorf("Expected sub.go NOT to be generated with recursive=false")
	}
}

func TestGenerateWithFSSuppressGoGenerate(t *testing.T) {
	input := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"app.go": &fstest.MapFile{Data: []byte("package app\n\n// Root is a subcommand `app`\nfunc Root() {}\n")},
	}

	generate := func(options ...any) string {
		t.Helper()
		writer := NewCollectingFileWriter()
		if err := GenerateWithFS(input, writer, ".", "", "commentv1", nil, false, false, nil, false, false, "", "", "", options...); err != nil {
			t.Fatalf("GenerateWithFS: %v", err)
		}
		return string(writer.Files["cmd/app/main.go"])
	}

	defaultOutput := generate()
	if !strings.Contains(defaultOutput, "//go:generate") {
		t.Fatalf("default generated main.go did not contain its regeneration directive:\n%s", defaultOutput)
	}

	suppressedOutput := generate(GenerateOptions{SuppressGoGenerate: true})
	if strings.Contains(suppressedOutput, "//go:generate") {
		t.Fatalf("suppressed generated main.go still contained its regeneration directive:\n%s", suppressedOutput)
	}

	if repeatedOutput := generate(GenerateOptions{SuppressGoGenerate: true}); suppressedOutput != repeatedOutput {
		t.Fatal("suppressed generation was not deterministic")
	}
}

func TestGenerate_Paths(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":      &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go":     &fstest.MapFile{Data: []byte("package main\n// Root is a subcommand `app`\nfunc Root() {}\n")},
		"pkg1/cmd.go": &fstest.MapFile{Data: []byte("package pkg1\n// Cmd1 is a subcommand `app cmd1`\nfunc Cmd1() {}\n")},
		"pkg2/cmd.go": &fstest.MapFile{Data: []byte("package pkg2\n// Cmd2 is a subcommand `app cmd2`\nfunc Cmd2() {}\n")},
	}

	// Test with specific path
	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fsys, writer, ".", "", "commentv1", &parsers.ParseOptions{
		SearchPaths: []string{"pkg1"},
		Recursive:   true,
	}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if _, ok := writer.Files["cmd/app/cmd1.go"]; !ok {
		t.Errorf("Expected cmd1.go to be generated")
	}
	if _, ok := writer.Files["cmd/app/cmd2.go"]; ok {
		t.Errorf("Expected cmd2.go NOT to be generated")
	}
}

func TestGenerate_TargetBuildConstraints(t *testing.T) {
	for _, target := range []struct {
		name      string
		goos      string
		wantCall  string
		avoidCall string
	}{
		{name: "linux", goos: "linux", wantCall: "PlatformLinux()", avoidCall: "PlatformWindows()"},
		{name: "windows", goos: "windows", wantCall: "PlatformWindows()", avoidCall: "PlatformLinux()"},
	} {
		t.Run(target.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/platform\n\ngo 1.22\n")
			writeRuntimeFixture(t, filepath.Join(dir, "application.go"), issue475RootSource)
			writeRuntimeFixture(t, filepath.Join(dir, "platform_linux.go"), issue475LinuxSource)
			writeRuntimeFixture(t, filepath.Join(dir, "platform_windows.go"), issue475WindowsSource)

			parseOptions := &parsers.ParseOptions{Recursive: true, GOOS: target.goos, GOARCH: "amd64"}
			if err := GenerateWithFS(os.DirFS(dir), &OSFileWriter{}, dir, "", "commentv1", parseOptions, false, false, nil, false, false, "", "", ""); err != nil {
				t.Fatalf("GenerateWithFS: %v", err)
			}
			generatedPath := filepath.Join(dir, "cmd", "platform", "platform.go")
			generated, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("read generated command: %v", err)
			}
			if !strings.Contains(string(generated), target.wantCall) || strings.Contains(string(generated), target.avoidCall) {
				t.Fatalf("target %s generated the wrong action:\n%s", target.name, generated)
			}

			before := append([]byte(nil), generated...)
			if err := GenerateWithFS(os.DirFS(dir), &OSFileWriter{}, dir, "", "commentv1", parseOptions, false, false, nil, false, false, "", "", ""); err != nil {
				t.Fatalf("repeat GenerateWithFS: %v", err)
			}
			after, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("read repeated generated command: %v", err)
			}
			if string(before) != string(after) {
				t.Fatalf("target %s generation was not deterministic", target.name)
			}

			build := exec.Command("go", "build", "./...")
			build.Dir = dir
			build.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH=amd64")
			if output, err := build.CombinedOutput(); err != nil {
				t.Fatalf("cross-build target %s: %v\n%s", target.name, err, output)
			}
		})
	}
}

func TestGenerate_GoVersionCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name       string
		goVersion  string
		wantAny    bool
		wantSlices bool
		wantErr    string
	}{
		{name: "minimum supported", goVersion: "1.16", wantAny: false, wantSlices: false},
		{name: "pre any", goVersion: "1.17", wantAny: false, wantSlices: false},
		{name: "any", goVersion: "1.18", wantAny: true, wantSlices: false},
		{name: "slices", goVersion: "1.21", wantAny: true, wantSlices: true},
		{name: "unsupported", goVersion: "1.15", wantErr: "generated applications require Go 1.16"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := fstest.MapFS{
				"go.mod": &fstest.MapFile{Data: []byte("module example.com/app\n\ngo " + tt.goVersion + "\n")},
				"app.go": &fstest.MapFile{Data: []byte("package app\n// Root is a subcommand `app`\nfunc Root() {}\n")},
			}
			writer := NewCollectingFileWriter()
			err := GenerateWithFS(input, writer, ".", "", "commentv1", nil, false, false, nil, false, false, "", "", "")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("GenerateWithFS error = %v, want %q", err, tt.wantErr)
				}
				if len(writer.Files) != 0 {
					t.Fatalf("unsupported target wrote files: %v", writer.Files)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateWithFS: %v", err)
			}
			root := string(mustGeneratedFile(t, writer, "cmd/app/root.go"))
			if strings.Contains(root, "data any") != tt.wantAny {
				t.Fatalf("Go %s any output mismatch:\n%s", tt.goVersion, root)
			}
			if strings.Contains(root, "slices.Contains") != tt.wantSlices {
				t.Fatalf("Go %s slices output mismatch:\n%s", tt.goVersion, root)
			}
			if !tt.wantSlices && strings.Contains(root, "\"slices\"") {
				t.Fatalf("Go %s unexpectedly imports slices:\n%s", tt.goVersion, root)
			}
		})
	}
}

func TestGenerate_RuntimeRequirements(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/e2e\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issueRuntimeSource)
	writeRuntimeFixture(t, filepath.Join(dir, "parserpkg", "parser.go"), issueRuntimeParserSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	generatedChild, err := os.ReadFile(filepath.Join(dir, "cmd", "app", "parent_child.go"))
	if err != nil {
		t.Fatalf("read generated child command: %v", err)
	}
	assertIssue268GeneratedChild(t, string(generatedChild))
	writeRuntimeFixture(t, filepath.Join(dir, "cmd", "app", "runtime_test.go"), issueRuntimeTestSource)

	cmd := exec.Command("go", "test", "./...")

	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		generatedTest, readErr := os.ReadFile(filepath.Join(dir, "cmd", "app", "runtime_test.go"))
		if readErr != nil {
			t.Fatalf("generated module tests failed: %v\n%s", err, output)
		}
		t.Fatalf("generated module tests failed: %v\n%s\nGenerated test:\n%s", err, output, generatedTest)
	}
}

func assertIssue268GeneratedChild(t *testing.T, source string) {
	t.Helper()
	start := strings.Index(source, "type ParentChild struct {")
	if start == -1 {
		t.Fatalf("generated child struct missing:\n%s", source)
	}
	end := strings.Index(source[start:], "\n}\n")
	if end == -1 {
		t.Fatalf("generated child struct is not terminated:\n%s", source[start:])
	}
	structSource := source[start : start+end+3]
	for _, expected := range []string{"*Parent", "Child(c.dir, c.sameDir,"} {
		if !strings.Contains(source, expected) {
			t.Errorf("generated child missing %q:\n%s", expected, source)
		}
	}
	for _, unexpected := range []string{"\n\td string", "\n\tsameDir string"} {
		if strings.Contains(structSource, unexpected) {
			t.Errorf("generated child has inherited local field %q:\n%s", unexpected, structSource)
		}
	}
	for _, unexpected := range []string{"case \"dir\":", "case \"same-dir\":", "&v.d, \"dir\"", "&v.sameDir, \"same-dir\""} {
		if strings.Contains(source, unexpected) {
			t.Errorf("generated child registers inherited flag %q:\n%s", unexpected, source)
		}
	}
}

func TestGenerate_AliasedIOCompiles(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/aliasio\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app", "app.go"), aliasIOSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	generatedPath := filepath.Join(dir, "cmd", "aliasio", "run.go")
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated command: %v", err)
	}
	generatedSource := string(generated)
	if got := strings.Count(generatedSource, `"io"`); got != 1 {
		t.Fatalf("io import count = %d, want 1\n%s", got, generatedSource)
	}
	for _, typeName := range []string{"io.Reader", "io.Writer", "io.ReadCloser", "io.WriteCloser"} {
		if !strings.Contains(generatedSource, typeName) {
			t.Errorf("generated source missing %s\n%s", typeName, generatedSource)
		}
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated aliased module did not compile: %v\n%s", err, output)
	}
}

func TestGenerate_AliasedRootIOCompiles(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/aliasroot\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app", "app.go"), aliasRootIOSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	generatedPath := filepath.Join(dir, "cmd", "aliasroot", "root.go")
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated root command: %v", err)
	}
	generatedSource := string(generated)
	if got := strings.Count(generatedSource, `"io"`); got != 1 {
		t.Fatalf("root io import count = %d, want 1\n%s", got, generatedSource)
	}
	if strings.Contains(generatedSource, `stream "io"`) {
		t.Fatalf("root command retained conflicting io alias\n%s", generatedSource)
	}
	for _, typeName := range []string{"io.Reader", "io.Writer"} {
		if !strings.Contains(generatedSource, typeName) {
			t.Errorf("generated root source missing %s\n%s", typeName, generatedSource)
		}
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated aliased root module did not compile: %v\n%s", err, output)
	}
}

func writeRuntimeFixture(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatalf("create %q parent: %v", name, err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", name, err)
	}
}

func TestGenerate_ReplaceTemplates(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":              &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go":             &fstest.MapFile{Data: []byte("package main\n// Root is a subcommand `app` -- Custom App\nfunc Root() {}\n")},
		"custom_usage.gotmpl": &fstest.MapFile{Data: []byte("OVERRIDDEN USAGE FOR {{.FullUsageString}}")},
	}

	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fsys, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, []string{"usage=custom_usage.gotmpl"}, false, false, "", "", "", fsys)
	if err != nil {
		t.Fatalf("GenerateWithFS with replaceTemplates failed: %v", err)
	}

	usageContent, ok := writer.Files["cmd/app/templates/app_usage.txt"]
	if !ok {
		t.Fatalf("Expected cmd/app/templates/app_usage.txt to be generated")
	}

	if string(usageContent) != "OVERRIDDEN USAGE FOR app" {
		t.Errorf("Expected 'OVERRIDDEN USAGE FOR app', got %q", string(usageContent))
	}
}

func TestCollectingFileWriter_ReadDir(t *testing.T) {
	writer := NewCollectingFileWriter()

	_ = writer.WriteFile(filepath.Join("dir", "file1.txt"), []byte("content1"), 0o644)
	_ = writer.WriteFile(filepath.Join("dir", "file2.txt"), []byte("content2"), 0o644)
	_ = writer.WriteFile(filepath.Join("dir", "subdir", "file3.txt"), []byte("content3"), 0o644)
	_ = writer.MkdirAll(filepath.Join("dir", "emptydir"), 0o755)
	_ = writer.WriteFile("rootfile.txt", []byte("root"), 0o644)

	entries, err := writer.ReadDir("dir")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("Expected 4 entries, got %d", len(entries))
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	expectedNames := []string{"file1.txt", "file2.txt", "subdir", "emptydir"}
	for _, expectedName := range expectedNames {
		found := false
		for _, name := range names {
			if name == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected entry %q not found in %v", expectedName, names)
		}
	}
}

func TestOSFileWriter_ReadDir(t *testing.T) {
	mockFS := fstest.MapFS{
		"testdir/file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"testdir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
		"testdir/subdir":    &fstest.MapFile{Mode: 0o755 | os.ModeDir},
	}

	writer := &OSFileWriter{}

	entries, err := writer.ReadDir("testdir", mockFS)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	expectedNames := []string{"file1.txt", "file2.txt", "subdir"}
	for _, expectedName := range expectedNames {
		found := false
		for _, name := range names {
			if name == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected entry %q not found in %v", expectedName, names)
		}
	}
}

func TestGenerate_DefaultExpressions(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte("package main\n// Root is a subcommand `app`\n// Flags:\n//   cores: (default: runtime.NumCPU())\n//   limit: (default: math.MaxInt32)\nfunc Root(cores int, limit int) {}\n")},
	}

	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fsys, writer, ".", "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "")
	if err != nil {
		t.Fatalf("GenerateWithFS failed: %v", err)
	}

	content, ok := writer.Files["cmd/app/root.go"]
	if !ok {
		t.Fatalf("root.go not generated")
	}

	s := string(content)
	if !strings.Contains(s, "\"runtime\"") {
		t.Errorf("Missing import for runtime")
	}
	if !strings.Contains(s, "\"math\"") {
		t.Errorf("Missing import for math")
	}
	if !strings.Contains(s, "runtime.NumCPU()") {
		t.Errorf("Missing assignment for cores expression: %s", s)
	}
	if !strings.Contains(s, "math.MaxInt32") {
		t.Errorf("Missing assignment for limit expression: %s", s)
	}
}

func TestGenerate_ImportIdentifierPlanCompilesCollidingRootAndSubcommand(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/importplan\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issue456ImportsSource)
	writeRuntimeFixture(t, filepath.Join(dir, "deps", "flag", "flag.go"), `package flag

type Value string

const Count = 1

func Parse(value string) (Value, error) { return Value(value), nil }
`)
	writeRuntimeFixture(t, filepath.Join(dir, "deps", "first", "shared", "shared.go"), "package shared\n\nfunc Parse(value string) (string, error) { return value, nil }\n")
	writeRuntimeFixture(t, filepath.Join(dir, "deps", "second", "shared", "shared.go"), "package shared\n\nfunc Parse(value string) (string, error) { return value, nil }\nfunc Generate() (string, error) { return \"generated\", nil }\n")

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	rootSource, err := os.ReadFile(filepath.Join(dir, "cmd", "plan", "root.go"))
	if err != nil {
		t.Fatalf("read generated root: %v", err)
	}
	for _, want := range []string{`pkg_os "example.com/importplan"`, "pkg_os.Root", `pkg_flag "example.com/importplan/deps/flag"`, "pkg_flag.Count"} {
		if !strings.Contains(string(rootSource), want) {
			t.Errorf("generated root missing %q:\n%s", want, rootSource)
		}
	}
	if strings.Contains(string(rootSource), "flag.Count") && !strings.Contains(string(rootSource), "pkg_flag.Count") {
		t.Fatalf("root action did not use the planned alias:\n%s", rootSource)
	}

	serveSource, err := os.ReadFile(filepath.Join(dir, "cmd", "plan", "serve.go"))
	if err != nil {
		t.Fatalf("read generated subcommand: %v", err)
	}
	for _, want := range []string{
		`pkg_flag "example.com/importplan/deps/flag"`, "pkg_flag.Value", "pkg_flag.Parse(value)",
		`"example.com/importplan/deps/first/shared"`, "shared.Parse(value)",
		`shared_2 "example.com/importplan/deps/second/shared"`, "shared_2.Parse(value)", "shared_2.Generate()", "pkg_os.Serve",
	} {
		if !strings.Contains(string(serveSource), want) {
			t.Errorf("generated subcommand missing %q:\n%s", want, serveSource)
		}
	}

	command := exec.Command("go", "test", "-run=^$", "./...")
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated module with colliding imports did not compile: %v\n%s", err, output)
	}
}

func TestDeduplicateAndSortImportsPlansDeterministicUniqueIdentifiers(t *testing.T) {
	plan := deduplicateAndSortImports([]templateImport{
		{Path: "example.com/a/shared"},
		{Path: "example.com/b/shared"},
		{Path: "example.com/external/flag", Alias: "flag"},
		{Path: "flag"},
		{Path: "os"},
	})
	identifiers := importIdentifiers(plan)
	for importPath, want := range map[string]string{
		"flag":                      "flag",
		"os":                        "os",
		"example.com/a/shared":      "shared",
		"example.com/b/shared":      "shared_2",
		"example.com/external/flag": "pkg_flag",
	} {
		if got := identifiers[importPath]; got != want {
			t.Errorf("identifier for %s = %q, want %q", importPath, got, want)
		}
	}
	if repeated := importIdentifiers(deduplicateAndSortImports([]templateImport{
		{Path: "os"}, {Path: "example.com/b/shared"}, {Path: "flag"}, {Path: "example.com/external/flag", Alias: "flag"}, {Path: "example.com/a/shared"},
	})); !reflect.DeepEqual(identifiers, repeated) {
		t.Fatalf("plan changed with input order:\nfirst: %#v\nsecond: %#v", identifiers, repeated)
	}
}

func TestGenerate_ImplicitNestedCommandPathsProduceOneExecutable(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/gogenmux\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "commands.go"), issue478NestedCommandsSource)

	generate := func() {
		t.Helper()
		if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
	}
	generate()
	if _, err := os.Stat(filepath.Join(dir, "cmd", "gogenmux")); err != nil {
		t.Fatalf("single generated executable tree is missing: %v", err)
	}
	for _, unexpected := range []string{"generate", "inspect", "validate", "show"} {
		if _, err := os.Stat(filepath.Join(dir, "cmd", unexpected)); !os.IsNotExist(err) {
			t.Fatalf("generated a separate %q executable tree", unexpected)
		}
	}

	compile := exec.Command("go", "test", "-run=^$", "./...")
	compile.Dir = dir
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("generated nested module did not compile: %v\n%s", err, output)
	}
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("go", append([]string{"run", "./cmd/gogenmux"}, args...)...)
		command.Dir = dir
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("run %v: %v\n%s", args, err, output)
		}
		return string(output)
	}
	for _, scenario := range []struct {
		args []string
		want string
	}{
		{args: []string{"generate", "--name", "Ada"}, want: "generate:Ada"},
		{args: []string{"inspect", "README.md"}, want: "inspect:README.md"},
		{args: []string{"validate", "--strict"}, want: "validate:true"},
		{args: []string{"config", "show", "--format", "json"}, want: "show:json"},
		{args: []string{"help"}, want: "gogenmux <subcommand>"},
	} {
		if output := run(scenario.args...); !strings.Contains(output, scenario.want) {
			t.Errorf("run %v output = %q, want %q", scenario.args, output, scenario.want)
		}
	}

	first, err := os.ReadFile(filepath.Join(dir, "cmd", "gogenmux", "root.go"))
	if err != nil {
		t.Fatalf("read first generated root: %v", err)
	}
	generate()
	second, err := os.ReadFile(filepath.Join(dir, "cmd", "gogenmux", "root.go"))
	if err != nil {
		t.Fatalf("read repeated generated root: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("repeat implicit nested generation changed root.go")
	}
}

func TestGenerate_Clean(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/cleantest\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issueRuntimeSource)
	writeRuntimeFixture(t, filepath.Join(dir, "parserpkg", "parser.go"), issueRuntimeParserSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	customFile := filepath.Join(dir, "cmd", "app", "custom.go")
	writeRuntimeFixture(t, customFile, "package app\n// Custom user file\n")

	if err := Generate(dir, "", "commentv1", nil, true, true, true, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate with clean failed: %v", err)
	}

	if _, err := os.Stat(customFile); os.IsNotExist(err) {
		t.Errorf("Custom user file custom.go was deleted by --clean!")
	}

	generatedFile := filepath.Join(dir, "cmd", "app", "main.go")
	if _, err := os.Stat(generatedFile); os.IsNotExist(err) {
		t.Errorf("Generated main.go was not created after clean")
	}
}

func TestGetProvenance(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1234567890")

	prov := GetProvenance([]string{"usage=foo.txt"}, true, true, "", "", "")
	if prov.Timestamp != "1234567890" {
		t.Errorf("Expected Timestamp 1234567890, got %s", prov.Timestamp)
	}
	if prov.TemplateID != "usage=foo.txt" {
		t.Errorf("Expected TemplateID usage=foo.txt, got %s", prov.TemplateID)
	}
}

func TestGetProvenanceExtensive(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")
	prov := GetProvenance(nil, false, false, "", "", "")
	if prov.ProjectCommit != "" || prov.Timestamp != "" || prov.ReplacedTemplates {
		t.Errorf("Unexpected default provenance values: %+v", prov)
	}

	prov = GetProvenance(nil, false, true, "", "", "")
	if prov.Timestamp == "" {
		t.Error("Expected timestamp to be generated")
	}

	t.Setenv("SOURCE_DATE_EPOCH", "987654321")
	prov = GetProvenance(nil, false, false, "", "", "")
	if prov.Timestamp != "987654321" {
		t.Errorf("Expected 987654321, got %v", prov.Timestamp)
	}

	prov = GetProvenance([]string{"a=b", "c=d"}, false, false, "", "", "")
	if !prov.ReplacedTemplates {
		t.Error("Expected replaced templates to be true")
	}
	if prov.TemplateID != "a=b,c=d" {
		t.Errorf("Expected template ID to be a=b,c=d, got %v", prov.TemplateID)
	}
}

func TestResolveCLIParser(t *testing.T) {
	tests := []struct {
		name     string
		local    string
		fallback string
		want     string
		wantErr  bool
	}{
		{name: "implicit default", want: "gnu"},
		{name: "generator default go flag", fallback: "go-flag", want: "go-flag"},
		{name: "explicit gnu overrides unsupported generator default", local: "gnu", fallback: "go-flag", want: "gnu"},
		{name: "explicit go flag", local: "go-flag", fallback: "gnu", want: "go-flag"},
		{name: "plus minus reserved", local: "plus-minus", wantErr: true},
		{name: "unknown", local: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCLIParser(tt.local, tt.fallback)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveCLIParser(%q, %q) unexpectedly succeeded with %q", tt.local, tt.fallback, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCLIParser(%q, %q): %v", tt.local, tt.fallback, err)
			}
			if got != tt.want {
				t.Fatalf("resolveCLIParser(%q, %q) = %q, want %q", tt.local, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestResolveCLIParsers(t *testing.T) {
	root := &model.Command{MainCmdName: "app"}
	legacy := &model.SubCommand{Command: root, SubCommandName: "legacy", CLIParser: "go-flag"}
	legacyImport := &model.SubCommand{Command: root, Parent: legacy, SubCommandName: "import"}
	modern := &model.SubCommand{Command: root, SubCommandName: "modern"}
	gnuOverride := &model.SubCommand{Command: root, Parent: legacy, SubCommandName: "gnu-again", CLIParser: "gnu"}
	legacy.SubCommands = []*model.SubCommand{legacyImport, gnuOverride}
	root.SubCommands = []*model.SubCommand{modern, legacy}

	if err := resolveCLIParsers([]*model.Command{root}, "gnu"); err != nil {
		t.Fatalf("resolveCLIParsers: %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{name: "root", got: root.ResolvedCLIParser, want: "gnu"},
		{name: "modern inherits root", got: modern.ResolvedCLIParser, want: "gnu"},
		{name: "legacy override", got: legacy.ResolvedCLIParser, want: "go-flag"},
		{name: "legacy child inherits nearest", got: legacyImport.ResolvedCLIParser, want: "go-flag"},
		{name: "descendant overrides back", got: gnuOverride.ResolvedCLIParser, want: "gnu"},
	}

	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s resolved parser = %q, want %q", check.name, check.got, check.want)
		}
	}
}

func TestGenerateWithFSSucceedsImplementedCLIParser(t *testing.T) {
	tests := []struct {
		name         string
		comment      string
		cliParser    string
		expectParser string
		ops          []any
	}{
		{
			name:         "generation default empty, implicit gnu",
			cliParser:    "",
			expectParser: "cli_parser_gnu",
		},
		{
			name:         "generation default explicit go-flag",
			cliParser:    "go-flag",
			expectParser: "cli_parser_go-flag",
		},
		{
			name:         "command metadata",
			cliParser:    "gnu",
			comment:      "// CLI-Parser: go-flag\n",
			expectParser: "cli_parser_go-flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fstest.MapFS{
				"go.mod":  &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
				"main.go": &fstest.MapFile{Data: []byte("package main\n\n// Root is a subcommand `app`\n" + tt.comment + "func Root() {}\n")},
			}
			writer := NewCollectingFileWriter()
			err := GenerateWithFS(input, writer, ".", "", "commentv1", nil, false, false, nil, false, false, "", "", "", append(tt.ops, GenerateOptions{CLIParser: tt.cliParser})...)
			if err != nil {
				t.Fatalf("GenerateWithFS unexpectedly failed with implemented go-flag: %v", err)
			}
			if len(writer.Files) == 0 {
				t.Fatalf("GenerateWithFS wrote %d files after accepting go-flag", len(writer.Files))
			}

			generatedContent := string(writer.Files["cmd/app/root.go"])

			if tt.expectParser == "cli_parser_go-flag" && !strings.Contains(generatedContent, "fs.Parse(args)") {
				t.Fatalf("Expected output to contain Go flag parser logic, got: \n%s", generatedContent)
			}
		})
	}
}

func TestGenerate_RootPointerParametersCompilesAndRuns(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/pointerroot\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app", "app.go"), `package app

import "time"

// App is a subcommand `+"`pointerroot`"+`.
func App(
	strVal *string, // flag: --str
	intVal *int, // flag: --int
	int64Val *int64, // flag: --int64
	floatVal *float64, // flag: --float
	boolVal *bool, // flag: --bool
	durVal *time.Duration, // flag: --dur
) error {
	return nil
}
`)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	generatedTestPath := filepath.Join(dir, "cmd", "pointerroot", "root_test.go")
	generatedTest, err := os.ReadFile(generatedTestPath)
	if err != nil {
		t.Fatalf("read generated root test: %v", err)
	}
	testSource := string(generatedTest)

	for _, expectedAssertion := range []string{
		"cmd.strVal == nil",
		"*cmd.strVal != \"test\"",
		"cmd.intVal == nil",
		"*cmd.intVal != 1",
		"cmd.int64Val == nil",
		"*cmd.int64Val != 1",
		"cmd.floatVal == nil",
		"*cmd.floatVal != 1.5",
		"cmd.boolVal == nil",
		"*cmd.boolVal != true",
		"cmd.durVal == nil",
		"*cmd.durVal != 1*time.Second",
	} {
		if !strings.Contains(testSource, expectedAssertion) {
			t.Errorf("generated test missing assertion %q\n%s", expectedAssertion, testSource)
		}
	}

	cmd := exec.Command("go", "test", "-v", "./...")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated pointer root module tests failed: %v\n%s\nGenerated test:\n%s", err, output, testSource)
	}
}

func TestRuntimeParserImports(t *testing.T) {
	params1 := []*model.FunctionParameter{
		{Type: "string", Name: "name"},
		{Type: "int", Name: "age"},
		{Type: "bool", Name: "verbose"},
	}
	cmd1 := &model.Command{Parameters: params1, ResolvedCLIParser: "go-flag"}

	imports1 := commandImports(cmd1, "")
	for _, imp := range append(imports1.Standard, imports1.Other...) {
		if imp.Path == "strings" || imp.Path == "strconv" {
			t.Errorf("Test 1: unexpected import %s", imp.Path)
		}
	}

	params2 := []*model.FunctionParameter{
		{Type: "*int", Name: "age"},
	}
	cmd2 := &model.Command{Parameters: params2, ResolvedCLIParser: "go-flag"}

	imports2 := commandImports(cmd2, "")
	hasStrconv2 := false
	for _, imp := range append(imports2.Standard, imports2.Other...) {
		if imp.Path == "strconv" {
			hasStrconv2 = true
		}
	}
	if !hasStrconv2 {
		t.Errorf("Test 2: expected strconv import for *int in go-flag")
	}

	params3 := []*model.FunctionParameter{
		{Type: "int", Name: "age"},
	}
	cmd3 := &model.Command{Parameters: params3, ResolvedCLIParser: "gnu"}

	imports3 := commandImports(cmd3, "")
	hasStrconv3 := false
	for _, imp := range append(imports3.Standard, imports3.Other...) {
		if imp.Path == "strconv" {
			hasStrconv3 = true
		}
	}

	if !hasStrconv3 {
		t.Errorf("Test 3: expected strconv import for int in gnu")
	}

	params4 := []*model.FunctionParameter{
		{Type: "bool", Name: "verbose"},
	}
	cmd4 := &model.Command{Parameters: params4, ResolvedCLIParser: "gnu"}

	imports4 := commandImports(cmd4, "")
	hasStrconv4 := false
	for _, imp := range append(imports4.Standard, imports4.Other...) {
		if imp.Path == "strconv" {
			hasStrconv4 = true
		}
	}
	if !hasStrconv4 {
		t.Errorf("Test 4: expected strconv import for bool in gnu")
	}
}

func generateWithOverlay(
	t *testing.T,
	source string,
	cliParser string,
	replacements []string,
	extra fstest.MapFS,
) map[string][]byte {
	t.Helper()
	fsys := fstest.MapFS{
		"go.mod":  &fstest.MapFile{Data: []byte("module example.com/test\n\ngo 1.22\n")},
		"main.go": &fstest.MapFile{Data: []byte(source)},
	}
	for k, v := range extra {
		fsys[k] = v
	}
	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fsys, writer, ".", "", "commentv1", nil, false, false, replacements, false, false, "", "", "", fsys, GenerateOptions{CLIParser: cliParser})
	if err != nil {
		t.Fatalf("GenerateWithFS failed: %v", err)
	}
	return writer.Files
}

func TestReplaceCLIParserTemplate(t *testing.T) {
	t.Run("Replacing GNU does not remove Go-flag", func(t *testing.T) {
		source := "package main\n\n// Root is a subcommand `app`\nfunc Root() {}\n"
		extra := fstest.MapFS{
			"custom-gnu.gotmpl": &fstest.MapFile{Data: []byte("{{- define \"cli_parser_gnu\" }}\n// CUSTOM GNU INJECTED\n{{- end }}")},
		}

		files := generateWithOverlay(t, source, "go-flag", []string{"cli-parsers/gnu.gotmpl=custom-gnu.gotmpl"}, extra)

		content := string(files["cmd/app/root.go"])
		if !strings.Contains(content, "fs.Parse(args)") {
			t.Errorf("Expected go-flag parser logic (fs.Parse(args)), got:\n%s", content)
		}
	})

	t.Run("Replacing an unrelated template preserves both parser backends", func(t *testing.T) {
		source := "package main\n\n// Root is a subcommand `app`\n// CLI-Parser: gnu\nfunc Root() {}\n\n// Child is a subcommand `app child`\n// CLI-Parser: go-flag\nfunc Child() {}\n"
		extra := fstest.MapFS{
			"custom-usage.gotmpl": &fstest.MapFile{Data: []byte("{{- define \"usage\" }}\nCUSTOM USAGE\n{{- end }}")},
		}

		files := generateWithOverlay(t, source, "gnu", []string{"usage=custom-usage.gotmpl"}, extra)

		rootContent := string(files["cmd/app/root.go"])
		if !strings.Contains(rootContent, "var remainingArgs []string") {
			t.Errorf("Expected gnu parser logic in root, got:\n%s", rootContent)
		}

		childContent := string(files["cmd/app/child.go"])
		if !strings.Contains(childContent, "fs.Parse(args)") {
			t.Errorf("Expected go-flag parser logic in child, got:\n%s", childContent)
		}
	})

	t.Run("Go-flag remains selectable after unrelated overlay", func(t *testing.T) {
		source := "package main\n\n// Root is a subcommand `app`\nfunc Root() {}\n"
		extra := fstest.MapFS{
			"custom-usage.gotmpl": &fstest.MapFile{Data: []byte("{{- define \"usage\" }}\nCUSTOM USAGE\n{{- end }}")},
		}

		files := generateWithOverlay(t, source, "go-flag", []string{"usage=custom-usage.gotmpl"}, extra)

		content := string(files["cmd/app/root.go"])
		if !strings.Contains(content, "fs.Parse(args)") {
			t.Errorf("Expected go-flag parser logic, got:\n%s", content)
		}
	})
}

func TestGenerate_GoFlagRuntimeFeatures(t *testing.T) {
	dir := t.TempDir()

	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/gff\n\ngo 1.22\n")

	const tick = "\x60"
	appCode := "package app\n\nimport (\n\t\"errors\"\n\t\"fmt\"\n\t\"io\"\n\t\"strings\"\n\t\"time\"\n)\n\n// Root is a subcommand " + tick + "app" + tick + "\n// CLI-Parser: go-flag\n// Flags:\n//\treq: (required)\n//\tdefVal: (default: \"default_str\")\n//\tnum: \n//\tdur: \n//\tptr: \n//\tslc: \n//\tmyParserVal: (parser: MyParser)\n//\tfailAction: \n//\tinputFile: \n//\toutputFile: \nfunc Root(req string, defVal string, num int, dur time.Duration, ptr *string, slc []string, myParserVal string, failAction bool, inputFile io.Reader, outputFile io.Writer) error {\n\tif failAction {\n\t\treturn GenerateActionError\n\t}\n\n\tfmt.Printf(\"req=%s\\n\", req)\n\tfmt.Printf(\"defVal=%s\\n\", defVal)\n\tfmt.Printf(\"num=%d\\n\", num)\n\tfmt.Printf(\"dur=%v\\n\", dur)\n\n\tif ptr != nil {\n\t\tfmt.Printf(\"ptr=%s\\n\", *ptr)\n\t} else {\n\t\tfmt.Printf(\"ptr=<nil>\\n\")\n\t}\n\n\tfmt.Printf(\"slc=%v\\n\", slc)\n\n\tif myParserVal != \"\" {\n\t\tcontent := myParserVal\n\t\tfmt.Printf(\"myParserVal=%s\\n\", content)\n\t} else {\n\t\tfmt.Printf(\"myParserVal=<empty>\\n\")\n\t}\n\n\tif inputFile != nil {\n\t\tcontent, _ := io.ReadAll(inputFile)\n\t\tfmt.Printf(\"inputFile=%s\\n\", strings.TrimSpace(string(content)))\n\t}\n\n\tif outputFile != nil {\n\t\toutputFile.Write([]byte(\"written_data\"))\n\t}\n\n\treturn nil\n}\n\n// Child is a subcommand " + tick + "app child" + tick + "\n// CLI-Parser: gnu\n// Flags:\n//\treq: (from parent)\nfunc Child(parentReq string, slc []string) error {\n\tfmt.Printf(\"child_req=%s\\n\", parentReq)\n\tfmt.Printf(\"child_slc=%v\\n\", slc)\n\treturn nil\n}\n\nfunc MyParser(val string) (string, error) {\n\tif val == \"fail_reader\" {\n\t\treturn \"\", errors.New(\"parser error\")\n\t}\n\tif val == \"\" {\n\t\treturn \"\", nil\n\t}\n\treturn \"PREFIX_\" + val, nil\n}\n\nvar GenerateActionError = errors.New(\"action sentinel error\")\n"

	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), appCode)

	if err := GenerateCLI(dir, "", "commentv1", "go-flag", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	binPath := filepath.Join(dir, "testbin")
	cmdBuild := exec.Command("go", "build", "-o", binPath, "./cmd/app")
	cmdBuild.Dir = dir
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\nOutput: %s", err, string(out))
	}

	runTest := func(name string, args []string, expectFail bool, expectStdout, expectStderr string) {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(binPath, args...)
			cmd.Dir = dir
			var outBuf, errBuf strings.Builder
			cmd.Stdout = &outBuf
			cmd.Stderr = &errBuf

			err := cmd.Run()

			if expectFail && err == nil {
				t.Fatalf("Expected failure but succeeded. Stdout: %s\nStderr: %s", outBuf.String(), errBuf.String())
			}
			if !expectFail && err != nil {
				t.Fatalf("Expected success but failed: %v\nStdout: %s\nStderr: %s", err, outBuf.String(), errBuf.String())
			}

			if expectStdout != "" && !strings.Contains(outBuf.String(), expectStdout) {
				t.Errorf("Expected stdout to contain %q, got %q", expectStdout, outBuf.String())
			}
			if expectStderr != "" && !strings.Contains(errBuf.String(), expectStderr) {
				t.Errorf("Expected stderr to contain %q, got %q", expectStderr, errBuf.String())
			}
		})
	}

	inputFile := filepath.Join(dir, "in.txt")
	_ = os.WriteFile(inputFile, []byte("hello file"), 0644)
	outputFile := filepath.Join(dir, "out.txt")

	runTest("required omitted", []string{}, true, "", "required flag --req not provided")
	runTest("required supplied", []string{"-req", "provided"}, false, "req=provided", "")
	runTest("non-zero default", []string{"-req", "provided"}, false, "defVal=default_str", "")
	runTest("numeric/duration scalar", []string{"-req", "provided", "-num", "10", "-dur", "1h"}, false, "num=10\ndur=1h0m0s", "")
	runTest("pointer", []string{"-req", "provided", "-ptr", "pointed"}, false, "ptr=pointed", "")
	runTest("pointer omitted", []string{"-req", "provided"}, false, "ptr=<nil>", "")
	runTest("slice/repeated flag", []string{"-req", "provided", "-slc", "a", "-slc", "b"}, false, "slc=[a b]", "")
	runTest("custom parser success", []string{"-req", "provided", "-my-parser-val", "mydata"}, false, "myParserVal=PREFIX_mydata", "")
	runTest("custom parser failure", []string{"-req", "provided", "-my-parser-val", "fail_reader"}, true, "", "parser error")
	runTest("explicit input file path", []string{"-req", "provided", "-input-file", inputFile}, false, "inputFile=hello file", "")
	runTest("input open failure", []string{"-req", "provided", "-input-file", "non_existent.txt"}, true, "", "non_existent.txt")
	runTest("explicit output file path", []string{"-req", "provided", "-output-file", outputFile}, false, "", "")

	// Verify output was written
	if data, err := os.ReadFile(outputFile); err != nil || string(data) != "written_data" {
		t.Errorf("Expected written_data to outputFile, got data=%s, err=%v", string(data), err)
	}

	runTest("action error propagation", []string{"-req", "provided", "-fail-action", "true"}, true, "", "action sentinel error")
	runTest("mixed boundary", []string{"-req", "parent", "child", "--slc=c"}, false, "child_req=parent\nchild_slc=[c]", "")
}

func TestGenerate_OutputInventoryIsParseableAndDeterministic(t *testing.T) {
	input := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/outputinventory\n\ngo 1.22\n")},
		"app.go": &fstest.MapFile{Data: []byte(outputInventorySource)},
	}
	generate := func(writer *CollectingFileWriter) {
		t.Helper()
		err := GenerateWithFS(input, writer, ".", "man", "commentv1", nil,
			false, false, nil, false, false, "", "", "")
		if err != nil {
			t.Fatalf("GenerateWithFS failed: %v", err)
		}
	}

	first := NewCollectingFileWriter()
	generate(first)
	want := []string{
		"cmd/agents.md",
		"cmd/app/child.go",
		"cmd/app/child_test.go",
		"cmd/app/flag_helpers.go",
		"cmd/app/main.go",
		"cmd/app/root.go",
		"cmd/app/root_test.go",
		"cmd/app/templates/app_usage.txt",
		"cmd/app/templates/child_usage.txt",
		"cmd/app/templates/templates.go",
		"cmd/errors.go",
		"man/app-child.1",
	}
	for _, name := range want {
		content, ok := first.Files[name]
		if !ok {
			t.Errorf("missing generated artifact %q; got %v", name, first.Files)
			continue
		}
		if strings.HasSuffix(name, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), name, content, parser.AllErrors); err != nil {
				t.Errorf("generated Go source %q does not parse: %v", name, err)
			}
			if strings.Contains(string(content), `"github.com/arran4/go-subcommand"`) {
				t.Errorf("generated consumer source %q has a gosubc runtime import", name)
			}
		}
	}

	second := NewCollectingFileWriter()
	generate(second)
	if len(first.Files) != len(second.Files) {
		t.Fatalf("artifact count changed on repeat generation: %d then %d", len(first.Files), len(second.Files))
	}
	for name, firstContent := range first.Files {
		secondContent, ok := second.Files[name]
		if !ok || string(firstContent) != string(secondContent) {
			t.Errorf("generated artifact %q changed on repeat generation", name)
		}
	}
}

func TestGenerate_LegacyAPI_Compiles(t *testing.T) {
	// Verify signature exists and matches compilation
	accept := func(_ func(
		dir string,
		manDir string,
		parserName string,
		paths []string,
		recursive bool,
		force bool,
		clean bool,
		replaceTemplates []string,
		projectProvenance bool,
		timestamp bool,
		provVersion string,
		provCommit string,
		provDate string,
	) error) {
	}

	accept(Generate)
}

func TestGenerateWithFS_LegacyAPI_Compiles(t *testing.T) {
	// Verify signature exists and matches compilation
	accept := func(_ func(inputFS fs.FS, writer FileWriter, dir string, manDir string, parserName string, options *parsers.ParseOptions, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, provVersion string, provCommit string, provDate string, ops ...any) error) {
	}

	accept(GenerateWithFS)
}

func TestGenerateCLI_ForwardsCLIParser(t *testing.T) {
	// 1. Verify signature exists and matches compilation
	accept := func(_ func(dir string, manDir string, parserName string, cliParser string, paths []string, recursive bool, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, provVersion string, provCommit string, provDate string) error) {
	}
	accept(GenerateCLI)

	// 2. Verify actual forwarding of --cli-parser to the internal system
	// Using the reserved identifier "plus-minus" ensures we get the exact rejection error
	// directly from resolveCLIParser to prove the string made it through GenerateCLI
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), "package app\n\n// Root is a subcommand `app`\nfunc Root() error { return nil }\n")

	err := GenerateCLI(dir, "", "commentv1", "plus-minus", nil, false, false, false, nil, false, false, "", "", "")
	if err == nil {
		t.Fatal("Expected GenerateCLI with 'plus-minus' to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("Expected 'not implemented' from CLIParser override forwarding, got: %v", err)
	}
}

func TestGenerateCLICommandControlsGoGenerateDirective(t *testing.T) {
	generate := func(t *testing.T, emitGoGenerate bool) string {
		t.Helper()
		dir := t.TempDir()
		writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/directive\n\ngo 1.22\n")
		writeRuntimeFixture(t, filepath.Join(dir, "app.go"), "package app\n\n// Root is a subcommand `app`\nfunc Root() {}\n")
		if err := GenerateCLICommand(dir, "", "commentv1", "gnu", nil, false, true, false, nil, false, false, emitGoGenerate, "", "", ""); err != nil {
			t.Fatalf("GenerateCLICommand: %v", err)
		}
		output, err := os.ReadFile(filepath.Join(dir, "cmd", "app", "main.go"))
		if err != nil {
			t.Fatalf("read generated main.go: %v", err)
		}
		return string(output)
	}

	if output := generate(t, true); !strings.Contains(output, "//go:generate") {
		t.Fatalf("--emit-go-generate=true did not emit the directive:\n%s", output)
	}
	if output := generate(t, false); strings.Contains(output, "//go:generate") {
		t.Fatalf("--emit-go-generate=false did not suppress the directive:\n%s", output)
	}
}

func TestGosubcGenerateEmitGoGenerateFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("self-hosted CLI integration test")
	}

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	generate := func(t *testing.T, emitGoGenerate bool) string {
		t.Helper()
		dir := t.TempDir()
		writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/directivecli\n\ngo 1.22\n")
		writeRuntimeFixture(t, filepath.Join(dir, "app.go"), "package app\n\n// Root is a subcommand `app`\nfunc Root() {}\n")

		command := exec.Command("go", "run", "./cmd/gosubc", "gen", "--dir", dir, "--project-provenance=false", "--timestamp=false", "--emit-go-generate="+strconv.FormatBool(emitGoGenerate))
		command.Dir = workingDir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("gosubc gen --emit-go-generate=%t: %v\n%s", emitGoGenerate, err, output)
		}

		output, err := os.ReadFile(filepath.Join(dir, "cmd", "app", "main.go"))
		if err != nil {
			t.Fatalf("read generated main.go: %v", err)
		}
		return string(output)
	}

	if output := generate(t, true); !strings.Contains(output, "//go:generate") {
		t.Fatalf("CLI --emit-go-generate=true did not emit the directive:\n%s", output)
	}
	if output := generate(t, false); strings.Contains(output, "//go:generate") {
		t.Fatalf("CLI --emit-go-generate=false did not suppress the directive:\n%s", output)
	}
}
