package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestResolveDependencies_IdempotentAndHealthBypass(t *testing.T) {
	authPkg := "example.com/app/auth"
	userType := `*` + `"` + authPkg + `".User`

	reqCurrentUser := func() *RequirementConfig {
		return &RequirementConfig{
			Func:             &FuncRef{ImportPath: authPkg, PackagePath: authPkg, CommandPackageName: "auth", FunctionName: "CurrentUser"},
			Kind:             RequirementKindProvider,
			ReturnType:       "*auth.User",
			CanonicalTypeKey: userType,
			TypeImportPath:   authPkg,
			TypePackageName:  "auth",
			TypeName:         "User",
			TypeImports:      []TypeImportRef{{ImportPath: authPkg, PackageName: "auth"}},
			ReturnsError:     true,
			SignatureKnown:   true,
		}
	}

	reqAdminGuard := &RequirementConfig{
		Func:           &FuncRef{ImportPath: authPkg, PackagePath: authPkg, CommandPackageName: "auth", FunctionName: "RequireAdmin"},
		Kind:           RequirementKindGuard,
		ReturnsError:   true,
		SignatureKnown: true,
		SignatureParams: []ResolvedParamSpec{{
			Name:             "user",
			Type:             "*auth.User",
			CanonicalTypeKey: userType,
		}},
	}

	root := &Command{MainCmdName: "app", PackagePath: "example.com/app"}
	api := &SubCommand{
		Command:              root,
		SubCommandName:       "api",
		SubCommandStructName: "ApiCmd",
		Requirements:         []*RequirementConfig{reqCurrentUser()},
	}
	health := &SubCommand{
		Command:                root,
		Parent:                 api,
		SubCommandName:         "health",
		SubCommandStructName:   "ApiHealthCmd",
		SubCommandFunctionName: "Health",
	}
	users := &SubCommand{
		Command:              root,
		Parent:               api,
		SubCommandName:       "users",
		SubCommandStructName: "ApiUsersCmd",
		Requirements:         []*RequirementConfig{reqAdminGuard},
	}
	userParam := &FunctionParameter{
		Name:             "user",
		Type:             "*auth.User",
		CanonicalTypeKey: userType,
		DeclaredIn:       "list",
	}
	list := &SubCommand{
		Command:                root,
		Parent:                 users,
		SubCommandName:         "list",
		SubCommandStructName:   "ApiUsersListCmd",
		SubCommandFunctionName: "List",
		Parameters:             []*FunctionParameter{userParam},
	}
	users.SubCommands = []*SubCommand{list}
	api.SubCommands = []*SubCommand{health, users}
	root.SubCommands = []*SubCommand{api}

	if err := root.ResolveDependencies(); err != nil {
		t.Fatalf("first ResolveDependencies: %v", err)
	}

	if len(health.ExecutionSteps) != 0 {
		t.Fatalf("health should have 0 execution steps, got %+v", health.ExecutionSteps)
	}
	if len(list.ExecutionSteps) != 1 || list.ExecutionSteps[0].Func.FunctionName != "RequireAdmin" {
		t.Fatalf("list.ExecutionSteps = %+v", list.ExecutionSteps)
	}
	if !list.ExecutionSteps[0].HasProviderInputs() || list.ExecutionSteps[0].Inputs[0].Provider == nil {
		t.Fatalf("expected RequireAdmin to consume CurrentUser provider, got %+v", list.ExecutionSteps[0].Inputs)
	}
	if userParam.SharedProvider == nil || userParam.SharedProvider != list.ExecutionSteps[0].Inputs[0].Provider {
		t.Fatalf("expected userParam to share the exact provider instance")
	}

	// Capture snapshot and re-run ResolveDependencies to prove idempotency.
	snapMethod := userParam.SharedProvider.MethodName()
	snapSteps := len(list.ExecutionSteps)
	snapUsersProviders := len(users.SharedProviders)
	snapAPIProviders := len(api.SharedProviders)

	if err := root.ResolveDependencies(); err != nil {
		t.Fatalf("second ResolveDependencies: %v", err)
	}
	if userParam.SharedProvider.MethodName() != snapMethod || len(list.ExecutionSteps) != snapSteps ||
		len(users.SharedProviders) != snapUsersProviders || len(api.SharedProviders) != snapAPIProviders {
		t.Fatalf("ResolveDependencies is not idempotent: method=%q(%q) steps=%d(%d) usersSP=%d(%d) apiSP=%d(%d)",
			userParam.SharedProvider.MethodName(), snapMethod, len(list.ExecutionSteps), snapSteps,
			len(users.SharedProviders), snapUsersProviders, len(api.SharedProviders), snapAPIProviders)
	}
}

