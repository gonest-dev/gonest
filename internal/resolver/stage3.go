package resolver

import (
	"context"
	"fmt"
	"reflect"

	"golang.org/x/sync/errgroup"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/resolve"
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

// contextType matches internal/provider's own contextType constant, used
// here to decide whether a Constructor wants a context.Context as its sole
// argument.
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// Resolve performs Stage 3 (Parallel Resolution): given the fully-assembled
// module tree (the return value of the root Module's Assemble, i.e. Stage
// 1 output, after Stage 2 has run every Provider/Controller's Declare), it
// resolves every registered scope.Singleton provider concurrently via
// errgroup, respecting the dependency graph recorded by MustResolve calls
// during Stage 2.
//
// Every provider registered in any of modules is resolved, not only ones
// reachable via a recorded pending edge -- design.md states NewApp resolves
// the WHOLE graph before returning, and Controllers (which are never graph
// nodes themselves, see BuildGraph) still need their target Providers
// resolved even though nothing in the Provider graph depends on them.
// BuildGraph's edges are used purely to determine start-after-dependency
// ordering for providers that DO have recorded MustResolve edges between
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

	for _, n := range nodes {
		n := n
		deps := graph[n]

		group.Go(func() (err error) {
			defer close(done[n])

			for _, dep := range deps {
				depDone, ok := done[dep]
				if !ok {
					// dep is not itself a node Resolve knows about (should
					// not happen for a well-formed graph built from
					// allProviders + BuildGraph, but fail safe rather than
					// block forever).
					continue
				}
				select {
				case <-depDone:
				case <-gctx.Done():
					return gctx.Err()
				}
			}

			// Also bail out early if ctx was already cancelled by a sibling
			// before this node's dependencies finished (covered above) or
			// if this node itself has no deps (covered here).
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}

			return invokeAndCopy(gctx, n)
		})
	}

	return group.Wait()
}

// scopedGraph builds the dependency graph for exactly nodes, filtering out
// any pending edge whose owner is not itself one of nodes. internal/resolve
// tracks pending edges in process-global state (every MustResolve call ever
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

// invokeAndCopy invokes node's Constructor (handling all 4 accepted
// signatures), recovering any panic and converting it to an error, then
// copies the resolved instance in place into every placeholder any pending
// MustResolve edge allocated for this exact provider.
func invokeAndCopy(ctx context.Context, node module.ProviderRef) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("gonest: provider for type %s panicked during resolution: %v", node.ResolvedType(), r)
		}
	}()

	c, ok := node.(constructable)
	if !ok {
		return fmt.Errorf("gonest: provider for type %s does not expose a Constructor", node.ResolvedType())
	}

	fn := c.ConstructorFunc()
	if !fn.IsValid() {
		return fmt.Errorf("gonest: provider for type %s has no Constructor registered", node.ResolvedType())
	}

	var args []reflect.Value
	if fn.Type().NumIn() == 1 && fn.Type().In(0) == contextType {
		args = []reflect.Value{reflect.ValueOf(ctx)}
	}

	out := fn.Call(args)

	if len(out) == 2 && !out[1].IsNil() {
		return out[1].Interface().(error)
	}

	real := out[0]

	for _, placeholder := range placeholdersFor(node) {
		placeholder.Elem().Set(real.Elem())
	}

	return nil
}

// placeholdersFor returns every placeholder reflect.Value that a recorded
// MustResolve pending edge allocated for a call that resolves to node
// (i.e. Find(edge.Owner.OwnerModule(), edge.TargetType) == node). A single
// provider can be depended upon by multiple owners (a Provider and one or
// more Controllers, or several Providers), each holding its own
// placeholder returned from its own MustResolve call -- all of them must be
// copied into, not just the first.
func placeholdersFor(node module.ProviderRef) []reflect.Value {
	var out []reflect.Value

	for _, edge := range resolve.PendingEdges() {
		ownerModule := edge.Owner.OwnerModule()
		if ownerModule == nil {
			continue
		}
		if edge.TargetType != node.ResolvedType() {
			continue
		}
		if resolved := Find(ownerModule, edge.TargetType); resolved == node {
			out = append(out, edge.Placeholder)
		}
	}

	return out
}
