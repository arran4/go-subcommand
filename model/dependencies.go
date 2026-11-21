package model

import (
	"fmt"
	"sort"
	"strings"
)

// RequirementPolicy controls whether a value-producing requirement runs on demand
// or unconditionally on the selected command path.
type RequirementPolicy string

const (
	RequirementPolicyOnDemand RequirementPolicy = ""
	RequirementPolicyAlways   RequirementPolicy = "always"
)

// RequirementKind classifies a resolved Requires declaration by its Go return signature.
type RequirementKind string

const (
	RequirementKindUnknown  RequirementKind = ""
	RequirementKindGuard    RequirementKind = "guard"
	RequirementKindProvider RequirementKind = "provider"
)

// TypeImportRef records an import path and declared package name referenced in a Go type expression.
type TypeImportRef struct {
	ImportPath  string
	PackageName string
}

// ResolvedParamSpec describes one parameter from a resolved Go function signature.
type ResolvedParamSpec struct {
	Name                  string
	Type                  string
	CanonicalTypeKey      string
	TypeImports           []TypeImportRef
	IsVariadic            bool
	IsCallable            bool
	CallableReturnTypeKey string
	CallableReturnsError  bool
}

// RequirementConfig represents a canonical Requires declaration parsed from source
// comments or native command metadata.
type RequirementConfig struct {
	Name             string
	Func             *FuncRef
	ExplicitInputs   []string
	HasExplicitArgs  bool
	Policy           RequirementPolicy
	Kind             RequirementKind
	ReturnType       string
	CanonicalTypeKey string
	TypeImportPath   string
	TypePackageName  string
	TypeName         string
	TypeImports      []TypeImportRef
	ReturnsError     bool
	SignatureParams  []ResolvedParamSpec
	SignatureKnown   bool
	Filename         string
	Line             int
}

// DependencyInputKind distinguishes a command parameter input from an upstream provider input.
type DependencyInputKind string

const (
	DependencyInputParam    DependencyInputKind = "param"
	DependencyInputProvider DependencyInputKind = "provider"
)

// DependencyInput describes a single resolved argument to a provider or guard function.
type DependencyInput struct {
	Kind           DependencyInputKind
	ParamName      string
	ParamOwnerPath string
	ParamExpr      string
	Provider       *SharedProviderConfig
	LazyCallable   bool
	LazyReturnsErr bool
}

// ExecutionStepKind identifies one mandatory step executed before a command action.
type ExecutionStepKind string

const (
	ExecutionStepGuard    ExecutionStepKind = "guard"
	ExecutionStepProvider ExecutionStepKind = "provider"
)

// ExecutionStep represents a single deduplicated, topologically ordered mandatory step
// for a specific executable command route.
type ExecutionStep struct {
	Kind         ExecutionStepKind
	IdentityKey  string
	GuardName    string
	Func         *FuncRef
	Inputs       []DependencyInput
	ReturnsError bool
	Provider     *SharedProviderConfig
}

// HasProviderInputs reports whether this step depends on at least one memoized provider.
func (es *ExecutionStep) HasProviderInputs() bool {
	for _, in := range es.Inputs {
		if in.Kind == DependencyInputProvider {
			return true
		}
	}
	return false
}

// CallFor renders the Go call expression for a mandatory guard execution step.
func (es *ExecutionStep) CallFor(receiver string) string {
	if es.Func == nil || es.Func.FunctionName == "" {
		return ""
	}
	args := make([]string, 0, len(es.Inputs))
	for i, in := range es.Inputs {
		if in.Kind == DependencyInputProvider {
			args = append(args, fmt.Sprintf("dep%d", i))
			continue
		}
		expr := in.ParamExpr
		if expr == "" {
			expr = in.ParamName
		}
		if receiver != "" {
			args = append(args, receiver+"."+expr)
		} else {
			args = append(args, expr)
		}
	}
	argumentList := strings.Join(args, ", ")
	if es.Func.CommandPackageName != "" {
		return fmt.Sprintf("%s.%s(%s)", es.Func.CommandPackageName, es.Func.FunctionName, argumentList)
	}
	return fmt.Sprintf("%s(%s)", es.Func.FunctionName, argumentList)
}

// EffectiveExecutionSteps returns the resolved mandatory execution steps for root,
// falling back to HierarchyInitializers if ResolveDependencies has not populated them.
func (cmd *Command) EffectiveExecutionSteps() []*ExecutionStep {
	if cmd == nil {
		return nil
	}
	if len(cmd.ExecutionSteps) > 0 {
		return cmd.ExecutionSteps
	}
	inits := cmd.HierarchyInitializers()
	if len(inits) == 0 {
		return nil
	}
	return stepsFromLegacyInitializers(inits)
}

// EffectiveExecutionSteps returns the resolved mandatory execution steps for sc,
// falling back to HierarchyInitializers if ResolveDependencies has not populated them.
func (sc *SubCommand) EffectiveExecutionSteps() []*ExecutionStep {
	if sc == nil {
		return nil
	}
	if len(sc.ExecutionSteps) > 0 {
		return sc.ExecutionSteps
	}
	inits := sc.HierarchyInitializers()
	if len(inits) == 0 {
		return nil
	}
	return stepsFromLegacyInitializers(inits)
}

func stepsFromLegacyInitializers(inits []*InitializerConfig) []*ExecutionStep {
	steps := make([]*ExecutionStep, 0, len(inits))
	for _, ic := range inits {
		inputs := make([]DependencyInput, 0, len(ic.Inputs))
		for _, in := range ic.Inputs {
			inputs = append(inputs, DependencyInput{
				Kind:      DependencyInputParam,
				ParamName: in,
				ParamExpr: in,
			})
		}
		steps = append(steps, &ExecutionStep{
			Kind:         ExecutionStepGuard,
			IdentityKey:  ic.IdentityKey,
			GuardName:    ic.Name,
			Func:         ic.Func,
			Inputs:       inputs,
			ReturnsError: ic.ReturnsError,
		})
	}
	return steps
}

func deriveDefaultProviderName(funcName, pkgName string) string {
	trimmed := funcName
	for _, prefix := range []string{"Open", "New", "Get"} {
		if strings.HasPrefix(trimmed, prefix) && len(trimmed) > len(prefix) {
			trimmed = trimmed[len(prefix):]
			break
		}
	}
	if trimmed == "" {
		trimmed = funcName
	}
	if trimmed == "" && pkgName != "" {
		return pkgName
	}
	if len(trimmed) > 0 {
		return strings.ToLower(trimmed[:1]) + trimmed[1:]
	}
	return trimmed
}

type depScope struct {
	isRoot             bool
	cmd                *Command
	sc                 *SubCommand
	parent             *depScope
	children           []*depScope
	pathKey            string
	structName         string
	depth              int
	displayName        string
	declProviders      []*providerDecl
	declGuards         []*guardDecl
	mandatoryProviders []*mandatoryProviderDecl
	params             []*FunctionParameter
}

type providerDecl struct {
	owner            *depScope
	isLegacy         bool
	legacySP         *SharedProviderConfig
	req              *RequirementConfig
	funcRef          *FuncRef
	name             string
	hasExplicitName  bool
	explicitInputs   []string
	hasExplicitArgs  bool
	signatureParams  []ResolvedParamSpec
	signatureKnown   bool
	returnType       string
	canonicalTypeKey string
	typeImportPath   string
	typePackageName  string
	typeName         string
	typeImports      []TypeImportRef
	returnsError     bool
	filename         string
	line             int
	declOrder        int
}

type mandatoryProviderDecl struct {
	owner     *depScope
	provider  *providerDecl
	declOrder int
}

type guardDecl struct {
	owner           *depScope
	isLegacy        bool
	legacyInit      *InitializerConfig
	req             *RequirementConfig
	name            string
	funcRef         *FuncRef
	explicitInputs  []string
	hasExplicitArgs bool
	signatureParams []ResolvedParamSpec
	signatureKnown  bool
	returnsError    bool
	filename        string
	line            int
	declOrder       int
}

type resolvedProvider struct {
	sp               *SharedProviderConfig
	declHost         *depScope
	demanders        []*depScope
	isFallible       bool
	canonicalTypeKey string
	firstOrder       int
	decl             *providerDecl
}

type depResolver struct {
	cmd              *Command
	root             *depScope
	allScopes        []*depScope
	providersByID map[string]*resolvedProvider
	providerOrder []string
	nextDeclOrder int
}

