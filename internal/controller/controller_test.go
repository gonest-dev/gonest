package controller

import (
	"testing"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/filter"
	"gonest.dev/gonest/internal/guard"
	"gonest.dev/gonest/internal/interceptor"
	"gonest.dev/gonest/internal/middleware"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/route"
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

// TestRouteXxx_AllVerbHelpers_DelegateToRouteWithMatchingMethod proves each
// RouteGet/RoutePost/etc shorthand is equivalent to calling Route with the
// matching route.HttpMethod -- no need to pass the method explicitly.
func TestRouteXxx_AllVerbHelpers_DelegateToRouteWithMatchingMethod(t *testing.T) {
	cases := []struct {
		name   string
		call   func(c *Controller)
		method route.HttpMethod
	}{
		{"RouteGet", func(c *Controller) { c.RouteGet("/x", nil) }, route.HttpGet},
		{"RoutePost", func(c *Controller) { c.RoutePost("/x", nil) }, route.HttpPost},
		{"RoutePut", func(c *Controller) { c.RoutePut("/x", nil) }, route.HttpPut},
		{"RoutePatch", func(c *Controller) { c.RoutePatch("/x", nil) }, route.HttpPatch},
		{"RouteDelete", func(c *Controller) { c.RouteDelete("/x", nil) }, route.HttpDelete},
		{"RouteHead", func(c *Controller) { c.RouteHead("/x", nil) }, route.HttpHead},
		{"RouteOptions", func(c *Controller) { c.RouteOptions("/x", nil) }, route.HttpOptions},
		{"RouteTrace", func(c *Controller) { c.RouteTrace("/x", nil) }, route.HttpTrace},
		{"RouteConnect", func(c *Controller) { c.RouteConnect("/x", nil) }, route.HttpConnect},
		{"RouteQuery", func(c *Controller) { c.RouteQuery("/x", nil) }, route.HttpQuery},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New(tc.call)
			c.Declare()

			routes := c.OwnRoutes()
			if len(routes) != 1 {
				t.Fatalf("OwnRoutes() returned %d routes, want 1", len(routes))
			}
			if got := routes[0].Method(); got != tc.method {
				t.Fatalf("%s registered method %v, want %v", tc.name, got, tc.method)
			}
			if got := routes[0].Path(); got != "/x" {
				t.Fatalf("%s registered path %q, want \"/x\"", tc.name, got)
			}
		})
	}
}

func TestOwnRoutes_ReturnsCopyNotInternalSlice(t *testing.T) {
	c := New(func(c *Controller) {
		c.Route(route.HttpGet, "/a", nil)
	})
	c.Declare()

	got := c.OwnRoutes()
	got[0] = route.New(nil, route.HttpPost, "/mutated", nil)

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

func TestFilters_StoresFiltersInRegistrationOrder(t *testing.T) {
	f1 := filter.New(nil)
	f2 := filter.New(nil)

	c := New(func(c *Controller) {
		c.Filters(f1, f2)
	})
	c.Declare()

	got := c.OwnFilters()
	if len(got) != 2 {
		t.Fatalf("OwnFilters() returned %d items, want 2", len(got))
	}
	if got[0] != f1 || got[1] != f2 {
		t.Fatalf("OwnFilters() = %v, want [f1, f2] in registration order", got)
	}
}

func TestOwnFilters_ReturnsCopyNotInternalSlice(t *testing.T) {
	f1 := filter.New(nil)
	f2 := filter.New(nil)

	c := New(func(c *Controller) {
		c.Filters(f1)
	})
	c.Declare()

	got := c.OwnFilters()
	got[0] = f2

	got2 := c.OwnFilters()
	if got2[0] != f1 {
		t.Fatalf("OwnFilters() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

func TestUse_StoresMiddlewareInRegistrationOrder(t *testing.T) {
	var order []string

	m1 := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(c *execution.HttpContext, next middleware.Next) {
			order = append(order, "m1")
			next(c)
		})
	})
	m2 := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(c *execution.HttpContext, next middleware.Next) {
			order = append(order, "m2")
			next(c)
		})
	})

	c := New(func(c *Controller) {
		c.Use(m1, m2)
	})
	c.Declare()

	got := c.OwnMiddleware()
	if len(got) != 2 {
		t.Fatalf("OwnMiddleware() returned %d items, want 2", len(got))
	}
	if got[0] != m1 || got[1] != m2 {
		t.Fatalf("OwnMiddleware() = %v, want [m1, m2] in registration order", got)
	}
}

