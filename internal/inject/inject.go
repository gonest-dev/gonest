// Package inject implements MustInject[T], the generic entry point used
// inside a Provider's or Controller's deferred builder fn to declare a
// dependency on another provider's type. During Stage 2 builder execution
// (a future task) it will perform the real module-scoped search; for now it
// allocates a placeholder via reflect and records the call as a pending
// edge (owner -> target type) for a future task (T7) to consult.
package inject

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/scope"
)

// PendingEdge records that Owner requested resolution of TargetType via
// MustInject. internal/resolver walks this bookkeeping (via PendingEdges)
// to perform the real module-scoped search and wire dependency edges into
// the DI graph, including cycle detection.
//
// Placeholder retains the exact reflect.Value MustInject allocated and
// returned to the caller (T in reflect.New(t.Elem()) form, i.e. the pointer
// itself, not "the struct it points to"). Stage 3 resolution needs this to
// copy the real resolved instance into the placeholder in place
// (*placeholder = *real via reflect) once the target provider's Constructor
// has run -- without retaining the placeholder here, that copy-in-place
// would have no way back from "a pending edge exists" to "here is the exact
// pointer callers are already holding a reference to".
type PendingEdge struct {
	Owner       module.Owner
	TargetType  reflect.Type
	Placeholder reflect.Value
}

var (
	pendingEdgesMu sync.Mutex
	pendingEdges   []PendingEdge
)

// lazyResolved records, for the CURRENT bootstrap only, every
// module.ProviderRef that mustLazy already eagerly constructed via
// Module.Lazy's MustInject[T](l) dispatch branch (see mustLazy below),
// keyed by the ref's own identity, alongside the exact reflect.Value it
// produced.
//
// SPEC_DEVIATION (module-lazy-loading feature, T3): design.md's Stage 3
// skip-if-already-resolved check was specified to read
// *provider.Provider's OWN ResolvedValue()/SetResolvedValue() -- reasoning
// "before this feature, ResolvedValue() could only ever return ok=false at
// this point in Stage 3 (nothing sets it earlier)". That is false once
// touched against the real test suite: invokeAndCopy (internal/resolver/
// stage3.go) ALREADY calls SetResolvedValue unconditionally on every
// successful resolution (pre-existing, for FindDirect/FindDirectAll's
// post-Stage-3 reads), and that value is NEVER cleared afterward -- a
// package-level *provider.Provider var reused across multiple separate
// NewApp calls in the same process (a long-established, explicitly
// documented-safe pattern in this codebase, see inject.Reset's own doc
// comment and TestNewApp_TwoSequentialUnrelatedCalls_DoNotLeakPendingEdges)
// would therefore short-circuit on its 2nd+ NewApp call, reusing a STALE
// value from a previous bootstrap instead of constructing fresh -- observed
// as a real regression across internal/app's existing test suite
// (UserProvider-sharing tests) once Stage 3 started consulting
// ResolvedValue() directly.
//
// The smallest safe fix: track "eagerly resolved via Lazy, THIS bootstrap
// only" separately, in this package (already scoped per-bootstrap via
// Reset(), the same contract pendingEdges/globalSingletons already rely
// on), instead of conflating it with Provider's own permanent
// post-resolution cache. Stage 3 consults LazyResolvedValue (below)
// instead of node's own ResolvedValue().
var (
	lazyResolvedMu sync.Mutex
	lazyResolved   map[module.ProviderRef]reflect.Value
)

// LazyResolvedValue returns the value mustLazy already eagerly constructed
// for ref during THIS bootstrap's Module.Lazy callback(s), and whether one
// exists. Exported so internal/resolver's Stage 3 (invokeAndCopy) can skip
// re-invoking ref's Constructor -- see lazyResolved's own doc comment for
// why this is a separate, bootstrap-scoped registry rather than Provider's
// own ResolvedValue().
func LazyResolvedValue(ref module.ProviderRef) (reflect.Value, bool) {
	lazyResolvedMu.Lock()
	defer lazyResolvedMu.Unlock()
	v, ok := lazyResolved[ref]
	return v, ok
}

