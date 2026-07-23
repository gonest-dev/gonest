package app_test

// This file proves the module-lazy-loading feature (Milestone 24,
// .specs/features/module-lazy-loading) end to end through a real
// coreapp.New[fiber.App] bootstrap: Module.Lazy picks between 2 sibling
// modules based on a config value resolved (eagerly, synchronously) via
// MustInject[T](l) from inside the Lazy callback, and the resulting app
// dispatches real HTTP requests through whichever module won -- proving
// route registration and DI resolution work end-to-end, not just that
// Imports was called with the right argument (LAZY-04). It also proves the
// eagerly-resolved provider's Constructor runs EXACTLY ONCE across the
// whole bootstrap (LAZY-03), even though it is referenced both inside Lazy
// (eager) and normally registered via Providers (which Stage 3 would
// otherwise also try to construct).
//
// package app_test (not app), same reason as fiber_dispatch_test.go's own
// doc comment: internal/adapter/fiber imports internal/app, so an internal
// test file in package app that also imports fiber would be a cycle.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gonest.dev/gonest/internal/adapter/fiber"
	"gonest.dev/gonest/internal/controller"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/route"

	coreapp "gonest.dev/gonest/internal/app"
)

// lazyDriverConfig is this file's own self-contained (no MustInject of its
// own) config type -- Lazy's whole eager-resolution mechanism is restricted
// to exactly this shape (see mustLazy's own doc comment in
// internal/inject/inject.go).
type lazyDriverConfig struct {
	Driver string
}

// buildLazyDrivenApp wires a root module whose own Config_ provider decides
// -- via Module.Lazy -- whether moduleA or moduleB (each owning one
// controller with a distinguishing route) enters the graph, mirroring
// .examples/notification-driver's real email/sms shape but self-contained
// for this test. constructorCalls is incremented once per real Constructor
// invocation, letting callers assert LAZY-03 (constructed exactly once).
func buildLazyDrivenApp(t *testing.T, driver string, constructorCalls *int) *fiber.App {
	t.Helper()

	configProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *lazyDriverConfig {
			*constructorCalls++
			return &lazyDriverConfig{Driver: driver}
		})
	})

	controllerA := controller.New(func(c *controller.Controller) {
		c.Route(route.HttpGet, "/which", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				res.Json(map[string]string{"which": "a"})
			})
		})
	})
	moduleA := module.New(func(m *module.Module) {
		m.Controllers(controllerA)
	})

	controllerB := controller.New(func(c *controller.Controller) {
		c.Route(route.HttpGet, "/which", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {
				res.Json(map[string]string{"which": "b"})
			})
		})
	})
	moduleB := module.New(func(m *module.Module) {
		m.Controllers(controllerB)
	})

	root := module.New(func(m *module.Module) {
		m.Providers(configProvider)
		m.Lazy(func(l *module.LazyModule) {
			cfg := inject.Must[*lazyDriverConfig](l)
			switch cfg.Driver {
			case "b":
				l.Imports(moduleB)
			default:
				l.Imports(moduleA)
			}
		})
	})

	app, err := coreapp.New[fiber.App](root, coreapp.Options{})
	if err != nil {
		t.Fatalf("coreapp.New() error = %v", err)
	}
	fiberAdapter, ok := app.Adapter().(*fiber.App)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.App: %T", app.Adapter())
	}
	return fiberAdapter
}

// TestLazyModule_PicksModuleA_RealHttpDispatch proves the "default" branch
// (driver != "b") wires moduleA into the graph, and its route genuinely
// dispatches -- not just that Imports was called with moduleA.
func TestLazyModule_PicksModuleA_RealHttpDispatch(t *testing.T) {
	var calls int
	fa := buildLazyDrivenApp(t, "a", &calls)

	req := httptest.NewRequest(http.MethodGet, "/which", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["which"] != "a" {
		t.Fatalf("body[which] = %q, want %q -- Lazy did not wire moduleA for driver=a", body["which"], "a")
	}
}

// TestLazyModule_PicksModuleB_RealHttpDispatch proves the "b" branch wires
// moduleB instead -- same route path, different controller answers,
// decided purely by the config value resolved inside Lazy.
func TestLazyModule_PicksModuleB_RealHttpDispatch(t *testing.T) {
	var calls int
	fa := buildLazyDrivenApp(t, "b", &calls)

	req := httptest.NewRequest(http.MethodGet, "/which", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["which"] != "b" {
		t.Fatalf("body[which] = %q, want %q -- Lazy did not wire moduleB for driver=b", body["which"], "b")
	}
}

// TestLazyModule_ConfigProviderConstructorRunsExactlyOnce proves LAZY-03:
// configProvider is referenced both inside Lazy (eager, via
// MustInject[*lazyDriverConfig](l)) AND registered normally via
// m.Providers(configProvider) -- without T3's Stage 3 skip-if-already-
// resolved check, Stage 3's own resolveGraph pass would invoke its
// Constructor a 2nd time. Asserted for BOTH driver branches, since the
// switch inside Lazy does not change how many times Config_ itself is
// touched.
func TestLazyModule_ConfigProviderConstructorRunsExactlyOnce(t *testing.T) {
	for _, driver := range []string{"a", "b"} {
		t.Run(driver, func(t *testing.T) {
			var calls int
			buildLazyDrivenApp(t, driver, &calls)

			if calls != 1 {
				t.Fatalf("configProvider.Constructor ran %d times for driver=%s, want exactly 1 (LAZY-03)", calls, driver)
			}
		})
	}
}
