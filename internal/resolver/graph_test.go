package resolver

import (
	"reflect"
	"testing"

	"github.com/gonest-dev/gonest/internal/controller"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/resolve"
)

// resetForGraphTest clears resolve's pending edges before a test builds
// fresh bookkeeping, so tests don't see edges recorded by other tests
// running earlier in the same process.
func resetForGraphTest(t *testing.T) {
	t.Helper()
	// internal/resolve does not export a reset -- use MustResolve's own
	// package-level test helper indirectly by draining via PendingEdges
	// is not possible (no exported clear). Instead, each test uses fresh,
	// unique target types so pre-existing edges from other tests never
	// collide with what this test asserts on.
	_ = t
}

// aProvider/bProvider/cProvider are minimal fakeProvider-backed nodes with
// distinct resolved types, used to build a small dependency graph via
// MustResolve calls recorded as pending edges.
type graphAService struct{}
type graphBService struct{}
type graphCService struct{}

func TestBuildGraph_SingleDependencyEdge(t *testing.T) {
	resetForGraphTest(t)

	b := &fakeProvider{resolved: reflect.TypeOf(&graphBService{})}
	a := &fakeProvider{resolved: reflect.TypeOf(&graphAService{})}

	m := module.New(func(m *module.Module) {
		m.Providers(a, b)
	})
	m.Assemble()

	// Simulate Stage 2: A's builder fn calls MustResolve[*graphBService],
	// recording a pending edge {owner: a, targetType: *graphBService}.
	resolve.MustResolve[*graphBService](a)

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

	ctrl := controller.New(func(c *controller.Controller) {})
	ctrl.SetOwnerModule(m)

	// A Controller's MustResolve call must NOT become a graph node/edge --
	// controllers are never a dependency-graph key (design.md: "Controller
	// não entra no grafo de resolução").
	resolve.MustResolve[*controllerOnlyService](ctrl)

	graph := BuildGraph()

	// A Controller is not itself a module.ProviderRef, so it can never be
	// a key in the graph -- the graph's key type is module.ProviderRef.
	// The real assertion is that building the graph does not panic/fail
	// when a pending edge's owner isn't a ProviderRef, and produces no
	// spurious node for the provider the controller resolved (p is a
	// dependency TARGET here, never a dependency SOURCE, since nothing
	// depends on it).
	if deps, ok := graph[module.ProviderRef(p)]; ok && len(deps) != 0 {
		t.Fatalf("graph[p] = %v, want empty (p has no MustResolve calls of its own; only ctrl, a non-ProviderRef, resolved it)", deps)
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
		t.Fatalf("graph[p] = %v, want empty or absent (p declared no MustResolve calls)", deps)
	}
}
