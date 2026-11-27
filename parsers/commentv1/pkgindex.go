package commentv1

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/arran4/go-subcommand/model"
)

type moduleIndex struct {
	fsys         fs.FS
	root         string
	modPath      string
	buildContext build.Context
	fset         *token.FileSet
	pkgs         map[string]*indexedPackage
	typeStack    map[string]bool
}

type indexedPackage struct {
	importPath string
	pkgName    string
	dir        string
	funcs      map[string]*indexedFunc
	types      map[string]*indexedType
}

type indexedFunc struct {
	name     string
	decl     *ast.FuncDecl
	file     *ast.File
	filename string
	line     int
}

type indexedType struct {
	name    string
	isAlias bool
	expr    ast.Expr
	file    *ast.File
}

type resolvedType struct {
	GeneratedType         string
	CanonicalKey          string
	PrimaryImportPath     string
	PrimaryPackageName    string
	PrimaryTypeName       string
	TypeImports           []model.TypeImportRef
	IsCallable            bool
	CallableReturnTypeKey string
	CallableReturnsError  bool
}

func newModuleIndex(fsys fs.FS, root, modPath string, buildContext build.Context, fset *token.FileSet) *moduleIndex {
	if root == "" {
		root = "."
	}
	return &moduleIndex{
		fsys:         fsys,
		root:         path.Clean(strings.ReplaceAll(root, "\\", "/")),
		modPath:      strings.TrimSuffix(modPath, "/"),
		buildContext: buildContext,
		fset:         fset,
		pkgs:         make(map[string]*indexedPackage),
		typeStack:    make(map[string]bool),
	}
}

func (m *moduleIndex) loadPackage(importPath string) (*indexedPackage, bool) {
	if m == nil || m.fsys == nil || importPath == "" || m.modPath == "" {
		return nil, false
	}
	if pkg, seen := m.pkgs[importPath]; seen {
		return pkg, pkg != nil
	}

	if importPath != m.modPath && !strings.HasPrefix(importPath, m.modPath+"/") {
		m.pkgs[importPath] = nil
		return nil, false
	}

	rel := strings.TrimPrefix(importPath, m.modPath)
	rel = strings.TrimPrefix(rel, "/")
	dir := m.root
	if rel != "" {
		if m.root == "." {
			dir = rel
		} else {
			dir = path.Join(m.root, rel)
		}
	}

	entries, err := fs.ReadDir(m.fsys, dir)
	if err != nil {
		m.pkgs[importPath] = nil
		return nil, false
	}

	pkg := &indexedPackage{
		importPath: importPath,
		dir:        dir,
		funcs:      make(map[string]*indexedFunc),
		types:      make(map[string]*indexedType),
	}

	var goFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		matches, matchErr := m.buildContext.MatchFile(dir, name)
		if matchErr != nil || !matches {
			continue
		}
		goFiles = append(goFiles, name)
	}
	sort.Strings(goFiles)

	parsedAny := false
	for _, name := range goFiles {
		filePath := name
		if dir != "." {
			filePath = path.Join(dir, name)
		}
		src, readErr := fs.ReadFile(m.fsys, filePath)
		if readErr != nil {
			continue
		}
		f, parseErr := parser.ParseFile(m.fset, filePath, src, parser.SkipObjectResolution)
		if parseErr != nil {
			continue
		}
		if f.Name == nil || strings.HasSuffix(f.Name.Name, "_test") {
			continue
		}
		parsedAny = true
		if pkg.pkgName == "" {
			pkg.pkgName = f.Name.Name
		}
		pkg.indexFile(m.fset, filePath, f)
	}

	if !parsedAny {
		m.pkgs[importPath] = nil
		return nil, false
	}
	m.pkgs[importPath] = pkg
	return pkg, true
}

func (p *indexedPackage) indexFile(fset *token.FileSet, filename string, f *ast.File) {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil || d.Name == nil {
				continue
			}
			line := 0
			if fset != nil {
				line = fset.Position(d.Pos()).Line
			}
			p.funcs[d.Name.Name] = &indexedFunc{
				name:     d.Name.Name,
				decl:     d,
				file:     f,
				filename: filename,
				line:     line,
			}
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil {
					continue
				}
				p.types[ts.Name.Name] = &indexedType{
					name:    ts.Name.Name,
					isAlias: ts.Assign.IsValid(),
					expr:    ts.Type,
					file:    f,
				}
			}
		}
	}
}