// ResolveDependencies validates and resolves the unified dependency graph
// (Requires guards, typed providers, legacy Initializers/SharedProviders,
// local generators, implicit & explicit consumers, LCA hoisting, topological order,
// and cycle checks) for the entire command tree. It is idempotent.
func (cmd *Command) ResolveDependencies() error {
	if cmd == nil {
		return nil
	}
	r := &depResolver{
		cmd:           cmd,
		providersByID: make(map[string]*resolvedProvider),
	}
	r.resetCommandTree()
	if err := r.buildScopes(); err != nil {
		return err
	}
	if err := r.resolveExecutableRoutes(); err != nil {
		return err
	}
	if err := r.validateUnusedRequirements(); err != nil {
		return err
	}
	if err := r.hoistRepeatedLocalGenerators(); err != nil {
		return err
	}
	r.placeResolvedProviders()
	if err := r.verifyNoCyclesInPlacedProviders(); err != nil {
		return err
	}
	cmd.allocateSharedProviderNames()
	r.populateParamAccessors()
	return nil
}

func (r *depResolver) resetCommandTree() {
	filterSP := func(sps []*SharedProviderConfig) []*SharedProviderConfig {
		var kept []*SharedProviderConfig
		for _, sp := range sps {
			if sp.GeneratedByResolver {
				continue
			}
			sp.ResolvedInputs = nil
			sp.IdentityKey = ""
			kept = append(kept, sp)
		}
		return kept
	}
	resetParams := func(params []*FunctionParameter) {
		for _, p := range params {
			if p.BoundByResolver {
				p.SharedProvider = nil
				p.IsLazyCallable = false
				p.IsCallableInfallible = false
				p.BoundByResolver = false
			}
		}
	}
	resetInits := func(inits []*InitializerConfig) {
		for _, ic := range inits {
			ic.ResolvedInputs = nil
			ic.IdentityKey = ""
		}
	}

	r.cmd.ExecutionSteps = nil
	r.cmd.SharedProviders = filterSP(r.cmd.SharedProviders)
	resetInits(r.cmd.Initializers)
	resetParams(r.cmd.Parameters)

	var walkSub func(sc *SubCommand)
	walkSub = func(sc *SubCommand) {
		sc.ExecutionSteps = nil
		sc.SharedProviders = filterSP(sc.SharedProviders)
		resetInits(sc.Initializers)
		resetParams(sc.Parameters)
		for _, child := range sc.SubCommands {
			walkSub(child)
		}
	}
	for _, sc := range r.cmd.SubCommands {
		walkSub(sc)
	}
}

