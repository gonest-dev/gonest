package resolver

import (
	"context"
	"fmt"
	"reflect"

	"golang.org/x/sync/errgroup"

	"github.com/gonest-dev/gonest/internal/inject"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/scope"
)

// constructable is satisfied by *provider.Provider. It is declared here
// (not as a method on module.ProviderRef) so internal/module stays ignorant
// of constructor invocation -- Stage 3 is this package's concern alone.
// module.ProviderRef only needs to identify/compare providers and expose
// the type they resolve; actually running the constructor is an
// internal/resolver-only capability, reached via a type assertion on the
// module.ProviderRef values Find/BuildGraph already hand back.
type constructable interface {
	ConstructorFunc() reflect.Value
}

// scoped is satisfied by *provider.Provider. Declared here for the same
// reason constructable is: module.ProviderRef only needs identity/type
// information, not scope -- deciding HOW to resolve based on scope is
// Stage 3's concern alone.
type scoped interface {
	ResolvedScope() scope.Scope
}

// resolvedSetter is satisfied by *provider.Provider. Declared here for the
// same reason constructable/scoped are: module.ProviderRef only needs
// identity/type information, not a way to store the fully-resolved value --
// that is Stage 3's concern alone. Calling SetResolvedValue here is purely
// additive to the existing placeholder-copy-in-place mechanism below (see
// invokeAndCopy/invokeAndCopyEdge): it lets internal/resolver's direct.go
// (phase 2/3 callers, post-Stage-3) read an already-resolved provider's
// value once, without needing any placeholder indirection.
type resolvedSetter interface {
	SetResolvedValue(v reflect.Value)
}

// contextType matches internal/provider's own contextType constant, used
// here to decide whether a Constructor wants a context.Context as its sole
// argument.
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// Resolve performs Stage 3 (Parallel Resolution): given the fully-assembled
// module tree (the return value of the root Module's Assemble, i.e. Stage
// 1 output, after Stage 2 has run every Provider/Controller's Declare), it
// resolves every registered scope.Singleton provider concurrently via
// errgroup, respecting the dependency graph recorded by MustInject calls
// during Stage 2.
//
// Every provider registered in any of modules is resolved, not only ones
// reachable via a recorded pending edge -- design.md states NewApp resolves
// the WHOLE graph before returning, and Controllers (which are never graph
// nodes themselves, see BuildGraph) still need their target Providers
// resolved even though nothing in the Provider graph depends on them.
// BuildGraph's edges are used purely to determine start-after-dependency
// ordering for providers that DO have recorded MustInject edges between
// them; providers with no such edges start immediately.
//
// ctx is passed to errgroup.WithContext, so a Constructor returning an
// error or panicking cancels ctx, unblocking sibling goroutines waiting on
// ctx.Done() instead of leaving them to run to completion. The same ctx (or
// its errgroup-derived child) is what a func(context.Context) T/​(T, error)
// Constructor receives -- callers configure a bootstrap timeout by passing
// a ctx already wrapped with context.WithTimeout.
func Resolve(ctx context.Context, modules []*module.Module) error {
	nodes := allProviders(modules)
	if len(nodes) == 0 {
		return nil
	}

	graph := scopedGraph(nodes)
	if err := DetectCycle(graph); err != nil {
		return err
	}

	done := make(map[module.ProviderRef]chan struct{}, len(nodes))
	for _, n := range nodes {
		done[n] = make(chan struct{})
	}

	group, gctx := errgroup.WithContext(ctx)

	// waitDeps blocks until every one of node's recorded dependencies (per
	// the node-based dependency graph, which stays node-based regardless of
	// scope -- see isTransient's doc comment) has finished, or gctx is
	// cancelled by a sibling failure. It is shared by both the Singleton
	// path (one call per node) and the Transient path (one call per
	// targeting edge, since every one of a Transient node's re-instantiations
	// still needs the SAME dependency set ready first).
	waitDeps := func(n module.ProviderRef) error {
		for _, dep := range graph[n] {
			depDone, ok := done[dep]
			if !ok {
				// dep is not itself a node Resolve knows about (should not
				// happen for a well-formed graph built from allProviders +
				// BuildGraph, but fail safe rather than block forever).
				continue
			}
			select {
			case <-depDone:
			case <-gctx.Done():
				return gctx.Err()
			}
		}

		select {
		case <-gctx.Done():
			return gctx.Err()
		default:
		}

		return nil
	}

	for _, n := range nodes {
		n := n

		if isTransient(n) {
			// Transient scope: node-based dependency ORDERING still
			// applies (waitDeps below), but "how many times does this
			// node's Constructor actually run" becomes edge-count-aware --
			// each pending edge that resolves to n is its own independent
			// unit of work, with its own Constructor invocation and its
			// own placeholder, never shared with another edge. n's `done`
			// channel is closed immediately once its dependencies are
			// ready (not after any Constructor runs) since nothing in the
			// graph depends on "n's instance" for a Transient node -- only
			// on n's dependencies being ready, which downstream consumers
			// of OTHER nodes may wait on via graph[other] containing n.
			edges := edgesFor(n)

			group.Go(func() (err error) {
				defer close(done[n])
				return waitDeps(n)
			})

			for _, edge := range edges {
				edge := edge
				group.Go(func() error {
					select {
					case <-done[n]:
					case <-gctx.Done():
						return gctx.Err()
					}
					return invokeAndCopyEdge(gctx, n, edge)
				})
			}
			continue
		}

		group.Go(func() (err error) {
			defer close(done[n])

			if err := waitDeps(n); err != nil {
				return err
			}

			return invokeAndCopy(gctx, n)
		})
	}

	return group.Wait()
}

