package commentv1

import (
	"go/ast"
	"go/token"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/go-subcommand/model"
)

func TestModuleIndex_PackageResolutionAndCrossFileLookup(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/modtest\n\ngo 1.22\n")},
		"app/cmd.go": {Data: []byte(`package app

import (
	myauth "example.com/modtest/go-auth"
	"example.com/modtest/other"
)

type Session struct{}
type UserAlias = myauth.User

// Cmd is a subcommand ` + "`app`" + `
func Cmd(
	u *UserAlias,
	s func() (*Session, error),
	inf func() *Session,
	b byte,
	r rune,
	a any,
	_ *other.Item,
) error {
	return nil
}
`)},
		"app/helpers.go": {Data: []byte(`package app

import myauth "example.com/modtest/go-auth"

func LocalHelper(u *myauth.User) error { return nil }
`)},
		"go-auth/auth.go": {Data: []byte(`package auth

type User struct{ Name string }

func CurrentUser() (*User, error) { return &User{Name: "alice"}, nil }
func unexportedHelper() error     { return nil }
`)},
		"other/item.go": {Data: []byte(`package other

type Item struct{}
`)},
	}

	fset := token.NewFileSet()
	idx := newModuleIndex(fsys, ".", "example.com/modtest", sourceBuildContext(fsys, nil), fset)

	appPkg, ok := idx.loadPackage("example.com/modtest/app")
	if !ok || appPkg.pkgName != "app" {
		t.Fatalf("loadPackage(app) = %v, %+v", ok, appPkg)
	}
	if _, ok := appPkg.funcs["LocalHelper"]; !ok {
		t.Fatalf("expected cross-file function LocalHelper in appPkg.funcs")
	}

	authPkg, ok := idx.loadPackage("example.com/modtest/go-auth")
	if !ok || authPkg.pkgName != "auth" {
		t.Fatalf("loadPackage(go-auth) = %v, %+v (want pkgName=auth)", ok, authPkg)
	}

	cmdFn := appPkg.funcs["Cmd"]
	if cmdFn == nil {
		t.Fatal("missing Cmd indexedFunc")
	}

	// Explicit alias in Go file imports
	if imp, name, ok := idx.resolveImportQualifier("myauth", cmdFn.file.Imports); !ok || imp != "example.com/modtest/go-auth" || name != "auth" {
		t.Fatalf("resolveImportQualifier(myauth) = %q, %q, %v", imp, name, ok)
	}
	// Non-imported package qualifier must fail
	if imp, name, ok := idx.resolveImportQualifier("unimported", cmdFn.file.Imports); ok {
		t.Fatalf("expected unimported qualifier to fail, got %q, %q", imp, name)
	}
	// Directory mismatch package without explicit alias in other file: direct import resolved via package declaration
	if imp, _, ok := idx.resolveImportQualifier("auth", []*ast.ImportSpec{{Path: &ast.BasicLit{Value: `"example.com/modtest/go-auth"`}}}); !ok || imp != "example.com/modtest/go-auth" {
		t.Fatalf("resolveImportQualifier(auth with go-auth dir) = %q, %v", imp, ok)
	}

	// Verify resolved parameter canonical keys, type alias expansion, predeclared aliases, and callables
	params := idx.resolveIndexedFuncParams(cmdFn, "example.com/modtest/app", "app")
	if len(params) != 7 {
		t.Fatalf("params len = %d, want 7", len(params))
	}

	// *UserAlias (type UserAlias = myauth.User) must match *"example.com/modtest/go-auth".User
	authParams := idx.resolveIndexedFuncParams(appPkg.funcs["LocalHelper"], "example.com/modtest/app", "app")
	if len(authParams) != 1 {
		t.Fatalf("authParams len = %d", len(authParams))
	}
	if params[0].CanonicalTypeKey != authParams[0].CanonicalTypeKey {
		t.Fatalf("alias canonical key = %q, direct = %q", params[0].CanonicalTypeKey, authParams[0].CanonicalTypeKey)
	}
	if params[0].CanonicalTypeKey != `*"example.com/modtest/go-auth".User` {
		t.Fatalf("unexpected canonical key %q", params[0].CanonicalTypeKey)
	}

	// Lazy callable with error
	if !params[1].IsCallable || !params[1].CallableReturnsError || params[1].CallableReturnTypeKey != `*"example.com/modtest/app".Session` {
		t.Fatalf("callable with error = %+v", params[1])
	}
	// Infallible lazy callable
	if !params[2].IsCallable || params[2].CallableReturnsError || params[2].CallableReturnTypeKey != `*"example.com/modtest/app".Session` {
		t.Fatalf("infallible callable = %+v", params[2])
	}

	// Predeclared alias normalization
	if params[3].CanonicalTypeKey != "uint8" {
		t.Fatalf("byte canonical key = %q, want uint8", params[3].CanonicalTypeKey)
	}
	if params[4].CanonicalTypeKey != "int32" {
		t.Fatalf("rune canonical key = %q, want int32", params[4].CanonicalTypeKey)
	}
	if params[5].CanonicalTypeKey != "interface{}" {
		t.Fatalf("any canonical key = %q, want interface{}", params[5].CanonicalTypeKey)
	}

	// Reject unimported package qualifier via resolveReqFunction
	if _, _, _, _, err := resolveReqFunction("", "missing", "Fn", "missing.Fn", "example.com/modtest/app", "app", cmdFn.file.Imports, true, idx, nil, nil); err == nil || !strings.Contains(err.Error(), "unimported package qualifier") {
		t.Fatalf("expected unimported qualifier error, got %v", err)
	}
	// Reject unexported function via resolveReqFunction
	if _, _, _, _, err := resolveReqFunction("", "myauth", "unexportedHelper", "myauth.unexportedHelper", "example.com/modtest/app", "app", cmdFn.file.Imports, true, idx, nil, nil); err == nil || !strings.Contains(err.Error(), "must be exported") {
		t.Fatalf("expected unexported error, got %v", err)
	}
	// Cross-file local helper resolves cleanly
	ref, ifn, impPath, pkgName, err := resolveReqFunction("", "", "LocalHelper", "LocalHelper", "example.com/modtest/app", "app", cmdFn.file.Imports, true, idx, nil, nil)
	if err != nil {
		t.Fatalf("cross-file LocalHelper error: %v", err)
	}
	if ifn == nil || ref.FunctionName != "LocalHelper" || impPath != "example.com/modtest/app" || pkgName != "app" {
		t.Fatalf("cross-file result = (%+v, %+v, %q, %q)", ref, ifn, impPath, pkgName)
	}
}