func (r *depResolver) buildScopes() error {
	rootName := r.cmd.MainCmdName
	if rootName == "" {
		rootName = "app"
	}
	r.root = &depScope{
		isRoot:      true,
		cmd:         r.cmd,
		pathKey:     "<root>",
		structName:  "RootCmd",
		depth:       0,
		displayName: rootName,
		params:      r.cmd.Parameters,
	}
	if err := r.collectScopeDecls(r.root, r.cmd.Initializers, r.cmd.SharedProviders, r.cmd.Requirements); err != nil {
		return err
	}
	r.allScopes = append(r.allScopes, r.root)

	var walk func(parent *depScope, sc *SubCommand) error
	walk = func(parent *depScope, sc *SubCommand) error {
		pathKey := sc.SubCommandName
		if !parent.isRoot {
			pathKey = parent.pathKey + "/" + sc.SubCommandName
		}
		scope := &depScope{
			sc:          sc,
			parent:      parent,
			pathKey:     pathKey,
			structName:  sc.SubCommandStructName,
			depth:       parent.depth + 1,
			displayName: sc.ProgName(),
			params:      sc.Parameters,
		}
		parent.children = append(parent.children, scope)
		if err := r.collectScopeDecls(scope, sc.Initializers, sc.SharedProviders, sc.Requirements); err != nil {
			return err
		}
		r.allScopes = append(r.allScopes, scope)
		for _, child := range sc.SubCommands {
			if err := walk(scope, child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, sc := range r.cmd.SubCommands {
		if err := walk(r.root, sc); err != nil {
			return err
		}
	}
	return nil
}

func (r *depResolver) collectScopeDecls(scope *depScope, inits []*InitializerConfig, legacySPs []*SharedProviderConfig, reqs []*RequirementConfig) error {
	seenNames := make(map[string]*providerDecl)

	for _, ic := range inits {
		if ic == nil || ic.Func == nil || ic.Func.FunctionName == "" {
			continue
		}
		decl := &guardDecl{
			owner:           scope,
			isLegacy:        true,
			legacyInit:      ic,
			name:            ic.Name,
			funcRef:         ic.Func,
			explicitInputs:  ic.Inputs,
			hasExplicitArgs: len(ic.Inputs) > 0,
			signatureParams: ic.SignatureParams,
			signatureKnown:  ic.SignatureKnown,
			returnsError:    ic.ReturnsError,
			filename:        ic.Filename,
			line:            ic.Line,
			declOrder:       r.nextDeclOrder,
		}
		r.nextDeclOrder++
		scope.declGuards = append(scope.declGuards, decl)
	}

	for _, sp := range legacySPs {
		if sp == nil || sp.Func == nil || sp.Func.FunctionName == "" {
			continue
		}
		decl := &providerDecl{
			owner:            scope,
			isLegacy:         true,
			legacySP:         sp,
			funcRef:          sp.Func,
			name:             sp.Name,
			hasExplicitName:  sp.Name != "",
			explicitInputs:   sp.Inputs,
			hasExplicitArgs:  len(sp.Inputs) > 0,
			signatureParams:  sp.SignatureParams,
			signatureKnown:   sp.SignatureKnown,
			returnType:       sp.Type,
			canonicalTypeKey: sp.CanonicalTypeKey,
			typeImportPath:   sp.TypeImportPath,
			typePackageName:  sp.TypePackageName,
			typeImports:      sp.TypeImports,
			returnsError:     sp.ReturnsError,
			filename:         sp.Filename,
			line:             sp.Line,
			declOrder:        r.nextDeclOrder,
		}
		r.nextDeclOrder++
		scope.declProviders = append(scope.declProviders, decl)
		if sp.Name != "" {
			seenNames[sp.Name] = decl
		}
	}

	for _, req := range reqs {
		if req == nil || req.Func == nil || req.Func.FunctionName == "" {
			return reqError(req, scope, "empty requirement declaration")
		}
		if req.Kind == RequirementKindUnknown {
			return reqError(req, scope, "requirement %s has unsupported signature", formatFuncRefName(req.Func))
		}
		if req.HasExplicitArgs && req.SignatureKnown {
			if err := validateExplicitArity(req, scope); err != nil {
				return err
			}
		}
		if req.Kind == RequirementKindGuard {
			if req.Name != "" {
				return reqError(req, scope, "error-only requirement %s cannot assign a named result (%s = ...)", formatFuncRefName(req.Func), req.Name)
			}
			decl := &guardDecl{
				owner:           scope,
				req:             req,
				name:            req.Func.FunctionName,
				funcRef:         req.Func,
				explicitInputs:  req.ExplicitInputs,
				hasExplicitArgs: req.HasExplicitArgs,
				signatureParams: req.SignatureParams,
				signatureKnown:  req.SignatureKnown,
				returnsError:    req.ReturnsError,
				filename:        req.Filename,
				line:            req.Line,
				declOrder:       r.nextDeclOrder,
			}
			r.nextDeclOrder++
			scope.declGuards = append(scope.declGuards, decl)
			continue
		}

		if req.Kind == RequirementKindProvider {
			derivedName := req.Name
			if derivedName == "" {
				derivedName = deriveDefaultProviderName(req.Func.FunctionName, req.Func.CommandPackageName)
			}
			decl := &providerDecl{
				owner:            scope,
				req:              req,
				funcRef:          req.Func,
				name:             derivedName,
				hasExplicitName:  req.Name != "",
				explicitInputs:   req.ExplicitInputs,
				hasExplicitArgs:  req.HasExplicitArgs,
				signatureParams:  req.SignatureParams,
				signatureKnown:   req.SignatureKnown,
				returnType:       req.ReturnType,
				canonicalTypeKey: req.CanonicalTypeKey,
				typeImportPath:   req.TypeImportPath,
				typePackageName:  req.TypePackageName,
				typeName:         req.TypeName,
				typeImports:      req.TypeImports,
				returnsError:     req.ReturnsError,
				filename:         req.Filename,
				line:             req.Line,
				declOrder:        r.nextDeclOrder,
			}
			r.nextDeclOrder++
			if decl.hasExplicitName {
				if prev := seenNames[decl.name]; prev != nil && !sameDeclFuncAndArgs(prev, decl) {
					return reqError(req, scope, "conflicting requirement name %q in command %s", decl.name, scope.displayName)
				}
				seenNames[decl.name] = decl
			}
			scope.declProviders = append(scope.declProviders, decl)
			if req.Policy == RequirementPolicyAlways {
				scope.mandatoryProviders = append(scope.mandatoryProviders, &mandatoryProviderDecl{
					owner:     scope,
					provider:  decl,
					declOrder: decl.declOrder,
				})
			}
		}
	}
	return nil
}

func validateExplicitArity(req *RequirementConfig, scope *depScope) error {
	paramCount := len(req.SignatureParams)
	hasVariadic := paramCount > 0 && req.SignatureParams[paramCount-1].IsVariadic
	argCount := len(req.ExplicitInputs)
	if hasVariadic {
		if argCount < paramCount-1 {
			return reqError(req, scope, "requirement %s expects at least %d arguments, got %d", formatFuncRefName(req.Func), paramCount-1, argCount)
		}
		return nil
	}
	if argCount != paramCount {
		return reqError(req, scope, "requirement %s expects %d arguments, got %d", formatFuncRefName(req.Func), paramCount, argCount)
	}
	return nil
}

func reqError(req *RequirementConfig, scope *depScope, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if req != nil && req.Filename != "" && req.Line > 0 {
		return fmt.Errorf("%s:%d: %s", req.Filename, req.Line, msg)
	}
	if scope != nil && scope.displayName != "" {
		return fmt.Errorf("command %s: %s", scope.displayName, msg)
	}
	return fmt.Errorf("%s", msg)
}

func formatFuncRefName(ref *FuncRef) string {
	if ref == nil {
		return ""
	}
	if ref.ImportPath != "" {
		return fmt.Sprintf("%q.%s", ref.ImportPath, ref.FunctionName)
	}
	if ref.CommandPackageName != "" {
		return ref.CommandPackageName + "." + ref.FunctionName
	}
	return ref.FunctionName
}

func sameDeclFuncAndArgs(a, b *providerDecl) bool {
	if !sameFuncRef(a.funcRef, b.funcRef) {
		return false
	}
	if len(a.explicitInputs) != len(b.explicitInputs) {
		return false
	}
	for i := range a.explicitInputs {
		if a.explicitInputs[i] != b.explicitInputs[i] {
			return false
		}
	}
	return a.hasExplicitArgs == b.hasExplicitArgs
}

func sameFuncRef(a, b *FuncRef) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ImportPath == b.ImportPath && a.FunctionName == b.FunctionName
}

func (s *depScope) hasAction() bool {
	if s.isRoot {
		return s.cmd.HasAction()
	}
	return s.sc.HasAction()
}

func (s *depScope) ancestorsRootFirst() []*depScope {
	var chain []*depScope
	for curr := s; curr != nil; curr = curr.parent {
		chain = append(chain, curr)
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func (r *depResolver) resolveExecutableRoutes() error {
	for _, scope := range r.allScopes {
		if !scope.hasAction() {
			continue
		}
		if err := r.resolveRoute(scope); err != nil {
			return err
		}
	}
	return nil
}

type mandatoryItem struct {
	declOrder int
	step      *ExecutionStep
}

func (r *depResolver) resolveRoute(execScope *depScope) error {
	chain := execScope.ancestorsRootFirst()

	var items []mandatoryItem
	seenSteps := make(map[string]bool)

	for _, s := range chain {
		for _, gd := range s.declGuards {
			inputs, err := r.resolveFunctionInputs(execScope, gd.owner, gd.funcRef, gd.explicitInputs, gd.hasExplicitArgs, gd.signatureParams, gd.signatureKnown, nil, true, gd.filename, gd.line, gd.name)
			if err != nil {
				return err
			}
			guardID := buildGuardIdentity(gd.funcRef, inputs)
			if seenSteps[guardID] {
				continue
			}
			seenSteps[guardID] = true
			if gd.isLegacy && gd.legacyInit != nil {
				gd.legacyInit.ResolvedInputs = inputs
				gd.legacyInit.IdentityKey = guardID
			}
			items = append(items, mandatoryItem{
				declOrder: gd.declOrder,
				step: &ExecutionStep{
					Kind:         ExecutionStepGuard,
					IdentityKey:  guardID,
					GuardName:    gd.name,
					Func:         cloneFuncRef(gd.funcRef),
					Inputs:       inputs,
					ReturnsError: gd.returnsError,
				},
			})
		}
		for _, mp := range s.mandatoryProviders {
			rp, err := r.resolveProvider(execScope, mp.provider, nil, true)
			if err != nil {
				return err
			}
			if seenSteps[rp.sp.IdentityKey] {
				continue
			}
			seenSteps[rp.sp.IdentityKey] = true
			items = append(items, mandatoryItem{
				declOrder: mp.declOrder,
				step: &ExecutionStep{
					Kind:        ExecutionStepProvider,
					IdentityKey: rp.sp.IdentityKey,
					Provider:    rp.sp,
				},
			})
		}
	}

	orderedSteps := topologicalSortMandatorySteps(items)
	if execScope.isRoot {
		execScope.cmd.ExecutionSteps = orderedSteps
	} else {
		execScope.sc.ExecutionSteps = orderedSteps
	}

	if err := r.bindActionParameters(execScope); err != nil {
		return err
	}
	return nil
}

func topologicalSortMandatorySteps(items []mandatoryItem) []*ExecutionStep {
	if len(items) <= 1 {
		res := make([]*ExecutionStep, len(items))
		for i, it := range items {
			res[i] = it.step
		}
		return res
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].declOrder < items[j].declOrder
	})

	providerStepIndex := make(map[string]int)
	for i, it := range items {
		if it.step.Kind == ExecutionStepProvider && it.step.Provider != nil {
			providerStepIndex[it.step.Provider.IdentityKey] = i
		}
	}

	n := len(items)
	adj := make([][]int, n)
	indeg := make([]int, n)

	for i, it := range items {
		var roots []DependencyInput
		if it.step.Kind == ExecutionStepGuard {
			roots = it.step.Inputs
		} else if it.step.Provider != nil {
			roots = it.step.Provider.ResolvedInputs
		}
		deps := collectTransitiveProviderIDs(roots)
		seenEdge := make(map[int]bool)
		for depID := range deps {
			u, ok := providerStepIndex[depID]
			if ok && u != i && !seenEdge[u] {
				seenEdge[u] = true
				adj[u] = append(adj[u], i)
				indeg[i]++
			}
		}
	}

	emitted := make([]bool, n)
	result := make([]*ExecutionStep, 0, n)
	for len(result) < n {
		next := -1
		for i := 0; i < n; i++ {
			if !emitted[i] && indeg[i] == 0 {
				next = i
				break
			}
		}
		if next == -1 {
			for i := 0; i < n; i++ {
				if !emitted[i] {
					next = i
					break
				}
			}
		}
		emitted[next] = true
		result = append(result, items[next].step)
		for _, v := range adj[next] {
			indeg[v]--
		}
	}
	return result
}

func collectTransitiveProviderIDs(inputs []DependencyInput) map[string]bool {
	seen := make(map[string]bool)
	var walk func([]DependencyInput)
	walk = func(ins []DependencyInput) {
		for _, in := range ins {
			if in.Kind != DependencyInputProvider || in.Provider == nil {
				continue
			}
			id := in.Provider.IdentityKey
			if seen[id] {
				continue
			}
			seen[id] = true
			walk(in.Provider.ResolvedInputs)
		}
	}
	walk(inputs)
	return seen
}

func (r *depResolver) resolveProvider(execScope *depScope, decl *providerDecl, stack []string, recordDemander bool) (*resolvedProvider, error) {
	cycleKey := fmt.Sprintf("%s|%s|%s", decl.owner.pathKey, funcCycleName(decl.funcRef), strings.Join(decl.explicitInputs, ","))
	for i, s := range stack {
		if s == cycleKey {
			cycle := append(append([]string(nil), stack[i:]...), cycleKey)
			return nil, declErr(decl.filename, decl.line, execScope, "dependency cycle detected: %s", formatCycle(cycle))
		}
	}
	nextStack := append(append([]string(nil), stack...), cycleKey)

	inputs, err := r.resolveFunctionInputs(execScope, decl.owner, decl.funcRef, decl.explicitInputs, decl.hasExplicitArgs, decl.signatureParams, decl.signatureKnown, nextStack, recordDemander, decl.filename, decl.line, decl.name)
	if err != nil {
		return nil, err
	}

	resolvedCanonOwner := decl.owner
	if !decl.isLegacy {
		resolvedCanonOwner = r.findCanonicalAncestorDeclScope(decl)
	}

	idKey := buildProviderIdentity(resolvedCanonOwner.pathKey, decl.funcRef, decl.canonicalTypeKey, decl.returnType, decl.returnsError, inputs)
	rp, exists := r.providersByID[idKey]
	if !exists {
		var sp *SharedProviderConfig
		if decl.isLegacy && decl.legacySP != nil {
			sp = decl.legacySP
			sp.ResolvedInputs = inputs
			sp.IdentityKey = idKey
		} else {
			sp = &SharedProviderConfig{
				Name:             decl.name,
				Type:             decl.returnType,
				TypeImportPath:   decl.typeImportPath,
				TypePackageName:  decl.typePackageName,
				CanonicalTypeKey: decl.canonicalTypeKey,
				TypeImports:      cloneTypeImports(decl.typeImports),
				Func:             cloneFuncRef(decl.funcRef),
				ReturnsError:     decl.returnsError,
				SignatureParams:  decl.signatureParams,
				SignatureKnown:   decl.signatureKnown,
				Filename:         decl.filename,
				Line:             decl.line,
				ResolvedInputs:   inputs,
				IdentityKey:      idKey,
				GeneratedByResolver: !decl.isLegacy,
			}
		}
		fallible := sp.ReturnsError
		for _, in := range inputs {
			if in.Kind == DependencyInputProvider && in.Provider != nil {
				if up := r.providersByID[in.Provider.IdentityKey]; up != nil && up.isFallible {
					fallible = true
				}
			}
		}
		var host *depScope
		if decl.isLegacy {
			host = decl.owner
		}
		rp = &resolvedProvider{
			sp:               sp,
			declHost:         host,
			isFallible:       fallible,
			canonicalTypeKey: decl.canonicalTypeKey,
			firstOrder:       decl.declOrder,
			decl:             decl,
		}
		r.providersByID[idKey] = rp
		r.providerOrder = append(r.providerOrder, idKey)
	}

	if recordDemander {
		r.recordDemanderRecursive(rp, execScope)
	}
	return rp, nil
}

func (r *depResolver) findCanonicalAncestorDeclScope(target *providerDecl) *depScope {
	chain := target.owner.ancestorsRootFirst()
	for _, anc := range chain {
		for _, cand := range anc.declProviders {
			if !sameFuncRef(cand.funcRef, target.funcRef) {
				continue
			}
			candType := cand.canonicalTypeKey
			if candType == "" {
				candType = cand.returnType
			}
			targetType := target.canonicalTypeKey
			if targetType == "" {
				targetType = target.returnType
			}
			if candType != "" && targetType != "" && candType != targetType {
				continue
			}
			if cand.hasExplicitArgs != target.hasExplicitArgs || len(cand.explicitInputs) != len(target.explicitInputs) {
				continue
			}
			sameExplicit := true
			for i := range cand.explicitInputs {
				if cand.explicitInputs[i] != target.explicitInputs[i] {
					sameExplicit = false
					break
				}
			}
			if !sameExplicit {
				continue
			}
			return anc
		}
	}
	return target.owner
}

func addDemander(rp *resolvedProvider, scope *depScope) {
	for _, existing := range rp.demanders {
		if existing == scope {
			return
		}
	}
	rp.demanders = append(rp.demanders, scope)
}

func funcCycleName(ref *FuncRef) string {
	if ref == nil {
		return ""
	}
	if ref.ImportPath != "" {
		return ref.ImportPath + "." + ref.FunctionName
	}
	return ref.FunctionName
}

func formatCycle(cycle []string) string {
	names := make([]string, 0, len(cycle))
	for _, item := range cycle {
		parts := strings.Split(item, "|")
		if len(parts) >= 2 && parts[1] != "" {
			names = append(names, parts[1])
		} else {
			names = append(names, item)
		}
	}
	return strings.Join(names, " -> ")
}

func (r *depResolver) resolveFunctionInputs(
	execScope *depScope,
	declOwner *depScope,
	fnRef *FuncRef,
	explicitInputs []string,
	hasExplicitArgs bool,
	sigParams []ResolvedParamSpec,
	sigKnown bool,
	stack []string,
	recordDemander bool,
	filename string,
	line int,
	diagName string,
) ([]DependencyInput, error) {
	if hasExplicitArgs || (len(explicitInputs) > 0) {
		inputs := make([]DependencyInput, 0, len(explicitInputs))
		for idx, rawName := range explicitInputs {
			var expectedSig *ResolvedParamSpec
			if sigKnown && idx < len(sigParams) {
				p := sigParams[idx]
				expectedSig = &p
			}
			in, err := r.resolveExplicitInput(execScope, rawName, expectedSig, stack, recordDemander, filename, line, diagName, fnRef)
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, in)
		}
		return inputs, nil
	}

	if !sigKnown || len(sigParams) == 0 {
		return nil, nil
	}

	inputs := make([]DependencyInput, 0, len(sigParams))
	for _, sp := range sigParams {
		if sp.IsVariadic {
			continue
		}
		in, err := r.resolveImplicitInput(execScope, declOwner, fnRef, sp, stack, recordDemander, filename, line)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, in)
	}
	return inputs, nil
}

