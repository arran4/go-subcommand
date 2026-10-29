package go_subcommand

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/arran4/go-subcommand/model"
	"github.com/arran4/go-subcommand/parsers"
	_ "github.com/arran4/go-subcommand/parsers/commentv1"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:embed templates
var TemplatesFS embed.FS

var templates *template.Template

// FileWriter interface allows mocking file system writes
type FileWriter interface {
	WriteFile(path string, content []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	ReadDir(path string, ops ...any) ([]fs.DirEntry, error)
}

// FileRemover is an optional FileWriter capability used only by migrations
// which must remove an obsolete generated file.  It is deliberately separate
// from FileWriter so existing user implementations of FileWriter remain source
// compatible.
type FileRemover interface {
	Remove(path string) error
}

// FileRenamer is an optional FileWriter capability used for a case-only
// generated-file migration on case-insensitive filesystems.
type FileRenamer interface {
	Rename(oldPath, newPath string) error
}

// FileIdentity is an optional FileWriter capability for determining whether
// two spellings refer to one underlying file.
type FileIdentity interface {
	SameFile(path1, path2 string) (bool, error)
}

// OSFileWriter implements FileWriter using os package
type OSFileWriter struct{}

func (w *OSFileWriter) WriteFile(path string, content []byte, perm os.FileMode) error {
	return os.WriteFile(path, content, perm)
}
func (w *OSFileWriter) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
func (w *OSFileWriter) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
func (w *OSFileWriter) ReadDir(path string, ops ...any) ([]fs.DirEntry, error) {
	for _, opt := range ops {
		switch o := opt.(type) {
		case fs.ReadDirFS:
			return o.ReadDir(path)
		}
	}
	return os.ReadDir(path)
}
func (w *OSFileWriter) Remove(path string) error { return os.Remove(path) }
func (w *OSFileWriter) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
func (w *OSFileWriter) SameFile(path1, path2 string) (bool, error) {
	first, err := os.Stat(path1)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	second, err := os.Stat(path2)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return os.SameFile(first, second), nil
}

// CollectingFileWriter collects files and directories to be written
type CollectingFileWriter struct {
	Files   map[string][]byte
	Dirs    map[string]bool
	Deletes map[string]bool
	Renames map[string]string
}

func NewCollectingFileWriter() *CollectingFileWriter {
	return &CollectingFileWriter{
		Files:   make(map[string][]byte),
		Dirs:    make(map[string]bool),
		Deletes: make(map[string]bool),
		Renames: make(map[string]string),
	}
}

func (w *CollectingFileWriter) Remove(path string) error {
	delete(w.Files, path)
	w.Deletes[path] = true
	return nil
}

func (w *CollectingFileWriter) Rename(oldPath, newPath string) error {
	if content, ok := w.Files[oldPath]; ok {
		delete(w.Files, oldPath)
		w.Files[newPath] = content
	}
	return nil
}

func (w *CollectingFileWriter) SameFile(path1, path2 string) (bool, error) {
	return path1 == path2, nil
}

func (w *CollectingFileWriter) WriteFile(path string, content []byte, perm os.FileMode) error {
	w.Files[path] = content
	return nil
}

func (w *CollectingFileWriter) MkdirAll(path string, perm os.FileMode) error {
	w.Dirs[path] = true
	return nil
}

func (w *CollectingFileWriter) ReadFile(path string) ([]byte, error) {
	if content, ok := w.Files[path]; ok {
		return content, nil
	}
	return nil, os.ErrNotExist
}

func (w *CollectingFileWriter) ReadDir(path string, ops ...any) ([]fs.DirEntry, error) {
	// Simple implementation: scan Files for entries with the given prefix
	var entries []fs.DirEntry
	// Ensure path has a trailing slash for prefix matching, unless it's just "."
	prefix := path
	if prefix != "." && !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	seen := make(map[string]bool)

	for p := range w.Files {
		if strings.HasPrefix(p, prefix) {
			rel := strings.TrimPrefix(p, prefix)
			parts := strings.SplitN(rel, string(filepath.Separator), 2)
			name := parts[0]
			if seen[name] {
				continue
			}
			seen[name] = true
			isDir := len(parts) > 1
			// We only have minimal info for the mock DirEntry
			entries = append(entries, &collectingDirEntry{name: name, isDir: isDir})
		}
	}
	// Also check known directories
	for d := range w.Dirs {
		if strings.HasPrefix(d, prefix) {
			rel := strings.TrimPrefix(d, prefix)
			parts := strings.SplitN(rel, string(filepath.Separator), 2)
			name := parts[0]
			if seen[name] {
				continue
			}
			seen[name] = true
			entries = append(entries, &collectingDirEntry{name: name, isDir: true})
		}
	}

	if len(entries) == 0 {
		// Directories exist only when explicitly collected or implied by a file.
		_, isDir := w.Dirs[path]
		if !isDir && path != "." {
			// Check if any file starts with this path
			found := false
			for p := range w.Files {
				if strings.HasPrefix(p, prefix) {
					found = true
					break
				}
			}
			if !found {
				return nil, os.ErrNotExist
			}
		}
	}

	return entries, nil
}

type collectingDirEntry struct {
	name  string
	isDir bool
}

func (d *collectingDirEntry) Name() string { return d.name }
func (d *collectingDirEntry) IsDir() bool  { return d.isDir }
func (d *collectingDirEntry) Type() fs.FileMode {
	if d.isDir {
		return fs.ModeDir
	}
	return 0
}
func (d *collectingDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func (w *CollectingFileWriter) Verify(writer FileWriter, force bool) error {
	for path, content := range w.Files {
		existing, err := writer.ReadFile(path)
		if err == nil {
			if bytes.Equal(existing, content) {
				continue
			}
			if !force && !isGenerated(existing) {
				return fmt.Errorf("file %s exists and was not generated by gosubc (use --force to overwrite)", path)
			}
		}
	}

	if !force {
		touchedDirs := make(map[string]bool)
		for path := range w.Files {
			touchedDirs[filepath.Dir(path)] = true
		}
		for dir := range w.Dirs {
			touchedDirs[dir] = true
		}

		for dir := range touchedDirs {
			entries, err := writer.ReadDir(dir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					fullPath := filepath.Join(dir, entry.Name())
					if _, ok := w.Files[fullPath]; !ok && !w.Deletes[fullPath] && w.Renames[fullPath] == "" {
						return fmt.Errorf("file %s is present in the directory but not in the generated set (use --force to ignore)", fullPath)
					}
				}
			}
		}
	}
	return nil
}

func (w *CollectingFileWriter) Commit(writer FileWriter) error {
	for oldPath, newPath := range w.Renames {
		renamer, ok := writer.(FileRenamer)
		if !ok {
			return fmt.Errorf("cannot rename legacy generated file %s to %s: supplied FileWriter does not implement FileRenamer", oldPath, newPath)
		}
		if err := renamer.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rename legacy generated file %s to %s: %w", oldPath, newPath, err)
		}
	}
	for path, content := range w.Files {
		if err := writer.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(path), err)
		}
		if err := writer.WriteFile(path, content, 0644); err != nil {
			return fmt.Errorf("failed to create file %s: %w", path, err)
		}
	}
	for path := range w.Deletes {
		remover, ok := writer.(FileRemover)
		if !ok {
			return fmt.Errorf("cannot remove obsolete generated file %s: supplied FileWriter does not implement FileRemover", path)
		}
		if err := remover.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove file %s: %w", path, err)
		}
	}
	return nil
}