func TestResolveDependencies_AmbiguityDisambiguationAndBuiltins(t *testing.T) {
	dbPkg := "example.com/app/db"
	dbType := `*` + `"` + dbPkg + `".Conn`
	strPkg := "example.com/app/cfg"

	makeDBReq := func(fnName string) *RequirementConfig {
		return &RequirementConfig{
			Func:             &FuncRef{ImportPath: dbPkg, PackagePath: dbPkg, CommandPackageName: "db", FunctionName: fnName},
			Kind:             RequirementKindProvider,
			ReturnType:       "*db.Conn",
			CanonicalTypeKey: dbType,
			TypeImportPath:   dbPkg,
			TypePackageName:  "db",
			TypeName:         "Conn",
			ReturnsError:     true,
			SignatureKnown:   true,
		}
	}

	root := &Command{
		MainCmdName:  "app",
		FunctionName: "Run",
		Requirements: []*RequirementConfig{
			makeDBReq("Primary"),
			makeDBReq("Replica"),
			{
				Func:             &FuncRef{ImportPath: strPkg, PackagePath: strPkg, CommandPackageName: "cfg", FunctionName: "Endpoint"},
				Kind:             RequirementKindProvider,
				ReturnType:       "string",
				CanonicalTypeKey: "string",
				SignatureKnown:   true,
			},
		},
		Parameters: []*FunctionParameter{
			{Name: "conn", Type: "*db.Conn", CanonicalTypeKey: dbType, DeclaredIn: "app"},
		},
	}

	if err := root.ResolveDependencies(); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error for *db.Conn, got %v", err)
	}

	// Disambiguate with explicit (provider: Replica) and add plain CLI string param + explicit CLI string param.
	connParam := root.Parameters[0]
	connParam.Generator = GeneratorConfig{
		Type: SourceTypeGenerator,
		Func: &FuncRef{ImportPath: dbPkg, PackagePath: dbPkg, CommandPackageName: "db", FunctionName: "Replica"},
	}
	cliStr := &FunctionParameter{Name: "label", Type: "string", CanonicalTypeKey: "string", DeclaredIn: "app"}
	explicitStr := &FunctionParameter{Name: "raw", Type: "string", CanonicalTypeKey: "string", HasExplicitCLISpec: true, DeclaredIn: "app"}
	root.Parameters = append(root.Parameters, cliStr, explicitStr)

	if err := root.ResolveDependencies(); err != nil {
		t.Fatalf("expected disambiguated ResolveDependencies to succeed, got %v", err)
	}
	if connParam.SharedProvider == nil || connParam.SharedProvider.Func.FunctionName != "Replica" {
		t.Fatalf("expected Replica provider, got %+v", connParam.SharedProvider)
	}
	if cliStr.SharedProvider != nil || explicitStr.SharedProvider != nil {
		t.Fatalf("built-in string CLI parameters must never be implicitly bound to Endpoint provider")
	}
}