func (r *depResolver) resolveExplicitInput(
	execScope *depScope,
	argName string,
	expectedSig *ResolvedParamSpec,
	stack []string,
	recordDemander bool,
	filename string,
	line int,
	diagName string,
	fnRef *FuncRef,
) (DependencyInput, error) {
	if ownerScope, param := r.findNearestParam(execScope, argName); param != nil {
		if param.Generator.Type == SourceTypeGenerator && !param.Inherited && param.SharedProvider == nil {
			rp, err := r.promoteLocalGenerator(ownerScope, param, stack, recordDemander)
			if err != nil {
				return DependencyInput{}, err
			}
			isCallable := expectedSig != nil && expectedSig.IsCallable
			if isCallable && !expectedSig.CallableReturnsError && rp.isFallible {
				return DependencyInput{}, declErr(filename, line, execScope, "requirement %s parameter %q has infallible callable type, but provider %s can return an error", formatFuncRefName(fnRef), expectedSig.Name, formatFuncRefName(rp.sp.Func))
			}
			return DependencyInput{
				Kind:           DependencyInputProvider,
				Provider:       rp.sp,
				LazyCallable:   isCallable,
				LazyReturnsErr: expectedSig != nil && expectedSig.CallableReturnsError,
			}, nil
		}
		if param.SharedProvider != nil {
			if recordDemander {
				if rp := r.providersByID[param.SharedProvider.IdentityKey]; rp != nil {
					r.recordDemanderRecursive(rp, execScope)
				}
			}
			return DependencyInput{
				Kind:           DependencyInputProvider,
				Provider:       param.SharedProvider,
				LazyCallable:   expectedSig != nil && expectedSig.IsCallable,
				LazyReturnsErr: expectedSig != nil && expectedSig.CallableReturnsError,
			}, nil
		}
		paramType := paramCanonicalType(param)
		if expectedSig != nil && expectedSig.CanonicalTypeKey != "" && paramType != "" && paramType != expectedSig.CanonicalTypeKey {
			return DependencyInput{}, declErr(filename, line, execScope, "requirement %s argument %q expects type %s, got parameter type %s", formatFuncRefName(fnRef), argName, expectedSig.Type, param.Type)
		}
		return DependencyInput{
			Kind:           DependencyInputParam,
			ParamName:      param.Name,
			ParamOwnerPath: ownerScope.pathKey,
		}, nil
	}

	matchingDecls := r.findVisibleProvidersByName(execScope, argName)
	if len(matchingDecls) == 1 {
		rp, err := r.resolveProvider(execScope, matchingDecls[0], stack, recordDemander)
		if err != nil {
			return DependencyInput{}, err
		}
		isCallable := expectedSig != nil && expectedSig.IsCallable
		if isCallable && !expectedSig.CallableReturnsError && rp.isFallible {
			return DependencyInput{}, declErr(filename, line, execScope, "requirement %s argument %q has infallible callable type, but provider %s can return an error", formatFuncRefName(fnRef), argName, formatFuncRefName(rp.sp.Func))
		}
		if expectedSig != nil && !isCallable && expectedSig.CanonicalTypeKey != "" && rp.canonicalTypeKey != "" && rp.canonicalTypeKey != expectedSig.CanonicalTypeKey {
			return DependencyInput{}, declErr(filename, line, execScope, "requirement %s argument %q expects type %s, got provider %s returning %s", formatFuncRefName(fnRef), argName, expectedSig.Type, argName, rp.sp.Type)
		}
		if expectedSig != nil && isCallable && expectedSig.CallableReturnTypeKey != "" && rp.canonicalTypeKey != "" && rp.canonicalTypeKey != expectedSig.CallableReturnTypeKey {
			return DependencyInput{}, declErr(filename, line, execScope, "requirement %s argument %q expects callable returning %s, got %s", formatFuncRefName(fnRef), argName, expectedSig.CallableReturnTypeKey, rp.canonicalTypeKey)
		}
		return DependencyInput{
			Kind:           DependencyInputProvider,
			Provider:       rp.sp,
			LazyCallable:   isCallable,
			LazyReturnsErr: expectedSig != nil && expectedSig.CallableReturnsError,
		}, nil
	}
	if len(matchingDecls) > 1 {
		return DependencyInput{}, declErr(filename, line, execScope, "requirement %s argument %q is ambiguous across multiple providers", formatFuncRefName(fnRef), argName)
	}

	label := diagName
	if label == "" {
		label = formatFuncRefName(fnRef)
	}
	return DependencyInput{}, declErr(filename, line, execScope, "%q references unknown parameter %q", label, argName)
}