// isTransient reports whether node is configured with scope.Transient. Any
// module.ProviderRef that does not expose ResolvedScope (should not happen
// for a well-formed *provider.Provider) is treated as non-Transient, i.e.
// falls back to Singleton's one-execution-per-node semantics.
func isTransient(node module.ProviderRef) bool {
	s, ok := node.(scoped)
	if !ok {
		return false
	}
	return s.ResolvedScope() == scope.Transient
}

// scopedGraph builds the dependency graph for exactly nodes, filtering out
// any pending edge whose owner is not itself one of nodes. internal/inject
// tracks pending edges in process-global state (every MustInject call ever
// made, across every module tree ever assembled in this process) -- without
// this filter, BuildGraph's raw output would let an unrelated, previously
// assembled (but never resolved) module tree's pending edges leak into this
// Resolve call's graph and cycle detection, and two independent NewApp
// calls in the same process would interfere with each other.
func scopedGraph(nodes []module.ProviderRef) map[module.ProviderRef][]module.ProviderRef {
	inScope := make(map[module.ProviderRef]bool, len(nodes))
	for _, n := range nodes {
		inScope[n] = true
	}

	full := BuildGraph()
	scoped := make(map[module.ProviderRef][]module.ProviderRef, len(nodes))
	for key, deps := range full {
		if !inScope[key] {
			continue
		}
		var filteredDeps []module.ProviderRef
		for _, d := range deps {
			if inScope[d] {
				filteredDeps = append(filteredDeps, d)
			}
		}
		scoped[key] = filteredDeps
	}

	return scoped
}