func TestResolveDependencies_Cycles(t *testing.T) {
	pkg := "example.com/app/cyc"
	typeA := `*` + `"` + pkg + `".A`
	typeB := `*` + `"` + pkg + `".B`
	typeC := `*` + `"` + pkg + `".C`

	mk := func(fn, retKey string, paramKey string) *RequirementConfig {
		var sig []ResolvedParamSpec
		if paramKey != "" {
			sig = append(sig, ResolvedParamSpec{Name: "dep", Type: "x", CanonicalTypeKey: paramKey})
		}
		return &RequirementConfig{
			Func:             &FuncRef{ImportPath: pkg, PackagePath: pkg, CommandPackageName: "cyc", FunctionName: fn},
			Kind:             RequirementKindProvider,
			ReturnType:       fn,
			CanonicalTypeKey: retKey,
			ReturnsError:     true,
			SignatureKnown:   true,
			SignatureParams:  sig,
		}
	}

	cases := []struct {
		name string
		reqs []*RequirementConfig
		want string
	}{
		{"self", []*RequirementConfig{mk("MakeA", typeA, typeA)}, "cycle"},
		{"two", []*RequirementConfig{mk("MakeA", typeA, typeB), mk("MakeB", typeB, typeA)}, "cycle"},
		{"three unused", []*RequirementConfig{mk("MakeA", typeA, typeB), mk("MakeB", typeB, typeC), mk("MakeC", typeC, typeA)}, "cycle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := &Command{
				MainCmdName:  "app",
				FunctionName: "Run",
				Requirements: tc.reqs,
			}
			if err := root.ResolveDependencies(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestResolveDependencies_IdentityOrderingPromotionAndCallables(t *testing.T) {
	pkgA := "example.com/app/a"
	pkgB := "example.com/app/b"
	tokType := `*` + `"` + pkgA + `".Token`
	clientType := `*` + `"` + pkgA + `".Client`
	localType := `*` + `"` + pkgA + `".Local`

	// Reverse declaration order: MakeClient depends on MakeToken, plus two independent guards.
	reqClient := &RequirementConfig{
		Func:             &FuncRef{ImportPath: pkgA, PackagePath: pkgA, CommandPackageName: "a", FunctionName: "MakeClient"},
		Policy:           RequirementPolicyAlways,
		Kind:             RequirementKindProvider,
		ReturnType:       "*a.Client",
		CanonicalTypeKey: clientType,
		ReturnsError:     false,
		SignatureKnown:   true,
		SignatureParams:  []ResolvedParamSpec{{Name: "t", Type: "*a.Token", CanonicalTypeKey: tokType}},
	}
	reqToken := &RequirementConfig{
		Func:             &FuncRef{ImportPath: pkgA, PackagePath: pkgA, CommandPackageName: "a", FunctionName: "MakeToken"},
		Kind:             RequirementKindProvider,
		ReturnType:       "*a.Token",
		CanonicalTypeKey: tokType,
		ReturnsError:     true, // Fallible!
		SignatureKnown:   true,
	}
	guard1 := &RequirementConfig{
		Func:           &FuncRef{ImportPath: pkgA, PackagePath: pkgA, CommandPackageName: "a", FunctionName: "GuardOne"},
		Kind:           RequirementKindGuard,
		ReturnsError:   true,
		SignatureKnown: true,
	}
	guard2 := &RequirementConfig{
		Func:           &FuncRef{ImportPath: pkgA, PackagePath: pkgA, CommandPackageName: "a", FunctionName: "GuardTwo"},
		Kind:           RequirementKindGuard,
		ReturnsError:   true,
		SignatureKnown: true,
	}
	// Deduplicate identical guard declared on parent + child, and verify different package same name isn't confused.
	dupGuard := &RequirementConfig{
		Func:           &FuncRef{ImportPath: pkgA, PackagePath: pkgA, CommandPackageName: "a", FunctionName: "GuardOne"},
		Kind:           RequirementKindGuard,
		ReturnsError:   true,
		SignatureKnown: true,
	}
	otherPkgGuard := &RequirementConfig{
		Func:           &FuncRef{ImportPath: pkgB, PackagePath: pkgB, CommandPackageName: "b", FunctionName: "GuardOne"},
		Kind:           RequirementKindGuard,
		ReturnsError:   true,
		SignatureKnown: true,
	}

	root := &Command{
		MainCmdName:  "app",
		FunctionName: "Run",
		Requirements: []*RequirementConfig{reqClient, reqToken, guard1, guard2},
		Parameters: []*FunctionParameter{
			{
				Name:                  "infallibleClient",
				Type:                  "func() *a.Client",
				CanonicalTypeKey:      "func() " + clientType,
				CallableReturnTypeKey: clientType,
				CallableReturnsError:  false,
				IsCallableInfallible:  true,
				DeclaredIn:            "app",
			},
		},
	}

	// Because MakeClient transitively depends on fallible MakeToken, func() *a.Client MUST be rejected!
	if err := root.ResolveDependencies(); err == nil || !strings.Contains(err.Error(), "infallible callable") {
		t.Fatalf("expected infallible callable error for transitive error upstream, got %v", err)
	}

	// Fix parameter to fallible callable func() (*a.Client, error), and test parent+child dedup + order + local promotion.
	root.Parameters[0].Type = "func() (*a.Client, error)"
	root.Parameters[0].CallableReturnsError = true
	root.Parameters[0].IsCallableInfallible = false

	localGenParam := &FunctionParameter{
		Name:             "loc",
		Type:             "*a.Local",
		CanonicalTypeKey: localType,
		DeclaredIn:       "child",
		Generator: GeneratorConfig{
			Type: SourceTypeGenerator,
			Func: &FuncRef{ImportPath: pkgA, PackagePath: pkgA, CommandPackageName: "a", FunctionName: "BuildLocal"},
		},
	}
	child := &SubCommand{
		Command:                root,
		SubCommandName:         "child",
		SubCommandStructName:   "ChildCmd",
		SubCommandFunctionName: "Child",
		Requirements:           []*RequirementConfig{dupGuard, otherPkgGuard},
		Parameters:             []*FunctionParameter{localGenParam},
	}
	child.Requirements = append(child.Requirements, &RequirementConfig{
		Func:            &FuncRef{ImportPath: pkgA, PackagePath: pkgA, CommandPackageName: "a", FunctionName: "CheckLocal"},
		Kind:            RequirementKindGuard,
		ExplicitInputs:  []string{"loc"},
		HasExplicitArgs: true,
		ReturnsError:    true,
	})
	root.SubCommands = []*SubCommand{child}

	if err := root.ResolveDependencies(); err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	var names []string
	for _, s := range root.EffectiveExecutionSteps() {
		if s.Kind == ExecutionStepProvider {
			names = append(names, s.Provider.Func.FunctionName)
		} else {
			names = append(names, s.Func.FunctionName)
		}
	}
	wantRoot := []string{"MakeClient", "GuardOne", "GuardTwo"}
	if !reflect.DeepEqual(names, wantRoot) {
		t.Fatalf("root step order = %v, want %v", names, wantRoot)
	}

	// Child must deduplicate GuardOne from pkgA while keeping GuardOne from pkgB, and promote local generator to SharedProviders!
	var childGuardNames []string
	for _, s := range child.EffectiveExecutionSteps() {
		if s.Kind == ExecutionStepGuard {
			childGuardNames = append(childGuardNames, s.Func.ImportPath+"."+s.Func.FunctionName)
		}
	}
	wantChild := []string{
		pkgA + ".GuardOne",
		pkgA + ".GuardTwo",
		pkgB + ".GuardOne",
		pkgA + ".CheckLocal",
	}
	if !reflect.DeepEqual(childGuardNames, wantChild) {
		t.Fatalf("child guards = %v, want %v", childGuardNames, wantChild)
	}
	if localGenParam.SharedProvider == nil {
		t.Fatal("expected local generator consumed by CheckLocal guard to be promoted")
	}
	foundInChild := false
	for _, sp := range child.SharedProviders {
		if sp == localGenParam.SharedProvider {
			foundInChild = true
		}
	}
	if !foundInChild {
		t.Fatalf("promoted local generator not placed in child.SharedProviders: %+v", child.SharedProviders)
	}

	// Also verify shadowed --env flag gives distinct provider identities in parent vs child.
	envReq := func() *RequirementConfig {
		return &RequirementConfig{
			Func:             &FuncRef{ImportPath: pkgA, PackagePath: pkgA, CommandPackageName: "a", FunctionName: "OpenEnv"},
			Kind:             RequirementKindProvider,
			Policy:           RequirementPolicyAlways,
			ReturnType:       "*a.Token",
			CanonicalTypeKey: tokType,
			ExplicitInputs:   []string{"env"},
			HasExplicitArgs:  true,
			ReturnsError:     true,
		}
	}
	shadowRoot := &Command{
		MainCmdName:  "app",
		FunctionName: "Run",
		Requirements: []*RequirementConfig{envReq()},
		Parameters:   []*FunctionParameter{{Name: "env", Type: "string", CanonicalTypeKey: "string", DeclaredIn: "app"}},
	}
	shadowChild := &SubCommand{
		Command:                shadowRoot,
		SubCommandName:         "custom",
		SubCommandStructName:   "CustomCmd",
		SubCommandFunctionName: "Custom",
		Requirements:           []*RequirementConfig{envReq()},
		Parameters:             []*FunctionParameter{{Name: "env", Type: "string", CanonicalTypeKey: "string", DeclaredIn: "custom"}},
	}
	shadowRoot.SubCommands = []*SubCommand{shadowChild}
	if err := shadowRoot.ResolveDependencies(); err != nil {
		t.Fatalf("shadowRoot ResolveDependencies: %v", err)
	}
	if shadowRoot.ExecutionSteps[0].Provider.IdentityKey == shadowChild.ExecutionSteps[0].Provider.IdentityKey {
		t.Fatalf("shadowed flag input produced identical provider IdentityKey %q", shadowRoot.ExecutionSteps[0].Provider.IdentityKey)
	}
}

func TestResolveDependencies_ExplicitInferredArgs(t *testing.T) {
	authPkg := "example.com/app/auth"
	userType := `*` + `"` + authPkg + `".User`
	badgeType := `*` + `"` + authPkg + `".Badge`

	makeCurrentUser := func() *RequirementConfig {
		return &RequirementConfig{
			Func:             &FuncRef{ImportPath: authPkg, PackagePath: authPkg, CommandPackageName: "auth", FunctionName: "CurrentUser"},
			Kind:             RequirementKindProvider,
			ReturnType:       "*auth.User",
			CanonicalTypeKey: userType,
			TypeImportPath:   authPkg,
			TypePackageName:  "auth",
			TypeName:         "User",
			ReturnsError:     true,
			SignatureKnown:   true,
		}
	}

	// 1. Explicit guard arg consuming inferred action parameter resolves to provider edge.
	root := &Command{MainCmdName: "app", PackagePath: "example.com/app"}
	userParam := &FunctionParameter{Name: "user", Type: "*auth.User", CanonicalTypeKey: userType, DeclaredIn: "list"}
	list := &SubCommand{
		Command:                root,
		SubCommandName:         "list",
		SubCommandStructName:   "ListCmd",
		SubCommandFunctionName: "List",
		Requirements: []*RequirementConfig{
			makeCurrentUser(),
			{
				Func:            &FuncRef{ImportPath: authPkg, PackagePath: authPkg, CommandPackageName: "auth", FunctionName: "RequireAdmin"},
				Kind:            RequirementKindGuard,
				ExplicitInputs:  []string{"user"},
				HasExplicitArgs: true,
				ReturnsError:    true,
				SignatureKnown:  true,
				SignatureParams: []ResolvedParamSpec{{Name: "u", Type: "*auth.User", CanonicalTypeKey: userType}},
			},
		},
		Parameters: []*FunctionParameter{userParam},
	}

	// 2. Explicit provider arg consuming inferred action parameter (reversed parameter order).
	badgeParam := &FunctionParameter{Name: "badge", Type: "*auth.Badge", CanonicalTypeKey: badgeType, DeclaredIn: "badge"}
	badgeUserParam := &FunctionParameter{Name: "user", Type: "*auth.User", CanonicalTypeKey: userType, DeclaredIn: "badge"}
	badge := &SubCommand{
		Command:                root,
		SubCommandName:         "badge",
		SubCommandStructName:   "BadgeCmd",
		SubCommandFunctionName: "ShowBadge",
		Requirements: []*RequirementConfig{
			makeCurrentUser(),
			{
				Func:             &FuncRef{ImportPath: authPkg, PackagePath: authPkg, CommandPackageName: "auth", FunctionName: "BuildBadge"},
				Kind:             RequirementKindProvider,
				ReturnType:       "*auth.Badge",
				CanonicalTypeKey: badgeType,
				TypeImportPath:   authPkg,
				TypePackageName:  "auth",
				TypeName:         "Badge",
				ExplicitInputs:   []string{"user"},
				HasExplicitArgs:  true,
				ReturnsError:     true,
				SignatureKnown:   true,
				SignatureParams:  []ResolvedParamSpec{{Name: "u", Type: "*auth.User", CanonicalTypeKey: userType}},
			},
		},
		Parameters: []*FunctionParameter{badgeParam, badgeUserParam},
	}
	root.SubCommands = []*SubCommand{list, badge}

	if err := root.ResolveDependencies(); err != nil {
		t.Fatalf("ResolveDependencies: %v", err)
	}
	if len(list.ExecutionSteps) != 1 {
		t.Fatalf("list.ExecutionSteps len = %d, want 1", len(list.ExecutionSteps))
	}
	step := list.ExecutionSteps[0]
	if len(step.Inputs) != 1 || step.Inputs[0].Kind != DependencyInputProvider || step.Inputs[0].Provider == nil {
		t.Fatalf("expected RequireAdmin(user) to bind DependencyInputProvider, got %+v", step.Inputs)
	}
	if userParam.SharedProvider == nil || step.Inputs[0].Provider != userParam.SharedProvider {
		t.Fatalf("RequireAdmin provider (%p) != List user provider (%p)", step.Inputs[0].Provider, userParam.SharedProvider)
	}
	if badgeParam.SharedProvider == nil || badgeUserParam.SharedProvider == nil {
		t.Fatalf("expected badge=%v and user=%v to bind", badgeParam.SharedProvider, badgeUserParam.SharedProvider)
	}
	if len(badgeParam.SharedProvider.ResolvedInputs) != 1 || badgeParam.SharedProvider.ResolvedInputs[0].Kind != DependencyInputProvider || badgeParam.SharedProvider.ResolvedInputs[0].Provider != badgeUserParam.SharedProvider {
		t.Fatalf("BuildBadge(user) inputs = %+v, want CurrentUser provider", badgeParam.SharedProvider.ResolvedInputs)
	}

	// 3. Cycle through explicit inferred args must be rejected.
	typeA := `*"example.com/cyc".A`
	typeB := `*"example.com/cyc".B`
	cycRoot := &Command{
		MainCmdName:  "app",
		FunctionName: "Run",
		Requirements: []*RequirementConfig{
			{
				Func:             &FuncRef{ImportPath: "example.com/cyc", PackagePath: "example.com/cyc", CommandPackageName: "cyc", FunctionName: "MakeA"},
				Kind:             RequirementKindProvider,
				ReturnType:       "*cyc.A",
				CanonicalTypeKey: typeA,
				ExplicitInputs:   []string{"b"},
				HasExplicitArgs:  true,
				ReturnsError:     true,
				SignatureKnown:   true,
				SignatureParams:  []ResolvedParamSpec{{Name: "b", Type: "*cyc.B", CanonicalTypeKey: typeB}},
			},
			{
				Func:             &FuncRef{ImportPath: "example.com/cyc", PackagePath: "example.com/cyc", CommandPackageName: "cyc", FunctionName: "MakeB"},
				Kind:             RequirementKindProvider,
				ReturnType:       "*cyc.B",
				CanonicalTypeKey: typeB,
				ExplicitInputs:   []string{"a"},
				HasExplicitArgs:  true,
				ReturnsError:     true,
				SignatureKnown:   true,
				SignatureParams:  []ResolvedParamSpec{{Name: "a", Type: "*cyc.A", CanonicalTypeKey: typeA}},
			},
		},
		Parameters: []*FunctionParameter{
			{Name: "a", Type: "*cyc.A", CanonicalTypeKey: typeA, DeclaredIn: "app"},
			{Name: "b", Type: "*cyc.B", CanonicalTypeKey: typeB, DeclaredIn: "app"},
		},
	}
	if err := cycRoot.ResolveDependencies(); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle through explicit inferred args, got %v", err)
	}

	// 4. Guard referencing unbound non-CLI param with missing provider must fail instead of passing zero value.
	missingRoot := &Command{
		MainCmdName:  "app",
		FunctionName: "Run",
		Requirements: []*RequirementConfig{
			{
				Func:            &FuncRef{ImportPath: authPkg, PackagePath: authPkg, CommandPackageName: "auth", FunctionName: "RequireAdmin"},
				Kind:            RequirementKindGuard,
				ExplicitInputs:  []string{"user"},
				HasExplicitArgs: true,
				ReturnsError:    true,
			},
		},
		Parameters: []*FunctionParameter{{Name: "user", Type: "*auth.User", CanonicalTypeKey: userType, DeclaredIn: "app"}},
	}
	if err := missingRoot.ResolveDependencies(); err == nil || !strings.Contains(err.Error(), "unbound dependency parameter") {
		t.Fatalf("expected unbound dependency parameter error, got %v", err)
	}
}

func TestResolveDependencies_LegacyNamedProvidersAndScopedMutation(t *testing.T) {
	dbPkg := "example.com/app/db"
	dbType := `*` + `"` + dbPkg + `".Conn`
	storePkg := "example.com/app/store"
	storeType := `*` + `"` + storePkg + `".Store`

	primarySP := &SharedProviderConfig{
		Name:             "primary",
		Type:             "*db.Conn",
		CanonicalTypeKey: dbType,
		Func:             &FuncRef{ImportPath: dbPkg, PackagePath: dbPkg, CommandPackageName: "db", FunctionName: "PrimaryDB"},
		ReturnsError:     true,
		SignatureKnown:   true,
	}
	replicaSP := &SharedProviderConfig{
		Name:             "replica",
		Type:             "*db.Conn",
		CanonicalTypeKey: dbType,
		Func:             &FuncRef{ImportPath: dbPkg, PackagePath: dbPkg, CommandPackageName: "db", FunctionName: "ReplicaDB"},
		ReturnsError:     true,
		SignatureKnown:   true,
	}

	root := &Command{MainCmdName: "app", PackagePath: "example.com/app"}
	dbCmd := &SubCommand{
		Command:              root,
		SubCommandName:       "db",
		SubCommandStructName: "DBCmd",
		SharedProviders:      []*SharedProviderConfig{primarySP, replicaSP},
	}
	primaryParam := &FunctionParameter{Name: "primary", Type: "*db.Conn", CanonicalTypeKey: dbType, DeclaredIn: "report"}
	replicaParam := &FunctionParameter{Name: "replica", Type: "*db.Conn", CanonicalTypeKey: dbType, DeclaredIn: "report"}
	report := &SubCommand{
		Command:                root,
		Parent:                 dbCmd,
		SubCommandName:         "report",
		SubCommandStructName:   "DBReportCmd",
		SubCommandFunctionName: "Report",
		Parameters:             []*FunctionParameter{primaryParam, replicaParam},
	}
	swapPrimary := &FunctionParameter{
		Name:             "primary",
		Type:             "*db.Conn",
		CanonicalTypeKey: dbType,
		DeclaredIn:       "swap",
		Generator: GeneratorConfig{
			Type: SourceTypeGenerator,
			Func: &FuncRef{ImportPath: dbPkg, PackagePath: dbPkg, CommandPackageName: "db", FunctionName: "ReplicaDB"},
		},
	}
	swap := &SubCommand{
		Command:                root,
		Parent:                 dbCmd,
		SubCommandName:         "swap",
		SubCommandStructName:   "DBSwapCmd",
		SubCommandFunctionName: "Swap",
		Parameters:             []*FunctionParameter{swapPrimary},
	}
	dbCmd.SubCommands = []*SubCommand{report, swap}
	root.SubCommands = []*SubCommand{dbCmd}

	if err := root.ResolveDependencies(); err != nil {
		t.Fatalf("legacy named binding failed: %v", err)
	}
	if primaryParam.SharedProvider != primarySP || replicaParam.SharedProvider != replicaSP {
		t.Fatalf("legacy named binding mismatch: primary=%+v replica=%+v", primaryParam.SharedProvider, replicaParam.SharedProvider)
	}
	if swapPrimary.SharedProvider != replicaSP {
		t.Fatalf("explicit override failed: got %+v, want replicaSP", swapPrimary.SharedProvider)
	}

	// Unmatched parameter name of same type still reports ambiguity.
	other := &SubCommand{
		Command:                root,
		Parent:                 dbCmd,
		SubCommandName:         "other",
		SubCommandStructName:   "DBOtherCmd",
		SubCommandFunctionName: "Other",
		Parameters:             []*FunctionParameter{{Name: "conn", Type: "*db.Conn", CanonicalTypeKey: dbType, DeclaredIn: "other"}},
	}
	dbCmd.SubCommands = append(dbCmd.SubCommands, other)
	if err := root.ResolveDependencies(); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity for unmatched parameter name, got %v", err)
	}

	// Shadowed --env input across legacy SharedProvider must not mutate root's SharedProviderConfig.
	rootStoreSP := &SharedProviderConfig{
		Name:             "store",
		Type:             "*store.Store",
		CanonicalTypeKey: storeType,
		Func:             &FuncRef{ImportPath: storePkg, PackagePath: storePkg, CommandPackageName: "store", FunctionName: "OpenStore"},
		Inputs:           []string{"env"},
		ReturnsError:     true,
		SignatureKnown:   true,
	}
	runStore := &FunctionParameter{Name: "store", Type: "*store.Store", CanonicalTypeKey: storeType, DeclaredIn: "run"}
	customStore := &FunctionParameter{Name: "store", Type: "*store.Store", CanonicalTypeKey: storeType, DeclaredIn: "custom"}
	shadowRoot := &Command{
		MainCmdName:     "app",
		SharedProviders: []*SharedProviderConfig{rootStoreSP},
		Parameters:      []*FunctionParameter{{Name: "env", Type: "string", CanonicalTypeKey: "string", DeclaredIn: "app"}},
		SubCommands: []*SubCommand{
			{
				SubCommandName:         "run",
				SubCommandStructName:   "RunCmd",
				SubCommandFunctionName: "Run",
				Parameters:             []*FunctionParameter{runStore},
			},
			{
				SubCommandName:         "custom",
				SubCommandStructName:   "CustomCmd",
				SubCommandFunctionName: "Custom",
				Parameters: []*FunctionParameter{
					{Name: "env", Type: "string", CanonicalTypeKey: "string", DeclaredIn: "custom"},
					customStore,
				},
			},
		},
	}
	for _, sc := range shadowRoot.SubCommands {
		sc.Command = shadowRoot
	}
	if err := shadowRoot.ResolveDependencies(); err != nil {
		t.Fatalf("shadowed legacy store ResolveDependencies: %v", err)
	}
	if runStore.SharedProvider != rootStoreSP {
		t.Fatalf("expected runStore to claim rootStoreSP")
	}
	if customStore.SharedProvider == nil || customStore.SharedProvider == rootStoreSP {
		t.Fatalf("customStore reused/mutated rootStoreSP instead of allocating scoped instance")
	}
	if rootStoreSP.ResolvedInputs[0].ParamOwnerPath != "<root>" || customStore.SharedProvider.ResolvedInputs[0].ParamOwnerPath != "custom" {
		t.Fatalf("unexpected input owner paths: root=%+v custom=%+v", rootStoreSP.ResolvedInputs, customStore.SharedProvider.ResolvedInputs)
	}
}