func (r *depResolver) resolveImplicitInput(
	execScope *depScope,
	declOwner *depScope,
	fnRef *FuncRef,
	sigParam ResolvedParamSpec,
	stack []string,
	recordDemander bool,
	filename string,
	line int,
) (DependencyInput, error) {
	targetType := sigParam.CanonicalTypeKey
	if sigParam.IsCallable {
		targetType = sigParam.CallableReturnTypeKey
	}

	candidates := r.findVisibleProvidersByType(execScope, targetType)
	var resolvedCandidates []*resolvedProvider
	seenIDs := make(map[string]bool)
	for _, cand := range candidates {
		rp, err := r.resolveProvider(execScope, cand, stack, false)
		if err != nil {
			return DependencyInput{}, err
		}
		if !seenIDs[rp.sp.IdentityKey] {
			seenIDs[rp.sp.IdentityKey] = true
			resolvedCandidates = append(resolvedCandidates, rp)
		}
	}

	if sigParam.IsCallable {
		switch len(resolvedCandidates) {
		case 0:
			return DependencyInput{}, declErr(filename, line, execScope, "requirement %s cannot resolve lazy callable parameter %q (%s)", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type)
		case 1:
			chosen := resolvedCandidates[0]
			if !sigParam.CallableReturnsError && chosen.isFallible {
				return DependencyInput{}, declErr(filename, line, execScope, "requirement %s parameter %q has infallible callable type %s, but provider %s can return an error", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type, formatFuncRefName(chosen.sp.Func))
			}
			if recordDemander {
				r.recordDemanderRecursive(chosen, execScope)
			}
			return DependencyInput{
				Kind:           DependencyInputProvider,
				Provider:       chosen.sp,
				LazyCallable:   true,
				LazyReturnsErr: sigParam.CallableReturnsError,
			}, nil
		default:
			return DependencyInput{}, declErr(filename, line, execScope, "requirement %s parameter %q (%s) is ambiguous; multiple providers supply that type (%s)", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type, formatCandidateNames(resolvedCandidates))
		}
	}

	var matchedOwner *depScope
	var matchedParam *FunctionParameter
	if sigParam.Name != "" {
		owner, p := r.findNearestParam(execScope, sigParam.Name)
		if p != nil && paramCanonicalType(p) == targetType {
			matchedOwner = owner
			matchedParam = p
		}
	}

	if matchedParam != nil {
		if matchedParam.Generator.Type == SourceTypeGenerator && !matchedParam.Inherited && matchedParam.SharedProvider == nil {
			if len(resolvedCandidates) > 0 {
				return DependencyInput{}, declErr(filename, line, execScope, "requirement %s parameter %q (%s) is ambiguous between local generator and declared provider", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type)
			}
			rp, err := r.promoteLocalGenerator(matchedOwner, matchedParam, stack, recordDemander)
			if err != nil {
				return DependencyInput{}, err
			}
			return DependencyInput{
				Kind:     DependencyInputProvider,
				Provider: rp.sp,
			}, nil
		}
		if matchedParam.SharedProvider != nil {
			if len(resolvedCandidates) > 0 {
				alreadySame := len(resolvedCandidates) == 1 && resolvedCandidates[0].sp == matchedParam.SharedProvider
				if !alreadySame {
					return DependencyInput{}, declErr(filename, line, execScope, "requirement %s parameter %q (%s) is ambiguous between bound provider and declared provider (%s)", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type, formatCandidateNames(resolvedCandidates))
				}
			}
			if recordDemander {
				if rp := r.providersByID[matchedParam.SharedProvider.IdentityKey]; rp != nil {
					r.recordDemanderRecursive(rp, execScope)
				}
			}
			return DependencyInput{
				Kind:     DependencyInputProvider,
				Provider: matchedParam.SharedProvider,
			}, nil
		}
		if !matchedParam.HasConstructor() && matchedParam.IsBuiltinCLIType() {
			return DependencyInput{
				Kind:           DependencyInputParam,
				ParamName:      matchedParam.Name,
				ParamOwnerPath: matchedOwner.pathKey,
			}, nil
		}
		if len(resolvedCandidates) > 0 && (matchedParam.HasExplicitCLISpec || matchedParam.HasConstructor()) && !matchedParam.IsBuiltinCLIType() {
			return DependencyInput{}, declErr(filename, line, execScope, "requirement %s parameter %q (%s) is ambiguous between command parameter and declared provider (%s)", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type, formatCandidateNames(resolvedCandidates))
		}
	}

	if len(resolvedCandidates) == 1 {
		chosen := resolvedCandidates[0]
		if recordDemander {
			r.recordDemanderRecursive(chosen, execScope)
		}
		return DependencyInput{
			Kind:     DependencyInputProvider,
			Provider: chosen.sp,
		}, nil
	}
	if len(resolvedCandidates) > 1 {
		return DependencyInput{}, declErr(filename, line, execScope, "requirement %s parameter %q (%s) is ambiguous; multiple providers match (%s)", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type, formatCandidateNames(resolvedCandidates))
	}

	if matchedParam != nil && (matchedParam.HasExplicitCLISpec || matchedParam.IsBuiltinCLIType() || matchedParam.HasConstructor()) {
		return DependencyInput{
			Kind:           DependencyInputParam,
			ParamName:      matchedParam.Name,
			ParamOwnerPath: matchedOwner.pathKey,
		}, nil
	}

	// Fallback: unique root constructor context parameter matching non-builtin type.
	if targetType != "" && !isBuiltinCanonicalType(targetType) {
		var ctxMatches []*FunctionParameter
		for _, cp := range r.cmd.Parameters {
			if cp.HasConstructor() && paramCanonicalType(cp) == targetType {
				ctxMatches = append(ctxMatches, cp)
			}
		}
		if len(ctxMatches) == 1 {
			return DependencyInput{
				Kind:           DependencyInputParam,
				ParamName:      ctxMatches[0].Name,
				ParamOwnerPath: r.root.pathKey,
			}, nil
		}
		if len(ctxMatches) > 1 {
			return DependencyInput{}, declErr(filename, line, execScope, "requirement %s parameter %q (%s) is ambiguous across multiple root context parameters", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type)
		}
	}

	return DependencyInput{}, declErr(filename, line, execScope, "requirement %s cannot resolve parameter %q (%s) in command %s", formatFuncRefName(fnRef), sigParam.Name, sigParam.Type, declOwner.displayName)
}

func (r *depResolver) recordDemanderRecursive(rp *resolvedProvider, scope *depScope) {
	if rp == nil {
		return
	}
	addDemander(rp, scope)
	for _, in := range rp.sp.ResolvedInputs {
		if in.Kind == DependencyInputProvider && in.Provider != nil {
			if up := r.providersByID[in.Provider.IdentityKey]; up != nil {
				r.recordDemanderRecursive(up, scope)
			}
		}
	}
}