// allProviders collects every module.ProviderRef registered (via
// Providers) across modules, deduplicated by identity. Order is stable
// (first-seen) but not otherwise significant -- Resolve's goroutines run
// concurrently regardless of this slice's order.
func allProviders(modules []*module.Module) []module.ProviderRef {
	seen := make(map[module.ProviderRef]bool)
	var out []module.ProviderRef

	for _, m := range modules {
		for _, p := range m.OwnProviders() {
			if seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}

	return out
}

// invokeAndCopy invokes node's Constructor exactly once (handling all 4
// accepted signatures), recovering any panic and converting it to an
// error, then copies the resolved instance in place into every placeholder
// any pending MustInject edge allocated for this exact provider. This is
// the Singleton path: one Constructor call, result shared across every
// edge targeting node.
func invokeAndCopy(ctx context.Context, node module.ProviderRef) (err error) {
	real, err := callConstructor(ctx, node)
	if err != nil {
		return err
	}

	if rs, ok := node.(resolvedSetter); ok {
		rs.SetResolvedValue(real)
	}

	for _, placeholder := range placeholdersFor(node) {
		placeholder.Elem().Set(real.Elem())
	}

	return nil
}

// invokeAndCopyEdge invokes node's Constructor once for this single edge
// (the Transient path: one independent Constructor call per pending edge
// targeting node), copying the result ONLY into edge's own placeholder --
// never shared with any other edge targeting the same node.
func invokeAndCopyEdge(ctx context.Context, node module.ProviderRef, edge inject.PendingEdge) error {
	real, err := callConstructor(ctx, node)
	if err != nil {
		return err
	}

	if rs, ok := node.(resolvedSetter); ok {
		rs.SetResolvedValue(real)
	}

	edge.Placeholder.Elem().Set(real.Elem())
	return nil
}

// callConstructor invokes node's Constructor (handling all 4 accepted
// signatures), recovering any panic and converting it to an error. Shared
// by both invokeAndCopy (Singleton, one call per node) and
// invokeAndCopyEdge (Transient, one call per edge) -- the actual mechanics
// of calling a Constructor and interpreting its return values do not
// depend on scope, only how many times it's called and where the result
// is copied does.
func callConstructor(ctx context.Context, node module.ProviderRef) (real reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("gonest: provider for type %s panicked during resolution: %v", node.ResolvedType(), r)
		}
	}()

	c, ok := node.(constructable)
	if !ok {
		return reflect.Value{}, fmt.Errorf("gonest: provider for type %s does not expose a Constructor", node.ResolvedType())
	}

	fn := c.ConstructorFunc()
	if !fn.IsValid() {
		return reflect.Value{}, fmt.Errorf("gonest: provider for type %s has no Constructor registered", node.ResolvedType())
	}

	var args []reflect.Value
	if fn.Type().NumIn() == 1 && fn.Type().In(0) == contextType {
		args = []reflect.Value{reflect.ValueOf(ctx)}
	}

	out := fn.Call(args)

	if len(out) == 2 && !out[1].IsNil() {
		return reflect.Value{}, out[1].Interface().(error)
	}

	return out[0], nil
}

// placeholdersFor returns every placeholder reflect.Value that a recorded
// MustInject pending edge allocated for a call that resolves to node
// (i.e. Find(edge.Owner.OwnerModule(), edge.TargetType) == node). A single
// provider can be depended upon by multiple owners (a Provider and one or
// more Controllers, or several Providers), each holding its own
// placeholder returned from its own MustInject call -- all of them must be
// copied into, not just the first.
//
// inject.PendingEdges() only ever contains edges from the module tree
// NewApp is currently bootstrapping: NewApp calls inject.Reset() at the
// start of every call (see inject.Reset's doc comment), so by the time
// Stage 3 runs there is no other tree's leftover state left to
// contaminate from. The Find(...) == node re-check below is therefore NOT
// a cross-tree contamination guard -- it is still load-bearing WITHIN a
// single tree: two different owners can request the same TargetType and
// legitimately resolve to two DIFFERENT providers (own-module providers
// take priority over imports, and diamond imports can shadow a type
// differently depending on the requesting module -- see
// TestFind_OwnModuleHasPriorityOverImports and
// TestFind_DiamondImport_DirectImporterResolvesSharedProvider in
// resolver_test.go), so edge.TargetType matching node.ResolvedType() alone
// is not sufficient to know this edge belongs to node.
func placeholdersFor(node module.ProviderRef) []reflect.Value {
	var out []reflect.Value

	for _, edge := range edgesFor(node) {
		out = append(out, edge.Placeholder)
	}

	return out
}

// edgesFor returns every recorded PendingEdge that resolves to node (i.e.
// Find(edge.Owner.OwnerModule(), edge.TargetType) == node), in registration
// order. Used by the Transient path to fan out one independent Constructor
// invocation per edge (see invokeAndCopyEdge) -- unlike placeholdersFor
// (which only needs the placeholder for the Singleton copy-in-place),
// Transient resolution needs the full PendingEdge since each edge gets its
// own Constructor call, not just its own placeholder write.
//
// See placeholdersFor's doc comment for why the Find(...) == node re-check
// is load-bearing within a single tree, not just a cross-tree contamination
// guard.
func edgesFor(node module.ProviderRef) []inject.PendingEdge {
	var out []inject.PendingEdge

	for _, edge := range inject.PendingEdges() {
		ownerModule := edge.Owner.OwnerModule()
		if ownerModule == nil {
			continue
		}
		if edge.TargetType != node.ResolvedType() {
			continue
		}
		if resolved := Find(ownerModule, edge.TargetType); resolved == node {
			out = append(out, edge)
		}
	}

	return out
}