// Generate generates subcommand code using the legacy public API.
func Generate(dir string, manDir string, parserName string, paths []string, recursive bool, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, provVersion string, provCommit string, provDate string) error {
	if dir == "." {
		if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
			if root, err := findModuleRoot("."); err == nil {
				dir = root
			}
		}
	}
	return GenerateWithFS(os.DirFS(dir), &OSFileWriter{}, dir, manDir, parserName, &parsers.ParseOptions{
		SearchPaths: paths,
		Recursive:   recursive,
	}, force, clean, replaceTemplates, projectProvenance, timestamp, provVersion, provCommit, provDate)
}

func findModuleRoot(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("go.mod not found")
		}
		abs = parent
	}
}

// CleanGeneratedFiles removes generated files in the cmd/ directory and manDir safely.
func CleanGeneratedFiles(dir string, manDir string) error {
	targets := []string{filepath.Join(dir, "cmd")}
	if manDir != "" {
		targets = append(targets, manDir)
	}

	for _, target := range targets {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				if isGenerated(content) {
					if err := os.Remove(path); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		_ = filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && path != target {
				entries, err := os.ReadDir(path)
				if err == nil && len(entries) == 0 {
					_ = os.Remove(path)
				}
			}
			return nil
		})
	}
	return nil
}

// GetProvenance constructs the provenance metadata.
func GetProvenance(replaceTemplates []string, projectProvenance bool, timestamp bool, provVersion string, provCommit string, provDate string) model.Provenance {
	prov := model.Provenance{}

	// Generator build info
	if info, ok := debug.ReadBuildInfo(); ok {
		prov.Version = info.Main.Version
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				prov.Commit = s.Value
			}
			if s.Key == "vcs.modified" && s.Value == "true" {
				prov.Dirty = true
			}
		}
	}

	if provVersion != "" {
		prov.Version = provVersion
	}
	if provCommit != "" {
		prov.Commit = provCommit
		prov.ProjectCommit = provCommit
	}

	// Target project info
	if projectProvenance {
		if out, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
			prov.ProjectCommit = strings.TrimSpace(string(out))
		}
		if out, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
			outStr := strings.TrimSpace(string(out))
			if len(outStr) > 0 {
				prov.ProjectDirty = true
			}
		}
	}

	// Timestamp
	if provDate != "" {
		prov.Timestamp = provDate
	} else if ts := os.Getenv("SOURCE_DATE_EPOCH"); ts != "" {
		prov.Timestamp = ts
	} else if timestamp {
		prov.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	if len(replaceTemplates) > 0 {
		prov.ReplacedTemplates = true
		prov.TemplateID = strings.Join(replaceTemplates, ",")
	}

	return prov
}

// GenerateOptions carries optional generator settings that do not change the legacy positional API.
type GenerateOptions struct {
	// CommandDeclarations are functionless metadata and documentation sources.
	CommandDeclarations []parsers.CommandDeclaration
	// CLIParser is the generation-wide runtime CLI parser backend fallback.
	CLIParser string
	// SuppressGoGenerate omits the nested go:generate directive from generated
	// main.go. Its zero value retains the compatible default of emitting it.
	SuppressGoGenerate bool
	// RegenerationCommand is the project-owned, pinned command described in
	// generated AGENTS.md. An empty value emits generic safe guidance.
	RegenerationCommand string
}

func extractGenerateOptions(ops []any) GenerateOptions {
	var result GenerateOptions
	for _, opt := range ops {
		switch value := opt.(type) {
		case GenerateOptions:
			result.CommandDeclarations = append(result.CommandDeclarations, value.CommandDeclarations...)
			if value.CLIParser != "" {
				result.CLIParser = value.CLIParser
			}
			if value.SuppressGoGenerate {
				result.SuppressGoGenerate = true
			}
			if value.RegenerationCommand != "" {
				result.RegenerationCommand = value.RegenerationCommand
			}
		case *GenerateOptions:
			if value != nil {
				result.CommandDeclarations = append(result.CommandDeclarations, value.CommandDeclarations...)
			}
			if value != nil && value.CLIParser != "" {
				result.CLIParser = value.CLIParser
			}
			if value != nil && value.SuppressGoGenerate {
				result.SuppressGoGenerate = true
			}
			if value != nil && value.RegenerationCommand != "" {
				result.RegenerationCommand = value.RegenerationCommand
			}
		}
	}
	return result
}

func resolveCLIParser(local, fallback string) (string, error) {
	backend := local
	if backend == "" {
		backend = fallback
	}
	if backend == "" {
		backend = "gnu"
	}

	switch backend {
	case "gnu":
		return backend, nil
	case "go-flag":
		return backend, nil
	case "plus-minus":
		return backend, nil
	default:
		return "", fmt.Errorf("unsupported cli-parser %q", backend)
	}
}

func resolveCLIParsers(commands []*model.Command, generatorDefault string) error {
	for _, cmd := range commands {
		resolved, err := resolveCLIParser(cmd.CLIParser, generatorDefault)
		if err != nil {
			return fmt.Errorf("command %q: %w", cmd.MainCmdName, err)
		}
		cmd.ResolvedCLIParser = resolved
		if resolved == "plus-minus" {
			if err := validatePlusMinusParameters(cmd.MainCmdName, cmd.Parameters); err != nil {
				return err
			}
		}
		if err := resolveSubCommandCLIParsers(cmd.SubCommands, resolved); err != nil {
			return err
		}
	}
	return nil
}

func resolveSubCommandCLIParsers(subCommands []*model.SubCommand, inherited string) error {
	for _, subCommand := range subCommands {
		resolved, err := resolveCLIParser(subCommand.CLIParser, inherited)
		if err != nil {
			return fmt.Errorf("subcommand %q: %w", subCommand.SubCommandName, err)
		}
		subCommand.ResolvedCLIParser = resolved
		if resolved == "plus-minus" {
			if err := validatePlusMinusParameters(subCommand.ProgName(), subCommand.Parameters); err != nil {
				return err
			}
		}
		if err := resolveSubCommandCLIParsers(subCommand.SubCommands, resolved); err != nil {
			return err
		}
	}
	return nil
}