func (r *depResolver) promoteLocalGenerator(scope *depScope, param *FunctionParameter, stack []string, recordDemander bool) (*resolvedProvider, error) {
	if param.SharedProvider != nil {
		if existing := r.providersByID[param.SharedProvider.IdentityKey]; existing != nil {
			if recordDemander {
				r.recordDemanderRecursive(existing, scope)
			}
			return existing, nil
		}
	}
	decl := &providerDecl{
		owner:            scope,
		funcRef:          param.Generator.Func,
		name:             param.Name,
		hasExplicitName:  true,
		explicitInputs:   param.Generator.Inputs,
		hasExplicitArgs:  len(param.Generator.Inputs) > 0,
		returnType:       param.Type,
		canonicalTypeKey: param.CanonicalTypeKey,
		typeImportPath:   param.TypeImportPath,
		typePackageName:  param.TypePackageName,
		typeName:         param.TypeName,
		typeImports:      param.TypeImports,
		returnsError:     true,
		declOrder:        r.nextDeclOrder,
	}
	r.nextDeclOrder++
	rp, err := r.resolveProvider(scope, decl, stack, recordDemander)
	if err != nil {
		return nil, err
	}
	rp.declHost = scope
	param.SharedProvider = rp.sp
	param.BoundByResolver = true
	return rp, nil
}

func (r *depResolver) bindActionParameters(execScope *depScope) error {
	for _, p := range execScope.params {
		if p.Inherited || p.HasConstructor() || p.IsUnmatchedArgv || p.IsFlagPresence() {
			continue
		}
		if p.Generator.Type == SourceTypeGenerator && p.Generator.Func != nil && p.Generator.Func.FunctionName != "" {
			if err := r.bindExplicitActionParam(execScope, p); err != nil {
				return err
			}
			continue
		}
		if p.HasExplicitCLISpec || p.HasGenerator() || p.IsBuiltinCLIType() {
			continue
		}
		if err := r.bindImplicitActionParam(execScope, p); err != nil {
			return err
		}
	}
	return nil
}

func (r *depResolver) bindExplicitActionParam(execScope *depScope, p *FunctionParameter) error {
	ref := p.Generator.Func
	var matching []*providerDecl
	for curr := execScope; curr != nil; curr = curr.parent {
		for _, decl := range curr.declProviders {
			if matchDeclForExplicitParam(p, ref, decl) {
				matching = append(matching, decl)
			}
		}
	}
	if len(matching) == 0 {
		return nil
	}

	var resolved []*resolvedProvider
	seenID := make(map[string]bool)
	for _, m := range matching {
		rp, err := r.resolveProvider(execScope, m, nil, false)
		if err != nil {
			return err
		}
		if !seenID[rp.sp.IdentityKey] {
			seenID[rp.sp.IdentityKey] = true
			resolved = append(resolved, rp)
		}
	}
	if len(resolved) > 1 {
		return fmt.Errorf("command %s: parameter %q references ambiguous provider (%s)", execScope.displayName, p.Name, formatCandidateNames(resolved))
	}

	chosen := resolved[0]
	targetType := p.CanonicalTypeKey
	if p.IsLazyCallableType() {
		targetType = p.CallableReturnTypeKey
		if !p.CallableReturnsError && chosen.isFallible {
			return fmt.Errorf("command %s: parameter %q has infallible callable type %s, but provider %s can return an error", execScope.displayName, p.Name, p.Type, formatFuncRefName(chosen.sp.Func))
		}
	}
	if targetType != "" && chosen.canonicalTypeKey != "" && targetType != chosen.canonicalTypeKey {
		return fmt.Errorf("command %s: parameter %q expects type %s, but provider %s returns %s", execScope.displayName, p.Name, p.Type, formatFuncRefName(chosen.sp.Func), chosen.sp.Type)
	}

	r.recordDemanderRecursive(chosen, execScope)
	applyProviderBindingToParam(p, chosen.sp)
	return nil
}

func matchDeclForExplicitParam(p *FunctionParameter, ref *FuncRef, decl *providerDecl) bool {
	if decl.isLegacy {
		if p.Name == decl.name || ref.FunctionName == decl.name {
			return true
		}
		if decl.funcRef != nil && ref.FunctionName == decl.funcRef.FunctionName {
			if ref.ImportPath == "" || ref.ImportPath == decl.funcRef.ImportPath {
				return true
			}
		}
		return false
	}

	if !p.Generator.ExplicitlyQualified && len(p.Generator.Inputs) == 0 && decl.hasExplicitName && ref.FunctionName == decl.name {
		return true
	}
	if decl.funcRef == nil || decl.funcRef.FunctionName != ref.FunctionName {
		return false
	}
	if ref.ImportPath != "" && decl.funcRef.ImportPath != "" && ref.ImportPath != decl.funcRef.ImportPath {
		return false
	}
	if len(p.Generator.Inputs) > 0 {
		if len(p.Generator.Inputs) != len(decl.explicitInputs) {
			return false
		}
		for i := range p.Generator.Inputs {
			if p.Generator.Inputs[i] != decl.explicitInputs[i] {
				return false
			}
		}
	}
	return true
}

func (r *depResolver) bindImplicitActionParam(execScope *depScope, p *FunctionParameter) error {
	targetType := p.CanonicalTypeKey
	if p.IsLazyCallableType() {
		targetType = p.CallableReturnTypeKey
	}

	var candidates []*providerDecl
	if targetType != "" {
		candidates = r.findVisibleProvidersByType(execScope, targetType)
	}
	if len(candidates) == 0 {
		candidates = r.findVisibleLegacyProvidersByName(execScope, p.Name)
	}
	if len(candidates) == 0 {
		return nil
	}

	var resolved []*resolvedProvider
	seenID := make(map[string]bool)
	for _, cand := range candidates {
		rp, err := r.resolveProvider(execScope, cand, nil, false)
		if err != nil {
			return err
		}
		if !seenID[rp.sp.IdentityKey] {
			seenID[rp.sp.IdentityKey] = true
			resolved = append(resolved, rp)
		}
	}

	if len(resolved) > 1 {
		return fmt.Errorf("command %s: parameter %q of type %s is ambiguous; multiple providers match (%s); disambiguate with (provider: ...)", execScope.displayName, p.Name, p.Type, formatCandidateNames(resolved))
	}

	chosen := resolved[0]
	if p.IsLazyCallableType() && !p.CallableReturnsError && chosen.isFallible {
		return fmt.Errorf("command %s: parameter %q has infallible callable type %s, but provider %s can return an error", execScope.displayName, p.Name, p.Type, formatFuncRefName(chosen.sp.Func))
	}

	r.recordDemanderRecursive(chosen, execScope)
	applyProviderBindingToParam(p, chosen.sp)
	return nil
}

func applyProviderBindingToParam(p *FunctionParameter, sp *SharedProviderConfig) {
	p.SharedProvider = sp
	p.BoundByResolver = true
	if p.IsLazyCallableType() {
		p.IsLazyCallable = true
		p.IsCallableInfallible = !p.CallableReturnsError
		if sp.Type == "" {
			sp.Type = ExtractCallableReturnType(p.Type)
			sp.TypeImportPath = p.TypeImportPath
			sp.TypePackageName = p.TypePackageName
		}
	} else {
		p.IsLazyCallable = false
		p.IsCallableInfallible = false
		if sp.Type == "" {
			sp.Type = p.Type
			sp.TypeImportPath = p.TypeImportPath
			sp.TypePackageName = p.TypePackageName
		}
	}
}

func (r *depResolver) findVisibleProvidersByType(start *depScope, canonicalTypeKey string) []*providerDecl {
	if canonicalTypeKey == "" {
		return nil
	}
	var out []*providerDecl
	for curr := start; curr != nil; curr = curr.parent {
		for _, decl := range curr.declProviders {
			if decl.canonicalTypeKey == canonicalTypeKey {
				out = append(out, decl)
			}
		}
	}
	return out
}

func (r *depResolver) findVisibleLegacyProvidersByName(start *depScope, name string) []*providerDecl {
	var out []*providerDecl
	for curr := start; curr != nil; curr = curr.parent {
		for _, decl := range curr.declProviders {
			if decl.isLegacy && decl.canonicalTypeKey == "" && decl.name == name {
				out = append(out, decl)
			}
		}
	}
	return out
}