func (m *moduleIndex) resolveImportQualifier(qualifier string, fileImports []*ast.ImportSpec) (importPath string, pkgName string, ok bool) {
	for _, imp := range fileImports {
		if imp.Path == nil {
			continue
		}
		impPath := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				continue
			}
			if imp.Name.Name == qualifier {
				if pkg, found := m.loadPackage(impPath); found && pkg.pkgName != "" {
					return impPath, pkg.pkgName, true
				}
				return impPath, qualifier, true
			}
			continue
		}
		actualPkg := path.Base(impPath)
		if pkg, found := m.loadPackage(impPath); found && pkg.pkgName != "" {
			actualPkg = pkg.pkgName
		}
		if actualPkg == qualifier {
			return impPath, actualPkg, true
		}
	}
	return "", "", false
}

func (m *moduleIndex) resolveTypeExpr(expr ast.Expr, currentImportPath, currentPkgName string, fileImports []*ast.ImportSpec) resolvedType {
	return m.resolveTypeExprWithOptions(expr, currentImportPath, currentPkgName, fileImports, false)
}

func (m *moduleIndex) resolveTypeExprWithOptions(expr ast.Expr, currentImportPath, currentPkgName string, fileImports []*ast.ImportSpec, preserveFileAliases bool) resolvedType {
	switch t := expr.(type) {
	case *ast.ParenExpr:
		return m.resolveTypeExprWithOptions(t.X, currentImportPath, currentPkgName, fileImports, preserveFileAliases)

	case *ast.StarExpr:
		elem := m.resolveTypeExprWithOptions(t.X, currentImportPath, currentPkgName, fileImports, preserveFileAliases)
		elem.GeneratedType = "*" + elem.GeneratedType
		elem.CanonicalKey = "*" + elem.CanonicalKey
		elem.IsCallable = false
		elem.CallableReturnTypeKey = ""
		elem.CallableReturnsError = false
		return elem

	case *ast.ArrayType:
		elem := m.resolveTypeExprWithOptions(t.Elt, currentImportPath, currentPkgName, fileImports, preserveFileAliases)
		if t.Len == nil {
			elem.GeneratedType = "[]" + elem.GeneratedType
			elem.CanonicalKey = "[]" + elem.CanonicalKey
		} else {
			lenStr := formatArrayLenExpr(t.Len)
			elem.GeneratedType = "[" + lenStr + "]" + elem.GeneratedType
			elem.CanonicalKey = "[" + lenStr + "]" + elem.CanonicalKey
			elem.PrimaryImportPath = ""
			elem.PrimaryPackageName = ""
			elem.PrimaryTypeName = ""
		}
		elem.IsCallable = false
		elem.CallableReturnTypeKey = ""
		elem.CallableReturnsError = false
		return elem

	case *ast.Ellipsis:
		elem := m.resolveTypeExprWithOptions(t.Elt, currentImportPath, currentPkgName, fileImports, preserveFileAliases)
		elem.GeneratedType = "[]" + elem.GeneratedType
		elem.CanonicalKey = "[]" + elem.CanonicalKey
		elem.IsCallable = false
		elem.CallableReturnTypeKey = ""
		elem.CallableReturnsError = false
		return elem

	case *ast.MapType:
		k := m.resolveTypeExprWithOptions(t.Key, currentImportPath, currentPkgName, fileImports, preserveFileAliases)
		v := m.resolveTypeExprWithOptions(t.Value, currentImportPath, currentPkgName, fileImports, preserveFileAliases)
		return resolvedType{
			GeneratedType: "map[" + k.GeneratedType + "]" + v.GeneratedType,
			CanonicalKey:  "map[" + k.CanonicalKey + "]" + v.CanonicalKey,
			TypeImports:   mergeTypeImportRefs(k.TypeImports, v.TypeImports),
		}

	case *ast.ChanType:
		v := m.resolveTypeExprWithOptions(t.Value, currentImportPath, currentPkgName, fileImports, preserveFileAliases)
		prefix := "chan "
		switch t.Dir {
		case ast.RECV:
			prefix = "<-chan "
		case ast.SEND:
			prefix = "chan<- "
		}
		return resolvedType{
			GeneratedType: prefix + v.GeneratedType,
			CanonicalKey:  prefix + v.CanonicalKey,
			TypeImports:   v.TypeImports,
		}

	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return resolvedType{
				GeneratedType: "interface{}",
				CanonicalKey:  "interface{}",
			}
		}
		return resolvedType{
			GeneratedType: "interface{}",
			CanonicalKey:  "interface{}",
		}

	case *ast.Ident:
		if canon, ok := normalizePredeclaredTypeKey(t.Name); ok {
			return resolvedType{
				GeneratedType: t.Name,
				CanonicalKey:  canon,
			}
		}
		var canonical string
		if m != nil && currentImportPath != "" {
			if pkg, ok := m.loadPackage(currentImportPath); ok {
				if ts, ok := pkg.types[t.Name]; ok && ts.isAlias {
					guardKey := currentImportPath + "." + t.Name
					if !m.typeStack[guardKey] {
						m.typeStack[guardKey] = true
						target := m.resolveTypeExprWithOptions(ts.expr, currentImportPath, pkg.pkgName, ts.file.Imports, false)
						delete(m.typeStack, guardKey)
						res := resolvedType{
							GeneratedType:         t.Name,
							CanonicalKey:          target.CanonicalKey,
							PrimaryTypeName:       t.Name,
							IsCallable:            target.IsCallable,
							CallableReturnTypeKey: target.CallableReturnTypeKey,
							CallableReturnsError:  target.CallableReturnsError,
						}
						if currentImportPath != "" && currentPkgName != "" && currentPkgName != "main" {
							res.GeneratedType = currentPkgName + "." + t.Name
							res.PrimaryImportPath = currentImportPath
							res.PrimaryPackageName = currentPkgName
							res.TypeImports = []model.TypeImportRef{{ImportPath: currentImportPath, PackageName: currentPkgName}}
						}
						return res
					}
				}
			}
		}
		if currentImportPath != "" && currentPkgName != "" && currentPkgName != "main" {
			canonical = fmt.Sprintf("%q.%s", currentImportPath, t.Name)
			return resolvedType{
				GeneratedType:      currentPkgName + "." + t.Name,
				CanonicalKey:       canonical,
				PrimaryImportPath:  currentImportPath,
				PrimaryPackageName: currentPkgName,
				PrimaryTypeName:    t.Name,
				TypeImports:        []model.TypeImportRef{{ImportPath: currentImportPath, PackageName: currentPkgName}},
			}
		}
		canonical = t.Name
		if currentImportPath != "" {
			canonical = fmt.Sprintf("%q.%s", currentImportPath, t.Name)
		}
		return resolvedType{
			GeneratedType:   t.Name,
			CanonicalKey:    canonical,
			PrimaryTypeName: t.Name,
		}

	case *ast.SelectorExpr:
		qualIdent, ok := t.X.(*ast.Ident)
		if !ok {
			return resolvedType{}
		}
		qualifier := qualIdent.Name
		typeName := t.Sel.Name

		impPath := ""
		declPkgName := ""
		if m != nil {
			impPath, declPkgName, ok = m.resolveImportQualifier(qualifier, fileImports)
		}
		if !ok {
			impPath, declPkgName = resolveImportFromASTList(qualifier, fileImports)
		}
		if impPath == "" {
			return resolvedType{
				GeneratedType: qualifier + "." + typeName,
				CanonicalKey:  qualifier + "." + typeName,
			}
		}

		genPkg := declPkgName
		if preserveFileAliases || genPkg == "" {
			genPkg = qualifier
		}

		res := resolvedType{
			GeneratedType:      genPkg + "." + typeName,
			CanonicalKey:       fmt.Sprintf("%q.%s", impPath, typeName),
			PrimaryImportPath:  impPath,
			PrimaryPackageName: genPkg,
			PrimaryTypeName:    typeName,
			TypeImports:        []model.TypeImportRef{{ImportPath: impPath, PackageName: genPkg}},
		}

		if m != nil {
			if pkg, found := m.loadPackage(impPath); found {
				if ts, isType := pkg.types[typeName]; isType && ts.isAlias {
					guardKey := impPath + "." + typeName
					if !m.typeStack[guardKey] {
						m.typeStack[guardKey] = true
						target := m.resolveTypeExprWithOptions(ts.expr, impPath, pkg.pkgName, ts.file.Imports, false)
						delete(m.typeStack, guardKey)
						res.CanonicalKey = target.CanonicalKey
						res.IsCallable = target.IsCallable
						res.CallableReturnTypeKey = target.CallableReturnTypeKey
						res.CallableReturnsError = target.CallableReturnsError
					}
				}
			}
		}
		return res

	case *ast.FuncType:
		var genParams []string
		var canonParams []string
		var allImports []model.TypeImportRef
		if t.Params != nil {
			for _, field := range t.Params.List {
				_, isVar := field.Type.(*ast.Ellipsis)
				pt := m.resolveTypeExprWithOptions(field.Type, currentImportPath, currentPkgName, fileImports, preserveFileAliases)
				genElem := pt.GeneratedType
				canonElem := pt.CanonicalKey
				if isVar && strings.HasPrefix(genElem, "[]") {
					genElem = "..." + strings.TrimPrefix(genElem, "[]")
				}
				count := len(field.Names)
				if count == 0 {
					count = 1
				}
				for i := 0; i < count; i++ {
					genParams = append(genParams, genElem)
					canonParams = append(canonParams, canonElem)
				}
				allImports = mergeTypeImportRefs(allImports, pt.TypeImports)
			}
		}
		var genResults []string
		var resolvedResults []resolvedType
		if t.Results != nil {
			for _, field := range t.Results.List {
				rt := m.resolveTypeExprWithOptions(field.Type, currentImportPath, currentPkgName, fileImports, preserveFileAliases)
				count := len(field.Names)
				if count == 0 {
					count = 1
				}
				for i := 0; i < count; i++ {
					genResults = append(genResults, rt.GeneratedType)
					resolvedResults = append(resolvedResults, rt)
				}
				allImports = mergeTypeImportRefs(allImports, rt.TypeImports)
			}
		}
		var canonResults []string
		for _, rr := range resolvedResults {
			canonResults = append(canonResults, rr.CanonicalKey)
		}

		resStr := ""
		if len(genResults) == 1 && !strings.Contains(genResults[0], ",") {
			resStr = " " + genResults[0]
		} else if len(genResults) > 0 {
			resStr = " (" + strings.Join(genResults, ", ") + ")"
		}
		canonResStr := ""
		if len(canonResults) == 1 {
			canonResStr = " " + canonResults[0]
		} else if len(canonResults) > 1 {
			canonResStr = " (" + strings.Join(canonResults, ", ") + ")"
		}

		out := resolvedType{
			GeneratedType: "func(" + strings.Join(genParams, ", ") + ")" + resStr,
			CanonicalKey:  "func(" + strings.Join(canonParams, ", ") + ")" + canonResStr,
			TypeImports:   allImports,
		}
		if len(canonParams) == 0 && len(resolvedResults) > 0 && resolvedResults[0].CanonicalKey != "error" {
			if len(resolvedResults) == 1 {
				out.IsCallable = true
				out.CallableReturnTypeKey = resolvedResults[0].CanonicalKey
				out.CallableReturnsError = false
				out.PrimaryImportPath = resolvedResults[0].PrimaryImportPath
				out.PrimaryPackageName = resolvedResults[0].PrimaryPackageName
				out.PrimaryTypeName = resolvedResults[0].PrimaryTypeName
			} else if len(resolvedResults) == 2 && resolvedResults[1].CanonicalKey == "error" {
				out.IsCallable = true
				out.CallableReturnTypeKey = resolvedResults[0].CanonicalKey
				out.CallableReturnsError = true
				out.PrimaryImportPath = resolvedResults[0].PrimaryImportPath
				out.PrimaryPackageName = resolvedResults[0].PrimaryPackageName
				out.PrimaryTypeName = resolvedResults[0].PrimaryTypeName
			}
		}
		return out

	default:
		formatted, _ := formatType(expr)
		return resolvedType{
			GeneratedType: formatted,
			CanonicalKey:  formatted,
		}
	}
}

