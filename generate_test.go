package go_subcommand

import (
	_ "embed"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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

//go:embed testdata/issue456_imports.go
var issue456ImportsSource string

//go:embed testdata/issue513_root_collision.go
var issue513RootCollisionSource string

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
	_ = os.Setenv("SOURCE_DATE_EPOCH", "1234567890")
	defer func() { _ = os.Unsetenv("SOURCE_DATE_EPOCH") }()

	prov := GetProvenance([]string{"usage=foo.txt"}, true, true, "", "", "")
	if prov.Timestamp != "1234567890" {
		t.Errorf("Expected Timestamp 1234567890, got %s", prov.Timestamp)
	}
	if prov.TemplateID != "usage=foo.txt" {
		t.Errorf("Expected TemplateID usage=foo.txt, got %s", prov.TemplateID)
	}
}

func TestGetProvenanceExtensive(t *testing.T) {
	prov := GetProvenance(nil, false, false, "", "", "")
	if prov.ProjectCommit != "" || prov.Timestamp != "" || prov.ReplacedTemplates {
		t.Errorf("Unexpected default provenance values: %+v", prov)
	}

	_ = os.Unsetenv("SOURCE_DATE_EPOCH")
	prov = GetProvenance(nil, false, true, "", "", "")
	if prov.Timestamp == "" {
		t.Error("Expected timestamp to be generated")
	}

	_ = os.Setenv("SOURCE_DATE_EPOCH", "987654321")
	defer func() { _ = os.Unsetenv("SOURCE_DATE_EPOCH") }()
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
	if !strings.Contains(mainGo, `root, err := NewRoot("downloadsd", version, commit, date)`) {
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
