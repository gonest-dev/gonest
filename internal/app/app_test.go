package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gonest-dev/gonest/internal/controller"
	"github.com/gonest-dev/gonest/internal/exception"
	"github.com/gonest-dev/gonest/internal/fiberapp"
	"github.com/gonest-dev/gonest/internal/guard"
	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/inject"
	"github.com/gonest-dev/gonest/internal/interceptor"
	"github.com/gonest-dev/gonest/internal/middleware"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/provider"
	"github.com/gonest-dev/gonest/internal/route"
	"github.com/gonest-dev/gonest/internal/scope"
)

// UserProperties/UserEntity/UserService/UserProvider/UserModule/AppModule
// below adapt the "exemplo mais simples" from INSIGHT.md into a real Go
// test -- stripped of anything HTTP/Controller-related (no Fiber, no
// routes), keeping just the DI shape: a UserService struct + UserProvider
// (a Provider with a Constructor), wired into a Module, resolved via NewApp
// + MustInject[*UserService].

type UserEntity struct {
	ID   int64
	Name string
}

type UserService struct {
	index int
	list  []*UserEntity
}

func (t *UserService) List() []*UserEntity { return t.list }

// Get returns the UserEntity with the given ID, or nil if none exists.
// INSIGHT.md's version panics via gonest.NewNotFoundException when the user
// is missing (Milestone 2's Exceptions feature, not built yet) -- T9 of
// this feature keeps it simple and returns nil, which the UserController
// route below turns into a 404 without a structured Exception type.
func (t *UserService) Get(userID int64) *UserEntity {
	for _, u := range t.list {
		if u.ID == userID {
			return u
		}
	}
	return nil
}

func (t *UserService) Create(name string) *UserEntity {
	t.index++
	u := &UserEntity{ID: int64(t.index), Name: name}
	t.list = append(t.list, u)
	return u
}

// Update sets the Name of the UserEntity with the given ID and returns it,
// or nil if no such user exists.
func (t *UserService) Update(userID int64, name string) *UserEntity {
	u := t.Get(userID)
	if u == nil {
		return nil
	}
	u.Name = name
	return u
}

// Delete removes the UserEntity with the given ID from the list and
// returns the removed entity, or nil if no such user exists.
func (t *UserService) Delete(userID int64) *UserEntity {
	for i, u := range t.list {
		if u.ID == userID {
			t.list = append(t.list[:i], t.list[i+1:]...)
			return u
		}
	}
	return nil
}

var UserProvider = provider.New(func(p *provider.Provider) {
	p.Scope(scope.Singleton)
	p.Constructor(func() *UserService {
		return &UserService{index: 0, list: make([]*UserEntity, 0)}
	})
})

var exampleUserModule = module.New(func(m *module.Module) {
	m.Providers(UserProvider)
	m.Exports(UserProvider)
})

func TestNewApp_UserProviderExample_ResolvesUsableUserService(t *testing.T) {
	appModule := module.New(func(m *module.Module) {
		m.Imports(exampleUserModule)
	})

	var userService *UserService
	consumer := provider.New(func(p *provider.Provider) {
		userService = inject.MustInject[*UserService](p)
		p.Constructor(func() *consumerMarker {
			return &consumerMarker{}
		})
	})
	appModule.Providers(consumer)

	app, err := NewApp[recordingFakeAdapter](appModule, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}

	if userService == nil {
		t.Fatalf("MustInject[*UserService] placeholder is nil after NewApp returned")
	}

	// Prove it's genuinely usable, not a zero-value placeholder: the
	// Constructor initialized list to a non-nil empty slice, and Create
	// mutates real state.
	if got := userService.List(); got == nil {
		t.Fatalf("userService.List() = nil, want non-nil empty slice from real Constructor initialization")
	}

	created := userService.Create("Ada")
	if created.ID != 1 || created.Name != "Ada" {
		t.Fatalf("userService.Create(%q) = %+v, want ID=1 Name=Ada", "Ada", created)
	}
	if len(userService.List()) != 1 {
		t.Fatalf("userService.List() len = %d after Create, want 1 -- resolved instance is not the real, usable one", len(userService.List()))
	}
}

type consumerMarker struct{}

func TestMustNewApp_PanicsOnAssembleError(t *testing.T) {
	type undeclaredService struct{}
	badProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *undeclaredService {
			return &undeclaredService{}
		})
	})

	root := module.New(func(m *module.Module) {
		// Exports a provider it never declared via Providers -- Stage 1
		// validation error (design.md's Error Handling Strategy table).
		m.Exports(badProvider)
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustNewApp() did not panic for a Stage 1 assemble error")
		}
	}()

	MustNewApp[recordingFakeAdapter](root, AppOptions{})
}

func TestMustNewApp_ReturnsAppOnSuccess(t *testing.T) {
	type simpleService struct{ Ready bool }
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *simpleService {
			return &simpleService{Ready: true}
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	app := MustNewApp[recordingFakeAdapter](root, AppOptions{})
	if app == nil {
		t.Fatalf("MustNewApp() returned nil *App")
	}
}

func TestNewApp_CircularDependency_ReturnsError(t *testing.T) {
	type cycleA struct{}
	type cycleB struct{}

	var pa, pb *provider.Provider
	pa = provider.New(func(p *provider.Provider) {
		inject.MustInject[*cycleB](pa)
		p.Constructor(func() *cycleA { return &cycleA{} })
	})
	pb = provider.New(func(p *provider.Provider) {
		inject.MustInject[*cycleA](pb)
		p.Constructor(func() *cycleB { return &cycleB{} })
	})

	root := module.New(func(m *module.Module) {
		m.Providers(pa, pb)
	})

	_, err := NewApp[recordingFakeAdapter](root, AppOptions{})
	if err == nil {
		t.Fatalf("NewApp() error = nil, want circular dependency error")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("NewApp() error = %q, want it to mention 'circular dependency'", err.Error())
	}
}

// TestNewApp_TwoSequentialUnrelatedCalls_DoNotLeakPendingEdges proves that
// calling NewApp twice in the same process, with two completely unrelated
// module trees, does not let the first call's MustInject bookkeeping leak
// into the second call's cycle detection or placeholder resolution.
// internal/inject's pendingEdges slice is process-global; without resetting
// it at the start of each NewApp call, the second call's Stage 3 would
// observe stale pending edges from the first call forever (unbounded growth
// across a long-running process, and a correctness risk for
// placeholdersFor's unscoped inject.PendingEdges() read).
func TestNewApp_TwoSequentialUnrelatedCalls_DoNotLeakPendingEdges(t *testing.T) {
	type firstTreeService struct{ Value string }
	firstProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *firstTreeService {
			return &firstTreeService{Value: "first"}
		})
	})
	firstRoot := module.New(func(m *module.Module) {
		m.Providers(firstProvider)
	})

	firstApp, err := NewApp[recordingFakeAdapter](firstRoot, AppOptions{})
	if err != nil {
		t.Fatalf("first NewApp() error = %v", err)
	}
	if firstApp == nil {
		t.Fatalf("first NewApp() returned nil *App")
	}

	// After the first call resolves, no pending edges should remain in
	// internal/inject's global bookkeeping -- NewApp must reset the log at
	// the start of ITS OWN call, not leave the previous call's edges parked
	// forever.
	if got := len(inject.PendingEdges()); got != 0 {
		t.Fatalf("inject.PendingEdges() len = %d after first NewApp() returned, want 0 -- pending edges must not accumulate across NewApp calls", got)
	}

	type secondTreeService struct{ Value string }
	var resolved *secondTreeService
	secondProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *secondTreeService {
			return &secondTreeService{Value: "second"}
		})
	})
	var consumer *provider.Provider
	consumer = provider.New(func(p *provider.Provider) {
		resolved = inject.MustInject[*secondTreeService](consumer)
		p.Constructor(func() *consumerMarker {
			return &consumerMarker{}
		})
	})
	secondRoot := module.New(func(m *module.Module) {
		m.Providers(secondProvider, consumer)
	})

	secondApp, err := NewApp[recordingFakeAdapter](secondRoot, AppOptions{})
	if err != nil {
		t.Fatalf("second NewApp() error = %v, want nil -- must not be affected by the first call's leftover state", err)
	}
	if secondApp == nil {
		t.Fatalf("second NewApp() returned nil *App")
	}

	if resolved == nil {
		t.Fatalf("second tree's MustInject[*secondTreeService] placeholder is nil after second NewApp() returned")
	}
	if resolved.Value != "second" {
		t.Fatalf("resolved.Value = %q, want %q -- second call's placeholder must be filled from the SECOND tree's provider, not confused with the first tree's leftover pending edges", resolved.Value, "second")
	}
}

