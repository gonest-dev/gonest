package resolver

import (
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
)

// fakeControllerOwner is a minimal module.Owner stand-in for
// *controller.Controller -- this package cannot import internal/controller
// directly (it imports internal/filter, which imports internal/resolver for
// FindDirect/FindDirectAll, so importing controller back here would cycle
// within this test binary). It deliberately does NOT implement
// internal/inject's directResolver (no ResolveDirect/ResolveDirectAll), so
// MustInject calls against it still fall through to the OLD
// placeholder+PendingEdge path below -- exactly what this file's
// TestBuildGraph_ExcludesControllerOwnedEdges needs to prove (a Controller's
// MustInject call must not become a graph node/edge).
type fakeControllerOwner struct {
	owner *module.Module
}

func (f *fakeControllerOwner) OwnerModule() *module.Module { return f.owner }

// resetForGraphTest clears inject's pending edges before a test builds
// fresh bookkeeping, so tests don't see edges recorded by other tests
// running earlier in the same process.
func resetForGraphTest(t *testing.T) {
	t.Helper()
	// internal/inject does not export a reset -- use MustInject's own
	// package-level test helper indirectly by draining via PendingEdges
	// is not possible (no exported clear). Instead, each test uses fresh,
	// unique target types so pre-existing edges from other tests never
	// collide with what this test asserts on.
	_ = t
}

// aProvider/bProvider/cProvider are minimal fakeProvider-backed nodes with
// distinct resolved types, used to build a small dependency graph via
// MustInject calls recorded as pending edges.
type graphAService struct{}
type graphBService struct{}

func TestBuildGraph_SingleDependencyEdge(t *testing.T) {
	resetForGraphTest(t)

	b := &fakeProvider{resolved: reflect.TypeOf(&graphBService{})}
	a := &fakeProvider{resolved: reflect.TypeOf(&graphAService{})}

	m := module.New(func(m *module.Module) {
		m.Providers(a, b)
	})
	m.Assemble()

	// Simulate Stage 2: A's builder fn calls MustInject[*graphBService],
	// recording a pending edge {owner: a, targetType: *graphBService}.
	inject.Must[*graphBService](a)

	graph := BuildGraph()

	deps := graph[module.ProviderRef(a)]
	if len(deps) != 1 || deps[0] != module.ProviderRef(b) {
		t.Fatalf("graph[a] = %v, want [b]", deps)
	}
}

func TestBuildGraph_ExcludesControllerOwnedEdges(t *testing.T) {
	resetForGraphTest(t)

	type controllerOnlyService struct{}
	p := &fakeProvider{resolved: reflect.TypeOf(&controllerOnlyService{})}

	m := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	m.Assemble()

	ctrl := &fakeControllerOwner{owner: m}

	// A Controller's MustInject call must NOT become a graph node/edge --
	// controllers are never a dependency-graph key (design.md: "Controller
	// não entra no grafo de resolução").
	inject.Must[*controllerOnlyService](ctrl)

	graph := BuildGraph()

	// A Controller is not itself a module.ProviderRef, so it can never be
	// a key in the graph -- the graph's key type is module.ProviderRef.
	// The real assertion is that building the graph does not panic/fail
	// when a pending edge's owner isn't a ProviderRef, and produces no
	// spurious node for the provider the controller resolved (p is a
	// dependency TARGET here, never a dependency SOURCE, since nothing
	// depends on it).
	if deps, ok := graph[module.ProviderRef(p)]; ok && len(deps) != 0 {
		t.Fatalf("graph[p] = %v, want empty (p has no MustInject calls of its own; only ctrl, a non-ProviderRef, resolved it)", deps)
	}
}

// graphIfaceService is the interface type used by
// TestBuildGraph_IncludesPendingAllEdges_AlongsidePendingEdges to exercise
// inject.MustAll's Provider-owned dispatch (mustAllProvider/findAllRefs),
// which requires T to be an interface kind and matches candidates by EXACT
// ResolvedType() equality (AD-053) -- so the fake matching providers below
// set resolved directly to this interface's reflect.Type, no ProviderAs
// wrapping needed for this unit test's purposes.
type graphIfaceService interface {
	DoStuff()
}

func TestBuildGraph_IncludesPendingAllEdges_AlongsidePendingEdges(t *testing.T) {
	resetForGraphTest(t)

	ifaceType := reflect.TypeOf((*graphIfaceService)(nil)).Elem()

	a := &fakeProvider{resolved: reflect.TypeOf(&graphAService{})}
	b := &fakeProvider{resolved: reflect.TypeOf(&graphBService{})}
	m1 := &fakeProvider{resolved: ifaceType}
	m2 := &fakeProvider{resolved: ifaceType}

	m := module.New(func(m *module.Module) {
		m.Providers(a, b, m1, m2)
	})
	m.Assemble()

	// Simulate Stage 2: A's builder fn calls both MustInject[*graphBService]
	// (a PendingEdge, single pointer target) and
	// MustInjectAll[graphIfaceService] (a PendingAllEdge, matching m1 and
	// m2) -- BuildGraph must expand BOTH kinds of pending bookkeeping into
	// edges from the SAME owner (a), coexisting in the same graph[a] entry.
	inject.Must[*graphBService](a)
	inject.MustAll[graphIfaceService](a)

	graph := BuildGraph()

	deps := graph[module.ProviderRef(a)]
	if len(deps) != 3 {
		t.Fatalf("graph[a] = %v, want 3 entries (1 PendingEdge target + 2 PendingAllEdge matches)", deps)
	}

	want := map[module.ProviderRef]bool{
		module.ProviderRef(b):  true,
		module.ProviderRef(m1): true,
		module.ProviderRef(m2): true,
	}
	for _, dep := range deps {
		if !want[dep] {
			t.Fatalf("graph[a] contains unexpected dep %v, want only b/m1/m2", dep)
		}
		delete(want, dep)
	}
	if len(want) != 0 {
		t.Fatalf("graph[a] = %v, missing expected deps %v", deps, want)
	}
}

func TestBuildGraph_NodeWithNoDependenciesHasEmptyList(t *testing.T) {
	resetForGraphTest(t)

	type standaloneService struct{}
	p := &fakeProvider{resolved: reflect.TypeOf(&standaloneService{})}

	m := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	m.Assemble()

	graph := BuildGraph()

	if deps, ok := graph[module.ProviderRef(p)]; ok && len(deps) != 0 {
		t.Fatalf("graph[p] = %v, want empty or absent (p declared no MustInject calls)", deps)
	}
}