func TestModuleIndex_ClassifySignaturesAndSplitComma(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/sigtest\n\ngo 1.22\n")},
		"sigs/sigs.go": {Data: []byte(`package sigs

type Val struct{}

func Guard() error                 { return nil }
func Pure() *Val                   { return &Val{} }
func Fallible() (*Val, error)      { return &Val{}, nil }
func Zero()                        {}
func Three() (int, string, error)  { return 0, "", nil }
func BadSecond() (int, string)     { return 0, "" }
func TwoErrors() (error, error)    { return nil, nil }
`)},
	}

	idx := newModuleIndex(fsys, ".", "example.com/sigtest", sourceBuildContext(fsys, nil), token.NewFileSet())
	pkg, ok := idx.loadPackage("example.com/sigtest/sigs")
	if !ok {
		t.Fatal("failed to load sigs package")
	}

	guardCls, err := idx.classifyFuncResults(pkg.funcs["Guard"], pkg.importPath, pkg.pkgName)
	if err != nil || guardCls.kind != model.RequirementKindGuard || !guardCls.returnsError {
		t.Fatalf("Guard = %+v, err=%v", guardCls, err)
	}
	pureCls, err := idx.classifyFuncResults(pkg.funcs["Pure"], pkg.importPath, pkg.pkgName)
	if err != nil || pureCls.kind != model.RequirementKindProvider || pureCls.returnsError || pureCls.canonicalTypeKey != `*"example.com/sigtest/sigs".Val` {
		t.Fatalf("Pure = %+v, err=%v", pureCls, err)
	}
	fallibleCls, err := idx.classifyFuncResults(pkg.funcs["Fallible"], pkg.importPath, pkg.pkgName)
	if err != nil || fallibleCls.kind != model.RequirementKindProvider || !fallibleCls.returnsError {
		t.Fatalf("Fallible = %+v, err=%v", fallibleCls, err)
	}

	for _, bad := range []struct {
		fn   string
		want string
	}{
		{"Zero", "0 return values"},
		{"Three", "returns 3 values"},
		{"BadSecond", "invalid second return type"},
		{"TwoErrors", "(error, error)"},
	} {
		if _, err := idx.classifyFuncResults(pkg.funcs[bad.fn], pkg.importPath, pkg.pkgName); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Fatalf("%s: got %v, want substring %q", bad.fn, err, bad.want)
		}
	}

	gotParts := splitTopLevelComma(`auth.Check, MakeSession(account, region), "example.com/a,b".Open`)
	wantParts := []string{"auth.Check", " MakeSession(account, region)", ` "example.com/a,b".Open`}
	if !reflect.DeepEqual(gotParts, wantParts) {
		t.Fatalf("splitTopLevelComma = %#v, want %#v", gotParts, wantParts)
	}

	emptySlot := splitTopLevelComma("a, , b")
	if len(emptySlot) != 3 || strings.TrimSpace(emptySlot[1]) != "" {
		t.Fatalf("expected empty middle slot preserved, got %#v", emptySlot)
	}
}