func TestNewApp_ConstructorError_ReturnsError(t *testing.T) {
	type failingService struct{}
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() (*failingService, error) {
			return nil, errors.New("boom")
		})
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	_, err := NewApp[recordingFakeAdapter](root, AppOptions{})
	if err == nil {
		t.Fatalf("NewApp() error = nil, want the Constructor's returned error surfaced")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("NewApp() error = %q, want it to contain the Constructor's error message %q", err.Error(), "boom")
	}
}

// fakeRegisteredRoute records one RegisterRoute call's method+path, as
// observed by recordingFakeAdapter below.
type fakeRegisteredRoute struct {
	method route.HttpMethod
	path   string
}

// TestNewApp_ControllerWithRoutes_RegistersEachOnAdapter proves Stage 2.5
// walks OwnControllers/OwnRoutes across the assembled module tree and calls
// RegisterRoute on the adapter for each one, with the full path (Controller
// PathPrefix + Route Path).
func TestNewApp_ControllerWithRoutes_RegistersEachOnAdapter(t *testing.T) {
	userController := controller.New(func(c *controller.Controller) {
		c.Path("/user")
		c.Route(route.HttpGet, "/:id", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {})
		})
		c.Route(route.HttpPost, "/", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(userController)
	})

	app, err := NewApp[recordingFakeAdapter](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}

	got := lastRecordingAdapter.registered
	want := []fakeRegisteredRoute{
		{method: route.HttpGet, path: "/user/:id"},
		{method: route.HttpPost, path: "/user/"},
	}
	if len(got) != len(want) {
		t.Fatalf("registered routes = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("registered[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestNewApp_DuplicateRoute_ReturnsErrorBeforeRegistering proves Stage 2.5
// detects a method+path collision (considering the controller's PathPrefix)
// BEFORE registering anything on the adapter, and returns the exact error
// format design.md specifies: "duplicate route: GET /user/:id".
func TestNewApp_DuplicateRoute_ReturnsErrorBeforeRegistering(t *testing.T) {
	dupeController := controller.New(func(c *controller.Controller) {
		c.Path("/user")
		c.Route(route.HttpGet, "/:id", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {})
		})
	})
	otherDupeController := controller.New(func(c *controller.Controller) {
		c.Path("/user")
		c.Route(route.HttpGet, "/:id", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(dupeController, otherDupeController)
	})

	_, err := NewApp[recordingFakeAdapter](root, AppOptions{})
	if err == nil {
		t.Fatalf("NewApp() error = nil, want a duplicate route error")
	}
	if err.Error() != "duplicate route: GET /user/:id" {
		t.Fatalf("NewApp() error = %q, want %q", err.Error(), "duplicate route: GET /user/:id")
	}

	if len(lastRecordingAdapter.registered) != 0 {
		t.Fatalf("registered = %+v, want no routes registered when a collision is detected", lastRecordingAdapter.registered)
	}
}

// TestNewApp_ZeroControllers_BootstrapsNormally proves an app with no
// Controllers at all (pure DI graph, like every pre-T8 test in this file)
// bootstraps successfully through Stage 2.5 -- the edge case spec.md calls
// out explicitly.
func TestNewApp_ZeroControllers_BootstrapsNormally(t *testing.T) {
	type plainService struct{ Ready bool }
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *plainService { return &plainService{Ready: true} })
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	app, err := NewApp[recordingFakeAdapter](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}
	if len(lastRecordingAdapter.registered) != 0 {
		t.Fatalf("registered = %+v, want none for a controller-less app", lastRecordingAdapter.registered)
	}
}

// TestNewApp_EmptyPathPrefix_RegistersRouteWithBarePath proves a Controller
// that never calls Path (empty prefix) still registers its routes correctly
// -- full path is just the Route's own Path, no leading prefix segment
// glued on.
func TestNewApp_EmptyPathPrefix_RegistersRouteWithBarePath(t *testing.T) {
	noPrefixController := controller.New(func(c *controller.Controller) {
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(noPrefixController)
	})

	_, err := NewApp[recordingFakeAdapter](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	got := lastRecordingAdapter.registered
	if len(got) != 1 || got[0] != (fakeRegisteredRoute{method: route.HttpGet, path: "/ping"}) {
		t.Fatalf("registered = %+v, want a single GET /ping route", got)
	}
}

// recordingFakeAdapter is identical to fakeAdapter but records itself into
// lastRecordingAdapter on Init, so tests using it as NewApp[T]'s type
// argument can read back what got registered on the specific instance
// NewApp[T] constructed internally.
type recordingFakeAdapter struct {
	registered []fakeRegisteredRoute
}

var lastRecordingAdapter *recordingFakeAdapter

func (f *recordingFakeAdapter) Init() {
	lastRecordingAdapter = f
}

func (f *recordingFakeAdapter) RegisterRoute(method route.HttpMethod, path string, h func(ctx *httpctx.Context)) error {
	f.registered = append(f.registered, fakeRegisteredRoute{method: method, path: path})
	return nil
}

func (f *recordingFakeAdapter) Listen(addr string, onListen func()) error {
	return nil
}

// TestNewApp_FiberApp_RealEndToEndWiring proves the generic wiring truly
// works with the real fiberapp.FiberApp adapter (not just the fake spy
// above): NewApp[fiberapp.FiberApp] constructs a genuinely usable FiberApp
// (Init sets a non-nil *fiber.App, see internal/fiberapp/fiberapp.go),
// registers a real route on it, and dispatches a real HTTP request through
// Fiber's own app.Test -- proving reflect.New(...).Interface() + Init()
// produces something other than a nil-panic waiting to happen.
func TestNewApp_FiberApp_RealEndToEndWiring(t *testing.T) {
	called := false
	pingController := controller.New(func(c *controller.Controller) {
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				called = true
				ctx.Json(map[string]string{"pong": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(pingController)
	})

	app, err := NewApp[fiberapp.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}

	fiberAdapter, ok := app.Adapter().(*fiberapp.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiberapp.FiberApp: %T", app.Adapter())
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Fatalf("expected the registered Handler to run, but it did not")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond is T9 of this
// feature: it adapts the UserController/UserService example from
// INSIGHT.md (lines 1-100) into a real integration test proving the whole
// feature -- Provider & DI Graph + Module Composition + Controller & Route
// Registration -- works together end-to-end via a real fiberapp.FiberApp
// dispatched through app.Test(req).
//
// SPEC_DEVIATION (documented, not a blocker -- spec.md's own Independent
// Test line for P1 explicitly allows "rotas... adaptadas"): INSIGHT.md's
// full example uses gonest.Value[T] field wrappers, HttpStatusOk/
// HttpStatusCreated named constants, and MustJsonBody[T] to parse a JSON
// request body into UserProperties for Create/Update. None of those exist
// in this codebase yet -- spec.md's CTRL-01..08 requirement list has no
// JSON-body-parsing requirement, it is out of scope for this feature (a
// future milestone, see spec.md's "Out of Scope" table). This test adapts
// by using a plain Go string for UserEntity.Name (no Value[T] wrapper),
// plain int status codes (200/201/404, no named constants), and for
// Create/Update -- since there is no body-parsing primitive available
// today -- accepts the "name" via a route param instead of a JSON body.
// This still exercises a real POST/PUT round-trip through app.Test: the
// route dispatches, MustParam converts params, and the response shape/
// status match INSIGHT.md's intent (200 for List/Get/Update/Delete, 201
// for Create).
func TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t *testing.T) {
	userController := controller.New(func(c *controller.Controller) {
		c.Path("/user")

		userService := inject.MustInject[*UserService](c)

		// QUERY /user/ -- List. INSIGHT.md uses gonest.HttpQuery for the
		// list route; route.HttpQuery maps to fiberapp's "QUERY" method
		// string (internal/fiberapp/fiberapp.go's fiberMethod), which
		// httptest.NewRequest below dispatches as a plain HTTP method
		// string like any other -- QUERY is not a net/http constant, but
		// Fiber and net/http/httptest both treat the method as an opaque
		// string, so this round-trips fine.
		c.Route(route.HttpQuery, "/", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(ctx *httpctx.Context) {
				ctx.Status(r.Code()).Json(userService.List())
			})
		})

		// GET /user/:user_id -- Get. 404 (plain int, no structured
		// Exception type yet -- Milestone 2) if the user doesn't exist.
		c.Route(route.HttpGet, "/:user_id", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(ctx *httpctx.Context) {
				userID := route.MustParam[int64](ctx, "user_id")
				u := userService.Get(userID)
				if u == nil {
					ctx.Status(404).Json(map[string]string{"error": "not found"})
					return
				}
				ctx.Status(r.Code()).Json(u)
			})
		})

		// POST /user/:name -- Create. SPEC_DEVIATION: INSIGHT.md POSTs to
		// "/" with a JSON body (MustJsonBody[*UserProperties]); no
		// body-parsing primitive exists yet, so this adaptation takes the
		// new user's name via a route param instead, proving the same
		// "POST creates and returns 201" round-trip.
		c.Route(route.HttpPost, "/:name", func(r *route.Route) {
			r.HttpCode(201)
			r.Handler(func(ctx *httpctx.Context) {
				name := route.MustParam[string](ctx, "name")
				ctx.Status(r.Code()).Json(userService.Create(name))
			})
		})

		// PUT /user/:user_id/:name -- Update. Same body-parsing
		// SPEC_DEVIATION as Create above: the new name travels as a
		// second route param instead of a JSON body.
		c.Route(route.HttpPut, "/:user_id/:name", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(ctx *httpctx.Context) {
				userID := route.MustParam[int64](ctx, "user_id")
				name := route.MustParam[string](ctx, "name")
				u := userService.Update(userID, name)
				if u == nil {
					ctx.Status(404).Json(map[string]string{"error": "not found"})
					return
				}
				ctx.Status(r.Code()).Json(u)
			})
		})

		// DELETE /user/:user_id -- Delete.
		c.Route(route.HttpDelete, "/:user_id", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(ctx *httpctx.Context) {
				userID := route.MustParam[int64](ctx, "user_id")
				u := userService.Delete(userID)
				if u == nil {
					ctx.Status(404).Json(map[string]string{"error": "not found"})
					return
				}
				ctx.Status(r.Code()).Json(u)
			})
		})
	})

	userModule := module.New(func(m *module.Module) {
		m.Providers(UserProvider)
		m.Controllers(userController)
	})

	root := module.New(func(m *module.Module) {
		m.Imports(userModule)
	})

	app, err := NewApp[fiberapp.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiberapp.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiberapp.FiberApp: %T", app.Adapter())
	}
	fa := fiberAdapter.FiberApp()

	// 1. List (empty) -- QUERY /user/
	{
		req := httptest.NewRequest("QUERY", "/user/", nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("List: app.Test error = %v", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Fatalf("List: status = %d, want 200", resp.StatusCode)
		}
		var got []*UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		// app.Test's response body is backed by a pooled fasthttp buffer
		// that is only guaranteed valid until the NEXT app.Test call on the
		// same *fiber.App -- draining+closing it here, right after
		// decoding and BEFORE the next block's fa.Test(...), avoids
		// cross-request buffer reuse corrupting an earlier response still
		// held open via `defer` (observed as a flaky, garbled JSON body
		// when this originally used `defer resp.Body.Close()` at function
		// scope instead of per-block).
		if decodeErr != nil {
			t.Fatalf("List: decode error = %v", decodeErr)
		}
		if len(got) != 0 {
			t.Fatalf("List: got %d users, want 0 before any Create", len(got))
		}
	}

	// 2. Create -- POST /user/Ada
	var created UserEntity
	{
		req := httptest.NewRequest(http.MethodPost, "/user/Ada", nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Create: app.Test error = %v", err)
		}
		if resp.StatusCode != 201 {
			resp.Body.Close()
			t.Fatalf("Create: status = %d, want 201", resp.StatusCode)
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&created)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("Create: decode error = %v", decodeErr)
		}
		if created.ID != 1 || created.Name != "Ada" {
			t.Fatalf("Create: got %+v, want ID=1 Name=Ada", created)
		}
	}

	// 3. Get -- GET /user/:user_id (MustParam[int64] round-trip)
	{
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/user/%d", created.ID), nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Get: app.Test error = %v", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Fatalf("Get: status = %d, want 200", resp.StatusCode)
		}
		var got UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("Get: decode error = %v", decodeErr)
		}
		if got.ID != created.ID || got.Name != "Ada" {
			t.Fatalf("Get: got %+v, want %+v", got, created)
		}
	}

	// 3b. Get -- unknown ID returns 404
	{
		req := httptest.NewRequest(http.MethodGet, "/user/999", nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Get(missing): app.Test error = %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Fatalf("Get(missing): status = %d, want 404", resp.StatusCode)
		}
	}

	// 4. Update -- PUT /user/:user_id/:name (MustParam[int64] round-trip)
	{
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/user/%d/Grace", created.ID), nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Update: app.Test error = %v", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Fatalf("Update: status = %d, want 200", resp.StatusCode)
		}
		var got UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("Update: decode error = %v", decodeErr)
		}
		if got.ID != created.ID || got.Name != "Grace" {
			t.Fatalf("Update: got %+v, want ID=%d Name=Grace", got, created.ID)
		}
	}

	// 5. Delete -- DELETE /user/:user_id (MustParam[int64] round-trip)
	{
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/user/%d", created.ID), nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("Delete: app.Test error = %v", err)
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Fatalf("Delete: status = %d, want 200", resp.StatusCode)
		}
		var got UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("Delete: decode error = %v", decodeErr)
		}
		if got.ID != created.ID {
			t.Fatalf("Delete: got %+v, want ID=%d", got, created.ID)
		}
	}

	// 5b. List (empty again) -- proves Delete actually removed the entry
	{
		req := httptest.NewRequest("QUERY", "/user/", nil)
		resp, err := fa.Test(req)
		if err != nil {
			t.Fatalf("List(after delete): app.Test error = %v", err)
		}
		var got []*UserEntity
		decodeErr := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if decodeErr != nil {
			t.Fatalf("List(after delete): decode error = %v", decodeErr)
		}
		if len(got) != 0 {
			t.Fatalf("List(after delete): got %d users, want 0 after Delete", len(got))
		}
	}
}