func resolveImportFromASTList(qualifier string, fileImports []*ast.ImportSpec) (importPath, pkgName string) {
	for _, imp := range fileImports {
		if imp.Path == nil {
			continue
		}
		impPath := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				continue
			}
			if imp.Name.Name == qualifier {
				return impPath, qualifier
			}
			continue
		}
		if path.Base(impPath) == qualifier {
			return impPath, qualifier
		}
	}
	return "", ""
}

func normalizePredeclaredTypeKey(name string) (string, bool) {
	switch name {
	case "byte":
		return "uint8", true
	case "rune":
		return "int32", true
	case "any":
		return "interface{}", true
	case "bool", "complex64", "complex128", "error", "float32", "float64",
		"int", "int8", "int16", "int32", "int64",
		"string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return name, true
	default:
		return "", false
	}
}

func formatArrayLenExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Value
	case *ast.Ellipsis:
		return "..."
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

func mergeTypeImportRefs(a, b []model.TypeImportRef) []model.TypeImportRef {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []model.TypeImportRef
	for _, list := range [][]model.TypeImportRef{a, b} {
		for _, item := range list {
			if item.ImportPath == "" || seen[item.ImportPath] {
				continue
			}
			seen[item.ImportPath] = true
			out = append(out, item)
		}
	}
	return out
}