// globalSingletons holds framework-provided singletons (today: only
// *emitter.Emitter, see internal/emitter) that MUST resolve via
// MustInject[T] from ANY owner, in ANY module, with no explicit
// registration anywhere -- unlike every other MustInject target, which
// requires a real Provider (or, for Guard/Middleware/Interceptor/Filter's
// own union-scoped search, a provider visible in scope). Declared here
// (not in internal/emitter) so this stays a GENERIC mechanism, reusable by
// any future framework-provided singleton (Scheduler/HealthCheck are
// candidates -- see ROADMAP.md's Milestones 10/11), rather than emitter-
// specific plumbing baked into MustInject itself.
var (
	globalSingletonsMu sync.Mutex
	globalSingletons   map[reflect.Type]reflect.Value
)

// RegisterGlobalSingleton registers v (of type t) as a framework singleton
// resolvable via MustInject[T] from any owner, any module, with no
// explicit Provider registration. Called once per bootstrap (by
// internal/app's NewApp/MustNewTestApp, right after Reset()) for each
// framework-provided singleton type.
func RegisterGlobalSingleton(t reflect.Type, v reflect.Value) {
	globalSingletonsMu.Lock()
	defer globalSingletonsMu.Unlock()
	if globalSingletons == nil {
		globalSingletons = make(map[reflect.Type]reflect.Value)
	}
	globalSingletons[t] = v
}

// GlobalSingletonFor returns the value registered via RegisterGlobalSingleton
// for t, and whether one exists. Exported so internal/emitter's
// Listener.Declare can look up the current bootstrap's Emitter singleton
// directly, the same instance MustInject[*Emitter] would resolve.
func GlobalSingletonFor(t reflect.Type) (reflect.Value, bool) {
	globalSingletonsMu.Lock()
	defer globalSingletonsMu.Unlock()
	v, ok := globalSingletons[t]
	return v, ok
}

// PendingEdges returns a copy of all pending edges recorded so far, in
// registration order. Read-only: mutating the returned slice does not
// affect this package's internal state. Used by internal/resolver to build
// the provider dependency graph (combined with resolver.Find).
func PendingEdges() []PendingEdge {
	pendingEdgesMu.Lock()
	defer pendingEdgesMu.Unlock()
	return append([]PendingEdge(nil), pendingEdges...)
}

// Reset clears all recorded pending edges. Exported so root NewApp/MustNewApp
// can call it at the very start of every bootstrap, before Stage 2
// (declareAll) runs -- pendingEdges is process-global state, so without this
// reset it would accumulate every MustInject call ever made across every
// NewApp call in the process lifetime (unbounded memory growth, and stale
// edges from a previous bootstrap leaking into the next one's cycle
// detection / placeholder resolution).
//
// Calling Reset() establishes a "one bootstrap at a time per process"
// contract: NewApp is meant to run once, synchronously, at process startup
// (see design.md) -- it is not safe to call NewApp concurrently from
// multiple goroutines in the same process, since a second call's Reset()
// (or its Stage 2 MustInject calls) would race with and corrupt a
// concurrently in-flight first call's pending-edge bookkeeping. Sequential
// calls (e.g. one NewApp finishing fully before another starts, as in a test
// suite) are safe -- see TestReset_ClearsAllPendingEdges and
// TestNewApp_TwoSequentialUnrelatedCalls_DoNotLeakPendingEdges.
func Reset() {
	pendingEdgesMu.Lock()
	pendingEdges = nil
	pendingEdgesMu.Unlock()

	globalSingletonsMu.Lock()
	globalSingletons = nil
	globalSingletonsMu.Unlock()

	lazyResolvedMu.Lock()
	lazyResolved = nil
	lazyResolvedMu.Unlock()
}