// TestNewApp_ZeroValueAppOptions_BootstrapsIdenticallyToPreT2Behavior proves
// NewApp[T, PT](root, AppOptions{}) bootstraps the DI graph and registers
// routes exactly as NewApp[T, PT](root) did before this task's signature
// change -- zero behavior regression from adding the required AppOptions
// parameter.
func TestNewApp_ZeroValueAppOptions_BootstrapsIdenticallyToPreT2Behavior(t *testing.T) {
	userController := controller.New(func(c *controller.Controller) {
		c.Path("/user")
		c.Route(route.HttpGet, "/:id", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Providers(UserProvider)
		m.Controllers(userController)
	})

	app, err := NewApp[recordingFakeAdapter](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}

	got := lastRecordingAdapter.registered
	want := []fakeRegisteredRoute{{method: route.HttpGet, path: "/user/:id"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("registered routes = %+v, want %+v", got, want)
	}

	if app.opts.BufferLogs != false || app.opts.LogLevels != nil {
		t.Fatalf("app.opts = %+v, want zero-value AppOptions{}", app.opts)
	}
}

// TestNewApp_NonZeroAppOptions_StoredOnApp proves NewApp stores whatever
// AppOptions it was given on the returned *App's unexported opts field,
// verified here via direct same-package field access (no public getter
// exists yet -- nothing public reads opts in this task, per design.md's
// "App (extended)" component: "stored, unused beyond storage").
func TestNewApp_NonZeroAppOptions_StoredOnApp(t *testing.T) {
	type plainService struct{ Ready bool }
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *plainService { return &plainService{Ready: true} })
	})
	root := module.New(func(m *module.Module) {
		m.Providers(p)
	})

	opts := AppOptions{
		BufferLogs: true,
		LogLevels:  []LogLevel{LogLevelWarn, LogLevelError},
	}

	app, err := NewApp[recordingFakeAdapter](root, opts)
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}

	if app.opts.BufferLogs != true {
		t.Fatalf("app.opts.BufferLogs = %v, want true", app.opts.BufferLogs)
	}
	if len(app.opts.LogLevels) != 2 || app.opts.LogLevels[0] != LogLevelWarn || app.opts.LogLevels[1] != LogLevelError {
		t.Fatalf("app.opts.LogLevels = %+v, want [Warn Error]", app.opts.LogLevels)
	}
}