func (m *moduleIndex) resolveIndexedFuncParams(ifn *indexedFunc, pkgImportPath, pkgName string) []model.ResolvedParamSpec {
	if ifn == nil || ifn.decl == nil || ifn.decl.Type == nil || ifn.decl.Type.Params == nil {
		return nil
	}
	var specs []model.ResolvedParamSpec
	for _, field := range ifn.decl.Type.Params.List {
		_, isVar := field.Type.(*ast.Ellipsis)
		rt := m.resolveTypeExpr(field.Type, pkgImportPath, pkgName, ifn.file.Imports)
		if len(field.Names) == 0 {
			specs = append(specs, model.ResolvedParamSpec{
				Type:                  rt.GeneratedType,
				CanonicalTypeKey:      rt.CanonicalKey,
				TypeImports:           rt.TypeImports,
				IsVariadic:            isVar,
				IsCallable:            rt.IsCallable,
				CallableReturnTypeKey: rt.CallableReturnTypeKey,
				CallableReturnsError:  rt.CallableReturnsError,
			})
			continue
		}
		for _, name := range field.Names {
			paramName := name.Name
			if paramName == "_" {
				paramName = ""
			}
			specs = append(specs, model.ResolvedParamSpec{
				Name:                  paramName,
				Type:                  rt.GeneratedType,
				CanonicalTypeKey:      rt.CanonicalKey,
				TypeImports:           rt.TypeImports,
				IsVariadic:            isVar,
				IsCallable:            rt.IsCallable,
				CallableReturnTypeKey: rt.CallableReturnTypeKey,
				CallableReturnsError:  rt.CallableReturnsError,
			})
		}
	}
	return specs
}