// GenerateWithFS generates code using provided FS and Writer. Optional variadic args ops can provide custom dependencies such as readFS (fs.FS).
func GenerateWithFS(inputFS fs.FS, writer FileWriter, dir string, manDir string, parserName string, options *parsers.ParseOptions, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, provVersion string, provCommit string, provDate string, ops ...any) error {

	hasFS := false

	for _, opt := range ops {
		if _, ok := opt.(fs.FS); ok {
			hasFS = true
			break
		}
	}
	if !hasFS && inputFS != nil {
		ops = append(ops, inputFS)
	}

	overlayFS, err := buildOverlayFS(TemplatesFS, replaceTemplates, ops...)
	if err != nil {
		return fmt.Errorf("failed to build templates overlay: %w", err)
	}

	if err := initTemplates(overlayFS); err != nil {
		return err
	}

	p, err := parsers.Get(parserName)
	if err != nil {
		return err
	}
	generateOptions := extractGenerateOptions(ops)
	if len(generateOptions.CommandDeclarations) != 0 {
		var copied parsers.ParseOptions
		if options != nil {
			copied = *options
		} else {
			copied.Recursive = true
		}
		copied.CommandDeclarations = append(append([]parsers.CommandDeclaration(nil), copied.CommandDeclarations...), generateOptions.CommandDeclarations...)
		options = &copied
	}
	if parserName != "commentv1" && options != nil && len(options.CommandDeclarations) != 0 {
		return fmt.Errorf("command metadata requires parser commentv1, got %q", parserName)
	}

	// inputFS is already rooted at the source directory, so we parse from "."
	dataModel, err := p.Parse(inputFS, ".", options)
	if err != nil {
		return err
	}
	if len(dataModel.Commands) == 0 {
		return fmt.Errorf("no commands found in %s", dir)
	}

	if err := resolveCLIParsers(dataModel.Commands, generateOptions.CLIParser); err != nil {
		return err
	}
	for _, cmd := range dataModel.Commands {
		cmd.SuppressGoGenerate = generateOptions.SuppressGoGenerate
		cmd.RegenerationCommand = generateOptions.RegenerationCommand
	}

	if err := dataModel.Validate(); err != nil {
		return err
	}

	dataModel.GoVersion = getGoVersion(inputFS)
	if err := validateGeneratedGoVersion(dataModel.GoVersion); err != nil {
		return err
	}
	prov := GetProvenance(replaceTemplates, projectProvenance, timestamp, provVersion, provCommit, provDate)
	dataModel.Provenance = &prov

	collector := NewCollectingFileWriter()

	for _, cmd := range dataModel.Commands {
		cmdOutDir := filepath.Join(dir, "cmd", cmd.MainCmdName)
		assignUsageFileNames(cmd.MainCmdName, cmd.SubCommands)
		if err := generateFile(collector, cmdOutDir, "main.go", "main.go.gotmpl", cmd, true); err != nil {
			return err
		}
		if err := generateFile(collector, cmdOutDir, "process.go", "process.go.gotmpl", cmd, true); err != nil {
			return err
		}
		if err := generateFile(collector, cmdOutDir, "root.go", "root.go.gotmpl", cmd, true); err != nil {
			return err
		}
		if err := generateFile(collector, filepath.Join(dir, "cmd"), "errors.go", "errors.go.gotmpl", cmd, true); err != nil {
			return err
		}
		if err := generateFile(collector, cmdOutDir, "flag_helpers.go", "flag_helpers.go.gotmpl", cmd, true); err != nil {
			return err
		}
		if commandUsesPlusMinus(cmd) {
			if err := generateFile(collector, cmdOutDir, "plus_minus.go", "plus_minus.go.gotmpl", cmd, true); err != nil {
				return err
			}
		}
		if err := generateFile(collector, filepath.Join(dir, "cmd"), "AGENTS.md", "agents.md.gotmpl", cmd, false); err != nil {
			return err
		}
		if err := generateFile(collector, cmdOutDir, "root_test.go", "root_test.go.gotmpl", cmd, true); err != nil {
			return err
		}
		cmdTemplatesDir := filepath.Join(cmdOutDir, "templates")
		if err := generateFile(collector, cmdTemplatesDir, "templates.go", "templates.go.gotmpl", cmd, true); err != nil {
			return err
		}
		rootUsage := &model.SubCommand{
			Metadata:               cmd.Metadata,
			ManSections:            cmd.ManSections,
			Command:                cmd,
			SubCommands:            cmd.SubCommands,
			SubCommandDescription:  cmd.Description,
			SubCommandExtendedHelp: cmd.ExtendedHelp,
			SubCommandFunctionName: cmd.FunctionName,
			Parameters:             cmd.Parameters,
			ReturnsError:           cmd.ReturnsError,
			ReturnCount:            cmd.ReturnCount,
			UsageFileName:          fmt.Sprintf("%s_usage.txt", strings.ToLower(cmd.MainCmdName)),
			SubCommandPackageName:  cmd.CommandPackageName,
			SubCommandStructName:   "RootCmd",
			SubCommandName:         "",
			ResolvedCLIParser:      cmd.ResolvedCLIParser,
		}
		if err := generateFile(collector, cmdTemplatesDir, rootUsage.UsageFileName, "usage.txt.gotmpl", rootUsage, false); err != nil {
			return err
		}
		if manDir != "" {
			if err := generateFile(collector, manDir, sanitizeManFileName(cmd.MainCmdName, ""), "man.gotmpl", rootUsage, false); err != nil {
				return err
			}
		}
		for _, subCmd := range cmd.SubCommands {
			if err := generateSubCommandFiles(collector, cmdOutDir, cmdTemplatesDir, manDir, subCmd); err != nil {
				return err
			}
		}
	}

	if err := planLegacyAgentsMigration(writer, collector, filepath.Join(dir, "cmd", "AGENTS.md"), filepath.Join(dir, "cmd", "agents.md")); err != nil {
		return err
	}

	if err := collector.Verify(writer, force); err != nil {
		return err
	}
	// Validate and render the entire request before removing owned old output.
	if clean {
		if err := CleanGeneratedFiles(dir, manDir); err != nil {
			return fmt.Errorf("failed to clean generated files: %w", err)
		}
	}

	return collector.Commit(writer)
}

const generatedAgentGuidanceHeader = "<!-- Code generated by github.com/arran4/go-subcommand/cmd/gosubc. DO NOT EDIT. -->"

func planLegacyAgentsMigration(writer FileWriter, collector *CollectingFileWriter, canonicalPath, legacyPath string) error {
	content, err := writer.ReadFile(legacyPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect legacy agent guidance %s: %w", legacyPath, err)
	}
	if !isOwnedGeneratedAgentGuidance(content) {
		return fmt.Errorf("legacy agent guidance %s is not generated by gosubc; rename or remove it before generating AGENTS.md", legacyPath)
	}
	if _, ok := writer.(FileRemover); !ok {
		return fmt.Errorf("cannot remove obsolete generated file %s: supplied FileWriter does not implement FileRemover", legacyPath)
	}
	if strings.EqualFold(canonicalPath, legacyPath) {
		identity, ok := writer.(FileIdentity)
		if !ok {
			return fmt.Errorf("cannot safely migrate legacy agent guidance %s: supplied FileWriter does not implement FileIdentity for a case-only rename", legacyPath)
		}
		same, err := identity.SameFile(canonicalPath, legacyPath)
		if err != nil {
			return fmt.Errorf("identify legacy agent guidance %s: %w", legacyPath, err)
		}
		if same {
			if _, ok := writer.(FileRenamer); !ok {
				return fmt.Errorf("cannot safely migrate legacy agent guidance %s: supplied FileWriter does not implement FileRenamer for a case-only rename", legacyPath)
			}
			collector.Renames[legacyPath] = canonicalPath
			return nil
		}
	}
	collector.Deletes[legacyPath] = true
	return nil
}

func isOwnedGeneratedAgentGuidance(content []byte) bool {
	return bytes.HasPrefix(content, []byte(generatedAgentGuidanceHeader+"\n")) || bytes.Equal(content, []byte(generatedAgentGuidanceHeader))
}