// listenSpyAdapter is a minimal HttpAdapter spy used only by
// MustListen's own tests (T4) -- it records the addr/onListen it was called
// with, blocks on a channel the test closes to release it (proving
// MustListen's underlying Listen call genuinely blocks rather than
// returning immediately), and can be configured to return an error instead,
// to prove MustListen's panic-on-error behavior without needing a real port
// bind.
type listenSpyAdapter struct {
	mu          sync.Mutex
	addr        string
	onListen    func()
	onListenRan int

	// unblock, when non-nil, is waited on before Listen returns -- letting a
	// test control exactly when the blocking call finishes.
	unblock chan struct{}
	// err, when non-nil, is returned by Listen instead of blocking.
	err error
}

func (f *listenSpyAdapter) Init() {}

func (f *listenSpyAdapter) RegisterRoute(method route.HttpMethod, path string, h func(ctx *httpctx.Context)) error {
	return nil
}

func (f *listenSpyAdapter) Listen(addr string, onListen func()) error {
	f.mu.Lock()
	f.addr = addr
	f.onListen = onListen
	f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	if onListen != nil {
		onListen()
		f.mu.Lock()
		f.onListenRan++
		f.mu.Unlock()
	}

	if f.unblock != nil {
		<-f.unblock
	}
	return nil
}

// TestMustListen_FiresOnListenOnceAndBlocks proves App.MustListen calls
// through to the adapter's Listen with a real OnListen wrapped into a plain
// func(), that the callback fires exactly once, and that MustListen itself
// blocks (does not return) until the underlying Listen call does.
func TestMustListen_FiresOnListenOnceAndBlocks(t *testing.T) {
	spy := &listenSpyAdapter{unblock: make(chan struct{})}
	a := &App{adapter: spy}

	var calls int
	fired := make(chan struct{})
	onListen := OnListen(func() {
		calls++
		close(fired)
	})

	returned := make(chan struct{})
	go func() {
		a.MustListen(":0", onListen)
		close(returned)
	}()

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatalf("onListen callback did not fire within timeout")
	}

	// MustListen must still be blocked -- Listen has not been released yet.
	select {
	case <-returned:
		t.Fatalf("MustListen() returned before the underlying adapter.Listen call did")
	default:
	}

	close(spy.unblock)

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatalf("MustListen() did not return after adapter.Listen was released")
	}

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if calls != 1 {
		t.Fatalf("onListen called %d times, want exactly 1", calls)
	}
	if spy.addr != ":0" {
		t.Fatalf("adapter.Listen addr = %q, want %q", spy.addr, ":0")
	}
}

// TestMustListen_NilOnListen_BlocksWithoutPanicOrCall proves passing a nil
// OnListen to MustListen is safe: the adapter's Listen still gets called (a
// nil onListen func passed straight through), MustListen blocks until
// Listen returns, and nothing panics.
func TestMustListen_NilOnListen_BlocksWithoutPanicOrCall(t *testing.T) {
	spy := &listenSpyAdapter{unblock: make(chan struct{})}
	a := &App{adapter: spy}

	returned := make(chan struct{})
	go func() {
		a.MustListen(":0", nil)
		close(returned)
	}()

	// Give MustListen a moment to reach the blocking Listen call, then prove
	// it has NOT returned yet.
	select {
	case <-returned:
		t.Fatalf("MustListen() returned before the underlying adapter.Listen call did")
	case <-time.After(100 * time.Millisecond):
	}

	close(spy.unblock)

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatalf("MustListen() did not return after adapter.Listen was released")
	}

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if spy.onListen != nil {
		t.Fatalf("adapter.Listen received a non-nil onListen, want nil to have been passed straight through")
	}
	if spy.onListenRan != 0 {
		t.Fatalf("onListenRan = %d, want 0 -- nil onListen must never be called", spy.onListenRan)
	}
}

// TestMustListen_ListenError_PanicsWithAddrAndError proves MustListen
// panics, with a message containing both addr and the underlying error's
// text, when adapter.Listen returns an error -- the "Must"-prefixed
// panic-on-error convention shared with MustNewApp/MustInject/MustParam.
func TestMustListen_ListenError_PanicsWithAddrAndError(t *testing.T) {
	wantErr := errors.New("address already in use")
	spy := &listenSpyAdapter{err: wantErr}
	a := &App{adapter: spy}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustListen() did not panic when adapter.Listen returned an error")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want a string", r, r)
		}
		if !strings.Contains(msg, ":9999") {
			t.Fatalf("panic message = %q, want it to contain the addr %q", msg, ":9999")
		}
		if !strings.Contains(msg, wantErr.Error()) {
			t.Fatalf("panic message = %q, want it to contain the underlying error %q", msg, wantErr.Error())
		}
	}()

	a.MustListen(":9999", nil)
}