type funcResultClassification struct {
	kind             model.RequirementKind
	returnsError     bool
	returnType       string
	canonicalTypeKey string
	typeImportPath   string
	typePackageName  string
	typeName         string
	typeImports      []model.TypeImportRef
}

func (m *moduleIndex) classifyFuncResults(ifn *indexedFunc, pkgImportPath, pkgName string) (funcResultClassification, error) {
	if ifn == nil || ifn.decl == nil || ifn.decl.Type == nil || ifn.decl.Type.Results == nil {
		return funcResultClassification{}, fmt.Errorf("function %s has 0 return values; expected error, T, or (T, error)", ifn.name)
	}
	var results []resolvedType
	for _, field := range ifn.decl.Type.Results.List {
		rt := m.resolveTypeExpr(field.Type, pkgImportPath, pkgName, ifn.file.Imports)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			results = append(results, rt)
		}
	}
	switch len(results) {
	case 0:
		return funcResultClassification{}, fmt.Errorf("function %s has 0 return values; expected error, T, or (T, error)", ifn.name)
	case 1:
		r0 := results[0]
		if r0.CanonicalKey == "error" {
			return funcResultClassification{
				kind:         model.RequirementKindGuard,
				returnsError: true,
			}, nil
		}
		return funcResultClassification{
			kind:             model.RequirementKindProvider,
			returnsError:     false,
			returnType:       r0.GeneratedType,
			canonicalTypeKey: r0.CanonicalKey,
			typeImportPath:   r0.PrimaryImportPath,
			typePackageName:  r0.PrimaryPackageName,
			typeName:         r0.PrimaryTypeName,
			typeImports:      r0.TypeImports,
		}, nil
	case 2:
		r0, r1 := results[0], results[1]
		if r1.CanonicalKey != "error" {
			return funcResultClassification{}, fmt.Errorf("function %s has invalid second return type %s; expected (T, error)", ifn.name, r1.GeneratedType)
		}
		if r0.CanonicalKey == "error" {
			return funcResultClassification{}, fmt.Errorf("function %s has invalid signature (error, error); expected error, T, or (T, error)", ifn.name)
		}
		return funcResultClassification{
			kind:             model.RequirementKindProvider,
			returnsError:     true,
			returnType:       r0.GeneratedType,
			canonicalTypeKey: r0.CanonicalKey,
			typeImportPath:   r0.PrimaryImportPath,
			typePackageName:  r0.PrimaryPackageName,
			typeName:         r0.PrimaryTypeName,
			typeImports:      r0.TypeImports,
		}, nil
	default:
		return funcResultClassification{}, fmt.Errorf("function %s returns %d values; expected error, T, or (T, error)", ifn.name, len(results))
	}
}