func TestOwnMiddleware_ReturnsCopyNotInternalSlice(t *testing.T) {
	m1 := middleware.New(nil)
	m2 := middleware.New(nil)

	c := New(func(c *Controller) {
		c.Use(m1)
	})
	c.Declare()

	got := c.OwnMiddleware()
	got[0] = m2

	got2 := c.OwnMiddleware()
	if got2[0] != m1 {
		t.Fatalf("OwnMiddleware() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

func TestGuards_StoresGuardsInRegistrationOrder(t *testing.T) {
	var order []string

	g1 := guard.New(func(g *guard.Guard) {
		g.Handler(func(c *execution.HttpContext) bool {
			order = append(order, "g1")
			return true
		})
	})
	g2 := guard.New(func(g *guard.Guard) {
		g.Handler(func(c *execution.HttpContext) bool {
			order = append(order, "g2")
			return true
		})
	})

	c := New(func(c *Controller) {
		c.Guards(g1, g2)
	})
	c.Declare()

	got := c.OwnGuards()
	if len(got) != 2 {
		t.Fatalf("OwnGuards() returned %d items, want 2", len(got))
	}
	if got[0] != g1 || got[1] != g2 {
		t.Fatalf("OwnGuards() = %v, want [g1, g2] in registration order", got)
	}
}

func TestOwnGuards_ReturnsCopyNotInternalSlice(t *testing.T) {
	g1 := guard.New(nil)
	g2 := guard.New(nil)

	c := New(func(c *Controller) {
		c.Guards(g1)
	})
	c.Declare()

	got := c.OwnGuards()
	got[0] = g2

	got2 := c.OwnGuards()
	if got2[0] != g1 {
		t.Fatalf("OwnGuards() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

func TestInterceptors_StoresInterceptorsInRegistrationOrder(t *testing.T) {
	var order []string

	i1 := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(c *execution.HttpContext, next interceptor.Next) {
			order = append(order, "i1")
			next(c)
		})
	})
	i2 := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(c *execution.HttpContext, next interceptor.Next) {
			order = append(order, "i2")
			next(c)
		})
	})

	c := New(func(c *Controller) {
		c.Interceptors(i1, i2)
	})
	c.Declare()

	got := c.OwnInterceptors()
	if len(got) != 2 {
		t.Fatalf("OwnInterceptors() returned %d items, want 2", len(got))
	}
	if got[0] != i1 || got[1] != i2 {
		t.Fatalf("OwnInterceptors() = %v, want [i1, i2] in registration order", got)
	}
}

func TestOwnInterceptors_ReturnsCopyNotInternalSlice(t *testing.T) {
	i1 := interceptor.New(nil)
	i2 := interceptor.New(nil)

	c := New(func(c *Controller) {
		c.Interceptors(i1)
	})
	c.Declare()

	got := c.OwnInterceptors()
	got[0] = i2

	got2 := c.OwnInterceptors()
	if got2[0] != i1 {
		t.Fatalf("OwnInterceptors() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

// TestTags_StoresTagsInRegistrationOrder proves Tags stores every tag passed,
// in order, retrievable via OwnTags -- inherited by every Route under this
// Controller unless the Route itself overrides (spec.md AC2, resolved at
// generation time, not here).
func TestTags_StoresTagsInRegistrationOrder(t *testing.T) {
	c := New(func(c *Controller) {
		c.Tags("users", "admin")
	})
	c.Declare()

	got := c.OwnTags()
	if len(got) != 2 {
		t.Fatalf("OwnTags() returned %d items, want 2", len(got))
	}
	if got[0] != "users" || got[1] != "admin" {
		t.Fatalf("OwnTags() = %v, want [users, admin] in registration order", got)
	}
}

// TestOwnTags_ReturnsCopyNotInternalSlice proves OwnTags is a defensive
// copy -- mutating the returned slice must not affect the Controller's own
// state, same pattern as OwnMiddleware/OwnInterceptors.
func TestOwnTags_ReturnsCopyNotInternalSlice(t *testing.T) {
	c := New(func(c *Controller) {
		c.Tags("users")
	})
	c.Declare()

	got := c.OwnTags()
	got[0] = "mutated"

	got2 := c.OwnTags()
	if got2[0] != "users" {
		t.Fatalf("OwnTags() leaked mutable internal slice: mutation of returned slice affected subsequent call, got %v", got2)
	}
}

// TestOwnTags_DefaultsEmpty proves OwnTags returns an empty (not nil-panic)
// slice before Tags is ever called.
func TestOwnTags_DefaultsEmpty(t *testing.T) {
	c := New(func(c *Controller) {})
	c.Declare()

	if got := c.OwnTags(); len(got) != 0 {
		t.Fatalf("OwnTags() = %v, want empty before Tags() was ever called", got)
	}
}

// TestBearerAuth_SetsFlag proves BearerAuth sets the flag returned by
// HasBearerAuth, and returns c so calls can chain.
func TestBearerAuth_SetsFlag(t *testing.T) {
	var c *Controller
	c = New(func(cc *Controller) {
		got := cc.BearerAuth()
		if got != cc {
			t.Fatal("Controller.BearerAuth did not return the same *Controller for chaining")
		}
	})
	c.Declare()

	if !c.HasBearerAuth() {
		t.Fatal("HasBearerAuth() = false, want true after BearerAuth() was called")
	}
}

// TestHasBearerAuth_DefaultsFalse proves HasBearerAuth returns false before
// BearerAuth is ever called.
func TestHasBearerAuth_DefaultsFalse(t *testing.T) {
	c := New(func(c *Controller) {})
	c.Declare()

	if c.HasBearerAuth() {
		t.Fatal("HasBearerAuth() = true, want false before BearerAuth() was ever called")
	}
}