// TestMustListen_RealFiberApp_IntegrationSmoke proves the whole chain works
// end-to-end through App.MustListen with a real fiberapp.FiberApp: it binds
// a real (OS-chosen) port, MustListen's wrapped OnListen fires exactly once,
// and a real HTTP request against the bound addr succeeds. Kept deliberately
// minimal -- a full DI+routing dial-the-whole-stack proof belongs to T6, not
// here.
func TestMustListen_RealFiberApp_IntegrationSmoke(t *testing.T) {
	root := module.New(func(m *module.Module) {})

	app, err := NewApp[fiberapp.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiberapp.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiberapp.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	const addr = "127.0.0.1:34579"

	fired := make(chan struct{})
	var once sync.Once
	onListen := OnListen(func() {
		once.Do(func() { close(fired) })
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.MustListen(addr, onListen)
	}()

	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatalf("onListen callback did not fire within timeout")
	}

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("http.Get error = %v", err)
	}
	resp.Body.Close()

	select {
	case <-done:
		t.Fatalf("MustListen() returned unexpectedly before shutdown")
	default:
	}
}

// TestNewApp_UserControllerRealHttpClient_EndToEndOverRealPort is T6 of this
// feature ("App Bootstrap & Listen") -- the final task. It closes the loop
// left open by TestMustListen_RealFiberApp_IntegrationSmoke (above, T4/T3
// territory: proves MustListen+OnListen work end-to-end, but with a
// controller-less root module and only http.Get, not a real net/http.Client
// dial) and by TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond (T9 of
// the earlier "Controller & Route Registration" feature: proves the full
// UserController/UserService/AppModule DI+routing example works, but only
// via Fiber's own in-process app.Test, never a real bound TCP port).
//
// This test reuses that same UserController/UserService example (the
// package-level UserProvider var and the userController builder shape
// defined right above TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond)
// but bootstraps it through App.MustListen against a real, OS-reachable
// 127.0.0.1 address, waits for OnListen to fire (channel-synchronized, no
// time.Sleep -- same pattern as TestMustListen_RealFiberApp_IntegrationSmoke
// and fiberapp's TestListen_OnListenFires_BeforeBlockingForGood), then
// dispatches an actual net/http.Client request (a genuine TCP dial, NOT
// app.Test) against the bound port to prove the WHOLE chain -- NewApp's DI
// bootstrap, Stage 2.5 route registration, App.MustListen, the adapter's
// real Listen, and Fiber's real accept loop -- works together, not just each
// layer in isolation.
func TestNewApp_UserControllerRealHttpClient_EndToEndOverRealPort(t *testing.T) {
	userController := controller.New(func(c *controller.Controller) {
		c.Path("/user")

		userService := inject.MustInject[*UserService](c)

		c.Route(route.HttpGet, "/:user_id", func(r *route.Route) {
			r.HttpCode(200)
			r.Handler(func(ctx *httpctx.Context) {
				userID := route.MustParam[int64](ctx, "user_id")
				u := userService.Get(userID)
				if u == nil {
					ctx.Status(404).Json(map[string]string{"error": "not found"})
					return
				}
				ctx.Status(r.Code()).Json(u)
			})
		})

		// POST /user/:name -- Create. Same SPEC_DEVIATION as T9's version
		// above (INSIGHT.md's JSON body is not available yet, see this
		// file's TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond doc
		// comment): the new user's name travels as a route param. Needed
		// here purely to seed a real user for the GET round-trip below --
		// this test's focus is the real net/http.Client dial, not exercising
		// every one of the 5 routes again (T9 already covers that via
		// app.Test).
		c.Route(route.HttpPost, "/:name", func(r *route.Route) {
			r.HttpCode(201)
			r.Handler(func(ctx *httpctx.Context) {
				name := route.MustParam[string](ctx, "name")
				ctx.Status(r.Code()).Json(userService.Create(name))
			})
		})
	})

	userModule := module.New(func(m *module.Module) {
		m.Providers(UserProvider)
		m.Controllers(userController)
	})

	root := module.New(func(m *module.Module) {
		m.Imports(userModule)
	})

	app, err := NewApp[fiberapp.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiberapp.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiberapp.FiberApp: %T", app.Adapter())
	}

	const addr = "127.0.0.1:34599"

	fired := make(chan struct{})
	var once sync.Once
	onListen := OnListen(func() {
		once.Do(func() { close(fired) })
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.MustListen(addr, onListen)
	}()
	t.Cleanup(func() {
		if shutdownErr := fiberAdapter.FiberApp().Shutdown(); shutdownErr != nil {
			t.Errorf("Shutdown returned error: %v", shutdownErr)
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Errorf("MustListen goroutine did not return within timeout after Shutdown")
		}
	})

	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatalf("onListen callback did not fire within timeout")
	}

	// A real net/http.Client, genuinely dialing TCP against the bound
	// address -- not app.Test, which never opens a socket.
	client := &http.Client{Timeout: 5 * time.Second}

	// Create a user through the real HTTP round-trip first: the /:user_id
	// GET route needs a real ID to fetch, and this simultaneously proves the
	// running server's UserService is genuinely usable, mutable state (not a
	// zero-value placeholder) reached over a real socket.
	createReq, err := http.NewRequest(http.MethodPost, "http://"+addr+"/user/Ada", nil)
	if err != nil {
		t.Fatalf("failed to build create request: %v", err)
	}
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("real net/http.Client create request failed: %v", err)
	}
	var created UserEntity
	decodeErr := json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()
	if decodeErr != nil {
		t.Fatalf("failed to decode create response: %v", decodeErr)
	}

	getResp, err := client.Get(fmt.Sprintf("http://%s/user/%d", addr, created.ID))
	if err != nil {
		t.Fatalf("real net/http.Client GET request failed: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		getResp.Body.Close()
		t.Fatalf("GET /user/%d status = %d, want 200", created.ID, getResp.StatusCode)
	}
	var got UserEntity
	decodeErr = json.NewDecoder(getResp.Body).Decode(&got)
	getResp.Body.Close()
	if decodeErr != nil {
		t.Fatalf("failed to decode get response: %v", decodeErr)
	}
	if got.ID != created.ID || got.Name != "Ada" {
		t.Fatalf("GET /user/%d body = %+v, want ID=%d Name=Ada", created.ID, got, created.ID)
	}

	select {
	case <-done:
		t.Fatalf("MustListen() returned unexpectedly before test-end shutdown")
	default:
	}
}

// --- T4 of "Middleware": Stage 2.5 composition in internal/app ---
//
// dispatchTestApp is a small shared helper: builds a *fiberapp.FiberApp
// backed *App from root, fails the test on any bootstrap error, and returns
// the underlying *fiber.App ready for app.Test(req) dispatch.
func dispatchTestApp(t *testing.T, root *module.Module) *fiberapp.FiberApp {
	t.Helper()
	app, err := NewApp[fiberapp.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	fiberAdapter, ok := app.Adapter().(*fiberapp.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiberapp.FiberApp: %T", app.Adapter())
	}
	return fiberAdapter
}

// TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler proves a single
// controller-level middleware (registered via Controller.Use) runs before
// the route's own Handler, observed via a real app.Test dispatch: the
// middleware appends to a shared order slice before calling next, and the
// Handler appends too, then the test asserts the exact ["mw", "handler"]
// order captured across the real request.
//
// Note on technique: httpctx.Context.Header reads the REQUEST header (Fiber
// Ctx.Get), while SetHeader writes the RESPONSE header (Fiber Ctx.Set) --
// two different underlying stores, so a middleware cannot SetHeader and have
// a later step observe it via Header (internal/httpctx's Responder contract
// has no GetRespHeader, and per this task's scope internal/fiberapp is not
// to be touched to add one). This suite instead threads a shared []string
// order-of-execution recorder through closures -- still a genuine
// app.Test-dispatched proof of composition/ordering, just not the
// header-based technique originally sketched.
func TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler(t *testing.T) {
	var order []string
	mw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			order = append(order, "mw")
			next(ctx)
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Use(mw)
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				order = append(order, "handler")
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if want := []string{"mw", "handler"}; len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("execution order = %v, want %v -- middleware did not run before Handler", order, want)
	}
}