// parseRequirementLine parses one `Requires:` directive value (supporting top-level comma separation).
func parseRequirementLine(
	value string,
	filename string,
	line int,
	declImportPath string,
	declPkgName string,
	fileImports []*ast.ImportSpec,
	isGoFile bool,
	idx *moduleIndex,
	localFileFuncs map[string]*ast.FuncDecl,
	localFile *ast.File,
) ([]*model.RequirementConfig, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("%s:%d: empty Requires directive", filename, line)
	}
	items := splitTopLevelComma(value)
	if len(items) == 0 {
		return nil, fmt.Errorf("%s:%d: empty Requires directive", filename, line)
	}
	reqs := make([]*model.RequirementConfig, 0, len(items))
	for _, rawItem := range items {
		req, err := parseSingleRequirement(rawItem, filename, line, declImportPath, declPkgName, fileImports, isGoFile, idx, localFileFuncs, localFile)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}

// splitTopLevelComma splits on commas outside parentheses and double quotes,
// preserving empty slots so callers can reject `a, , b` or `a,`.
func splitTopLevelComma(s string) []string {
	var parts []string
	var cur strings.Builder
	depth := 0
	inQuote := false
	escaped := false
	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			cur.WriteRune(r)
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
		}
		if !inQuote {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		if depth == 0 && !inQuote && r == ',' {
			parts = append(parts, cur.String())
			cur.Reset()
		} else {
			cur.WriteRune(r)
		}
	}
	parts = append(parts, cur.String())
	return parts
}

func parseSingleRequirement(
	rawItem string,
	filename string,
	line int,
	declImportPath string,
	declPkgName string,
	fileImports []*ast.ImportSpec,
	isGoFile bool,
	idx *moduleIndex,
	localFileFuncs map[string]*ast.FuncDecl,
	localFile *ast.File,
) (*model.RequirementConfig, error) {
	item := strings.TrimSpace(rawItem)
	if item == "" {
		return nil, fmt.Errorf("%s:%d: empty entry in Requires directive", filename, line)
	}
	if strings.HasPrefix(item, "!") {
		return nil, fmt.Errorf("%s:%d: negative requirement %q is not supported (reserved for future use)", filename, line, item)
	}

	aliasName := ""
	rest := item
	if eqIdx := findTopLevelChar(item, '='); eqIdx >= 0 {
		aliasName = strings.TrimSpace(item[:eqIdx])
		rest = strings.TrimSpace(item[eqIdx+1:])
		if aliasName == "" || !isGoIdentifier(aliasName) {
			return nil, fmt.Errorf("%s:%d: invalid requirement name %q", filename, line, aliasName)
		}
		if rest == "" {
			return nil, fmt.Errorf("%s:%d: missing requirement function after %q =", filename, line, aliasName)
		}
		if strings.HasPrefix(rest, "!") {
			return nil, fmt.Errorf("%s:%d: negative requirement %q is not supported (reserved for future use)", filename, line, rest)
		}
	}

	quotedPath, pkgAlias, fnName, tail, err := parseReqFuncRef(rest)
	if err != nil {
		return nil, fmt.Errorf("%s:%d: invalid requirement %q: %w", filename, line, item, err)
	}

	var explicitArgs []string
	hasExplicitArgs := false
	if len(tail) > 0 && tail[0] == '(' {
		endParen := findMatchingParen(tail, 0)
		if endParen < 0 {
			return nil, fmt.Errorf("%s:%d: unclosed argument list in requirement %q", filename, line, item)
		}
		argsBody := tail[1:endParen]
		hasExplicitArgs = true
		if strings.TrimSpace(argsBody) != "" {
			argParts := splitTopLevelComma(argsBody)
			for _, ap := range argParts {
				ap = strings.TrimSpace(ap)
				if ap == "" {
					return nil, fmt.Errorf("%s:%d: empty argument in requirement %q", filename, line, item)
				}
				explicitArgs = append(explicitArgs, ap)
			}
		}
		tail = tail[endParen+1:]
	}

	policy := model.RequirementPolicyOnDemand
	tail = strings.TrimSpace(tail)
	for tail != "" {
		if tail[0] != '(' {
			return nil, fmt.Errorf("%s:%d: unexpected trailing token %q in requirement %q", filename, line, tail, item)
		}
		endParen := findMatchingParen(tail, 0)
		if endParen < 0 {
			return nil, fmt.Errorf("%s:%d: unclosed modifier in requirement %q", filename, line, item)
		}
		modText := strings.ToLower(strings.TrimSpace(tail[1:endParen]))
		switch modText {
		case "always":
			if policy == model.RequirementPolicyAlways {
				return nil, fmt.Errorf("%s:%d: duplicate modifier (always) in requirement %q", filename, line, item)
			}
			policy = model.RequirementPolicyAlways
		default:
			return nil, fmt.Errorf("%s:%d: unsupported requirement modifier (%s) in %q; only (always) is supported", filename, line, modText, item)
		}
		tail = strings.TrimSpace(tail[endParen+1:])
	}

	ref, ifn, pkgImpPath, pkgRealName, err := resolveReqFunction(quotedPath, pkgAlias, fnName, item, declImportPath, declPkgName, fileImports, isGoFile, idx, localFileFuncs, localFile)
	if err != nil {
		return nil, fmt.Errorf("%s:%d: %w", filename, line, err)
	}

	cls, err := idx.classifyFuncResults(ifn, pkgImpPath, pkgRealName)
	if err != nil {
		return nil, fmt.Errorf("%s:%d: requirement %q: %w", filename, line, item, err)
	}
	sigParams := idx.resolveIndexedFuncParams(ifn, pkgImpPath, pkgRealName)

	return &model.RequirementConfig{
		Name:             aliasName,
		Func:             ref,
		ExplicitInputs:   explicitArgs,
		HasExplicitArgs:  hasExplicitArgs,
		Policy:           policy,
		Kind:             cls.kind,
		ReturnType:       cls.returnType,
		CanonicalTypeKey: cls.canonicalTypeKey,
		TypeImportPath:   cls.typeImportPath,
		TypePackageName:  cls.typePackageName,
		TypeName:         cls.typeName,
		TypeImports:      cls.typeImports,
		ReturnsError:     cls.returnsError,
		SignatureParams:  sigParams,
		SignatureKnown:   true,
		Filename:         filename,
		Line:             line,
	}, nil
}

