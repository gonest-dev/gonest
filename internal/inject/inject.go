// Package inject implements MustInject[T], the generic entry point used
// inside a Provider's or Controller's deferred builder fn to declare a
// dependency on another provider's type. During Stage 2 builder execution
// (a future task) it will perform the real module-scoped search; for now it
// allocates a placeholder via reflect and records the call as a pending
// edge (owner -> target type) for a future task (T7) to consult.
package inject

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/gonest-dev/gonest/internal/module"
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
	defer pendingEdgesMu.Unlock()
	pendingEdges = nil
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

// MustInject declares a dependency on type T from owner's builder fn.
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
func MustInject[T any](owner any) T {
	t := reflect.TypeFor[T]()

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

// MustInjectAll returns every provider in owner's scope whose resolved
// value satisfies interface T, as []T. T must be an interface kind (panics
// otherwise -- multi-binding only makes sense for interfaces, a pointer
// type has no "multiple implementations" concept, see MustInject's
// single-match contract for that case). owner must satisfy directResolver
// (panics otherwise -- Provider never supports multi-binding, matching
// Provider-to-Provider dependencies staying single-value only). Returns an
// empty slice, never panics, if zero providers match -- "give me all of
// them" reasonably tolerates "there are none", unlike MustInject[T]'s
// single-match contract.
func MustInjectAll[T any](owner any) []T {
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