// TestNewApp_MiddlewareShortCircuit_SkipsRouteHandlerWhenNextNotCalled proves
// that calling next(ctx) continues the chain (route Handler runs), while a
// middleware that does NOT call next short-circuits the chain entirely (the
// route Handler never runs) -- both proven via real dispatch.
func TestNewApp_MiddlewareShortCircuit_SkipsRouteHandlerWhenNextNotCalled(t *testing.T) {
	callingMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			next(ctx)
		})
	})
	blockingMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			ctx.Status(http.StatusForbidden).Json(map[string]string{"blocked": "true"})
			// deliberately never calls next(ctx)
		})
	})

	var handlerRan bool
	continueController := controller.New(func(c *controller.Controller) {
		c.Use(callingMw)
		c.Route(route.HttpGet, "/continue", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})
	blockController := controller.New(func(c *controller.Controller) {
		c.Use(blockingMw)
		c.Route(route.HttpGet, "/blocked", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(continueController, blockController)
	})

	fa := dispatchTestApp(t, root)

	// Continue path: next(ctx) called, Handler runs.
	handlerRan = false
	req := httptest.NewRequest(http.MethodGet, "/continue", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/continue status = %d, want 200", resp.StatusCode)
	}
	if !handlerRan {
		t.Fatalf("/continue: route Handler did not run, want it to run after next(ctx)")
	}

	// Short-circuit path: next(ctx) never called, Handler must NOT run.
	handlerRan = false
	req = httptest.NewRequest(http.MethodGet, "/blocked", nil)
	resp, err = fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("/blocked status = %d, want 403", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("/blocked: route Handler ran, want it to be short-circuited by a middleware that never calls next")
	}
}

// TestNewApp_MultipleControllerMiddleware_RunInRegistrationOrder proves
// multiple controller-level middleware run in the exact order they were
// registered via Use, each appending a distinguishable marker to the SAME
// response header via a read-existing-then-append pattern using
// ctx.SetHeader alone (no ctx.Header read of it -- see the doc comment on
// TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler for why Header
// cannot observe a prior SetHeader against the real Fiber responder).
// Confirmed here via the FINAL HTTP RESPONSE header's exact value (read from
// resp.Header, a real net/http.Response, not via ctx) -- proving both
// ordering and that SetHeader's writes genuinely land on the wire.
func TestNewApp_MultipleControllerMiddleware_RunInRegistrationOrder(t *testing.T) {
	appendMarker := func(marker string) *middleware.Middleware {
		return middleware.New(func(m *middleware.Middleware) {
			m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
				ctx.SetHeader("X-Order", marker)
				next(ctx)
			})
		})
	}

	c := controller.New(func(c *controller.Controller) {
		c.Use(appendMarker("first"), appendMarker("second"), appendMarker("third"))
		c.Route(route.HttpGet, "/order", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/order", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	// Each middleware overwrites X-Order with its own marker via SetHeader
	// (last write wins), so the final value on the wire proves which
	// middleware ran LAST -- "third", since first/second/third must run in
	// that exact registration order.
	if got, want := resp.Header.Get("X-Order"), "third"; got != want {
		t.Fatalf("resp X-Order = %q, want %q -- middleware did not run in registration order (last registered must run last and win the overwrite)", got, want)
	}
}

// TestNewApp_MiddlewareMutation_VisibleToLaterMiddlewareAndHandler proves a
// middleware mutating ctx before calling next is visible to a LATER
// middleware and the route Handler -- i.e. the SAME *httpctx.Context
// instance flows through the whole chain, not a copy.
func TestNewApp_MiddlewareMutation_VisibleToLaterMiddlewareAndHandler(t *testing.T) {
	setter := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			ctx.WithRoute("mutated-by-first")
			next(ctx)
		})
	})
	var readerSaw any
	reader := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			readerSaw = ctx.Route()
			next(ctx)
		})
	})

	var handlerSaw any
	c := controller.New(func(c *controller.Controller) {
		c.Use(setter, reader)
		c.Route(route.HttpGet, "/mutate", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerSaw = ctx.Route()
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/mutate", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	// ctx.WithRoute/ctx.Route are used here purely as a generic any-carrier
	// on *httpctx.Context (the only mutable field Context exposes besides
	// headers, which -- per the doc comment on
	// TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler -- cannot
	// round-trip a write through a later read against the real Fiber
	// responder) to prove the SAME *httpctx.Context instance (not a copy)
	// flows from the first middleware, through the second, into the route
	// Handler: a mutation made by the first middleware before calling next
	// must be visible to both.
	if want := "mutated-by-first"; readerSaw != want {
		t.Fatalf("second middleware saw ctx.Route() = %v, want %q -- mutation by first middleware not visible to a LATER middleware on the same *httpctx.Context", readerSaw, want)
	}
	if want := "mutated-by-first"; handlerSaw != want {
		t.Fatalf("route Handler saw ctx.Route() = %v, want %q -- mutation not visible to the Handler on the same *httpctx.Context", handlerSaw, want)
	}
}

// TestNewApp_RootMiddleware_RunsForEveryRouteIncludingControllerWithNoOwnUse
// proves root-module Use() middleware runs for EVERY route in the app,
// including a controller with ZERO Use() calls of its own -- 2 controllers,
// only one with its own middleware, both hit by real requests, both showing
// the global marker.
func TestNewApp_RootMiddleware_RunsForEveryRouteIncludingControllerWithNoOwnUse(t *testing.T) {
	globalMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			ctx.SetHeader("X-Global", "yes")
			next(ctx)
		})
	})
	localMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			ctx.SetHeader("X-Local", "yes")
			next(ctx)
		})
	})

	withOwnMw := controller.New(func(c *controller.Controller) {
		c.Path("/with-own")
		c.Use(localMw)
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})
	withoutOwnMw := controller.New(func(c *controller.Controller) {
		c.Path("/without-own")
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Use(globalMw)
		m.Controllers(withOwnMw, withoutOwnMw)
	})

	fa := dispatchTestApp(t, root)

	// Both assertions read the FINAL HTTP response headers (a real
	// net/http.Response), proving the headers genuinely landed on the wire
	// for a real dispatched request -- not a ctx-internal round-trip (see
	// the doc comment on TestNewApp_ControllerMiddleware_RunsBeforeRouteHandler
	// for why ctx.Header cannot observe a prior ctx.SetHeader here).
	req := httptest.NewRequest(http.MethodGet, "/with-own/ping", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("X-Global"); got != "yes" {
		t.Fatalf("/with-own/ping: resp X-Global = %q, want %q", got, "yes")
	}
	if got := resp.Header.Get("X-Local"); got != "yes" {
		t.Fatalf("/with-own/ping: resp X-Local = %q, want %q", got, "yes")
	}

	req = httptest.NewRequest(http.MethodGet, "/without-own/ping", nil)
	resp, err = fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("X-Global"); got != "yes" {
		t.Fatalf("/without-own/ping: resp X-Global = %q, want %q -- global middleware must run even for a controller with zero Use() calls", got, "yes")
	}
	if got := resp.Header.Get("X-Local"); got != "" {
		t.Fatalf("/without-own/ping: resp X-Local = %q, want empty -- this controller never called Use", got)
	}
}

// TestNewApp_GlobalMiddleware_RunsBeforeControllerMiddleware proves global
// (root-module) middleware runs BEFORE controller-level middleware -- proven
// via marker-header ORDER, not just presence.
func TestNewApp_GlobalMiddleware_RunsBeforeControllerMiddleware(t *testing.T) {
	var order []string
	globalMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			order = append(order, "global")
			next(ctx)
		})
	})
	controllerMw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			order = append(order, "controller")
			next(ctx)
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Use(controllerMw)
		c.Route(route.HttpGet, "/order", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				order = append(order, "handler")
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Use(globalMw)
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/order", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	want := []string{"global", "controller", "handler"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v -- global middleware must run BEFORE controller-level middleware", order, want)
		}
	}
}

