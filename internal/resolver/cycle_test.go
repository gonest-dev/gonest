package resolver

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gonest-dev/gonest/internal/module"
)

type cycleAService struct{}
type cycleBService struct{}
type cycleCService struct{}

func TestDetectCycle_NoCycle_ReturnsNil(t *testing.T) {
	a := &fakeProvider{resolved: reflect.TypeOf(&cycleAService{})}
	b := &fakeProvider{resolved: reflect.TypeOf(&cycleBService{})}
	c := &fakeProvider{resolved: reflect.TypeOf(&cycleCService{})}

	graph := map[module.ProviderRef][]module.ProviderRef{
		module.ProviderRef(a): {module.ProviderRef(b)},
		module.ProviderRef(b): {module.ProviderRef(c)},
		module.ProviderRef(c): {},
	}

	if err := DetectCycle(graph); err != nil {
		t.Fatalf("DetectCycle() = %v, want nil for an acyclic graph", err)
	}
}

func TestDetectCycle_DirectCycle_ReturnsFullChain(t *testing.T) {
	a := &fakeProvider{resolved: reflect.TypeOf(&cycleAService{})}
	b := &fakeProvider{resolved: reflect.TypeOf(&cycleBService{})}

	graph := map[module.ProviderRef][]module.ProviderRef{
		module.ProviderRef(a): {module.ProviderRef(b)},
		module.ProviderRef(b): {module.ProviderRef(a)},
	}

	err := DetectCycle(graph)
	if err == nil {
		t.Fatalf("DetectCycle() = nil, want error for A -> B -> A cycle")
	}

	aName := a.ResolvedType().String()
	bName := b.ResolvedType().String()
	want := "circular dependency: " + aName + " -> " + bName + " -> " + aName
	wantAlt := "circular dependency: " + bName + " -> " + aName + " -> " + bName

	got := err.Error()
	if got != want && got != wantAlt {
		t.Fatalf("DetectCycle() error = %q, want %q or %q", got, want, wantAlt)
	}
}

func TestDetectCycle_IndirectCycle_ReturnsFullChainNotJustFoundCycle(t *testing.T) {
	a := &fakeProvider{resolved: reflect.TypeOf(&cycleAService{})}
	b := &fakeProvider{resolved: reflect.TypeOf(&cycleBService{})}
	c := &fakeProvider{resolved: reflect.TypeOf(&cycleCService{})}

	graph := map[module.ProviderRef][]module.ProviderRef{
		module.ProviderRef(a): {module.ProviderRef(b)},
		module.ProviderRef(b): {module.ProviderRef(c)},
		module.ProviderRef(c): {module.ProviderRef(a)},
	}

	err := DetectCycle(graph)
	if err == nil {
		t.Fatalf("DetectCycle() = nil, want error for A -> B -> C -> A cycle")
	}

	msg := err.Error()
	if !strings.HasPrefix(msg, "circular dependency: ") {
		t.Fatalf("DetectCycle() error = %q, want prefix %q", msg, "circular dependency: ")
	}

	aName := a.ResolvedType().String()
	bName := b.ResolvedType().String()
	cName := c.ResolvedType().String()

	// Must contain the full chain, not just a generic "cycle found" message
	// -- all three node names must appear, and in a rotation of A->B->C->A.
	for _, name := range []string{aName, bName, cName} {
		if !strings.Contains(msg, name) {
			t.Fatalf("DetectCycle() error = %q, missing node %q from chain", msg, name)
		}
	}
	// Count arrows: a 3-node cycle chain has exactly 3 "->" separators
	// (A -> B -> C -> A).
	if got := strings.Count(msg, "->"); got != 3 {
		t.Fatalf("DetectCycle() error = %q, arrow count = %d, want 3 (full chain, not truncated)", msg, got)
	}
}

func TestDetectCycle_SelfLoop_IsCaught(t *testing.T) {
	a := &fakeProvider{resolved: reflect.TypeOf(&cycleAService{})}

	graph := map[module.ProviderRef][]module.ProviderRef{
		module.ProviderRef(a): {module.ProviderRef(a)},
	}

	err := DetectCycle(graph)
	if err == nil {
		t.Fatalf("DetectCycle() = nil, want error for A -> A self-loop")
	}

	aName := a.ResolvedType().String()
	want := "circular dependency: " + aName + " -> " + aName
	if err.Error() != want {
		t.Fatalf("DetectCycle() error = %q, want %q", err.Error(), want)
	}
}

func TestDetectCycle_DisconnectedAcyclicComponents_NoFalsePositive(t *testing.T) {
	a := &fakeProvider{resolved: reflect.TypeOf(&cycleAService{})}
	b := &fakeProvider{resolved: reflect.TypeOf(&cycleBService{})}
	c := &fakeProvider{resolved: reflect.TypeOf(&cycleCService{})}

	// Two separate components, neither cyclic: A -> B, C alone.
	graph := map[module.ProviderRef][]module.ProviderRef{
		module.ProviderRef(a): {module.ProviderRef(b)},
		module.ProviderRef(b): {},
		module.ProviderRef(c): {},
	}

	if err := DetectCycle(graph); err != nil {
		t.Fatalf("DetectCycle() = %v, want nil for disconnected acyclic graph", err)
	}
}