func isGenerated(content []byte) bool {
	s := string(content)
	markers := []string{
		"Code generated by github.com/arran4/go-subcommand/cmd/gosubc",
		"Generated by github.com/arran4/go-subcommand/cmd/gosubc",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
func assignUsageFileNames(mainCmdName string, subCommands []*model.SubCommand) {
	type usageTarget struct {
		command *model.SubCommand
		path    []string
	}

	var targets []usageTarget
	var collect func([]*model.SubCommand, []string)
	collect = func(commands []*model.SubCommand, parentPath []string) {
		for _, sc := range commands {
			path := append(append([]string(nil), parentPath...), sc.SubCommandName)
			targets = append(targets, usageTarget{command: sc, path: path})
			collect(sc.SubCommands, path)
		}
	}
	collect(subCommands, nil)

	leafCount := make(map[string]int)
	for _, target := range targets {
		leafCount[strings.ToLower(target.path[len(target.path)-1])]++
	}

	seen := make(map[string]int)
	if mainCmdName != "" {
		seen[strings.ToLower(mainCmdName)] = 1
	}
	for _, target := range targets {
		name := strings.ToLower(target.path[len(target.path)-1])
		if leafCount[name] > 1 && len(target.path) > 1 {
			parts := make([]string, len(target.path))
			for i, part := range target.path {
				parts[i] = strings.ToLower(part)
			}
			name = strings.Join(parts, "_")
		}

		count := seen[name]
		seen[name]++
		if count > 0 {
			name = fmt.Sprintf("%s_%d", name, count)
		}
		target.command.UsageFileName = fmt.Sprintf("%s_usage.txt", name)
	}
}

func sanitizeManFileName(mainCmdName, subCmdSequence string) string {
	safeMain := filepath.Base(mainCmdName)
	// subCmdSequence is space separated
	parts := strings.Fields(subCmdSequence)
	var safeParts []string
	for _, p := range parts {
		safeParts = append(safeParts, filepath.Base(p))
	}
	safeSeq := strings.Join(safeParts, "-")
	if safeSeq == "" {
		return fmt.Sprintf("%s.1", safeMain)
	}
	return fmt.Sprintf("%s-%s.1", safeMain, safeSeq)
}

func subCommandFileBaseName(subCmd *model.SubCommand) string {
	fileName := strings.ReplaceAll(parsers.ToKebabCase(subCmd.SubCommandStructName), "-", "_")
	if !subCmd.IsAppRoot() {
		switch fileName {
		case "root", "main", "process", "flag_helpers", "errors":
			return fileName + "2"
		case "plus_minus":
			if subCmd.Command != nil && commandUsesPlusMinus(subCmd.Command) {
				return fileName + "2"
			}
		}
	}
	return fileName
}

func generateSubCommandFiles(writer FileWriter, cmdOutDir, cmdTemplatesDir, manDir string, subCmd *model.SubCommand) error {
	fileName := subCommandFileBaseName(subCmd)
	if err := generateFile(writer, cmdOutDir, fileName+".go", "cmd.go.gotmpl", subCmd, true); err != nil {
		return err
	}
	if err := generateFile(writer, cmdOutDir, fileName+"_test.go", "cmd_test.go.gotmpl", subCmd, true); err != nil {
		return err
	}
	if err := generateFile(writer, cmdTemplatesDir, subCmd.UsageFileName, "usage.txt.gotmpl", subCmd, false); err != nil {
		return err
	}
	if manDir != "" {
		manFileName := sanitizeManFileName(subCmd.MainCmdName, subCmd.SubCommandSequence())
		if err := generateFile(writer, manDir, manFileName, "man.gotmpl", subCmd, false); err != nil {
			return err
		}
	}
	for _, s := range subCmd.SubCommands {
		if err := generateSubCommandFiles(writer, cmdOutDir, cmdTemplatesDir, manDir, s); err != nil {
			return err
		}
	}
	return nil
}

// Helper to bridge legacy parse calls
func parse(dir string, parserName string, options *parsers.ParseOptions, ops ...any) (*model.DataModel, error) {
	if dir == "" {
		dir = "."
	}
	var fsys fs.FS
	for _, opt := range ops {
		if f, ok := opt.(fs.FS); ok {
			fsys = f
			break
		}
	}
	if fsys == nil {
		fsys = os.DirFS(dir)
	}
	p, err := parsers.Get(parserName)
	if err != nil {
		return nil, err
	}
	return p.Parse(fsys, ".", options)
}
func initTemplates(fsys fs.FS) error {
	var err error
	templates, err = ParseTemplates(fsys)
	return err
}

var usageTemplateDelimsReplacer = strings.NewReplacer(
	"{{", `{{"{{"}}`,
	"}}", `{{"}}"}}`,
)

func escapeUsageText(s string) string {
	return usageTemplateDelimsReplacer.Replace(s)
}

func ParseTemplates(fsys fs.FS) (*template.Template, error) {
	tmpl := template.New("").Funcs(template.FuncMap{
		"plusMinusNames":  plusMinusParameterNames,
		"lower":           strings.ToLower,
		"title":           func(s string) string { return cases.Title(language.Und, cases.NoLower).String(s) },
		"upper":           strings.ToUpper,
		"replace":         strings.ReplaceAll,
		"escapeUsageText": escapeUsageText,
		"add":             func(a, b int) int { return a + b },
		"until": func(n int) []int {
			res := make([]int, n)
			for i := 0; i < n; i++ {
				res[i] = i
			}
			return res
		},
		"list": func(args ...interface{}) []interface{} {
			return args
		},
		"append": func(list []interface{}, args ...interface{}) []interface{} {
			return append(list, args...)
		},
		"paramImports":       parameterImports,
		"paramImportsExcept": parameterImportsExcept,
		"commandImports":     commandImports,
		"subCommandImports":  subCommandImports,
		"minGoVersion": func(min, current string) bool {
			return semver.Compare("v"+current, "v"+min) >= 0
		},
		"base":                filepath.Base,
		"goString":            strconv.Quote,
		"isDefaultExpression": isDefaultExpression,
	})

	var parseFS func(currentFS fs.FS) error
	parseFS = func(currentFS fs.FS) error {
		if ofs, ok := currentFS.(*overlayFS); ok {
			if err := parseFS(ofs.base); err != nil {
				return err
			}
			return parseFS(ofs.overlay)
		}

		var patterns []string
		err := fs.WalkDir(currentFS, "templates", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".gotmpl") {
				patterns = append(patterns, path)
			}
			return nil
		})
		if err != nil {
			// Some overlays might not have a full templates/ tree, ignore not found errors
			if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			// Wait, the walk dir will fail if it's not a folder, we can just walk the root ""
			return fmt.Errorf("error walking templates: %w", err)
		}

		if len(patterns) > 0 {
			var parseErr error
			tmpl, parseErr = tmpl.ParseFS(currentFS, patterns...)
			if parseErr != nil {
				return fmt.Errorf("error parsing templates: %w", parseErr)
			}
		}
		return nil
	}

	err := parseFS(fsys)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

type templateImport struct {
	Alias string
	Path  string
	Fixed bool
}

type fileImports struct {
	Standard []templateImport
	Other    []templateImport
}

func parameterImports(params []*model.FunctionParameter) []templateImport {
	return parameterImportsExcept(params, "")
}

func parameterImportsExcept(params []*model.FunctionParameter, excludedPath string) []templateImport {
	seen := map[string]bool{}
	var imports []templateImport
	add := func(ref *model.FuncRef) {
		if ref == nil || ref.ImportPath == "" || ref.ImportPath == excludedPath || seen[ref.ImportPath] {
			return
		}
		seen[ref.ImportPath] = true
		alias := ""
		if ref.CommandPackageName != "" && ref.CommandPackageName != path.Base(ref.ImportPath) {
			alias = ref.CommandPackageName
		}
		imports = append(imports, templateImport{Alias: alias, Path: ref.ImportPath})
	}
	for _, p := range params {
		// Constructor context is supplied through anonymous parent embedding. This
		// generated file does not spell its type or provider directly.
		if p.Inherited && p.HasConstructor() {
			continue
		}
		if p.TypeImportPath != "" {
			alias := ""
			if !p.IsReader() && !p.IsWriter() {
				qualifier := strings.SplitN(p.BaseType(), ".", 2)[0]
				if qualifier != path.Base(p.TypeImportPath) {
					alias = qualifier
				}
			}
			add(&model.FuncRef{ImportPath: p.TypeImportPath, CommandPackageName: alias})
		}
		if p.Parser.Type == model.ParserTypeCustom {
			add(p.Parser.Func)
		}
		if p.Generator.Type == model.SourceTypeGenerator {
			add(p.Generator.Func)
		}
		if p.DefaultExpr == nil && isDefaultExpression(p.Default) {
			if expr, err := parser.ParseExpr(p.Default); err == nil {
				var extractPkg func(e ast.Expr) string
				extractPkg = func(e ast.Expr) string {
					switch v := e.(type) {
					case *ast.ParenExpr:
						return extractPkg(v.X)
					case *ast.CallExpr:
						if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
							if id, ok := sel.X.(*ast.Ident); ok {
								return id.Name
							}
						}
						return ""
					case *ast.SelectorExpr:
						if id, ok := v.X.(*ast.Ident); ok {
							return id.Name
						}
						return ""
					default:
						return ""
					}
				}

				pkgName := extractPkg(expr)
				if pkgName != "" {
					// DefaultExpr carries resolved metadata when available; this
					// fallback preserves legacy selector defaults.
					add(&model.FuncRef{ImportPath: pkgName, CommandPackageName: pkgName})
				}
			}
		}
		if p.DefaultExpr != nil {
			add(p.DefaultExpr)
		}
		if p.IsWriter() || p.IsReader() {
			if !seen["os"] {
				seen["os"] = true
				imports = append(imports, templateImport{Path: "os"})
			}
		}
	}
	return imports
}