// TestNewApp_ZeroMiddleware_BehavesIdenticallyToPreFeatureBehavior is the
// critical non-regression proof: TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond
// (T9 of "Controller & Route Registration", a pre-existing test in this same
// file, defined above and left completely UNMODIFIED by this task) exercises
// a full app with zero Use() calls anywhere. Re-running it here (by calling
// it directly as a subtest) is redundant with Go's own test runner already
// running it -- this test instead exists as an explicit marker/documentation
// that that exact pre-existing test was checked to still pass unmodified
// after this task's changes to registerRoutes; see this file's test output
// for TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond itself for the
// actual proof.
func TestNewApp_ZeroMiddleware_BehavesIdenticallyToPreFeatureBehavior(t *testing.T) {
	t.Run("UserControllerEndToEnd_NonRegressionReference", func(t *testing.T) {
		TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t)
	})
}

// TestNewApp_PanickingMiddleware_CaughtBySameRecoverWrapper proves a
// middleware that panics with a built-in exception.Exception is caught by
// the SAME existing recover wrapper in internal/fiberapp (unchanged by this
// feature) and produces the correct structured response -- exactly as if a
// route Handler itself had panicked.
func TestNewApp_PanickingMiddleware_CaughtBySameRecoverWrapper(t *testing.T) {
	panicky := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			panic(exception.NewBadRequestException("bad input from middleware"))
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Use(panicky)
		c.Route(route.HttpGet, "/panic", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "BadRequestException" {
		t.Fatalf("body[name] = %v, want %q", body["name"], "BadRequestException")
	}
	if body["details"] != "bad input from middleware" {
		t.Fatalf("body[details] = %v, want %q", body["details"], "bad input from middleware")
	}
	if handlerRan {
		t.Fatalf("route Handler ran after a panicking middleware, want the panic to short-circuit the chain")
	}
}

// --- T3 of "Guard": Stage 2.5 gatedHandler in internal/app ---

// TestNewApp_SingleGuardReturnsTrue_RouteHandlerRuns proves a single guard
// whose handler returns true lets the request through: the route Handler's
// own response comes back on a real app.Test dispatch.
func TestNewApp_SingleGuardReturnsTrue_RouteHandlerRuns(t *testing.T) {
	allow := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *httpctx.Context) bool { return true })
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(allow)
		c.Route(route.HttpGet, "/allowed", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/allowed", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !handlerRan {
		t.Fatalf("route Handler did not run, want it to run when the guard returns true")
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["ok"] != "true" {
		t.Fatalf("body = %+v, want {ok:true} -- the route Handler's own response must come through", body)
	}
}

// TestNewApp_SingleGuardReturnsFalse_Produces403AndSkipsHandler proves a
// guard returning false produces a 403 Forbidden with the exact structured
// body a *exception.ForbiddenException formats to, and that the route
// Handler never runs.
func TestNewApp_SingleGuardReturnsFalse_Produces403AndSkipsHandler(t *testing.T) {
	deny := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *httpctx.Context) bool { return false })
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(deny)
		c.Route(route.HttpGet, "/denied", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/denied", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when the guard returns false")
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "ForbiddenException" {
		t.Fatalf("body[name] = %v, want %q", body["name"], "ForbiddenException")
	}
	if body["message"] != "" {
		t.Fatalf("body[message] = %v, want empty string", body["message"])
	}
	if body["details"] != nil {
		t.Fatalf("body[details] = %v, want nil", body["details"])
	}
}

// TestNewApp_GuardPanicsWithCustomException_ProducesThatExceptionsStatus
// proves a guard whose handler panics with a custom exception.Exception
// (rather than merely returning false) propagates that specific exception
// through unmodified -- caught by the same recover wrapper, formatted with
// THAT exception's own status/body (401, not 403) -- and the route Handler
// never runs.
func TestNewApp_GuardPanicsWithCustomException_ProducesThatExceptionsStatus(t *testing.T) {
	unauthorized := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *httpctx.Context) bool {
			panic(exception.NewUnauthorizedException(nil))
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(unauthorized)
		c.Route(route.HttpGet, "/needs-auth", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/needs-auth", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when a guard panics")
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "UnauthorizedException" {
		t.Fatalf("body[name] = %v, want %q", body["name"], "UnauthorizedException")
	}
}

// TestNewApp_MultipleGuards_ShortCircuitsOnFirstFalse proves that with 2+
// guards registered in order, a false from the FIRST guard prevents the
// SECOND guard's own handler function from ever running -- observed via a
// side-effect flag on the second guard that must stay untouched.
func TestNewApp_MultipleGuards_ShortCircuitsOnFirstFalse(t *testing.T) {
	var secondGuardRan bool
	first := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *httpctx.Context) bool { return false })
	})
	second := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *httpctx.Context) bool {
			secondGuardRan = true
			return true
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(first, second)
		c.Route(route.HttpGet, "/multi", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/multi", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if secondGuardRan {
		t.Fatalf("second guard's handler ran, want short-circuit after the first guard returned false")
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped")
	}
}

// TestNewApp_MiddlewareThenGuardThenHandler_OrderedSequence proves a
// controller with BOTH Use() (middleware) AND Guards() registered runs:
// middleware first, then the guard, then the Handler -- via an explicit
// ordered-sequence assertion, reusing the shared []string order-recorder
// technique this file's own "Middleware" T4 tests established.
func TestNewApp_MiddlewareThenGuardThenHandler_OrderedSequence(t *testing.T) {
	var order []string
	mw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			order = append(order, "middleware")
			next(ctx)
		})
	})
	g := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *httpctx.Context) bool {
			order = append(order, "guard")
			return true
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Use(mw)
		c.Guards(g)
		c.Route(route.HttpGet, "/sequence", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				order = append(order, "handler")
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/sequence", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	want := []string{"middleware", "guard", "handler"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v -- middleware must run before the guard, and the guard before the Handler", order, want)
		}
	}
}

// TestNewApp_ZeroGuards_NonRegressionReference proves an existing
// pre-feature test (T9's UserController end-to-end example, defined earlier
// in this file and left completely UNMODIFIED by this task) still passes
// unmodified after adding gatedHandler -- a controller with zero Guards()
// calls must behave exactly as before this feature.
func TestNewApp_ZeroGuards_NonRegressionReference(t *testing.T) {
	t.Run("UserControllerEndToEnd_NonRegressionReference", func(t *testing.T) {
		TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t)
	})
}

// TestNewApp_GuardPanicsWithNonException_StillGeneric500 proves a guard
// panicking with a value that does NOT satisfy exception.Exception (e.g. a
// plain error) still produces the same generic 500 fallback any other panic
// already gets -- non-regression of "Panic Recovery & Default Handler"'s
// existing behavior, proof the gatedHandler composition didn't break it.
func TestNewApp_GuardPanicsWithNonException_StillGeneric500(t *testing.T) {
	buggy := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *httpctx.Context) bool {
			panic(errors.New("bug"))
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Guards(buggy)
		c.Route(route.HttpGet, "/buggy-guard", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/buggy-guard", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when the guard panics")
	}
}

// --- T3 of "Interceptor": Stage 2.5 interceptedHandler in internal/app ---