func (r *depResolver) findVisibleProvidersByName(start *depScope, name string) []*providerDecl {
	var out []*providerDecl
	for curr := start; curr != nil; curr = curr.parent {
		for _, decl := range curr.declProviders {
			if decl.hasExplicitName && decl.name == name {
				out = append(out, decl)
			}
		}
	}
	return out
}

func (r *depResolver) findNearestParam(start *depScope, name string) (*depScope, *FunctionParameter) {
	targetName := name
	for curr := start; curr != nil; curr = curr.parent {
		for _, p := range curr.params {
			if p.Name != targetName {
				continue
			}
			if p.Inherited {
				if p.InheritedFrom != "" {
					targetName = p.InheritedFrom
				}
				break
			}
			return curr, p
		}
	}
	return nil, nil
}

func (r *depResolver) validateUnusedRequirements() error {
	for _, scope := range r.allScopes {
		for _, gd := range scope.declGuards {
			if len(gd.explicitInputs) > 0 {
				if !r.hasScopedParamInSubtree(scope, gd.explicitInputs) {
					return r.diagnoseMissingExplicitInputs(scope, gd.filename, gd.line, gd.name, gd.explicitInputs)
				}
			}
		}
		for _, decl := range scope.declProviders {
			if len(decl.explicitInputs) > 0 {
				if !r.hasScopedParamInSubtree(scope, decl.explicitInputs) {
					return r.diagnoseMissingExplicitInputs(scope, decl.filename, decl.line, decl.name, decl.explicitInputs)
				}
			}
			if !decl.isLegacy {
				if err := r.probeCyclesForUnusedProvider(decl, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *depResolver) hasScopedParamInSubtree(scope *depScope, names []string) bool {
	for _, name := range names {
		if r.paramOrProviderExists(scope, name) {
			continue
		}
		if r.existsInDescendant(scope, name) {
			continue
		}
		return false
	}
	return true
}

func (r *depResolver) diagnoseMissingExplicitInputs(scope *depScope, filename string, line int, diagName string, names []string) error {
	for _, name := range names {
		if !r.paramOrProviderExists(scope, name) && !r.existsInDescendant(scope, name) {
			return declErr(filename, line, scope, "%q references unknown parameter %q", diagName, name)
		}
	}
	return nil
}

func (r *depResolver) paramOrProviderExists(scope *depScope, name string) bool {
	if _, p := r.findNearestParam(scope, name); p != nil {
		return true
	}
	return len(r.findVisibleProvidersByName(scope, name)) > 0
}

func (r *depResolver) existsInDescendant(scope *depScope, name string) bool {
	for _, child := range scope.children {
		for _, p := range child.params {
			if p.Name == name {
				return true
			}
		}
		for _, dp := range child.declProviders {
			if dp.hasExplicitName && dp.name == name {
				return true
			}
		}
		if r.existsInDescendant(child, name) {
			return true
		}
	}
	return false
}

func (r *depResolver) probeCyclesForUnusedProvider(decl *providerDecl, stack []string) error {
	cycleKey := fmt.Sprintf("%s|%s|%s", decl.owner.pathKey, funcCycleName(decl.funcRef), strings.Join(decl.explicitInputs, ","))
	for i, s := range stack {
		if s == cycleKey {
			cycle := append(append([]string(nil), stack[i:]...), cycleKey)
			return declErr(decl.filename, decl.line, decl.owner, "dependency cycle detected: %s", formatCycle(cycle))
		}
	}
	nextStack := append(append([]string(nil), stack...), cycleKey)
	if !decl.signatureKnown {
		return nil
	}
	for _, sp := range decl.signatureParams {
		if sp.IsVariadic {
			continue
		}
		targetType := sp.CanonicalTypeKey
		if sp.IsCallable {
			targetType = sp.CallableReturnTypeKey
		}
		if targetType == "" {
			continue
		}
		cands := r.findVisibleProvidersByType(decl.owner, targetType)
		if len(cands) == 1 {
			if err := r.probeCyclesForUnusedProvider(cands[0], nextStack); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *depResolver) hoistRepeatedLocalGenerators() error {
	type localUsage struct {
		scope  *depScope
		param  *FunctionParameter
		inputs []DependencyInput
	}
	type usageGroup struct {
		key   string
		items []localUsage
	}

	groups := make(map[string]*usageGroup)
	var groupOrder []string

	for _, scope := range r.allScopes {
		if scope.isRoot {
			continue
		}
		for _, p := range scope.params {
			if p.Inherited || p.SharedProvider != nil {
				continue
			}
			if p.Generator.Type != SourceTypeGenerator || p.Generator.Func == nil || p.Generator.Func.FunctionName == "" {
				continue
			}
			resolvedIns := make([]DependencyInput, 0, len(p.Generator.Inputs))
			canHoist := true
			for _, inName := range p.Generator.Inputs {
				owner, inp := r.findNearestParam(scope, inName)
				if inp == nil || (inp.Generator.Type == SourceTypeGenerator && inp.SharedProvider == nil) {
					canHoist = false
					break
				}
				if inp.SharedProvider != nil {
					resolvedIns = append(resolvedIns, DependencyInput{
						Kind:     DependencyInputProvider,
						Provider: inp.SharedProvider,
					})
				} else {
					resolvedIns = append(resolvedIns, DependencyInput{
						Kind:           DependencyInputParam,
						ParamName:      inp.Name,
						ParamOwnerPath: owner.pathKey,
					})
				}
			}
			if !canHoist {
				continue
			}

			key := buildProviderIdentity("<local>", p.Generator.Func, paramCanonicalType(p), p.Type, true, resolvedIns)
			g, ok := groups[key]
			if !ok {
				g = &usageGroup{key: key}
				groups[key] = g
				groupOrder = append(groupOrder, key)
			}
			g.items = append(g.items, localUsage{scope: scope, param: p, inputs: resolvedIns})
		}
	}

	for _, key := range groupOrder {
		g := groups[key]
		if len(g.items) < 2 {
			continue
		}
		distinctFuncs := make(map[string]bool)
		var scopes []*depScope
		for _, item := range g.items {
			fnName := item.scope.sc.SubCommandFunctionName
			if fnName != "" && fnName != r.cmd.FunctionName {
				distinctFuncs[item.scope.pathKey+"#"+fnName] = true
			}
			scopes = append(scopes, item.scope)
		}
		if len(distinctFuncs) < 2 {
			continue
		}

		lca := lowestCommonAncestor(scopes)
		allInputsInLCA := true
		for _, in := range g.items[0].inputs {
			if in.Kind == DependencyInputParam {
				ownerScope := r.scopeByPathKey(in.ParamOwnerPath)
				if ownerScope == nil || ownerScope.depth > lca.depth || !isAncestorScope(ownerScope, lca) {
					allInputsInLCA = false
					break
				}
			}
		}
		if !allInputsInLCA {
			continue
		}

		p0 := g.items[0].param
		actualType := p0.Type
		typeKey := p0.CanonicalTypeKey
		if p0.IsLazyCallableType() {
			actualType = ExtractCallableReturnType(p0.Type)
			typeKey = p0.CallableReturnTypeKey
		}

		idKey := buildProviderIdentity(lca.pathKey, p0.Generator.Func, typeKey, actualType, true, g.items[0].inputs)
		rp, exists := r.providersByID[idKey]
		if !exists {
			sp := &SharedProviderConfig{
				Name:                p0.Name,
				Type:                actualType,
				TypeImportPath:      p0.TypeImportPath,
				TypePackageName:     p0.TypePackageName,
				CanonicalTypeKey:    typeKey,
				TypeImports:         cloneTypeImports(p0.TypeImports),
				Func:                cloneFuncRef(p0.Generator.Func),
				Inputs:              append([]string(nil), p0.Generator.Inputs...),
				ReturnsError:        true,
				ResolvedInputs:      g.items[0].inputs,
				IdentityKey:         idKey,
				GeneratedByResolver: true,
			}
			rp = &resolvedProvider{
				sp:               sp,
				canonicalTypeKey: typeKey,
				firstOrder:       r.nextDeclOrder,
			}
			r.nextDeclOrder++
			r.providersByID[idKey] = rp
			r.providerOrder = append(r.providerOrder, idKey)
		}
		for _, item := range g.items {
			addDemander(rp, item.scope)
			applyProviderBindingToParam(item.param, rp.sp)
		}
	}
	return nil
}

func (r *depResolver) scopeByPathKey(key string) *depScope {
	for _, s := range r.allScopes {
		if s.pathKey == key {
			return s
		}
	}
	return nil
}

func (r *depResolver) placeResolvedProviders() {
	for _, idKey := range r.providerOrder {
		rp := r.providersByID[idKey]
		if rp == nil || !rp.sp.GeneratedByResolver {
			continue
		}
		host := rp.declHost
		if host == nil {
			if len(rp.demanders) == 0 {
				continue
			}
			host = lowestCommonAncestor(rp.demanders)
			if host == nil {
				host = r.root
			}
		}
		if host.isRoot {
			host.cmd.SharedProviders = append(host.cmd.SharedProviders, rp.sp)
		} else {
			host.sc.SharedProviders = append(host.sc.SharedProviders, rp.sp)
		}
	}
}

func (r *depResolver) verifyNoCyclesInPlacedProviders() error {
	state := make(map[string]uint8)
	var visit func(sp *SharedProviderConfig, chain []string) error
	visit = func(sp *SharedProviderConfig, chain []string) error {
		switch state[sp.IdentityKey] {
		case 1:
			return fmt.Errorf("dependency cycle detected involving %s", formatFuncRefName(sp.Func))
		case 2:
			return nil
		}
		state[sp.IdentityKey] = 1
		next := append(chain, formatFuncRefName(sp.Func))
		for _, in := range sp.ResolvedInputs {
			if in.Kind == DependencyInputProvider && in.Provider != nil {
				if err := visit(in.Provider, next); err != nil {
					return err
				}
			}
		}
		state[sp.IdentityKey] = 2
		return nil
	}
	for _, idKey := range r.providerOrder {
		rp := r.providersByID[idKey]
		if err := visit(rp.sp, nil); err != nil {
			return err
		}
	}
	return nil
}

func (r *depResolver) populateParamAccessors() {
	hostOfProvider := make(map[string]*depScope)
	for _, s := range r.allScopes {
		var sps []*SharedProviderConfig
		if s.isRoot {
			sps = s.cmd.SharedProviders
		} else {
			sps = s.sc.SharedProviders
		}
		for _, sp := range sps {
			if sp.IdentityKey != "" {
				hostOfProvider[sp.IdentityKey] = s
			}
		}
	}

	for _, idKey := range r.providerOrder {
		rp := r.providersByID[idKey]
		host := hostOfProvider[idKey]
		if host == nil {
			host = rp.declHost
		}
		if host == nil {
			continue
		}
		for i := range rp.sp.ResolvedInputs {
			if rp.sp.ResolvedInputs[i].Kind != DependencyInputParam {
				continue
			}
			owner := r.scopeByPathKey(rp.sp.ResolvedInputs[i].ParamOwnerPath)
			rp.sp.ResolvedInputs[i].ParamExpr = computeParamExpr(host, owner, rp.sp.ResolvedInputs[i].ParamName)
		}
	}

	for _, s := range r.allScopes {
		var steps []*ExecutionStep
		if s.isRoot {
			steps = s.cmd.ExecutionSteps
		} else {
			steps = s.sc.ExecutionSteps
		}
		for _, step := range steps {
			for i := range step.Inputs {
				if step.Inputs[i].Kind != DependencyInputParam {
					continue
				}
				owner := r.scopeByPathKey(step.Inputs[i].ParamOwnerPath)
				step.Inputs[i].ParamExpr = computeParamExpr(s, owner, step.Inputs[i].ParamName)
			}
		}
	}
}

func computeParamExpr(fromScope, ownerScope *depScope, paramName string) string {
	if fromScope == nil || ownerScope == nil || fromScope == ownerScope {
		return paramName
	}
	shadowed := false
	for curr := fromScope; curr != nil && curr != ownerScope; curr = curr.parent {
		for _, p := range curr.params {
			if !p.Inherited && p.Name == paramName {
				shadowed = true
				break
			}
		}
		if shadowed {
			break
		}
		var sps []*SharedProviderConfig
		if curr.isRoot {
			sps = curr.cmd.SharedProviders
		} else {
			sps = curr.sc.SharedProviders
		}
		for _, sp := range sps {
			if sp.ValueFieldName() == paramName || sp.MethodName() == paramName {
				shadowed = true
				break
			}
		}
		if shadowed {
			break
		}
	}
	if !shadowed {
		return paramName
	}

	var chain []string
	for curr := fromScope; curr != nil && curr != ownerScope; curr = curr.parent {
		if curr.parent != nil {
			chain = append(chain, curr.parent.structName)
		}
	}
	if len(chain) == 0 {
		return paramName
	}
	return strings.Join(chain, ".") + "." + paramName
}

func lowestCommonAncestor(scopes []*depScope) *depScope {
	if len(scopes) == 0 {
		return nil
	}
	prefix := scopes[0].ancestorsRootFirst()
	for _, s := range scopes[1:] {
		cand := s.ancestorsRootFirst()
		n := len(prefix)
		if len(cand) < n {
			n = len(cand)
		}
		cut := 0
		for cut < n && prefix[cut] == cand[cut] {
			cut++
		}
		prefix = prefix[:cut]
	}
	if len(prefix) == 0 {
		return nil
	}
	return prefix[len(prefix)-1]
}

func isAncestorScope(anc, desc *depScope) bool {
	for curr := desc; curr != nil; curr = curr.parent {
		if curr == anc {
			return true
		}
	}
	return false
}

func paramCanonicalType(p *FunctionParameter) string {
	if p == nil {
		return ""
	}
	if p.CanonicalTypeKey != "" {
		return p.CanonicalTypeKey
	}
	return p.Type
}

func buildGuardIdentity(ref *FuncRef, inputs []DependencyInput) string {
	return "guard|" + formatFuncRefIdentity(ref) + "|" + formatInputsIdentity(inputs)
}

func buildProviderIdentity(scopeKey string, ref *FuncRef, canonicalTypeKey, returnType string, returnsError bool, inputs []DependencyInput) string {
	typePart := canonicalTypeKey
	if typePart == "" {
		typePart = returnType
	}
	return fmt.Sprintf("provider|%s|%s|%s|%t|%s", scopeKey, formatFuncRefIdentity(ref), typePart, returnsError, formatInputsIdentity(inputs))
}

func formatFuncRefIdentity(ref *FuncRef) string {
	if ref == nil {
		return ""
	}
	return ref.ImportPath + "." + ref.FunctionName
}

func formatInputsIdentity(inputs []DependencyInput) string {
	if len(inputs) == 0 {
		return ""
	}
	parts := make([]string, len(inputs))
	for i, in := range inputs {
		if in.Kind == DependencyInputParam {
			parts[i] = "param:" + in.ParamOwnerPath + ":" + in.ParamName
		} else if in.Provider != nil {
			parts[i] = fmt.Sprintf("node:%s:lazy=%t", in.Provider.IdentityKey, in.LazyCallable)
		}
	}
	return strings.Join(parts, ",")
}

func formatCandidateNames(rps []*resolvedProvider) string {
	names := make([]string, len(rps))
	for i, rp := range rps {
		names[i] = formatFuncRefName(rp.sp.Func)
	}
	return strings.Join(names, ", ")
}

func cloneFuncRef(ref *FuncRef) *FuncRef {
	if ref == nil {
		return nil
	}
	cp := *ref
	return &cp
}

func cloneTypeImports(refs []TypeImportRef) []TypeImportRef {
	if len(refs) == 0 {
		return nil
	}
	cp := make([]TypeImportRef, len(refs))
	copy(cp, refs)
	return cp
}

func isBuiltinCanonicalType(canon string) bool {
	trimmed := strings.TrimPrefix(canon, "[]")
	trimmed = strings.TrimLeft(trimmed, "*")
	if isPredeclaredGoTypeName(trimmed) {
		return true
	}
	switch trimmed {
	case `"time".Duration`, `"io".Reader`, `"io".ReadCloser`, `"io".Writer`, `"io".WriteCloser`:
		return true
	default:
		return false
	}
}

func declErr(filename string, line int, scope *depScope, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if filename != "" && line > 0 {
		return fmt.Errorf("%s:%d: %s", filename, line, msg)
	}
	if scope != nil && scope.displayName != "" {
		return fmt.Errorf("command %s: %s", scope.displayName, msg)
	}
	return fmt.Errorf("%s", msg)
}
