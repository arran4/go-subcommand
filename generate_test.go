package go_subcommand

import (
	"bytes"
	_ "embed"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
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

//go:embed testdata/issue483_root_context.go
var issue483RootContextSource string

//go:embed testdata/issue490_dispatcher_main.go
var issue490DispatcherMainSource string

//go:embed testdata/issue479_policy_app.go
var issue479PolicyAppSource string

//go:embed testdata/issue479_policy.go
var issue479PolicySource string
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

//go:embed testdata/issue513_root_collision.go
var issue513RootCollisionSource string

//go:embed testdata/issue513_aliases.go
var issue513AliasesSource string

//go:embed testdata/issue513_config1.go
var issue513Config1Source string

//go:embed testdata/issue513_config2.go
var issue513Config2Source string

//go:embed testdata/issue513_config4.go
var issue513Config4Source string

//go:embed testdata/issue513_config5.go
var issue513Config5Source string

//go:embed testdata/issue513_alias_root.go
var issue513AliasRootSource string

//go:embed testdata/output_inventory.go
var outputInventorySource string

//go:embed testdata/issue513_default_help_route.go
var issue513DefaultHelpRouteSource string

//go:embed testdata/issue513_nested_action_alias.go
var issue513NestedActionAliasSource string

//go:embed testdata/issue513_subcommand_default.go
var issue513SubcommandDefaultSource string

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

func TestGenerate_ConstructorInjectedRootContext(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/contextapp\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "application.go"), issue483RootContextSource)
	writeRuntimeFixture(t, filepath.Join(dir, "buildinfo", "buildinfo.go"), `package buildinfo

type BuildInfo struct { Version string }
`)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	rootSource, err := os.ReadFile(filepath.Join(dir, "cmd", "contextapp", "root.go"))
	if err != nil {
		t.Fatalf("read generated root: %v", err)
	}
	for _, want := range []string{
		`"example.com/contextapp/buildinfo"`,
		"buildInfo buildinfo.BuildInfo",
		"func NewRootWithContext(name, version, commit, date string, buildInfo buildinfo.BuildInfo)",
		"buildInfo: buildInfo,",
	} {
		if !strings.Contains(string(rootSource), want) {
			t.Errorf("generated root missing %q:\n%s", want, rootSource)
		}
	}
	childSource, err := os.ReadFile(filepath.Join(dir, "cmd", "contextapp", "child.go"))
	if err != nil {
		t.Fatalf("read generated child: %v", err)
	}
	if strings.Contains(string(childSource), "\n\tbuildInfo buildinfo.BuildInfo\n") {
		t.Fatalf("child redeclared root context:\n%s", childSource)
	}

	writeRuntimeFixture(t, filepath.Join(dir, "cmd", "contextapp", "context_test.go"), `package main

import (
    "testing"
    "example.com/contextapp/buildinfo"
)

func TestConstructorContextIsInherited(t *testing.T) {
    context := buildinfo.BuildInfo{Version: "v1"}
    root, err := NewRootWithContext("contextapp", "", "", "", context)
    if err != nil { t.Fatal(err) }
    if root.buildInfo != context { t.Fatalf("root context = %#v", root.buildInfo) }
    if err := root.Execute([]string{"child"}); err != nil { t.Fatalf("child: %v", err) }
    if err := root.Execute([]string{"child", "grandchild"}); err != nil { t.Fatalf("grandchild: %v", err) }
}
`)

	compile := exec.Command("go", "test", "./...")
	compile.Dir = dir
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("generated root-context module failed: %v\n%s", err, output)
	}
}

func TestGenerate_ImportableDispatcher(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/contextapp\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "application.go"), issue483RootContextSource)
	writeRuntimeFixture(t, filepath.Join(dir, "buildinfo", "buildinfo.go"), `package buildinfo

type BuildInfo struct { Version string }
`)

	err := GenerateWithFS(os.DirFS(dir), &OSFileWriter{}, dir, "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "", GenerateOptions{DispatcherPackage: "dispatch"})
	if err != nil {
		t.Fatalf("GenerateWithFS dispatcher mode failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "cmd", "contextapp", "main.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dispatcher mode generated main.go, stat error = %v", err)
	}
	rootSource, err := os.ReadFile(filepath.Join(dir, "cmd", "contextapp", "root.go"))
	if err != nil {
		t.Fatalf("read generated dispatcher root: %v", err)
	}
	if !strings.Contains(string(rootSource), "package dispatch") {
		t.Fatalf("dispatcher root is not importable:\n%s", rootSource)
	}
	if _, err := os.Stat(filepath.Join(dir, "cmd", "contextapp", "templates", "contextapp_usage.txt")); err != nil {
		t.Fatalf("dispatcher usage resource missing: %v", err)
	}

	writeRuntimeFixture(t, filepath.Join(dir, "launcher", "main.go"), issue490DispatcherMainSource)
	writeRuntimeFixture(t, filepath.Join(dir, "launcher", "main_test.go"), `package main

import "testing"

func TestExternalMainUsesGeneratedDispatcher(t *testing.T) {
	if err := run([]string{"help"}); err != nil {
		t.Fatalf("help dispatch: %v", err)
	}
	if err := run([]string{"child"}); err != nil {
		t.Fatalf("child dispatch: %v", err)
	}
	if err := run([]string{"child", "grandchild"}); err != nil {
		t.Fatalf("nested dispatch: %v", err)
	}
}
`)

	compile := exec.Command("go", "test", "./...")
	compile.Dir = dir
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("generated dispatcher module failed: %v\n%s", err, output)
	}
	deps := exec.Command("go", "list", "-deps", "./launcher")
	deps.Dir = dir
	if output, err := deps.CombinedOutput(); err != nil {
		t.Fatalf("list generated dispatcher dependencies: %v\n%s", err, output)
	} else if strings.Contains(string(output), "github.com/arran4/go-subcommand") {
		t.Fatalf("generated dispatcher retained a gosubc runtime dependency:\n%s", output)
	}

	second := NewCollectingFileWriter()
	if err := GenerateWithFS(os.DirFS(dir), second, dir, "", "commentv1", &parsers.ParseOptions{Recursive: true}, false, false, nil, false, false, "", "", "", GenerateOptions{DispatcherPackage: "dispatch"}); err != nil {
		t.Fatalf("repeat dispatcher generation failed: %v", err)
	}
	if _, ok := second.Files[filepath.Join(dir, "cmd", "contextapp", "main.go")]; ok {
		t.Fatal("repeat dispatcher generation collected main.go")
	}
}