// resetPendingEdges is Reset, kept under its original unexported name for
// this package's own tests (predates Reset's export -- avoids a mechanical
// rename churn across every _test.go call site in this file).
func resetPendingEdges() {
	Reset()
}

// pendingEdgesFor returns the pending edges recorded for owner, in
// registration order. Test-only helper.
func pendingEdgesFor(owner module.Owner) []PendingEdge {
	pendingEdgesMu.Lock()
	defer pendingEdgesMu.Unlock()

	var out []PendingEdge
	for _, e := range pendingEdges {
		if e.Owner == owner {
			out = append(out, e)
		}
	}
	return out
}

// declarable is satisfied by *provider.Provider. Declared here, unexported,
// same cross-package rationale as internal/resolver/stage3.go's/internal/
// app's own private mirrors of *provider.Provider's methods -- this
// package cannot import internal/provider (provider already imports
// module, and internal/inject already imports module too; importing
// provider from here would additionally require provider to expose
// exactly the methods below, achievable structurally without a real
// import). Used by the Lazy dispatch branch below to run Declare on every
// own-module provider so ResolvedType() becomes available for matching.
type declarable interface {
	Declare()
}

// constructable is satisfied by *provider.Provider, mirroring internal/
// resolver/stage3.go's own private interface of the same name/shape (see
// its doc comment for the full cross-package rationale). Used by the Lazy
// dispatch branch below to invoke a matched provider's Constructor
// directly.
type constructable interface {
	ConstructorFunc() reflect.Value
}

// lazyScoped mirrors internal/resolver/stage3.go's own private `scoped`
// interface (renamed here only to avoid colliding with this package's own
// unrelated identifiers) -- same *provider.Provider method, same
// cross-package rationale.
type lazyScoped interface {
	ResolvedScope() scope.Scope
}

// lazyResolvedSetter mirrors internal/resolver/stage3.go's own private
// resolvedSetter interface -- same *provider.Provider method
// (SetResolvedValue), same cross-package rationale. Named with a lazy
// prefix only to avoid any future collision with this package's own
// identifiers. Calling SetResolvedValue here is purely additive to
// Provider's existing post-resolution cache (used by internal/resolver's
// FindDirect/FindDirectAll, post-Stage-3 reads) -- LAZY-08/LAZY-03's own
// "already resolved, reuse it" short-circuits are checked against
// lazyResolved (this package's own bootstrap-scoped bookkeeping) instead,
// see lazyResolved's own doc comment for why.
type lazyResolvedSetter interface {
	SetResolvedValue(v reflect.Value)
}