func deduplicateAndSortImports(imports []templateImport) fileImports {
	// Plan import identifiers once per generated file.  A path is semantic
	// identity; Alias is merely the preferred spelling found in parsed source.
	// The resulting Alias is the only spelling emitted by the import block.
	// Callers that render references prepare their model from this same plan.
	seen := make(map[string]templateImport)
	for _, imp := range imports {
		if imp.Path == "" {
			continue
		}
		previous, ok := seen[imp.Path]
		if imp.Fixed {
			seen[imp.Path] = imp
			continue
		}
		if previous.Fixed {
			continue
		}
		if !ok || (previous.Alias == "" && imp.Alias != "") || (previous.Alias != "" && imp.Alias != "" && imp.Alias < previous.Alias) {
			seen[imp.Path] = imp
		}
	}

	paths := make([]string, 0, len(seen))
	for importPath := range seen {
		paths = append(paths, importPath)
	}
	sort.Slice(paths, func(i, j int) bool {
		leftFixed := seen[paths[i]].Fixed || isGeneratedImport(paths[i])
		rightFixed := seen[paths[j]].Fixed || isGeneratedImport(paths[j])
		if leftFixed != rightFixed {
			return leftFixed
		}
		return paths[i] < paths[j]
	})

	used := make(map[string]bool, len(paths))
	finalImports := make([]templateImport, 0, len(paths))
	for _, importPath := range paths {
		imp := seen[importPath]
		preferred := imp.Alias
		if preferred == "" {
			preferred = path.Base(importPath)
		}
		identifier := ""
		if imp.Fixed || isGeneratedImport(importPath) {
			identifier = path.Base(importPath)
		} else {
			identifier = uniqueImportIdentifier(preferred, used)
		}
		used[identifier] = true
		if identifier == path.Base(importPath) {
			imp.Alias = ""
		} else {
			imp.Alias = identifier
		}
		finalImports = append(finalImports, imp)
	}

	var standard []templateImport
	var other []templateImport
	for _, imp := range finalImports {
		if !strings.Contains(imp.Path, ".") || imp.Path == "errors" || imp.Path == "time" || imp.Path == "strconv" || imp.Path == "flag" || imp.Path == "fmt" || imp.Path == "io" || imp.Path == "os" || imp.Path == "strings" || imp.Path == "sync" {
			standard = append(standard, imp)
		} else {
			other = append(other, imp)
		}
	}

	sort.Slice(standard, func(i, j int) bool {
		return standard[i].Path < standard[j].Path
	})

	sort.Slice(other, func(i, j int) bool {
		return other[i].Path < other[j].Path
	})

	return fileImports{
		Standard: standard,
		Other:    other,
	}
}

func isGeneratedImport(importPath string) bool {
	switch importPath {
	case "errors", "flag", "fmt", "io", "os", "slices", "strconv", "strings", "sync", "time":
		return true
	default:
		return false
	}
}