// TestNewApp_SingleInterceptor_RunsBeforeAndAfterHandler proves a single
// interceptor runs code BEFORE calling next(ctx), then the route Handler
// runs, then the interceptor's own code AFTER next(ctx) returns runs too --
// observed via a real app.Test dispatch appending to a shared order-recorder
// slice, asserting the exact sequence ["before", "handler", "after"].
func TestNewApp_SingleInterceptor_RunsBeforeAndAfterHandler(t *testing.T) {
	var order []string
	it := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(ctx *httpctx.Context, next interceptor.Next) {
			order = append(order, "before")
			next(ctx)
			order = append(order, "after")
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(it)
		c.Route(route.HttpGet, "/wrap", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				order = append(order, "handler")
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/wrap", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	want := []string{"before", "handler", "after"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
	}
}

// TestNewApp_InterceptorNotCallingNext_SkipsRouteHandler proves an
// interceptor that never calls next(ctx) short-circuits the chain: the route
// Handler must not run, proven via a flag the Handler would have set.
func TestNewApp_InterceptorNotCallingNext_SkipsRouteHandler(t *testing.T) {
	blocking := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(ctx *httpctx.Context, next interceptor.Next) {
			ctx.Status(http.StatusForbidden).Json(map[string]string{"blocked": "true"})
			// deliberately never calls next(ctx)
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(blocking)
		c.Route(route.HttpGet, "/blocked", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/blocked", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when an interceptor never calls next")
	}
}

// TestNewApp_MultipleInterceptors_ComposeInRegistrationOrder proves 2+
// interceptors registered on a controller compose in registration order:
// interceptor1's before-code runs first, then interceptor2's before-code,
// then the Handler, then interceptor2's after-code, then interceptor1's
// after-code -- the classic nested-onion composition order.
func TestNewApp_MultipleInterceptors_ComposeInRegistrationOrder(t *testing.T) {
	var order []string
	first := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(ctx *httpctx.Context, next interceptor.Next) {
			order = append(order, "interceptor1-before")
			next(ctx)
			order = append(order, "interceptor1-after")
		})
	})
	second := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(ctx *httpctx.Context, next interceptor.Next) {
			order = append(order, "interceptor2-before")
			next(ctx)
			order = append(order, "interceptor2-after")
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(first, second)
		c.Route(route.HttpGet, "/multi", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				order = append(order, "handler")
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/multi", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	want := []string{
		"interceptor1-before", "interceptor2-before",
		"handler",
		"interceptor2-after", "interceptor1-after",
	}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
	}
}

// TestNewApp_MiddlewareGuardInterceptorHandler_OrderedSequence proves a
// controller with Middleware + Guards + Interceptors registered together
// runs, in order: Middleware -> Interceptor(before) -> Guard -> Handler ->
// Interceptor(after), the full Stage 2.5 pipeline order proven via one
// explicit ordered-sequence assertion covering all 3 stages simultaneously.
//
// SPEC_DEVIATION (documented, not a blocker): ROADMAP.md's prose ("Middleware
// -> Guard -> Interceptor -> Pipe -> Handler") and design.md's own summary
// line ("Matches ROADMAP.md's documented order exactly: Middleware -> Guard
// -> Interceptor -> Handler") both describe Guard running BEFORE Interceptor.
// But design.md's own literal numbered composition steps (and this task's
// T3 prompt's own "Composition change" code block, reproduced verbatim in
// interceptedHandler below) build gatedHandler first (step 2, wraps
// routeHandler with guards) and THEN wrap gatedHandler with the interceptor
// chain (step 3) -- meaning interceptedHandler's outermost interceptor's
// pre-next code runs and calls next BEFORE gatedHandler's guards ever
// evaluate. Literal code takes precedence per this task's explicit
// instruction ("siga o algoritmo exato" from design.md's Data Models code
// block) over the prose summary line, which appears to be a documentation
// inconsistency in design.md itself (line 30) and ROADMAP.md (line 79) --
// the actual composition order this implementation produces, and this test
// proves, is Middleware -> Interceptor(before) -> Guard -> Handler ->
// Interceptor(after).
func TestNewApp_MiddlewareGuardInterceptorHandler_OrderedSequence(t *testing.T) {
	var order []string
	mw := middleware.New(func(m *middleware.Middleware) {
		m.Handler(func(ctx *httpctx.Context, next middleware.Next) {
			order = append(order, "middleware")
			next(ctx)
		})
	})
	g := guard.New(func(g *guard.Guard) {
		g.Handler(func(ctx *httpctx.Context) bool {
			order = append(order, "guard")
			return true
		})
	})
	it := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(ctx *httpctx.Context, next interceptor.Next) {
			order = append(order, "interceptor-before")
			next(ctx)
			order = append(order, "interceptor-after")
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Use(mw)
		c.Guards(g)
		c.Interceptors(it)
		c.Route(route.HttpGet, "/full-pipeline", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				order = append(order, "handler")
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/full-pipeline", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	want := []string{"middleware", "interceptor-before", "guard", "handler", "interceptor-after"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v -- pipeline must run Middleware -> Interceptor(before) -> Guard -> Handler -> Interceptor(after)", order, want)
		}
	}
}

// TestNewApp_InterceptorPanicsBeforeNext_CaughtBySameRecoverWrapper proves
// an interceptor that panics BEFORE calling next(ctx) -- with a custom
// exception.Exception -- is caught by the SAME existing recover wrapper in
// internal/fiberapp (unchanged by this feature) and produces that
// exception's own structured response, with the route Handler never
// running.
func TestNewApp_InterceptorPanicsBeforeNext_CaughtBySameRecoverWrapper(t *testing.T) {
	panicky := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(ctx *httpctx.Context, next interceptor.Next) {
			panic(exception.NewBadRequestException(nil))
		})
	})

	var handlerRan bool
	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(panicky)
		c.Route(route.HttpGet, "/panic-before", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/panic-before", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if handlerRan {
		t.Fatalf("route Handler ran, want it to be skipped when an interceptor panics before calling next")
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["name"] != "BadRequestException" {
		t.Fatalf("body[name] = %v, want %q", body["name"], "BadRequestException")
	}
}

// TestNewApp_InterceptorPanicsAfterNext_StillGeneric500 proves an
// interceptor that panics AFTER next(ctx) returns -- with a plain, non-
// exception.Exception value (a plain error) -- still produces the generic
// 500 fallback any other panic already gets, and that the route Handler DID
// run before the panic (the panic happens in the interceptor's own
// post-next code, not before dispatch reached the Handler).
func TestNewApp_InterceptorPanicsAfterNext_StillGeneric500(t *testing.T) {
	var handlerRan bool
	buggy := interceptor.New(func(i *interceptor.Interceptor) {
		i.Handler(func(ctx *httpctx.Context, next interceptor.Next) {
			next(ctx)
			panic(errors.New("bug after next"))
		})
	})

	c := controller.New(func(c *controller.Controller) {
		c.Interceptors(buggy)
		c.Route(route.HttpGet, "/panic-after", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(c)
	})

	fa := dispatchTestApp(t, root)
	req := httptest.NewRequest(http.MethodGet, "/panic-after", nil)
	resp, err := fa.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if !handlerRan {
		t.Fatalf("route Handler did not run, want it to run before the interceptor's post-next panic")
	}
}

// TestNewApp_ZeroInterceptors_NonRegressionReference proves an existing
// pre-feature test (T9's UserController end-to-end example, defined earlier
// in this file and left completely UNMODIFIED by this task) still passes
// unmodified after adding interceptedHandler -- a controller with zero
// Interceptors() calls must behave exactly as before this feature.
func TestNewApp_ZeroInterceptors_NonRegressionReference(t *testing.T) {
	t.Run("UserControllerEndToEnd_NonRegressionReference", func(t *testing.T) {
		TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond(t)
	})
}