// lazyContextType mirrors internal/provider's own contextType constant
// (and internal/resolver/stage3.go's own copy of the same), used to decide
// whether a Constructor wants a context.Context as its sole argument.
// Duplicated rather than imported -- see design.md's Tech Decisions
// (internal/inject cannot import internal/provider or internal/resolver
// without creating an import cycle).
var lazyContextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// mustLazy implements Must[T]'s 3rd dispatch branch: owner is a
// *module.LazyModule (see .specs/features/module-lazy-loading/design.md's
// Components section, "inject.Must[T]'s Lazy dispatch branch", for the
// full 10-step algorithm this mirrors exactly).
//
// T must be a pointer type -- Lazy injection is scoped to concrete
// config-like values, the same pointer-only rule the existing
// Provider-to-Provider path below already enforces, not the interface
// path directResolver above handles.
//
// Only providers registered on lm's OWN module, via Providers earlier in
// the same Lazy callback's enclosing fn, are visible -- see LazyModule's
// own doc comment for why imports are never searched (they are not
// finalized yet, Lazy is literally what decides them).
func mustLazy[T any](lm *module.LazyModule, t reflect.Type) T {
	if t.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("gonest: MustInject[T](l) requires T to be a pointer type, got %s", t.String()))
	}

	var match module.ProviderRef
	for _, p := range lm.OwnProviders() {
		if d, ok := p.(declarable); ok {
			d.Declare()
		}
		if p.ResolvedType() == t {
			match = p
		}
	}

	if match == nil {
		panic(fmt.Sprintf("gonest: MustInject[T](l) found no provider for type %s registered via this module's own Providers(...) earlier in the same fn -- Lazy only sees this module's own providers", t.String()))
	}

	// LAZY-08's repeated-call short-circuit is checked against lazyResolved
	// (this bootstrap only), NOT match's own ResolvedValue() -- see
	// lazyResolved's own doc comment (SPEC_DEVIATION) for why a permanent,
	// never-cleared cache on *provider.Provider itself would incorrectly
	// short-circuit a LATER, separate bootstrap reusing the same
	// package-level Provider var.
	if v, ok := LazyResolvedValue(match); ok {
		return v.Interface().(T)
	}

	if sc, ok := match.(lazyScoped); ok && sc.ResolvedScope() != scope.Singleton {
		panic(fmt.Sprintf("gonest: MustInject[T](l) requires a ScopeSingleton provider, type %s is scoped %v", t.String(), sc.ResolvedScope()))
	}

	c, ok := match.(constructable)
	if !ok {
		panic(fmt.Sprintf("gonest: MustInject[T](l) matched provider for type %s does not expose a Constructor", t.String()))
	}
	fn := c.ConstructorFunc()
	if !fn.IsValid() {
		panic(fmt.Sprintf("gonest: MustInject[T](l) matched provider for type %s has no Constructor registered", t.String()))
	}

	before := len(PendingEdges())

	var args []reflect.Value
	if fn.Type().NumIn() == 1 && fn.Type().In(0) == lazyContextType {
		args = []reflect.Value{reflect.ValueOf(context.Background())}
	}
	out := fn.Call(args)

	var real reflect.Value
	if len(out) == 2 {
		if errVal := out[1]; !errVal.IsNil() {
			panic(fmt.Sprintf("gonest: MustInject[T](l) matched provider for type %s returned an error during eager Lazy construction: %v", t.String(), errVal.Interface()))
		}
	}
	real = out[0]

	if len(PendingEdges()) != before {
		panic(fmt.Sprintf("gonest: MustInject[T](l) matched provider for type %s recorded a new dependency during eager construction -- Lazy only supports self-contained providers with no MustInject calls of their own", t.String()))
	}

	if rs, ok := match.(lazyResolvedSetter); ok {
		rs.SetResolvedValue(real)
	}

	lazyResolvedMu.Lock()
	if lazyResolved == nil {
		lazyResolved = make(map[module.ProviderRef]reflect.Value)
	}
	lazyResolved[match] = real
	lazyResolvedMu.Unlock()

	return real.Interface().(T)
}

// directResolver is satisfied by *controller.Controller (once Provider
// phase/phase 1 has completed) and by *middleware.Middleware/*guard.Guard/
// *interceptor.Interceptor/*filter.Filter (once Controller phase/phase 2 has
// completed) -- see .specs/features/test-app-bootstrap/design.md's
// Architecture Overview. Declared here, unexported: owner (a module.Owner
// value) satisfying this interface is what lets MustInject/MustInjectAll
// dispatch to DIRECT resolution (no placeholder, no PendingEdge, the real
// value already exists by the time these types' builder closures run)
// instead of the OLD placeholder+PendingEdge path below, which remains
// exclusively how *provider.Provider (Provider-to-Provider dependencies)
// resolves -- Provider deliberately does NOT implement this interface.
type directResolver interface {
	ResolveDirect(t reflect.Type) (reflect.Value, bool)
	ResolveDirectAll(t reflect.Type) []reflect.Value
}

