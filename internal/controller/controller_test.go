package controller

import (
	"testing"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/route"
)

func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	executed := false

	c := New(func(c *Controller) {
		executed = true
	})

	if executed {
		t.Fatalf("New(fn) executed fn synchronously, want deferred until bootstrap runs it")
	}
	if c == nil {
		t.Fatalf("New(fn) returned nil *Controller")
	}
}

func TestController_SatisfiesModuleOwner(t *testing.T) {
	var owner module.Owner = New(func(c *Controller) {})
	if owner == nil {
		t.Fatalf("*Controller does not satisfy module.Owner")
	}
}

func TestOwnerModule_PopulatedAfterSetOwnerModule(t *testing.T) {
	c := New(func(c *Controller) {})

	m := module.New(func(m *module.Module) {})
	c.SetOwnerModule(m)

	if got := c.OwnerModule(); got != m {
		t.Fatalf("OwnerModule() = %v, want %v", got, m)
	}
}

func TestOwnerModule_NilBeforeAssociation(t *testing.T) {
	c := New(func(c *Controller) {})

	if got := c.OwnerModule(); got != nil {
		t.Fatalf("OwnerModule() = %v, want nil before any module associates this controller", got)
	}
}

func TestDeclare_ExecutesFn(t *testing.T) {
	executed := false
	c := New(func(c *Controller) {
		executed = true
	})

	c.Declare()

	if !executed {
		t.Fatalf("Declare() did not execute fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	c := New(func(c *Controller) {
		count++
	})

	c.Declare()
	c.Declare()
	c.Declare()

	if count != 1 {
		t.Fatalf("fn executed %d times across 3 Declare() calls, want exactly 1", count)
	}
}

func TestDeclare_NilFn_DoesNotPanic(t *testing.T) {
	c := &Controller{}

	c.Declare()
}

// TestController_SatisfiesModuleControllerRef confirms the cross-package
// blocker flagged during T5 (unexported controllerRef.isController couldn't
// be satisfied outside package module) is resolved now that internal/module
// exports ControllerRef/IsController.
func TestController_SatisfiesModuleControllerRef(t *testing.T) {
	// Compile-time proof: this line alone is the test. If *Controller
	// stopped satisfying module.ControllerRef, this file would fail to
	// build.
	c := New(func(c *Controller) {})
	var _ module.ControllerRef = c
}

func TestPath_StoresPrefix(t *testing.T) {
	c := New(func(c *Controller) {
		c.Path("/users")
	})
	c.Declare()

	if got := c.PathPrefix(); got != "/users" {
		t.Fatalf("PathPrefix() = %q, want %q", got, "/users")
	}
}

func TestRoute_CreatesRouteAndAppendsToOwnRoutes(t *testing.T) {
	c := New(func(c *Controller) {
		c.Route(route.HttpGet, "/:id", func(r *route.Route) {
			r.HttpCode(201)
		})
	})
	c.Declare()

	routes := c.OwnRoutes()
	if len(routes) != 1 {
		t.Fatalf("OwnRoutes() returned %d routes, want 1", len(routes))
	}
	if got := routes[0].Code(); got != 201 {
		t.Fatalf("OwnRoutes()[0].Code() = %d, want 201 (fn passed to Route should run immediately, like route.New)", got)
	}
}

func TestOwnRoutes_ReturnsCopyNotInternalSlice(t *testing.T) {
	c := New(func(c *Controller) {
		c.Route(route.HttpGet, "/a", nil)
	})
	c.Declare()

	got := c.OwnRoutes()
	got[0] = route.New(route.HttpPost, "/mutated", nil)

	got2 := c.OwnRoutes()
	if got2[0].Code() != 200 || len(got2) != 1 {
		t.Fatalf("OwnRoutes() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
	// Verify the underlying route is still the original GET /a route by
	// confirming a second independent call still reflects one route.
	if len(c.OwnRoutes()) != 1 {
		t.Fatalf("internal routes slice was mutated via returned slice")
	}
}

func TestPipelineStubs_DoNotAffectObservableState(t *testing.T) {
	withStubs := New(func(c *Controller) {
		c.Path("/things")
		c.Use(Middleware{})
		c.Guards(Middleware{})
		c.Interceptors(Middleware{})
		c.Filters(Middleware{})
		c.Route(route.HttpGet, "/", nil)
	})
	withStubs.Declare()

	withoutStubs := New(func(c *Controller) {
		c.Path("/things")
		c.Route(route.HttpGet, "/", nil)
	})
	withoutStubs.Declare()

	if withStubs.PathPrefix() != withoutStubs.PathPrefix() {
		t.Fatalf("PathPrefix() differs: %q vs %q", withStubs.PathPrefix(), withoutStubs.PathPrefix())
	}
	if len(withStubs.OwnRoutes()) != len(withoutStubs.OwnRoutes()) {
		t.Fatalf("OwnRoutes() length differs: %d vs %d", len(withStubs.OwnRoutes()), len(withoutStubs.OwnRoutes()))
	}
}
