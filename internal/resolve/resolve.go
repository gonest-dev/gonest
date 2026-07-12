// Package resolve implements MustResolve[T], the generic entry point used
// inside a Provider's or Controller's deferred builder fn to declare a
// dependency on another provider's type. During Stage 2 builder execution
// (a future task) it will perform the real module-scoped search; for now it
// allocates a placeholder via reflect and records the call as a pending
// edge (owner -> target type) for a future task (T7) to consult.
package resolve

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/gonest-dev/gonest/internal/module"
)

// PendingEdge records that Owner requested resolution of TargetType via
// MustResolve. internal/resolver walks this bookkeeping (via PendingEdges)
// to perform the real module-scoped search and wire dependency edges into
// the DI graph, including cycle detection.
//
// Placeholder retains the exact reflect.Value MustResolve allocated and
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

// resetPendingEdges clears all recorded pending edges. Test-only helper
// (used by _test.go files in this package to isolate test cases from each
// other's bookkeeping).
func resetPendingEdges() {
	pendingEdgesMu.Lock()
	defer pendingEdgesMu.Unlock()
	pendingEdges = nil
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

// MustResolve declares a dependency on type T (which must be a pointer
// type, e.g. *Foo) from owner's builder fn. It allocates and returns a
// placeholder value via reflect, and records a pending edge (owner -> T)
// for a future task to consult when performing real module-scoped
// resolution. It panics if T is not a pointer type.
func MustResolve[T any](owner module.Owner) T {
	t := reflect.TypeFor[T]()

	if t.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("gonest: MustResolve[T] requires T to be a pointer type, got %s", t.String()))
	}

	placeholder := reflect.New(t.Elem())

	pendingEdgesMu.Lock()
	pendingEdges = append(pendingEdges, PendingEdge{Owner: owner, TargetType: t, Placeholder: placeholder})
	pendingEdgesMu.Unlock()

	return placeholder.Interface().(T)
}
