package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gonest-dev/gonest/internal/controller"
	"github.com/gonest-dev/gonest/internal/fiberapp"
	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/inject"
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

func (f *recordingFakeAdapter) Listen(addr string) error {
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