func TestGenerate_ImportableDispatcherRejectsUnimportablePackage(t *testing.T) {
	writer := NewCollectingFileWriter()
	err := GenerateWithFS(fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.22\n")},
		"app.go": &fstest.MapFile{Data: []byte("package app\n// Root is a subcommand `app`\nfunc Root() {}\n")},
	}, writer, ".", "", "commentv1", nil, false, false, nil, false, false, "", "", "", GenerateOptions{DispatcherPackage: "main"})
	if err == nil || !strings.Contains(err.Error(), "dispatcher package") {
		t.Fatalf("invalid dispatcher package error = %v", err)
	}
}

func TestGenerate_MainPolicy(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/policyapp\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "application.go"), issue479PolicyAppSource)
	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	writeRuntimeFixture(t, filepath.Join(dir, "cmd", "policyapp", "policy.go"), issue479PolicySource)

	buildPath := filepath.Join(dir, "policyapp")
	build := exec.Command("go", "build", "-o", buildPath, "./cmd/policyapp")
	build.Dir = dir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build policy executable: %v\n%s", err, output)
	}

	version := exec.Command(buildPath, "version")
	version.Dir = dir
	versionOutput, err := version.CombinedOutput()
	if err != nil {
		t.Fatalf("run custom-version executable: %v\n%s", err, versionOutput)
	}
	if !strings.Contains(string(versionOutput), "policy initialized") || !strings.Contains(string(versionOutput), "Version: custom-version") {
		t.Fatalf("custom root version was not used:\n%s", versionOutput)
	}

	fail := exec.Command(buildPath, "fail")
	fail.Dir = dir
	failOutput, err := fail.CombinedOutput()
	if err == nil {
		t.Fatalf("custom policy failure unexpectedly succeeded: %s", failOutput)
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 23 {
		t.Fatalf("custom policy exit = %v, want status 23; output:\n%s", err, failOutput)
	}
	if !strings.Contains(string(failOutput), "custom error:") || !strings.Contains(string(failOutput), "application failure") {
		t.Fatalf("custom policy rendering missing:\n%s", failOutput)
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

func TestGenerate_Issue513_AppRootAndExplicitRootSubcommand(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/downloadsd\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issue513RootCollisionSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("first Generate failed: %v", err)
	}

	snapshotTree := func() map[string]string {
		t.Helper()
		snap := make(map[string]string)
		err := filepath.WalkDir(filepath.Join(dir, "cmd"), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snap[rel] = string(data)
			return nil
		})
		if err != nil {
			t.Fatalf("failed to snapshot generated tree: %v", err)
		}
		return snap
	}

	firstSnapshot := snapshotTree()
	if err := Generate(dir, "", "commentv1", nil, true, true, true, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("second Generate failed: %v", err)
	}
	secondSnapshot := snapshotTree()
	if !reflect.DeepEqual(firstSnapshot, secondSnapshot) {
		t.Fatalf("deterministic double generation changed output")
	}

	// Verify generated files and usage resources do not collide.
	for _, wantFile := range []string{
		filepath.Join("cmd", "downloadsd", "main.go"),
		filepath.Join("cmd", "downloadsd", "root.go"),
		filepath.Join("cmd", "downloadsd", "root_test.go"),
		filepath.Join("cmd", "downloadsd", "root2.go"),
		filepath.Join("cmd", "downloadsd", "root2_test.go"),
		filepath.Join("cmd", "downloadsd", "services.go"),
		filepath.Join("cmd", "downloadsd", "services_list.go"),
		filepath.Join("cmd", "downloadsd", "queue.go"),
		filepath.Join("cmd", "downloadsd", "queue_list.go"),
		filepath.Join("cmd", "downloadsd", "templates", "downloadsd_usage.txt"),
		filepath.Join("cmd", "downloadsd", "templates", "root_usage.txt"),
		filepath.Join("cmd", "downloadsd", "templates", "services_usage.txt"),
		filepath.Join("cmd", "downloadsd", "templates", "services_list_usage.txt"),
		filepath.Join("cmd", "downloadsd", "templates", "queue_usage.txt"),
		filepath.Join("cmd", "downloadsd", "templates", "queue_list_usage.txt"),
	} {
		if _, ok := firstSnapshot[wantFile]; !ok {
			t.Fatalf("expected generated file %s to exist; got files: %v", wantFile, firstSnapshot)
		}
	}

	mainGo := firstSnapshot[filepath.Join("cmd", "downloadsd", "main.go")]
	if !strings.Contains(mainGo, `NewRoot("downloadsd", version, commit, date)`) {
		t.Fatalf("main.go must construct application root via NewRoot:\n%s", mainGo)
	}
	rootGo := firstSnapshot[filepath.Join("cmd", "downloadsd", "root.go")]
	if !strings.Contains(rootGo, "type RootCmd struct") || !strings.Contains(rootGo, `c.Commands["root"] = subCmd`) {
		t.Fatalf("root.go must define RootCmd and register the explicit root subcommand:\n%s", rootGo)
	}
	for rel, content := range firstSnapshot {
		if strings.HasSuffix(rel, ".go") && strings.Contains(content, `"github.com/arran4/go-subcommand`) {
			t.Fatalf("generated file %s must not have a runtime import of go-subcommand:\n%s", rel, content)
		}
	}

	// Verify all generated unit tests compile and pass.
	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = dir
	if out, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("generated go test ./... failed: %v\n%s", err, out)
	}

	// Build the standalone executable and exercise all invocation forms from issue #513.
	binPath := filepath.Join(dir, "downloadsd-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/downloadsd")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/downloadsd failed: %v\n%s", err, out)
	}

	markerFile := filepath.Join(dir, "markers.txt")
	readMarkers := func() []string {
		t.Helper()
		data, err := os.ReadFile(markerFile)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			t.Fatalf("read markers: %v", err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return nil
		}
		return strings.Split(trimmed, "\n")
	}
	resetMarkers := func() {
		t.Helper()
		_ = os.Remove(markerFile)
	}

	runExec := func(failOnAction bool, args ...string) (string, error) {
		t.Helper()
		resetMarkers()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = dir
		failVal := "0"
		if failOnAction {
			failVal = "1"
		}
		cmd.Env = append(os.Environ(),
			"ISSUE513_MARKER_FILE="+markerFile,
			"ISSUE513_FAIL_ON_ACTION="+failVal,
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. Help requests must succeed, show the intended usage, and invoke ZERO actions or providers.
	helpCases := []struct {
		args        []string
		wantSubstrs []string
		notSubstrs  []string
	}{
		{
			args:        []string{"--help"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] <subcommand>", "Manage downloads and services.", "Run the privileged system supervisor."},
			notSubstrs:  []string{"Usage: downloadsd [flags...] root"},
		},
		{
			args:        []string{"-h"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] <subcommand>", "Manage downloads and services."},
			notSubstrs:  []string{"Usage: downloadsd [flags...] root"},
		},
		{
			args:        []string{"help"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] <subcommand>", "Manage downloads and services."},
			notSubstrs:  []string{"Usage: downloadsd [flags...] root"},
		},
		{
			args:        []string{"usage"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] <subcommand>", "Manage downloads and services."},
			notSubstrs:  []string{"Usage: downloadsd [flags...] root"},
		},
		{
			args:        []string{"help", "-deep"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] <subcommand>", "services list", "queue list", "List configured services.", "List queued downloads."},
		},
		{
			args:        []string{"root", "--help"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] root [flags...]", "Run the privileged system supervisor.", "--serve"},
			notSubstrs:  []string{"Manage downloads and services."},
		},
		{
			args:        []string{"help", "root"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] root [flags...]", "Run the privileged system supervisor.", "--serve"},
		},
		{
			args:        []string{"help", "services", "list"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] services list [flags...] [filter]", "List configured services.", "--all"},
			notSubstrs:  []string{"List queued downloads."},
		},
		{
			args:        []string{"services", "list", "--help"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] services list [flags...] [filter]", "List configured services.", "--all"},
			notSubstrs:  []string{"List queued downloads."},
		},
		{
			args:        []string{"help", "queue", "list"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] queue list [flags...] <target> [extra...]", "List queued downloads.", "--limit"},
			notSubstrs:  []string{"List configured services."},
		},
		{
			args:        []string{"queue", "list", "--help"},
			wantSubstrs: []string{"Usage: downloadsd [flags...] queue list [flags...] <target> [extra...]", "List queued downloads.", "--limit"},
			notSubstrs:  []string{"List configured services."},
		},
	}

	for _, tc := range helpCases {
		out, err := runExec(true, tc.args...)
		if err != nil {
			t.Fatalf("help invocation %v failed: %v\n%s", tc.args, err, out)
		}
		if markers := readMarkers(); len(markers) > 0 {
			t.Fatalf("help invocation %v unexpectedly executed actions/providers: %v", tc.args, markers)
		}
		for _, want := range tc.wantSubstrs {
			if !strings.Contains(out, want) {
				t.Errorf("help invocation %v missing %q:\n%s", tc.args, want, out)
			}
		}
		for _, notWant := range tc.notSubstrs {
			if strings.Contains(out, notWant) {
				t.Errorf("help invocation %v unexpectedly contained %q:\n%s", tc.args, notWant, out)
			}
		}
	}

	// 2. Bare invocation executes the application root action (not the explicit root child).
	out, err := runExec(false)
	if err != nil {
		t.Fatalf("bare downloadsd invocation failed: %v\n%s", err, out)
	}
	wantRootMarkers := []string{"provider:runtime-state", "app-root:config=/etc/downloadsd.conf,host=127.0.0.1,state=ready"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantRootMarkers) {
		t.Fatalf("bare downloadsd markers = %v, want %v", got, wantRootMarkers)
	}

	// 3. Explicit `downloadsd root` invocation executes only the child root action.
	out, err = runExec(false, "--config", "/custom/root.conf", "root", "--serve")
	if err != nil {
		t.Fatalf("downloadsd root invocation failed: %v\n%s", err, out)
	}
	wantChildRootMarkers := []string{"provider:runtime-state", "child-root:config=/custom/root.conf,serve=true,state=ready"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantChildRootMarkers) {
		t.Fatalf("downloadsd root markers = %v, want %v", got, wantChildRootMarkers)
	}

	// 4. Nested commands with root flags, child flags, positional arguments, and `--` termination.
	out, err = runExec(false, "--config", "/custom/svc.conf", "services", "list", "--all", "running")
	if err != nil {
		t.Fatalf("downloadsd services list failed: %v\n%s", err, out)
	}
	if got, want := readMarkers(), []string{"services-list:config=/custom/svc.conf,all=true,filter=running"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("services list markers = %v, want %v", got, want)
	}

	out, err = runExec(false, "--config", "/custom/queue.conf", "queue", "list", "--limit", "25", "--", "--dash-target", "--literal-extra")
	if err != nil {
		t.Fatalf("downloadsd queue list with -- failed: %v\n%s", err, out)
	}
	if got, want := readMarkers(), []string{"queue-list:config=/custom/queue.conf,limit=25,target=--dash-target,extra=[--literal-extra]"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("queue list markers = %v, want %v", got, want)
	}

	// 5. Unknown commands fail nonzero and do not invoke root or child business actions.
	for _, badArgs := range [][]string{
		{"unknown-cmd"},
		{"services", "unknown-sub"},
		{"help", "unknown-cmd"},
		{"help", "services", "unknown-sub"},
	} {
		out, err := runExec(true, badArgs...)
		if err == nil {
			t.Fatalf("expected nonzero error for unknown command %v, but succeeded with:\n%s", badArgs, out)
		}
		if !strings.Contains(out, "unknown command:") {
			t.Errorf("expected 'unknown command:' in output for %v, got:\n%s", badArgs, out)
		}
		if markers := readMarkers(); len(markers) > 0 {
			t.Fatalf("unknown command %v unexpectedly invoked actions/providers: %v", badArgs, markers)
		}
	}
}

func TestGenerate_Issue513_AllDefaultActionConfigurations(t *testing.T) {
	configs := []struct {
		name       string
		source     string
		testInvocs func(t *testing.T, run func(args ...string) (string, error))
	}{
		{
			name:   "config1_default_root_no_children",
			source: issue513Config1Source,
			testInvocs: func(t *testing.T, run func(args ...string) (string, error)) {
				// Bare invocation runs root action
				out, err := run()
				if err != nil {
					t.Fatalf("bare invocation failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:config1:flag=default_val") {
					t.Errorf("unexpected output: %s", out)
				}
				// Flag override
				out, err = run("--flag", "custom_val")
				if err != nil {
					t.Fatalf("flag invocation failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:config1:flag=custom_val") {
					t.Errorf("unexpected output: %s", out)
				}
				// Help invocation prints usage without executing action
				out, err = run("--help")
				if err != nil {
					t.Fatalf("help failed: %v\n%s", err, out)
				}
				if strings.Contains(out, "executed:") {
					t.Errorf("action executed during help: %s", out)
				}
				if !strings.Contains(out, "Usage: app [flags...]") {
					t.Errorf("usage missing in help output: %s", out)
				}
			},
		},
		{
			name:   "config2_default_root_with_ordinary_children",
			source: issue513Config2Source,
			testInvocs: func(t *testing.T, run func(args ...string) (string, error)) {
				// Bare invocation runs root action
				out, err := run()
				if err != nil {
					t.Fatalf("bare invocation failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:config2-root:flag=root_val") {
					t.Errorf("unexpected output: %s", out)
				}
				// Child command runs child action
				out, err = run("child", "--val", "custom_child")
				if err != nil {
					t.Fatalf("child invocation failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:config2-child:val=custom_child") {
					t.Errorf("unexpected output: %s", out)
				}
				// Unknown command fails
				out, err = run("unknown-cmd")
				if err == nil {
					t.Fatalf("expected error for unknown command, got success:\n%s", out)
				}
				if !strings.Contains(out, "unknown command: unknown-cmd") {
					t.Errorf("expected unknown command error, got: %s", out)
				}
			},
		},
		{
			name:   "config4_no_default_root_with_children",
			source: issue513Config4Source,
			testInvocs: func(t *testing.T, run func(args ...string) (string, error)) {
				// Bare invocation displays usage and exits 0 (no default action declared)
				out, err := run()
				if err != nil {
					t.Fatalf("bare invocation failed: %v\n%s", err, out)
				}
				if strings.Contains(out, "executed:") {
					t.Errorf("action executed on bare invocation: %s", out)
				}
				if !strings.Contains(out, "Usage: app <subcommand>") {
					t.Errorf("usage missing in bare output: %s", out)
				}
				// Child command runs
				out, err = run("child", "--val", "active")
				if err != nil {
					t.Fatalf("child invocation failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:config4-child:val=active") {
					t.Errorf("unexpected output: %s", out)
				}
				// Unknown command fails
				out, err = run("unknown-cmd")
				if err == nil {
					t.Fatalf("expected error for unknown command, got success:\n%s", out)
				}
				if !strings.Contains(out, "unknown command: unknown-cmd") {
					t.Errorf("expected unknown command error, got: %s", out)
				}
			},
		},
		{
			name:   "config5_no_default_root_with_explicit_root_child",
			source: issue513Config5Source,
			testInvocs: func(t *testing.T, run func(args ...string) (string, error)) {
				// Bare invocation displays usage and exits 0 (no default action declared)
				out, err := run()
				if err != nil {
					t.Fatalf("bare invocation failed: %v\n%s", err, out)
				}
				if strings.Contains(out, "executed:") {
					t.Errorf("action executed on bare invocation: %s", out)
				}
				if !strings.Contains(out, "Usage: app <subcommand>") {
					t.Errorf("usage missing in bare output: %s", out)
				}
				// app root runs the explicit child root action!
				out, err = run("root", "--serve")
				if err != nil {
					t.Fatalf("app root failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:config5-child-root:serve=true") {
					t.Errorf("unexpected output: %s", out)
				}
				// app other runs the other child
				out, err = run("other")
				if err != nil {
					t.Fatalf("app other failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:config5-other") {
					t.Errorf("unexpected output: %s", out)
				}
				// Unknown command fails
				out, err = run("unknown-cmd")
				if err == nil {
					t.Fatalf("expected error for unknown command, got success:\n%s", out)
				}
				if !strings.Contains(out, "unknown command: unknown-cmd") {
					t.Errorf("expected unknown command error, got: %s", out)
				}
			},
		},
		{
			name:   "child_alias_named_root_without_canonical_root_child",
			source: issue513AliasRootSource,
			testInvocs: func(t *testing.T, run func(args ...string) (string, error)) {
				// Child has alias "root" when no canonical root child exists
				out, err := run("root")
				if err != nil {
					t.Fatalf("app root via alias failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:status-via-root-alias") {
					t.Errorf("unexpected output: %s", out)
				}
				out, err = run("status")
				if err != nil {
					t.Fatalf("app status via canonical failed: %v\n%s", err, out)
				}
				if !strings.Contains(out, "executed:status-via-root-alias") {
					t.Errorf("unexpected output: %s", out)
				}
			},
		},
	}

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/testapp\n\ngo 1.22\n")
			writeRuntimeFixture(t, filepath.Join(dir, "app.go"), cfg.source)

			if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			binPath := filepath.Join(dir, "app-bin")
			buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/app")
			buildCmd.Dir = dir
			if out, err := buildCmd.CombinedOutput(); err != nil {
				t.Fatalf("go build failed: %v\n%s", err, out)
			}

			run := func(args ...string) (string, error) {
				cmd := exec.Command(binPath, args...)
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				return string(out), err
			}

			cfg.testInvocs(t, run)
		})
	}
}

func TestGenerate_Issue513_AliasesAndHelpTraversal(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/aliasapp\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issue513AliasesSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	binPath := filepath.Join(dir, "aliasapp-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/aliasapp")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	markerFile := filepath.Join(dir, "markers.txt")
	resetMarkers := func() {
		t.Helper()
		_ = os.Remove(markerFile)
	}
	readMarkers := func() []string {
		t.Helper()
		data, err := os.ReadFile(markerFile)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			t.Fatalf("read markers: %v", err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return nil
		}
		return strings.Split(trimmed, "\n")
	}

	runExec := func(failOnAction bool, args ...string) (string, error) {
		t.Helper()
		resetMarkers()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = dir
		failVal := "0"
		if failOnAction {
			failVal = "1"
		}
		cmd.Env = append(os.Environ(),
			"ISSUE513_ALIAS_MARKER_FILE="+markerFile,
			"ISSUE513_ALIAS_FAIL_ON_ACTION="+failVal,
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. Bare invocation runs application root (custom function RunApp, not Root)
	out, err := runExec(false)
	if err != nil {
		t.Fatalf("bare invocation failed: %v\n%s", err, out)
	}
	wantRootMarkers := []string{"provider:alias-runtime", "app-root:config=default.cfg,state=alias-ready"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantRootMarkers) {
		t.Fatalf("markers = %v, want %v", got, wantRootMarkers)
	}

	// 2. Explicit root child invocation via canonical name and aliases
	for _, aliasArgs := range [][]string{
		{"root", "--serve"},
		{"supervisor", "--serve"},
		{"r", "--serve"},
	} {
		out, err = runExec(false, aliasArgs...)
		if err != nil {
			t.Fatalf("explicit root invocation %v failed: %v\n%s", aliasArgs, err, out)
		}
		wantChildMarkers := []string{"provider:alias-runtime", "child-root:config=default.cfg,serve=true,state=alias-ready"}
		if got := readMarkers(); !reflect.DeepEqual(got, wantChildMarkers) {
			t.Fatalf("markers for %v = %v, want %v", aliasArgs, got, wantChildMarkers)
		}
	}

	// 3. Nested child dispatch via canonical and alias names
	for _, nestedArgs := range [][]string{
		{"svc", "list", "--all"},
		{"service", "ls", "--all"},
		{"s", "list", "--all"},
		{"service", "list", "--all"},
		{"s", "ls", "--all"},
	} {
		out, err = runExec(false, nestedArgs...)
		if err != nil {
			t.Fatalf("nested invocation %v failed: %v\n%s", nestedArgs, err, out)
		}
		if got, want := readMarkers(), []string{"svc-list:all=true"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("markers for %v = %v, want %v", nestedArgs, got, want)
		}
	}

	// 4. Aliased help and usage traversal executes ZERO actions or providers
	for _, helpArgs := range [][]string{
		{"help", "supervisor"},
		{"supervisor", "--help"},
		{"r", "--help"},
		{"help", "r"},
		{"help", "service", "ls"},
		{"service", "ls", "--help"},
		{"help", "s", "list"},
		{"s", "ls", "--help"},
	} {
		out, err = runExec(true, helpArgs...)
		if err != nil {
			t.Fatalf("help invocation %v failed: %v\n%s", helpArgs, err, out)
		}
		if markers := readMarkers(); len(markers) > 0 {
			t.Fatalf("help invocation %v unexpectedly executed actions/providers: %v", helpArgs, markers)
		}
	}
}

func TestGenerate_Issue513_DefaultHelpRouteEquivalence(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/helprouteapp\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issue513DefaultHelpRouteSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	binPath := filepath.Join(dir, "helprouteapp-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/prog")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	markerFile := filepath.Join(dir, "markers.txt")
	resetMarkers := func() {
		t.Helper()
		_ = os.Remove(markerFile)
	}
	readMarkers := func() []string {
		t.Helper()
		data, err := os.ReadFile(markerFile)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			t.Fatalf("read markers: %v", err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return nil
		}
		return strings.Split(trimmed, "\n")
	}

	runExec := func(failOnAction bool, args ...string) (string, error) {
		t.Helper()
		resetMarkers()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = dir
		failVal := "0"
		if failOnAction {
			failVal = "1"
		}
		cmd.Env = append(os.Environ(),
			"ISSUE513_MARKER_FILE="+markerFile,
			"ISSUE513_FAIL_ON_ACTION="+failVal,
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. Bare invocation and "help simple" must produce identical output and execute zero actions/providers
	outBare, err := runExec(true)
	if err != nil {
		t.Fatalf("bare invocation failed: %v\n%s", err, outBare)
	}
	if markers := readMarkers(); len(markers) > 0 {
		t.Fatalf("bare invocation unexpectedly executed actions/providers: %v", markers)
	}

	outHelpSimple, err := runExec(true, "help", "simple")
	if err != nil {
		t.Fatalf("help simple invocation failed: %v\n%s", err, outHelpSimple)
	}
	if markers := readMarkers(); len(markers) > 0 {
		t.Fatalf("help simple unexpectedly executed actions/providers: %v", markers)
	}

	if outBare != outHelpSimple {
		t.Errorf("expected bare invocation output to match 'help simple':\nbare:\n%s\nhelp simple:\n%s", outBare, outHelpSimple)
	}
	if !strings.Contains(outBare, "Usage: prog simple") {
		t.Errorf("bare usage output missing 'Usage: prog simple':\n%s", outBare)
	}

	// 2. prog help shows root usage, not simple's usage
	outHelp, err := runExec(true, "help")
	if err != nil {
		t.Fatalf("help invocation failed: %v\n%s", err, outHelp)
	}
	if !strings.Contains(outHelp, "Usage: prog <subcommand>") {
		t.Errorf("root help output missing expected root usage:\n%s", outHelp)
	}

	// 3. prog help -deep shows recursive usage
	outDeep, err := runExec(true, "help", "-deep")
	if err != nil {
		t.Fatalf("help -deep invocation failed: %v\n%s", err, outDeep)
	}
	if !strings.Contains(outDeep, "Usage: prog <subcommand>") || !strings.Contains(outDeep, "Simple demonstration command.") {
		t.Errorf("recursive help output missing expected command trees:\n%s", outDeep)
	}

	// 4. Action execution when explicitly invoked
	out, err := runExec(false, "simple", "--name", "alice")
	if err != nil {
		t.Fatalf("prog simple failed: %v\n%s", err, out)
	}
	wantSimpleMarkers := []string{"provider:state", "action:simple:name=alice,state=ready"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantSimpleMarkers) {
		t.Fatalf("markers for simple = %v, want %v", got, wantSimpleMarkers)
	}

	// 5. Explicit root child execution
	out, err = runExec(false, "root", "--serve")
	if err != nil {
		t.Fatalf("prog root failed: %v\n%s", err, out)
	}
	wantRootMarkers := []string{"action:root:serve=true"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantRootMarkers) {
		t.Fatalf("markers for root = %v, want %v", got, wantRootMarkers)
	}

	// 6. Unknown command returns error
	out, err = runExec(false, "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent command, got nil:\n%s", out)
	}
	if !strings.Contains(out, "unknown command: nonexistent") {
		t.Errorf("expected unknown command error message, got:\n%s", out)
	}
}

func TestGenerate_Issue513_NestedActionAliasMultiLevel(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/nestedaliasapp\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issue513NestedActionAliasSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	binPath := filepath.Join(dir, "nestedaliasapp-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/prog")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	markerFile := filepath.Join(dir, "markers.txt")
	resetMarkers := func() {
		t.Helper()
		_ = os.Remove(markerFile)
	}
	readMarkers := func() []string {
		t.Helper()
		data, err := os.ReadFile(markerFile)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			t.Fatalf("read markers: %v", err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return nil
		}
		return strings.Split(trimmed, "\n")
	}

	runExec := func(failOnAction bool, args ...string) (string, error) {
		t.Helper()
		resetMarkers()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = dir
		failVal := "0"
		if failOnAction {
			failVal = "1"
		}
		cmd.Env = append(os.Environ(),
			"ISSUE513_MARKER_FILE="+markerFile,
			"ISSUE513_FAIL_ON_ACTION="+failVal,
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. Bare invocation runs application root action
	out, err := runExec(false)
	if err != nil {
		t.Fatalf("bare invocation failed: %v\n%s", err, out)
	}
	wantRootMarkers := []string{"provider:state", "app-root:config=default.cfg,state=ready"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantRootMarkers) {
		t.Fatalf("markers for bare invocation = %v, want %v", got, wantRootMarkers)
	}

	// 2. Explicit 3-level-deep command path invocations (canonical and intermediate aliases)
	// All select the EXACT same underlying action without duplicate invocations
	for _, pathArgs := range [][]string{
		{"tools", "docs", "simple"},
		{"tools", "docs", "s"},
		{"t", "d", "simple"},
		{"t", "d", "s"},
	} {
		out, err = runExec(false, pathArgs...)
		if err != nil {
			t.Fatalf("nested path invocation %v failed: %v\n%s", pathArgs, err, out)
		}
		if got := readMarkers(); !reflect.DeepEqual(got, wantRootMarkers) {
			t.Fatalf("markers for %v = %v, want %v", pathArgs, got, wantRootMarkers)
		}
	}

	// 3. Flags and inherited properties on nested path
	out, err = runExec(false, "tools", "docs", "simple", "--config", "custom.cfg")
	if err != nil {
		t.Fatalf("nested path with flags failed: %v\n%s", err, out)
	}
	wantCustomMarkers := []string{"provider:state", "app-root:config=custom.cfg,state=ready"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantCustomMarkers) {
		t.Fatalf("markers for nested path with flag = %v, want %v", got, wantCustomMarkers)
	}

	// 4. Explicit root child invocation is independent and not confused with root action
	out, err = runExec(false, "root", "--serve")
	if err != nil {
		t.Fatalf("explicit root child failed: %v\n%s", err, out)
	}
	wantChildMarkers := []string{"child-root:serve=true"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantChildMarkers) {
		t.Fatalf("markers for root child = %v, want %v", got, wantChildMarkers)
	}

	out, err = runExec(false, "supervisor", "--serve")
	if err != nil {
		t.Fatalf("explicit root child via supervisor alias failed: %v\n%s", err, out)
	}
	if got := readMarkers(); !reflect.DeepEqual(got, wantChildMarkers) {
		t.Fatalf("markers for supervisor alias = %v, want %v", got, wantChildMarkers)
	}

	out, err = runExec(false, "r")
	if err != nil {
		t.Fatalf("explicit root child via r alias failed: %v\n%s", err, out)
	}
	wantRMarkers := []string{"child-root:serve=false"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantRMarkers) {
		t.Fatalf("markers for r alias = %v, want %v", got, wantRMarkers)
	}

	// 5. Help traversal on nested paths and aliases executes ZERO actions or providers
	for _, helpArgs := range [][]string{
		{"help", "tools", "docs", "simple"},
		{"tools", "docs", "simple", "--help"},
		{"tools", "docs", "s", "--help"},
		{"t", "d", "s", "--help"},
		{"help", "root"},
		{"root", "--help"},
		{"supervisor", "--help"},
		{"r", "--help"},
	} {
		out, err = runExec(true, helpArgs...)
		if err != nil {
			t.Fatalf("help invocation %v failed: %v\n%s", helpArgs, err, out)
		}
		if markers := readMarkers(); len(markers) > 0 {
			t.Fatalf("help invocation %v unexpectedly executed actions/providers: %v", helpArgs, markers)
		}
	}
}

func TestGenerate_Issue513_SubcommandDefaultRoute(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/subdefaultapp\n\ngo 1.22\n")
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), issue513SubcommandDefaultSource)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	binPath := filepath.Join(dir, "subdefaultapp-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/prog")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	markerFile := filepath.Join(dir, "markers.txt")
	resetMarkers := func() {
		t.Helper()
		_ = os.Remove(markerFile)
	}
	readMarkers := func() []string {
		t.Helper()
		data, err := os.ReadFile(markerFile)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			t.Fatalf("read markers: %v", err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return nil
		}
		return strings.Split(trimmed, "\n")
	}

	runExec := func(failOnAction bool, args ...string) (string, error) {
		t.Helper()
		resetMarkers()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = dir
		failVal := "0"
		if failOnAction {
			failVal = "1"
		}
		cmd.Env = append(os.Environ(),
			"ISSUE513_MARKER_FILE="+markerFile,
			"ISSUE513_FAIL_ON_ACTION="+failVal,
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. Bare invocation executes default subcommand
	out, err := runExec(false)
	if err != nil {
		t.Fatalf("bare invocation failed: %v\n%s", err, out)
	}
	wantDefaultMarkers := []string{"simple:val=nested-default"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantDefaultMarkers) {
		t.Fatalf("markers for bare invocation = %v, want %v", got, wantDefaultMarkers)
	}

	// 2. Direct nested invocation executes with custom flags
	out, err = runExec(false, "tools", "docs", "simple", "--val", "custom-val")
	if err != nil {
		t.Fatalf("direct invocation failed: %v\n%s", err, out)
	}
	wantCustomMarkers := []string{"simple:val=custom-val"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantCustomMarkers) {
		t.Fatalf("markers for direct invocation = %v, want %v", got, wantCustomMarkers)
	}

	// 3. Explicit root child
	out, err = runExec(false, "root")
	if err != nil {
		t.Fatalf("root child failed: %v\n%s", err, out)
	}
	wantRootMarkers := []string{"child-root"}
	if got := readMarkers(); !reflect.DeepEqual(got, wantRootMarkers) {
		t.Fatalf("markers for root child = %v, want %v", got, wantRootMarkers)
	}
}

func TestValidate_Issue513_Collisions(t *testing.T) {
	testCollision := func(name, src, wantErrSubstring string) {
		t.Run(name, func(t *testing.T) {
			fs := setupProject(t, src)
			p, err := parsers.Get("commentv1")
			if err != nil {
				t.Fatalf("get parser: %v", err)
			}
			dm, err := p.Parse(fs, ".", nil)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			err = dm.Validate()
			if err == nil {
				t.Fatalf("expected validation error containing %q, got nil", wantErrSubstring)
			}
			if !strings.Contains(err.Error(), wantErrSubstring) {
				t.Errorf("validation error mismatch:\nwant substring: %q\ngot: %v", wantErrSubstring, err)
			}
		})
	}

	// Canonical conflicts with built-in commands
	testCollision("canonical_help", `package main
// Help is a subcommand `+"`app help`"+`
func Help() {}
`, `subcommand "help" conflicts with built-in command "help"`)

	testCollision("canonical_usage", `package main
// Usage is a subcommand `+"`app usage`"+`
func Usage() {}
`, `subcommand "usage" conflicts with built-in command "usage"`)

	testCollision("canonical_version", `package main
// Version is a subcommand `+"`app version`"+`
func Version() {}
`, `subcommand "version" conflicts with built-in command "version"`)

	// Alias conflicts with built-in commands
	testCollision("alias_help", `package main
// Child is a subcommand `+"`app child`"+` (alias: help)
func Child() {}
`, `alias "help" on subcommand "child" conflicts with built-in command "help"`)

	testCollision("alias_usage", `package main
// Child is a subcommand `+"`app child`"+`
// Aliases: usage
func Child() {}
`, `alias "usage" on subcommand "child" conflicts with built-in command "usage"`)

	testCollision("alias_version", `package main
// Child is a subcommand `+"`app child`"+` (aka: version)
func Child() {}
`, `alias "version" on subcommand "child" conflicts with built-in command "version"`)

	// Alias conflicts with canonical command at same level
	testCollision("alias_conflicts_with_canonical_root", `package main
// Root is a subcommand `+"`app root`"+`
func Root() {}

// Child is a subcommand `+"`app child`"+` (aka: root)
func Child() {}
`, `alias "root" on subcommand "child" conflicts with subcommand "root"`)

	testCollision("alias_conflicts_with_other_canonical", `package main
// Status is a subcommand `+"`app status`"+`
func Status() {}

// Child is a subcommand `+"`app child`"+` (aka: status)
func Child() {}
`, `alias "status" on subcommand "child" conflicts with subcommand "status"`)

	// Alias conflicts with another alias at same level
	testCollision("alias_conflicts_with_other_alias", `package main
// Status is a subcommand `+"`app status`"+` (alias: s)
func Status() {}

// Child is a subcommand `+"`app child`"+` (alias: s)
func Child() {}
`, `conflicts with alias on subcommand`)

	// Duplicate alias on same command
	testCollision("duplicate_alias_on_same_command", `package main
// Status is a subcommand `+"`app status`"+` (aliases: s, s)
func Status() {}
`, `duplicate alias "s" on subcommand "status"`)

	// Root alias collisions and validation
	testCollision("alias_root_self", `package main
// App is a subcommand `+"`app`"+` (alias: app)
func App() {}
`, `alias "app" cannot alias the root command itself`)

	testCollision("alias_root_builtin_help", `package main
// App is a subcommand `+"`app`"+` (alias: help)
func App() {}
`, `alias "help" conflicts with built-in command "help"`)

	testCollision("alias_root_conflicts_with_subcommand", `package main
// App is a subcommand `+"`app`"+` (alias: child)
func App() {}

// Child is a subcommand `+"`app child`"+`
func Child() {}
`, `alias "child" on root command conflicts with subcommand "child"`)

	testCollision("alias_root_duplicate", `package main
// App is a subcommand `+"`app`"+` (aliases: run, run)
func App() {}
`, `duplicate alias "run" on root command`)

	// Default route validation
	testCollision("default_route_self", `package main
// Child is a subcommand `+"`app child`"+`
// Default route: app
func Child() {}
`, `default route cannot reference the root command itself`)

	testCollision("default_route_not_found", `package main
// Child is a subcommand `+"`app child`"+`
// Default route: nonexistent
func Child() {}
`, `default route target "nonexistent" not found in command tree`)

	testCollision("default_route_help_not_found", `package main
// Child is a subcommand `+"`app child`"+`
// Default route: help nonexistent
func Child() {}
`, `default route target "help nonexistent" not found in command tree`)

	testCollision("default_route_action_conflict", `package main
// App is a subcommand `+"`app`"+`
// Default route: child
func App() {}

// Child is a subcommand `+"`app child`"+`
func Child() {}
`, `cannot declare default route "child" when root command already has an action (App)`)
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

func TestGenerate_RuntimeUsageHelpFormatting(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/issue481\n\ngo 1.22\n")
	src := strings.Replace(issue481Source, "package main", "package issue481", 1)
	writeRuntimeFixture(t, filepath.Join(dir, "app.go"), src)

	if err := Generate(dir, "", "commentv1", nil, true, true, false, nil, false, false, "", "", ""); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	binPath := filepath.Join(dir, "app-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/app")
	buildCmd.Dir = dir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build generated CLI: %v\n%s", err, out)
	}

	runCLIAtWidth := func(columns string, args ...string) string {
		t.Helper()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "COLUMNS="+columns)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("CLI %v (COLUMNS=%s) failed: %v\n%s", args, columns, err, out)
		}
		return string(out)
	}
	runCLI := func(args ...string) string {
		t.Helper()
		return runCLIAtWidth("140", args...)
	}

	rootShallow := runCLI("help")
	for _, want := range []string{
		"Short root description with {{.Injected}} literal braces.",
		`Extended help with {{printf "%s" .Exploit}} and unmatched }} or {{.`,
		"    render                      Render {{.Injected}} template output }} safely {{.",
		"    scenario",
		"    serve                       Run the primary server.",
		"    service                     Manage services.",
		"    up                          Start everything.",
		"    very-long-top-level-command Execute the long top-level command and wrap continuation lines under the description column.",
		"    help                        Print this help message",
		"    usage                       Print this usage message",
	} {
		if !strings.Contains(rootShallow, want) {
			t.Errorf("root help missing %q:\n%s", want, rootShallow)
		}
	}

	rootShallowNarrow := runCLIAtWidth("52", "help")
	wantRootNarrowWrapped := strings.Join([]string{
		"    very-long-top-level-command Execute the long",
		"                                top-level command",
		"                                and wrap",
		"                                continuation lines",
		"                                under the",
		"                                description column.",
	}, "\n")
	if !strings.Contains(rootShallowNarrow, wantRootNarrowWrapped) {
		t.Errorf("root narrow help did not wrap with hanging indent:\nwant substring:\n%s\ngot:\n%s", wantRootNarrowWrapped, rootShallowNarrow)
	}

	serviceShallow := runCLI("service", "--help")
	for _, want := range []string{
		"    silent-group",
		"    status       Show service status and health details across all configured environments.",
		"    help         Print this help message",
		"    usage        Print this usage message",
	} {
		if !strings.Contains(serviceShallow, want) {
			t.Errorf("service --help missing %q:\n%s", want, serviceShallow)
		}
	}

	serviceDeep := runCLI("service", "help", "-deep")
	for _, want := range []string{
		"    service silent-group",
		"    service silent-group deep-leaf-command Deep nested leaf action.",
		"    service status                         Show service status and health details across all configured environments.",
		"    help                                   Print this help message",
		"    usage                                  Print this usage message",
	} {
		if !strings.Contains(serviceDeep, want) {
			t.Errorf("service help -deep missing %q:\n%s", want, serviceDeep)
		}
	}

	serviceDeepNarrow := runCLIAtWidth("62", "service", "help", "-deep")
	wantServiceDeepNarrow := strings.Join([]string{
		"    service status                         Show service status",
		"                                           and health details",
		"                                           across all",
		"                                           configured",
		"                                           environments.",
	}, "\n")
	if !strings.Contains(serviceDeepNarrow, wantServiceDeepNarrow) {
		t.Errorf("service deep narrow help did not wrap with hanging indent:\nwant substring:\n%s\ngot:\n%s", wantServiceDeepNarrow, serviceDeepNarrow)
	}

	serveShallow := runCLIAtWidth("64", "serve", "help")
	serveDeep := runCLIAtWidth("64", "serve", "help", "-deep")
	if serveShallow != serveDeep {
		t.Errorf("expected identical shallow and deep help for leaf command serve:\nshallow:\n%q\ndeep:\n%q", serveShallow, serveDeep)
	}
	wantServeFlagNarrow := strings.Join([]string{
		"    --port int          (default: 8080)    Port to listen on",
		"                                           when starting the",
		"                                           HTTP listener and",
		"                                           accepting client",
		"                                           connections",
	}, "\n")
	if !strings.Contains(serveShallow, wantServeFlagNarrow) {
		t.Errorf("serve narrow flag help did not wrap with hanging indent:\nwant substring:\n%s\ngot:\n%s", wantServeFlagNarrow, serveShallow)
	}
	if !strings.Contains(serveShallow, "\n    --quiet             (default: false)\n") {
		t.Errorf("serve flag without description should have no trailing spaces:\n%q", serveShallow)
	}

	serveUsageTemplate, err := os.ReadFile(filepath.Join(dir, "cmd", "app", "templates", "serve_usage.txt"))
	if err != nil {
		t.Fatalf("failed to read generated serve_usage.txt: %v", err)
	}
	if strings.Contains(string(serveUsageTemplate), ".Recursive") {
		t.Errorf("leaf command serve_usage.txt must not contain redundant .Recursive branch:\n%s", serveUsageTemplate)
	}

	serveCompact40 := runCLIAtWidth("40", "serve", "help")
	wantServeCompact40 := strings.Join([]string{
		"    --port int (default: 8080)",
		"        Port to listen on when starting",
		"        the HTTP listener and accepting",
		"        client connections",
	}, "\n")
	if !strings.Contains(serveCompact40, wantServeCompact40) {
		t.Errorf("serve COLUMNS=40 flag help did not switch to compact fallback:\nwant substring:\n%s\ngot:\n%s", wantServeCompact40, serveCompact40)
	}
	if !strings.Contains(serveCompact40, "https://service.example.com/very/long/endpoint/path/without/spaces") {
		t.Errorf("serve COLUMNS=40 truncated unbreakable URL token:\n%s", serveCompact40)
	}
	for _, line := range strings.Split(serveCompact40, "\n") {
		if strings.Contains(line, "https://service.example.com/") {
			continue
		}
		if len([]rune(line)) > 40 {
			t.Errorf("serve COLUMNS=40 line exceeded 40 columns (len=%d): %q", len([]rune(line)), line)
		}
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

func TestGenerate_ManWhitespace(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":      &fstest.MapFile{Data: []byte("module example.com/app\n\ngo 1.25.0\n")},
		"main.go":     &fstest.MapFile{Data: []byte("package main\n\n// Root is a subcommand `app` -- application root\nfunc Root() {}\n")},
		"pkg1/cmd.go": &fstest.MapFile{Data: []byte("package pkg1\n\n// Cmd1 is a subcommand `app cmd1` -- test command\n// Flags:\n//\tparam1: (default: \"foo\") Parameter 1\n//\tparam2: Parameter 2\nfunc Cmd1(param1 string, param2 int) {}\n")},
	}

	generate := func() []byte {
		t.Helper()
		writer := NewCollectingFileWriter()
		if err := GenerateWithFS(fsys, writer, ".", "man", "commentv1", nil, false, false, nil, false, false, "", "", ""); err != nil {
			t.Fatalf("GenerateWithFS failed: %v", err)
		}
		content, ok := writer.Files["man/app-cmd1.1"]
		if !ok {
			t.Fatal("man/app-cmd1.1 was not generated")
		}
		return append([]byte(nil), content...)
	}

	first := generate()
	second := generate()
	if string(first) != string(second) {
		t.Fatal("repeated man-page generation was not byte-for-byte stable")
	}

	output := string(first)
	if !strings.Contains(output, "\n[param1] [param2]\n") {
		t.Fatalf("synopsis parameters are not separated correctly:\n%s", output)
	}

	sawDefault := false
	sawNoDefault := false
	for i, line := range strings.Split(output, "\n") {
		if strings.TrimRight(line, " \t") != line {
			t.Errorf("line %d has trailing whitespace: %q", i+1, line)
		}
		if strings.Contains(line, "Parameter 1") {
			sawDefault = true
			if !strings.Contains(line, "(default:") {
				t.Errorf("defaulted option lost its default: %q", line)
			}
		}
		if strings.Contains(line, "Parameter 2") {
			sawNoDefault = true
			if strings.Contains(line, "(default:") {
				t.Errorf("option without a default gained one: %q", line)
			}
		}
	}
	if !sawDefault || !sawNoDefault {
		t.Fatalf("expected both defaulted and non-defaulted option descriptions:\n%s", output)
	}
}