func resolveReqFunction(
	quotedPath string,
	pkgAlias string,
	fnName string,
	rawItem string,
	declImportPath string,
	declPkgName string,
	fileImports []*ast.ImportSpec,
	isGoFile bool,
	idx *moduleIndex,
	localFileFuncs map[string]*ast.FuncDecl,
	localFile *ast.File,
) (*model.FuncRef, *indexedFunc, string, string, error) {
	var targetImportPath string
	var resolvedPkgName string

	if quotedPath != "" {
		targetImportPath = quotedPath
		resolvedPkgName = path.Base(quotedPath)
	} else if pkgAlias != "" {
		if !isGoFile {
			return nil, nil, "", "", fmt.Errorf("requirement %q uses unqualified package alias %q; use an explicit quoted import path like \"example.com/pkg\".%s in metadata declarations", rawItem, pkgAlias, fnName)
		}
		impPath, pkgReal, ok := idx.resolveImportQualifier(pkgAlias, fileImports)
		if !ok {
			return nil, nil, "", "", fmt.Errorf("requirement %q references unimported package qualifier %q", rawItem, pkgAlias)
		}
		targetImportPath = impPath
		resolvedPkgName = pkgReal
	} else {
		if !isGoFile {
			return nil, nil, "", "", fmt.Errorf("requirement %q uses local function %q outside a Go file; use an explicit quoted import path", rawItem, fnName)
		}
		targetImportPath = declImportPath
		resolvedPkgName = declPkgName
	}

	var pkg *indexedPackage
	var foundPkg bool
	if idx != nil && targetImportPath != "" {
		pkg, foundPkg = idx.loadPackage(targetImportPath)
	}
	if foundPkg {
		if pkg.pkgName != "" {
			resolvedPkgName = pkg.pkgName
		}
		if !ast.IsExported(fnName) && resolvedPkgName != "main" {
			return nil, nil, "", "", fmt.Errorf("requirement %q: function %s in package %q must be exported", rawItem, fnName, targetImportPath)
		}
		ifn, ok := pkg.funcs[fnName]
		if !ok {
			return nil, nil, "", "", fmt.Errorf("requirement %q: function %s not found in package %q", rawItem, fnName, targetImportPath)
		}
		ref := &model.FuncRef{FunctionName: fnName}
		if resolvedPkgName != "main" {
			ref.ImportPath = targetImportPath
			ref.PackagePath = targetImportPath
			ref.CommandPackageName = resolvedPkgName
		}
		return ref, ifn, targetImportPath, resolvedPkgName, nil
	}

	if quotedPath == "" && pkgAlias == "" && localFileFuncs != nil {
		if decl, ok := localFileFuncs[fnName]; ok {
			if !ast.IsExported(fnName) && declPkgName != "main" {
				return nil, nil, "", "", fmt.Errorf("requirement %q: function %s must be exported", rawItem, fnName)
			}
			ref := &model.FuncRef{FunctionName: fnName}
			if declPkgName != "" && declPkgName != "main" {
				ref.ImportPath = declImportPath
				ref.PackagePath = declImportPath
				ref.CommandPackageName = declPkgName
			}
			ifn := &indexedFunc{name: fnName, decl: decl, file: localFile}
			return ref, ifn, declImportPath, declPkgName, nil
		}
	}

	return nil, nil, "", "", fmt.Errorf("requirement %q: cannot resolve function %s in package %q", rawItem, fnName, targetImportPath)
}

