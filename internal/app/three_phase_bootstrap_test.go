package app

import (
	"sync"
	"testing"

	"gonest.dev/gonest/internal/controller"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/guard"
	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/route"
)

// orderLog is a mutex-protected append-only log, shared across concurrent
// Stage 3 goroutines and later sequential bootstrap phases -- the same
// technique pipeline_ordering_test.go already uses to prove cross-stage
// ordering without relying on wall-clock timing.
type orderLog struct {
	mu   sync.Mutex
	rows []string
}

func (l *orderLog) add(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rows = append(l.rows, s)
}

func (l *orderLog) indexOf(s string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, r := range l.rows {
		if r == s {
			return i
		}
	}
	return -1
}

type tpProviderC struct{}
type tpProviderB struct{}
type tpProviderA struct{}

// TestThreePhaseBootstrap_ProvidersFullyResolveBeforeControllerDeclares
// proves phase 1 (Provider resolution, A depends on B depends on C) fully
// completes before phase 2 (Controller declares, itself depending on A via
// MustInject) ever runs -- spec.md TB-00 AC1.
func TestThreePhaseBootstrap_ProvidersFullyResolveBeforeControllerDeclares(t *testing.T) {
	log := &orderLog{}

	providerC := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *tpProviderC {
			log.add("providerC")
			return &tpProviderC{}
		})
	})
	providerB := provider.New(func(p *provider.Provider) {
		inject.MustInject[*tpProviderC](p)
		p.Constructor(func() *tpProviderB {
			log.add("providerB")
			return &tpProviderB{}
		})
	})
	providerA := provider.New(func(p *provider.Provider) {
		inject.MustInject[*tpProviderB](p)
		p.Constructor(func() *tpProviderA {
			log.add("providerA")
			return &tpProviderA{}
		})
	})

	ctrl := controller.New(func(c *controller.Controller) {
		inject.MustInject[*tpProviderA](c)
		log.add("controllerDeclared")
	})

	root := module.New(func(m *module.Module) {
		m.Providers(providerA, providerB, providerC)
		m.Controllers(ctrl)
	})

	if _, err := NewApp[recordingFakeAdapter](root, Options{}); err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	for _, want := range []string{"providerC", "providerB", "providerA"} {
		if log.indexOf(want) < 0 {
			t.Fatalf("log %v missing %q", log.rows, want)
		}
	}
	ctrlIdx := log.indexOf("controllerDeclared")
	if ctrlIdx < 0 {
		t.Fatalf("log %v missing controllerDeclared", log.rows)
	}
	for _, providerEvent := range []string{"providerA", "providerB", "providerC"} {
		if log.indexOf(providerEvent) >= ctrlIdx {
			t.Fatalf("log %v: %q ran at/after controllerDeclared (index %d), want strictly before", log.rows, providerEvent, ctrlIdx)
		}
	}
}

type tpProviderX struct{}
type tpProviderY struct{}

// TestThreePhaseBootstrap_GuardReferencedByTwoModules_DeclaresOnceWithUnionScope
// proves spec.md TB-00b/AC3: a *Guard referenced by controllers in TWO
// different modules has its own (now-deferred) builder closure run EXACTLY
// ONCE, with a union-scoped MustInject search that succeeds against EITHER
// referencing module's own providers.
func TestThreePhaseBootstrap_GuardReferencedByTwoModules_DeclaresOnceWithUnionScope(t *testing.T) {
	declareCount := 0
	var sawX, sawY bool

	sharedGuard := guard.New(func(g *guard.Guard) {
		declareCount++
		sawX = inject.MustInject[*tpProviderX](g) != nil
		sawY = inject.MustInject[*tpProviderY](g) != nil
		g.Handler(func(req *execution.Request, res *execution.Response) bool { return true })
	})

	providerX := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *tpProviderX { return &tpProviderX{} })
	})
	providerY := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *tpProviderY { return &tpProviderY{} })
	})

	ctrlA := controller.New(func(c *controller.Controller) {
		c.Guards(sharedGuard)
		c.Route(route.HttpGet, "/a", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})
	ctrlB := controller.New(func(c *controller.Controller) {
		c.Guards(sharedGuard)
		c.Route(route.HttpGet, "/b", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})

	moduleA := module.New(func(m *module.Module) {
		m.Providers(providerX)
		m.Controllers(ctrlA)
	})
	moduleB := module.New(func(m *module.Module) {
		m.Providers(providerY)
		m.Controllers(ctrlB)
	})
	root := module.New(func(m *module.Module) {
		m.Imports(moduleA, moduleB)
	})

	if _, err := NewApp[recordingFakeAdapter](root, Options{}); err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	if declareCount != 1 {
		t.Fatalf("guard's own builder closure ran %d times, want exactly 1", declareCount)
	}
	if !sawX {
		t.Fatalf("guard's union scope did not find moduleA's own provider tpProviderX")
	}
	if !sawY {
		t.Fatalf("guard's union scope did not find moduleB's own provider tpProviderY")
	}
}

// TestThreePhaseBootstrap_DeclareIdempotent_AcrossAllFourPipelineStageTypes
// proves spec.md TB-00's idempotent-Declare requirement holds for all 4
// types when reached through NewApp's own orchestration (not just the
// package-level unit tests each already has).
func TestThreePhaseBootstrap_DeclareIdempotent_AcrossAllFourPipelineStageTypes(t *testing.T) {
	guardRuns := 0
	g := guard.New(func(g *guard.Guard) {
		guardRuns++
		g.Handler(func(req *execution.Request, res *execution.Response) bool { return true })
	})

	ctrlA := controller.New(func(c *controller.Controller) {
		c.Guards(g)
		c.Route(route.HttpGet, "/a", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})
	ctrlB := controller.New(func(c *controller.Controller) {
		c.Guards(g) // SAME guard, second controller, SAME module
		c.Route(route.HttpGet, "/b", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(ctrlA, ctrlB)
	})

	if _, err := NewApp[recordingFakeAdapter](root, Options{}); err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	if guardRuns != 1 {
		t.Fatalf("guard builder closure ran %d times across 2 controllers in the SAME module, want exactly 1", guardRuns)
	}
}