// Must declares a dependency on type T from owner's builder fn.
// owner is typed any (not module.Owner) specifically because
// Middleware/Guard/Interceptor/Filter values have no single owning Module
// (their ownership is the UNION of every referencing module, discovered
// only after Controller phase completes -- see
// .specs/features/test-app-bootstrap/context.md's Decision 4) and so
// cannot implement module.Owner's OwnerModule() at all; only their
// directResolver capability matters for dispatch here.
//
// If owner satisfies directResolver (a Controller, Middleware, Guard,
// Interceptor, or Filter -- see directResolver's doc comment), resolution
// happens DIRECTLY, right now: for an interface T, every provider in
// owner's scope implementing T is found via ResolveDirectAll, panicking on
// zero or 2+ matches (ambiguous -- use MustInjectAll); for a pointer T,
// ResolveDirect performs a single exact-match lookup, panicking if not
// found. No placeholder, no PendingEdge bookkeeping for either case.
//
// Otherwise (owner is a *provider.Provider, Provider-to-Provider
// dependency -- the only remaining caller, which DOES implement
// module.Owner), behavior is UNCHANGED from before this dispatch existed: T
// must be a pointer type (panics otherwise), a placeholder is allocated via
// reflect and returned immediately, and a PendingEdge is recorded for
// internal/resolver's Stage 3 (topological/errgroup/cycle-detecting) to
// resolve and copy-in-place later.
func Must[T any](owner any) T {
	t := reflect.TypeFor[T]()

	if v, ok := GlobalSingletonFor(t); ok {
		return v.Interface().(T)
	}

	if lm, ok := owner.(*module.LazyModule); ok {
		return mustLazy[T](lm, t)
	}

	if dr, ok := owner.(directResolver); ok {
		if t.Kind() == reflect.Interface {
			matches := dr.ResolveDirectAll(t)
			switch len(matches) {
			case 0:
				panic(fmt.Sprintf("gonest: no provider implements interface %s", t.String()))
			case 1:
				return matches[0].Interface().(T)
			default:
				panic(fmt.Sprintf("gonest: ambiguous: %d providers implement interface %s, use MustInjectAll", len(matches), t.String()))
			}
		}

		v, ok := dr.ResolveDirect(t)
		if !ok {
			panic(fmt.Sprintf("gonest: no provider registered for type %s", t.String()))
		}
		return v.Interface().(T)
	}

	if t.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("gonest: MustInject[T] requires T to be a pointer type, got %s", t.String()))
	}

	moOwner, ok := owner.(module.Owner)
	if !ok {
		panic("gonest: MustInject[T] requires owner to be a *provider.Provider when T is a pointer type and owner is not a Controller/Middleware/Guard/Interceptor/Filter")
	}

	placeholder := reflect.New(t.Elem())

	pendingEdgesMu.Lock()
	pendingEdges = append(pendingEdges, PendingEdge{Owner: moOwner, TargetType: t, Placeholder: placeholder})
	pendingEdgesMu.Unlock()

	return placeholder.Interface().(T)
}

// MustAll returns every provider in owner's scope whose resolved
// value satisfies interface T, as []T. T must be an interface kind (panics
// otherwise -- multi-binding only makes sense for interfaces, a pointer
// type has no "multiple implementations" concept, see MustInject's
// single-match contract for that case). owner must satisfy directResolver
// (panics otherwise -- Provider never supports multi-binding, matching
// Provider-to-Provider dependencies staying single-value only). Returns an
// empty slice, never panics, if zero providers match -- "give me all of
// them" reasonably tolerates "there are none", unlike MustInject[T]'s
// single-match contract.
func MustAll[T any](owner any) []T {
	t := reflect.TypeFor[T]()

	if t.Kind() != reflect.Interface {
		panic(fmt.Sprintf("gonest: MustInjectAll[T] requires T to be an interface type, got %s", t.String()))
	}

	dr, ok := owner.(directResolver)
	if !ok {
		panic("gonest: MustInjectAll[T] is not supported from this owner (Provider-to-Provider dependencies stay single-value via MustInject)")
	}

	matches := dr.ResolveDirectAll(t)
	out := make([]T, len(matches))
	for i, v := range matches {
		out[i] = v.Interface().(T)
	}
	return out
}