func findTopLevelChar(s string, target rune) int {
	depth := 0
	inQuote := false
	escaped := false
	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
		}
		if !inQuote {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
			default:
				if depth == 0 && r == target {
					return i
				}
			}
		}
	}
	return -1
}

func findMatchingParen(s string, openIdx int) int {
	if openIdx < 0 || openIdx >= len(s) || s[openIdx] != '(' {
		return -1
	}
	depth := 0
	inQuote := false
	escaped := false
	for i := openIdx; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inQuote {
			escaped = true
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseReqFuncRef(s string) (quotedImportPath, pkgAlias, fnName, tail string, err error) {
	if strings.HasPrefix(s, "\"") {
		endQuote := -1
		escaped := false
		for i := 1; i < len(s); i++ {
			if escaped {
				escaped = false
				continue
			}
			if s[i] == '\\' {
				escaped = true
				continue
			}
			if s[i] == '"' {
				endQuote = i
				break
			}
		}
		if endQuote < 0 {
			return "", "", "", "", fmt.Errorf("unclosed quoted import path")
		}
		unquoted, uerr := strconv.Unquote(s[:endQuote+1])
		if uerr != nil || unquoted == "" {
			return "", "", "", "", fmt.Errorf("invalid quoted import path %s", s[:endQuote+1])
		}
		rest := s[endQuote+1:]
		if !strings.HasPrefix(rest, ".") {
			return "", "", "", "", fmt.Errorf("expected .FunctionName after quoted import path")
		}
		rest = rest[1:]
		ident, after := readIdentPrefix(rest)
		if ident == "" {
			return "", "", "", "", fmt.Errorf("missing function name after import path")
		}
		if strings.HasPrefix(after, ".") {
			return "", "", "", "", fmt.Errorf("unexpected '.' after function name %q", ident)
		}
		return unquoted, "", ident, after, nil
	}

	ident1, after1 := readIdentPrefix(s)
	if ident1 == "" {
		return "", "", "", "", fmt.Errorf("expected function identifier")
	}
	if strings.HasPrefix(after1, ".") {
		ident2, after2 := readIdentPrefix(after1[1:])
		if ident2 == "" {
			return "", "", "", "", fmt.Errorf("missing function name after qualifier %q", ident1)
		}
		if strings.HasPrefix(after2, ".") {
			return "", "", "", "", fmt.Errorf("unexpected extra '.' in qualified reference %q.%q", ident1, ident2)
		}
		return "", ident1, ident2, after2, nil
	}
	return "", "", ident1, after1, nil
}

func readIdentPrefix(s string) (string, string) {
	end := 0
	for end < len(s) {
		c := s[end]
		isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
		isDigit := c >= '0' && c <= '9'
		if end == 0 && !isLetter {
			break
		}
		if !isLetter && !isDigit {
			break
		}
		end++
	}
	return s[:end], s[end:]
}

func isGoIdentifier(s string) bool {
	if s == "" {
		return false
	}
	ident, rest := readIdentPrefix(s)
	return ident == s && rest == ""
}