func uniqueImportIdentifier(preferred string, used map[string]bool) string {
	identifier := sanitizeImportIdentifier(preferred)
	if identifier == "" {
		identifier = "pkg"
	}
	if !used[identifier] {
		return identifier
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s_%d", identifier, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}

func sanitizeImportIdentifier(identifier string) string {
	var b strings.Builder
	for _, r := range identifier {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	result := b.String()
	if result == "" {
		return ""
	}
	if first := result[0]; first >= '0' && first <= '9' {
		result = "pkg_" + result
	}
	for _, reserved := range model.ReservedKeywords {
		if result == reserved {
			return "pkg_" + result
		}
	}
	return result
}

func subCommandImports(cmd *model.SubCommand, excludedPath string) fileImports {
	imports := parameterImportsExcept(cmd.Parameters, excludedPath)
	imports = append(imports, templateImport{Path: "flag"}, templateImport{Path: "fmt"}, templateImport{Path: "os"})
	imports = append(imports, templateImport{Path: cmd.PackagePath + "/cmd", Fixed: true})

	if cmd.SubCommandFunctionName != "" && cmd.ReturnsError {
		imports = append(imports, templateImport{Path: "errors"})
	}

	for _, p := range cmd.Parameters {
		if p.Type == "time.Duration" || p.Type == "[]time.Duration" || p.Type == "*time.Duration" || p.Type == "[]*time.Duration" {
			imports = append(imports, templateImport{Path: "time"})
		}

	}

	if cmd.ImportPath != "" && (cmd.ImportPath != excludedPath || (cmd.HasAction() && cmd.CallPackage() != "")) {
		alias := cmd.ImportAlias()
		if cmd.HasAction() || cmd.CallPackage() != "" {
			imports = append(imports, templateImport{Alias: alias, Path: cmd.ImportPath})
		}
	}
	if cmd.ImportPath != "" && cmd.CallPackage() != "" && cmd.CallPackage() != "main" && cmd.ImportPath != excludedPath {
		imports = append(imports, templateImport{Alias: cmd.ImportAlias(), Path: cmd.ImportPath})
	}
	imports = append(imports, runtimeParserImports(cmd.ResolvedCLIParser, cmd.Parameters)...)
	return deduplicateAndSortImports(imports)
}

func commandImports(cmd *model.Command, excludedPath string) fileImports {
	imports := parameterImportsExcept(cmd.Parameters, excludedPath)
	imports = append(imports, templateImport{Path: "flag"}, templateImport{Path: "fmt"}, templateImport{Path: "io"}, templateImport{Path: "os"}, templateImport{Path: "sync"})
	imports = append(imports, templateImport{Path: cmd.PackagePath + "/cmd", Fixed: true})

	if cmd.FunctionName != "" && cmd.ReturnsError {
		imports = append(imports, templateImport{Path: "errors"})
	}

	for _, p := range cmd.Parameters {
		if p.Type == "time.Duration" || p.Type == "[]time.Duration" || p.Type == "*time.Duration" || p.Type == "[]*time.Duration" {
			imports = append(imports, templateImport{Path: "time"})
		}

	}

	if cmd.ImportPath != "" && (cmd.ImportPath != excludedPath || (cmd.HasAction() && cmd.CallPackage() != "")) {
		alias := cmd.ImportAlias()
		if cmd.HasAction() || cmd.CallPackage() != "" {
			imports = append(imports, templateImport{Alias: alias, Path: cmd.ImportPath})
		}
	}
	if cmd.ImportPath != "" && cmd.CallPackage() != "" && cmd.CallPackage() != "main" && cmd.ImportPath != excludedPath {
		imports = append(imports, templateImport{Alias: cmd.ImportAlias(), Path: cmd.ImportPath})
	}
	imports = append(imports, runtimeParserImports(cmd.ResolvedCLIParser, cmd.Parameters)...)
	return deduplicateAndSortImports(imports)
}

func runtimeParserImports(parser string, params []*model.FunctionParameter) []templateImport {
	if parser == "plus-minus" {
		if plusMinusConversionNeedsStrconv(params) {
			return []templateImport{{Path: "strconv"}}
		}
		return nil
	}
	imports := []templateImport{}

	needsStrconv := false
	for _, param := range params {
		if param.IsPositional && !param.HasGenerator() && !param.Inherited && !param.HasCustomParser() && strings.Contains(param.ParserCall("arg"), "strconv.") {
			needsStrconv = true
		}
	}

	if parser == "gnu" || parser == "" {
		imports = append(imports, templateImport{Path: "strings"})

		for _, param := range params {
			if !param.HasGenerator() && !param.Inherited && !param.IsPositional {
				t := param.Type

				// boolean flags with a long alias emit ParseBool inside gnu branch
				if t == "bool" || t == "*bool" || t == "[]bool" || t == "[]*bool" {
					hasLong := false
					for _, alias := range param.FlagAliases {
						if strings.HasPrefix(alias, "--") {
							hasLong = true
							break
						}
					}
					if len(param.FlagAliases) == 0 {
						if len(param.Name) > 1 {
							hasLong = true
						}
					}

					if len(param.Name) > 1 {
						hasLong = true
					}

					if hasLong {
						needsStrconv = true
					}
				}

				// non-bool values rendered through ParserCall
				if t != "bool" && t != "string" && !strings.Contains(t, "io.") {
					call := param.ParserCall("value")
					if strings.Contains(call, "strconv.") {
						needsStrconv = true
					}
				}
			}
		}
	} else if parser == "go-flag" {
		for _, param := range params {
			if !param.HasGenerator() && !param.Inherited && !param.IsPositional {
				t := param.Type
				isPointer := strings.HasPrefix(t, "*")
				if isPointer {
					t = t[1:]
				}

				// Pointers and manually-converted types in go-flag
				if isPointer && (t == "bool" || t == "int" || t == "int64" || t == "int32" || t == "int16" || t == "int8" || t == "uint" || t == "uint64" || t == "uint32" || t == "uint16" || t == "uint8" || t == "float64" || t == "float32") {
					needsStrconv = true
				}
				if !isPointer && (t == "int32" || t == "int16" || t == "int8" || t == "uint32" || t == "uint16" || t == "uint8" || t == "float32") {
					needsStrconv = true
				}

				// Any custom parsers that emit strconv internally in the generated file?
				if param.HasCustomParser() {
					call := param.ParserCall("value")
					if strings.Contains(call, "strconv.") {
						needsStrconv = true
					}
				}
			}
		}
	}

	if needsStrconv {
		imports = append(imports, templateImport{Path: "strconv"})
	}
	return imports
}

func isDefaultExpression(def string) bool {
	expr, err := parser.ParseExpr(def)
	if err != nil {
		return false
	}

	var checkExpr func(e ast.Expr) bool
	checkExpr = func(e ast.Expr) bool {
		switch v := e.(type) {
		case *ast.ParenExpr:
			return checkExpr(v.X)
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				if _, ok := sel.X.(*ast.Ident); ok {
					return true
				}
			}
			if _, ok := v.Fun.(*ast.Ident); ok {
				return true
			}
			return false
		case *ast.SelectorExpr:
			if _, ok := v.X.(*ast.Ident); ok {
				// In Go, exported package constants/functions are always TitleCase.
				// This avoids treating filenames like "config.yaml" as expressions.
				if len(v.Sel.Name) > 0 {
					firstChar := v.Sel.Name[:1]
					if strings.ToUpper(firstChar) == firstChar && strings.ToLower(firstChar) != firstChar {
						return true
					}
				}
				return false
			}
			return false
		case *ast.Ident:
			if v.Name == "true" || v.Name == "false" || v.Name == "nil" {
				return false
			}
			// In Go, variables/constants from the current package are valid expressions,
			// but we want to avoid treating unquoted strings like "commentv1" as expressions.
			// A simple heuristic is to check if it's explicitly a known type. But we don't have type info.
			// Usually, default values that are identifiers and NOT true/false/nil are either unquoted strings
			// which should have been quoted, or they are actual constants in the code.
			// To be safe and since this was added for qualified constants/function calls:
			// Let's assume standalone identifiers (not selectors, not calls) are NOT expressions to be injected raw
			// unless they are true/false/nil (handled separately by the generator anyway).
			// If someone wants a constant like `myConst`, they should use `pkg.myConst` or we might misinterpret.
			// The issue asks for: "Qualified zero-argument function calls. Possibly qualified constants."
			// So standalone identifiers can be considered literal strings.
			return false
		default:
			return false
		}
	}

	return checkExpr(expr)
}

func getGoVersion(fsys fs.FS) string {
	b, err := fs.ReadFile(fsys, "go.mod")
	if err != nil {
		return "1.16"
	}
	f, err := modfile.Parse("go.mod", b, nil)
	if err != nil {
		return "1.16"
	}
	if f.Go == nil {
		return "1.16"
	}
	return f.Go.Version
}

// prepareGeneratedIdentifiers applies the import plan to the small rendering
// view held by a command.  The semantic model continues to retain import
// paths; only transient Go spelling is changed while one Go template is being
// rendered.  This keeps type identity distinct from emitted identifiers and
// avoids teaching every template a parallel alias lookup API.
func prepareGeneratedIdentifiers(data interface{}, templateName string) func() {
	if templateName != "root.go.gotmpl" && templateName != "cmd.go.gotmpl" {
		return func() {}
	}

	var params []*model.FunctionParameter
	var imports fileImports
	var commandRef *model.FuncRef
	switch value := data.(type) {
	case *model.Command:
		params = value.Parameters
		imports = commandImports(value, value.ImportPath)
		commandRef = &model.FuncRef{ImportPath: value.ImportPath, CommandPackageName: value.CommandPackageName}
	case *model.SubCommand:
		params = value.Parameters
		imports = subCommandImports(value, value.ImportPath)
		commandRef = &model.FuncRef{ImportPath: value.ImportPath, CommandPackageName: value.SubCommandPackageName}
	default:
		return func() {}
	}

	identifiers := importIdentifiers(imports)
	var restores []func()
	rewriteRef := func(ref *model.FuncRef) {
		if ref == nil || ref.ImportPath == "" {
			return
		}
		identifier, ok := identifiers[ref.ImportPath]
		if !ok || ref.CommandPackageName == identifier {
			return
		}
		previous := ref.CommandPackageName
		ref.CommandPackageName = identifier
		restores = append(restores, func() { ref.CommandPackageName = previous })
	}

	for _, parameter := range params {
		if parameter.TypeImportPath != "" && parameter.TypeName != "" {
			if identifier, ok := identifiers[parameter.TypeImportPath]; ok {
				base := parameter.BaseType()
				if dot := strings.IndexByte(base, '.'); dot >= 0 {
					previous := parameter.Type
					prefix := strings.TrimSuffix(parameter.Type, base)
					parameter.Type = prefix + identifier + base[dot:]
					restores = append(restores, func() { parameter.Type = previous })
				}
			}
		}
		if parameter.DefaultExpr != nil {
			previousDefault := parameter.Default
			previousQualifier := parameter.DefaultExpr.CommandPackageName
			rewriteRef(parameter.DefaultExpr)
			if previousQualifier != "" && parameter.DefaultExpr.CommandPackageName != previousQualifier {
				parameter.Default = strings.ReplaceAll(parameter.Default, previousQualifier+".", parameter.DefaultExpr.CommandPackageName+".")
				restores = append(restores, func() { parameter.Default = previousDefault })
			}
		}
		rewriteRef(parameter.Parser.Func)
		rewriteRef(parameter.Generator.Func)
	}

	if commandRef.ImportPath != "" {
		if identifier, ok := identifiers[commandRef.ImportPath]; ok {
			switch value := data.(type) {
			case *model.Command:
				previous := value.CommandPackageName
				value.CommandPackageName = identifier
				restores = append(restores, func() { value.CommandPackageName = previous })
			case *model.SubCommand:
				previous := value.SubCommandPackageName
				value.SubCommandPackageName = identifier
				restores = append(restores, func() { value.SubCommandPackageName = previous })
			}
		}
	}

	return func() {
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
	}
}

func importIdentifiers(imports fileImports) map[string]string {
	identifiers := make(map[string]string, len(imports.Standard)+len(imports.Other))
	for _, group := range [][]templateImport{imports.Standard, imports.Other} {
		for _, imp := range group {
			identifier := imp.Alias
			if identifier == "" {
				identifier = path.Base(imp.Path)
			}
			identifiers[imp.Path] = identifier
		}
	}
	return identifiers
}

const minimumGeneratedGoVersion = "1.16"

func validateGeneratedGoVersion(version string) error {
	if semver.Compare("v"+version, "v"+minimumGeneratedGoVersion) < 0 {
		return fmt.Errorf("generated applications require Go %s or newer; target module declares go %s", minimumGeneratedGoVersion, version)
	}
	return nil
}

func generateFile(writer FileWriter, dir, fileName, templateName string, data interface{}, formatCode bool) error {
	restore := prepareGeneratedIdentifiers(data, templateName)
	defer restore()
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	var content []byte
	if formatCode {
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to format generated code for %s: %w\n%s", fileName, err, buf.String())
		}
		content = formatted
	} else {
		content = buf.Bytes()
	}

	if err := writer.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Use filepath.Join for file paths as it is OS-aware, which is appropriate for FileWriter
	filePath := filepath.Join(dir, fileName)

	if err := writer.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	return nil
}

// GenerateCLI generates code using the legacy public CLI-facing API. It remains
// available so previously generated gosubc commands can bootstrap newer ones.
func GenerateCLI(dir string, manDir string, parserName string, cliParser string, paths []string, recursive bool, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, provVersion string, provCommit string, provDate string) error {
	return generateCLIWithOptions(dir, manDir, parserName, paths, recursive, force, clean, replaceTemplates, projectProvenance, timestamp, provVersion, provCommit, provDate, GenerateOptions{CLIParser: cliParser})
}

// GenerateCLICommand generates code using the compatible command API.
func GenerateCLICommand(dir string, manDir string, parserName string, cliParser string, paths []string, recursive bool, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, emitGoGenerate bool, provVersion string, provCommit string, provDate string) error {
	return generateCLIWithOptions(dir, manDir, parserName, paths, recursive, force, clean, replaceTemplates, projectProvenance, timestamp, provVersion, provCommit, provDate, GenerateOptions{
		CLIParser:          cliParser,
		SuppressGoGenerate: !emitGoGenerate,
	})
}

// GenerateCLICommandWithGuidance is the backwards-compatible explicit guidance entry point.
// Aliases: gen
//
// This command supports customizing templates via the --replace-template flag.
// You can provide multiple replacements in the following formats:
//
//	--replace-template <alias>=<file>    Replace a specific template by its alias (e.g., usage=myusage.gotmpl)
//	--replace-template <folder>          Overlay a folder containing templates onto the default templates
//	--replace-template <txtar>           Overlay a txtar archive containing templates
//
// Available aliases for individual file replacement include 'usage' (for usage.txt.gotmpl), 'man' (for man.gotmpl), etc.
// Flags:
//
//	dir:		--dir		(default: ".")		Project root directory containing go.mod
//	manDir:		--man-dir	(default: "")		Directory to generate man pages in optional
//	parserName:	--parser-name	(default: "commentv1")	Name of the parser to use
//	cliParser:	--cli-parser	(default: "gnu")	Name of the runtime CLI parser backend to use
//	paths:		--path		(default: nil)		Paths to search for subcommands (relative to dir)
//	recursive:	--recursive	(default: true)		Search recursively
//	force:		--force		(default: false)	Force overwrite of files not generated by gosubc
//	clean:		--clean		(default: false)	Clean/remove generated files before generating
//	replaceTemplates:	--replace-template			Replace templates. Formats: <alias>=<file>, <folder>, <txtar>.
//	projectProvenance: --project-provenance, --project (default: true) Include target Git metadata in provenance
//	timestamp:         --timestamp (default: true) Include timestamp in provenance
//	emitGoGenerate:    --emit-go-generate (default: true) Emit nested //go:generate directive in generated main.go
//	regenerationCommand: --regenerate-command (default: "") Project pinned regeneration command for generated AGENTS.md
//	provVersion:       --prov-version    (default: "") Overwrite provenance version
//	provCommit:        --prov-commit     (default: "") Overwrite provenance commit
//	provDate:          --prov-date       (default: "") Overwrite provenance date
func GenerateCLICommandWithGuidance(dir string, manDir string, parserName string, cliParser string, paths []string, recursive bool, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, emitGoGenerate bool, regenerationCommand string, provVersion string, provCommit string, provDate string) error {
	return generateCLIWithOptions(dir, manDir, parserName, paths, recursive, force, clean, replaceTemplates, projectProvenance, timestamp, provVersion, provCommit, provDate, GenerateOptions{
		CLIParser:           cliParser,
		SuppressGoGenerate:  !emitGoGenerate,
		RegenerationCommand: regenerationCommand,
	})
}

// GenerateCLIOptions specifies configuration and explicit flag overrides for code generation.
type GenerateCLIOptions struct {
	Dir                 string
	ManDir              string
	ParserName          string
	CLIParser           string
	Paths               []string
	Recursive           bool
	Force               bool
	Clean               bool
	ReplaceTemplates    []string
	ProjectProvenance   bool
	Timestamp           bool
	EmitGoGenerate      bool
	ProvVersion         string
	ProvCommit          string
	ProvDate            string
	ConfigPath          string
	RegenerationCommand string
	ExplicitFlags       map[string]bool
}

// GenerateCLIConfiguredWithOptions performs code generation with typed options and explicit flag overrides.
// It never reads ambient process arguments (os.Args).
func GenerateCLIConfiguredWithOptions(opts GenerateCLIOptions) error {
	var declarations []parsers.CommandDeclaration
	values := generateCLIValues{
		dir:               opts.Dir,
		manDir:            opts.ManDir,
		parserName:        opts.ParserName,
		cliParser:         opts.CLIParser,
		paths:             opts.Paths,
		recursive:         opts.Recursive,
		force:             opts.Force,
		clean:             opts.Clean,
		replaceTemplates:  opts.ReplaceTemplates,
		projectProvenance: opts.ProjectProvenance,
		timestamp:         opts.Timestamp,
		emitGoGenerate:    opts.EmitGoGenerate,
		provVersion:       opts.ProvVersion,
		provCommit:        opts.ProvCommit,
		provDate:          opts.ProvDate,
	}
	if opts.ConfigPath != "" {
		config, err := LoadGenerateConfig(opts.ConfigPath)
		if err != nil {
			return err
		}
		values.applyConfig(config, opts.ExplicitFlags)
		declarations = config.CommandDeclarations
	}
	return generateCLIWithOptions(values.dir, values.manDir, values.parserName, values.paths, values.recursive, values.force, values.clean, values.replaceTemplates, values.projectProvenance, values.timestamp, values.provVersion, values.provCommit, values.provDate, GenerateOptions{
		CommandDeclarations: declarations,
		CLIParser:           values.cliParser,
		SuppressGoGenerate:  !values.emitGoGenerate,
		RegenerationCommand: opts.RegenerationCommand,
	})
}

// GenerateCLIConfiguredAction is a subcommand `gosubc generate` generates the subcommand code
// Aliases: gen
//
// This command supports customizing templates via the --replace-template flag.
// You can provide multiple replacements in the following formats:
//
//	--replace-template <alias>=<file>    Replace a specific template by its alias (e.g., usage=myusage.gotmpl)
//	--replace-template <folder>          Overlay a folder containing templates onto the default templates
//	--replace-template <txtar>           Overlay a txtar archive containing templates
//
// Available aliases for individual file replacement include 'usage' (for usage.txt.gotmpl), 'man' (for man.gotmpl), etc.
// Flags:
//
//	dir:		--dir		(default: ".")		Project root directory containing go.mod
//	manDir:		--man-dir	(default: "")		Directory to generate man pages in optional
//	parserName:	--parser-name	(default: "commentv1")	Name of the parser to use
//	cliParser:	--cli-parser	(default: "gnu")	Name of the runtime CLI parser backend to use
//	paths:		--path		(default: nil)		Paths to search for subcommands (relative to dir)
//	recursive:	--recursive	(default: true)		Search recursively
//	force:		--force		(default: false)	Force overwrite of files not generated by gosubc
//	clean:		--clean		(default: false)	Clean/remove generated files before generating
//	replaceTemplates:	--replace-template			Replace templates. Formats: <alias>=<file>, <folder>, <txtar>.
//	projectProvenance: --project-provenance, --project (default: true) Include target Git metadata in provenance
//	timestamp:         --timestamp (default: true) Include timestamp in provenance
//	emitGoGenerate:    --emit-go-generate (default: true) Emit nested //go:generate directive in generated main.go
//	provVersion:       --prov-version    (default: "") Overwrite provenance version
//	provCommit:        --prov-commit     (default: "") Overwrite provenance commit
//	provDate:          --prov-date       (default: "") Overwrite provenance date
//	regenerationCommand: --regenerate-command (default: "") Project pinned regeneration command for generated AGENTS.md
//	configPath:        --config (default: "") Optional generation configuration file.
//	explicit:          (flag presence) Explicitly supplied parameters, keyed by Go parameter name.
func GenerateCLIConfiguredAction(dir string, manDir string, parserName string, cliParser string, paths []string, recursive bool, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, emitGoGenerate bool, regenerationCommand string, provVersion string, provCommit string, provDate string, configPath string, explicit map[string]bool) error {
	return GenerateCLIConfiguredWithOptions(GenerateCLIOptions{
		Dir:                 dir,
		ManDir:              manDir,
		ParserName:          parserName,
		CLIParser:           cliParser,
		Paths:               paths,
		Recursive:           recursive,
		Force:               force,
		Clean:               clean,
		ReplaceTemplates:    replaceTemplates,
		ProjectProvenance:   projectProvenance,
		Timestamp:           timestamp,
		EmitGoGenerate:      emitGoGenerate,
		RegenerationCommand: regenerationCommand,
		ProvVersion:         provVersion,
		ProvCommit:          provCommit,
		ProvDate:            provDate,
		ConfigPath:          configPath,
		ExplicitFlags:       explicit,
	})
}

// GenerateCLIConfiguredCommand generates with configuration overriding the supplied
// fallback values. It never reads process arguments. To explicitly override file
// settings, use GenerateCLIConfiguredWithOptions and its ExplicitFlags field.
func GenerateCLIConfiguredCommand(dir string, manDir string, parserName string, cliParser string, paths []string, recursive bool, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, emitGoGenerate bool, provVersion string, provCommit string, provDate string, configPath string) error {
	return GenerateCLIConfiguredAction(dir, manDir, parserName, cliParser, paths, recursive, force, clean, replaceTemplates, projectProvenance, timestamp, emitGoGenerate, "", provVersion, provCommit, provDate, configPath, nil)
}

func generateCLIWithOptions(dir string, manDir string, parserName string, paths []string, recursive bool, force bool, clean bool, replaceTemplates []string, projectProvenance bool, timestamp bool, provVersion string, provCommit string, provDate string, options GenerateOptions) error {
	if dir == "." {
		if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
			if root, err := findModuleRoot("."); err == nil {
				dir = root
			}
		}
	}
	return GenerateWithFS(os.DirFS(dir), &OSFileWriter{}, dir, manDir, parserName, &parsers.ParseOptions{
		SearchPaths: paths,
		Recursive:   recursive,
	}, force, clean, replaceTemplates, projectProvenance, timestamp, provVersion, provCommit, provDate, options)
}
