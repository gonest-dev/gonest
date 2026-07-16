package resolver

import (
	"context"
	"sync/atomic"
	"testing"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/scope"
)

// --- fixtures ---------------------------------------------------------

type transientService struct{ ID int }
type transientSingletonDep struct{ ID int }
type transientWithDepService struct {
	Dep *transientSingletonDep
}
type transientConsumerA struct{ Value *transientService }
type transientConsumerB struct{ Value *transientService }

// --- per-edge resolution -----------------------------------------------

// TestResolve_TransientProvider_ResolvesPerEdge proves that two distinct
// pending edges targeting the same Transient-scoped provider each trigger
// their own Constructor execution, each copied only into its own
// placeholder.
func TestResolve_TransientProvider_ResolvesPerEdge(t *testing.T) {
	var invocations atomic.Int32

	transient := provider.New(func(p *provider.Provider) {
		p.Scope(scope.Transient)
		p.Constructor(func() *transientService {
			n := invocations.Add(1)
			return &transientService{ID: int(n)}
		})
	})

	var consumerA, consumerB *provider.Provider
	consumerA = provider.New(func(p *provider.Provider) {
		dep := inject.MustInject[*transientService](consumerA)
		p.Constructor(func() *transientConsumerA {
			return &transientConsumerA{Value: dep}
		})
	})
	consumerB = provider.New(func(p *provider.Provider) {
		dep := inject.MustInject[*transientService](consumerB)
		p.Constructor(func() *transientConsumerB {
			return &transientConsumerB{Value: dep}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(transient, consumerA, consumerB)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	transient.Declare()
	consumerA.Declare()
	consumerB.Declare()

	if err := Resolve(context.Background(), []*module.Module{m}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if invocations.Load() != 2 {
		t.Fatalf("Constructor invocations = %d, want 2 (one per pending edge)", invocations.Load())
	}
}

// TestResolve_TransientProvider_TwoResolutionsReturnDifferentPointers
// proves the two edges' placeholders end up populated with data from two
// distinct instances (different pointers), not the same shared instance.
func TestResolve_TransientProvider_TwoResolutionsReturnDifferentPointers(t *testing.T) {
	transient := provider.New(func(p *provider.Provider) {
		p.Scope(scope.Transient)
		p.Constructor(func() *transientService {
			return &transientService{ID: 0}
		})
	})

	var consumerA, consumerB *provider.Provider
	var placeholderA, placeholderB *transientService
	consumerA = provider.New(func(p *provider.Provider) {
		placeholderA = inject.MustInject[*transientService](consumerA)
		p.Constructor(func() *transientConsumerA {
			return &transientConsumerA{Value: placeholderA}
		})
	})
	consumerB = provider.New(func(p *provider.Provider) {
		placeholderB = inject.MustInject[*transientService](consumerB)
		p.Constructor(func() *transientConsumerB {
			return &transientConsumerB{Value: placeholderB}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(transient, consumerA, consumerB)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	transient.Declare()
	consumerA.Declare()
	consumerB.Declare()

	if err := Resolve(context.Background(), []*module.Module{m}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if placeholderA == nil || placeholderB == nil {
		t.Fatalf("placeholderA or placeholderB is nil")
	}
	if placeholderA == placeholderB {
		t.Fatalf("placeholderA and placeholderB are the same pointer, want distinct instances for Transient scope")
	}
}

// --- singleton dependency of a transient provider -----------------------

// TestResolve_TransientProvider_SingletonDependencyIsSharedAcrossEdges
// proves a Transient provider's own Singleton-scoped dependency is resolved
// once and reused (same pointer) across both re-instantiations of the
// transient provider triggered by its two pending edges.
func TestResolve_TransientProvider_SingletonDependencyIsSharedAcrossEdges(t *testing.T) {
	var depInvocations atomic.Int32

	singletonDep := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *transientSingletonDep {
			depInvocations.Add(1)
			return &transientSingletonDep{ID: 1}
		})
	})

	var transient *provider.Provider
	transient = provider.New(func(p *provider.Provider) {
		p.Scope(scope.Transient)
		dep := inject.MustInject[*transientSingletonDep](transient)
		p.Constructor(func() *transientWithDepService {
			return &transientWithDepService{Dep: dep}
		})
	})

	var consumerA, consumerB *provider.Provider
	var gotA, gotB *transientWithDepService
	consumerA = provider.New(func(p *provider.Provider) {
		gotA = inject.MustInject[*transientWithDepService](consumerA)
		p.Constructor(func() *transientConsumerA {
			return &transientConsumerA{}
		})
	})
	consumerB = provider.New(func(p *provider.Provider) {
		gotB = inject.MustInject[*transientWithDepService](consumerB)
		p.Constructor(func() *transientConsumerB {
			return &transientConsumerB{}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(singletonDep, transient, consumerA, consumerB)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	singletonDep.Declare()
	transient.Declare()
	consumerA.Declare()
	consumerB.Declare()

	if err := Resolve(context.Background(), []*module.Module{m}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if depInvocations.Load() != 1 {
		t.Fatalf("singleton dependency Constructor invocations = %d, want 1 (shared across transient re-instantiations)", depInvocations.Load())
	}
	if gotA == nil || gotB == nil {
		t.Fatalf("gotA or gotB is nil")
	}
	if gotA.Dep == nil || gotB.Dep == nil {
		t.Fatalf("gotA.Dep or gotB.Dep is nil")
	}
	if gotA.Dep != gotB.Dep {
		t.Fatalf("gotA.Dep (%p) != gotB.Dep (%p), want same singleton instance shared across both transient resolutions", gotA.Dep, gotB.Dep)
	}
}

// --- singleton regression -------------------------------------------------

// TestResolve_SingletonProvider_StillResolvesOncePerNode_Regression proves
// T10's per-edge restructuring for Transient scope did not change Singleton
// (T9's default) behavior: a Singleton provider targeted by two distinct
// pending edges still runs its Constructor exactly once, and both edges'
// placeholders end up pointing at data from that single shared instance.
func TestResolve_SingletonProvider_StillResolvesOncePerNode_Regression(t *testing.T) {
	var invocations atomic.Int32

	singleton := provider.New(func(p *provider.Provider) {
		// no explicit Scope call -- default is Singleton
		p.Constructor(func() *transientService {
			n := invocations.Add(1)
			return &transientService{ID: int(n)}
		})
	})

	var consumerA, consumerB *provider.Provider
	var placeholderA, placeholderB *transientService
	consumerA = provider.New(func(p *provider.Provider) {
		placeholderA = inject.MustInject[*transientService](consumerA)
		p.Constructor(func() *transientConsumerA {
			return &transientConsumerA{Value: placeholderA}
		})
	})
	consumerB = provider.New(func(p *provider.Provider) {
		placeholderB = inject.MustInject[*transientService](consumerB)
		p.Constructor(func() *transientConsumerB {
			return &transientConsumerB{Value: placeholderB}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(singleton, consumerA, consumerB)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	singleton.Declare()
	consumerA.Declare()
	consumerB.Declare()

	if err := Resolve(context.Background(), []*module.Module{m}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if invocations.Load() != 1 {
		t.Fatalf("Constructor invocations = %d, want 1 (Singleton shared across edges)", invocations.Load())
	}
	if placeholderA.ID != placeholderB.ID {
		t.Fatalf("placeholderA.ID (%d) != placeholderB.ID (%d), want same shared instance data", placeholderA.ID, placeholderB.ID)
	}
}
