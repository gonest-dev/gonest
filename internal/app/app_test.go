package app

import (
	"errors"
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

func (t *UserService) Create(name string) *UserEntity {
	t.index++
	u := &UserEntity{ID: int64(t.index), Name: name}
	t.list = append(t.list, u)
	return u
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

	app, err := NewApp[recordingFakeAdapter](appModule)
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

	MustNewApp[recordingFakeAdapter](root)
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

	app := MustNewApp[recordingFakeAdapter](root)
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

	_, err := NewApp[recordingFakeAdapter](root)
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

	firstApp, err := NewApp[recordingFakeAdapter](firstRoot)
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

	secondApp, err := NewApp[recordingFakeAdapter](secondRoot)
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

	_, err := NewApp[recordingFakeAdapter](root)
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

	app, err := NewApp[recordingFakeAdapter](root)
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

	_, err := NewApp[recordingFakeAdapter](root)
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

	app, err := NewApp[recordingFakeAdapter](root)
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

	_, err := NewApp[recordingFakeAdapter](root)
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

	app, err := NewApp[fiberapp.FiberApp](root)
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
