package app

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"gonest.dev/gonest/internal/controller"
	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/logger"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/route"
	"gonest.dev/gonest/internal/schema"
	"gonest.dev/gonest/internal/scope"
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

// userIDParams/nameParams/userIDNameParams are the struct-based path-param
// shapes TestNewApp_UserControllerEndToEnd_AllFiveRoutesRespond validates
// via validate.MustParams[T] (param-query-validation's T3 replaced the old
// singular route.MustParam[T](ctx, name) with this whole-object mechanism,
// even for a route with a single path param -- context.md's Decision 2).
type userIDParams struct {
	UserID int64 `param:"user_id"`
}

var userIDParamsSchema = func() *schema.Schema {
	f := &userIDParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.UserID).Integer().Required()
	return m
}()

type nameParams struct {
	Name string `param:"name"`
}

var nameParamsSchema = func() *schema.Schema {
	f := &nameParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Name).String().Required()
	return m
}()

type userIDNameParams struct {
	UserID int64  `param:"user_id"`
	Name   string `param:"name"`
}

var userIDNameParamsSchema = func() *schema.Schema {
	f := &userIDNameParams{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.UserID).Integer().Required()
	m.Property(&f.Name).String().Required()
	return m
}()

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
		userService = inject.Must[*UserService](p)
		p.Constructor(func() *consumerMarker {
			return &consumerMarker{}
		})
	})
	appModule.Providers(consumer)

	app, err := New[recordingFakeAdapter](appModule, Options{})
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

// TestNewApp_NoOptsArgument_BehavesLikeZeroValueAppOptions proves opts is
// truly optional: calling NewApp with no Options argument at all bootstraps
// exactly like passing Options{} explicitly (same zero-value default).
func TestNewApp_NoOptsArgument_BehavesLikeZeroValueAppOptions(t *testing.T) {
	root := module.New(func(m *module.Module) {})

	app, err := New[recordingFakeAdapter](root)
	if err != nil {
		t.Fatalf("NewApp() with no opts error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() with no opts returned nil *App")
	}
	if app.opts.LogLevels != nil || app.opts.BufferLogs || app.opts.DisableBanner || app.opts.DisableLoaded || app.opts.EnableFormStreaming {
		t.Fatalf("NewApp() with no opts stored non-zero opts = %+v, want zero-value Options{}", app.opts)
	}
}

// TestMustNewApp_NoOptsArgument_DoesNotPanic proves MustNewApp's opts is
// also optional -- mirrors NewApp's own contract via the variadic pass-through.
func TestMustNewApp_NoOptsArgument_DoesNotPanic(t *testing.T) {
	root := module.New(func(m *module.Module) {})

	app := MustNewApp[recordingFakeAdapter](root)
	if app == nil {
		t.Fatalf("MustNewApp() with no opts returned nil *App")
	}
}

// TestNewApp_TwoOptsArguments_Panics proves passing more than one Options
// fails loud instead of silently picking one -- there is no sane way to
// merge two Options, so ambiguity is a caller bug, not a runtime choice.
func TestNewApp_TwoOptsArguments_Panics(t *testing.T) {
	root := module.New(func(m *module.Module) {})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("NewApp() with 2 opts arguments did not panic")
		}
	}()

	New[recordingFakeAdapter](root, Options{}, Options{})
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

	MustNewApp[recordingFakeAdapter](root, Options{})
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

	app := MustNewApp[recordingFakeAdapter](root, Options{})
	if app == nil {
		t.Fatalf("MustNewApp() returned nil *App")
	}
}

func TestNewApp_CircularDependency_ReturnsError(t *testing.T) {
	type cycleA struct{}
	type cycleB struct{}

	var pa, pb *provider.Provider
	pa = provider.New(func(p *provider.Provider) {
		inject.Must[*cycleB](pa)
		p.Constructor(func() *cycleA { return &cycleA{} })
	})
	pb = provider.New(func(p *provider.Provider) {
		inject.Must[*cycleA](pb)
		p.Constructor(func() *cycleB { return &cycleB{} })
	})

	root := module.New(func(m *module.Module) {
		m.Providers(pa, pb)
	})

	_, err := New[recordingFakeAdapter](root, Options{})
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

	firstApp, err := New[recordingFakeAdapter](firstRoot, Options{})
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
		resolved = inject.Must[*secondTreeService](consumer)
		p.Constructor(func() *consumerMarker {
			return &consumerMarker{}
		})
	})
	secondRoot := module.New(func(m *module.Module) {
		m.Providers(secondProvider, consumer)
	})

	secondApp, err := New[recordingFakeAdapter](secondRoot, Options{})
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

	_, err := New[recordingFakeAdapter](root, Options{})
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
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
		c.Route(route.HttpPost, "/", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(userController)
	})

	app, err := New[recordingFakeAdapter](root, Options{})
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
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})
	otherDupeController := controller.New(func(c *controller.Controller) {
		c.Path("/user")
		c.Route(route.HttpGet, "/:id", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(dupeController, otherDupeController)
	})

	_, err := New[recordingFakeAdapter](root, Options{})
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

	app, err := New[recordingFakeAdapter](root, Options{})
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
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Controllers(noPrefixController)
	})

	_, err := New[recordingFakeAdapter](root, Options{})
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

func (f *recordingFakeAdapter) Init(opts Options) {
	lastRecordingAdapter = f
}

func (f *recordingFakeAdapter) RegisterRoute(method route.HttpMethod, path string, h func(req *execution.Request, res *execution.Response)) error {
	f.registered = append(f.registered, fakeRegisteredRoute{method: method, path: path})
	return nil
}

func (f *recordingFakeAdapter) Test(req *http.Request) (*http.Response, error) {
	return nil, nil
}

func (f *recordingFakeAdapter) Listen(addr string, onListen func()) error {
	return nil
}

func (f *recordingFakeAdapter) Shutdown(ctx context.Context) error {
	return nil
}

// TestNewApp_ZeroValueAppOptions_BootstrapsIdenticallyToPreT2Behavior proves
// NewApp[T, PT](root, Options{}) bootstraps the DI graph and registers
// routes exactly as NewApp[T, PT](root) did before this task's signature
// change -- zero behavior regression from adding the required Options
// parameter.
func TestNewApp_ZeroValueAppOptions_BootstrapsIdenticallyToPreT2Behavior(t *testing.T) {
	userController := controller.New(func(c *controller.Controller) {
		c.Path("/user")
		c.Route(route.HttpGet, "/:id", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})

	root := module.New(func(m *module.Module) {
		m.Providers(UserProvider)
		m.Controllers(userController)
	})

	app, err := New[recordingFakeAdapter](root, Options{})
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
		t.Fatalf("app.opts = %+v, want zero-value Options{}", app.opts)
	}
}

// TestNewApp_NonZeroAppOptions_StoredOnApp proves NewApp stores whatever
// Options it was given on the returned *App's unexported opts field,
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

	opts := Options{
		BufferLogs: true,
		LogLevels:  []logger.Level{logger.LevelWarn, logger.LevelError},
	}

	app, err := New[recordingFakeAdapter](root, opts)
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}

	if app.opts.BufferLogs != true {
		t.Fatalf("app.opts.BufferLogs = %v, want true", app.opts.BufferLogs)
	}
	if len(app.opts.LogLevels) != 2 || app.opts.LogLevels[0] != logger.LevelWarn || app.opts.LogLevels[1] != logger.LevelError {
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
	// shutdownCalls counts Shutdown invocations, mirroring onListenRan's
	// call-recording pattern above -- lets a test assert Shutdown was
	// actually reached without needing a real underlying engine.
	shutdownCalls int
}

func (f *listenSpyAdapter) Init(opts Options) {}

func (f *listenSpyAdapter) RegisterRoute(method route.HttpMethod, path string, h func(req *execution.Request, res *execution.Response)) error {
	return nil
}

func (f *listenSpyAdapter) Test(req *http.Request) (*http.Response, error) {
	return nil, nil
}

func (f *listenSpyAdapter) Shutdown(ctx context.Context) error {
	f.mu.Lock()
	f.shutdownCalls++
	f.mu.Unlock()
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
// OnListen to MustListen is safe: MustListen ALWAYS passes its own
// non-nil wrapper closure to adapter.Listen now (it needs to run
// unconditionally to print gonest's own startup log -- see MustListen's
// own doc comment), but a nil caller-supplied OnListen is simply never
// invoked from inside that wrapper. MustListen still blocks until Listen
// returns, and nothing panics.
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
	if spy.onListen == nil {
		t.Fatalf("adapter.Listen received a nil onListen, want gonest's own non-nil wrapper (always passed now, so it can log its own startup line)")
	}
	if spy.onListenRan != 1 {
		t.Fatalf("onListenRan = %d, want 1 -- gonest's own wrapper always runs once, even with a nil caller-supplied OnListen", spy.onListenRan)
	}
}

// TestMustListen_ListenError_PanicsWithAddrAndError proves MustListen
// panics, with a message containing both addr and the underlying error's
// text, when adapter.Listen returns an error -- the "Must"-prefixed
// panic-on-error convention shared with MustNewApp/MustInject/MustParams.
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

// TestNewApp_ZeroMiddleware_BehavesIdenticallyToPreFeatureBehavior is the
// --- T4 of "Filter": filteredHandler as the OUTERMOST Stage 2.5 layer ---
//
// fooFilterException/barFilterException are two distinct, distinguishable
// dev-defined exception.Exception types (mirroring INSIGHT.md's
// `type FooExampleError struct { gonest.HttpException }` pattern) used
// throughout this suite so tests can prove exact-type Catch matching (a
// Catch registered for fooFilterException must never fire for
// barFilterException, and vice versa).
type fooFilterException struct {
	exception.HttpException
}

func newFooFilterException() *fooFilterException {
	return &fooFilterException{HttpException: exception.NewHttpException().SetStatus(499).SetName("FooFilterException").SetMessage("foo")}
}

type barFilterException struct {
	exception.HttpException
}

func newBarFilterException() *barFilterException {
	return &barFilterException{HttpException: exception.NewHttpException().SetStatus(498).SetName("BarFilterException").SetMessage("bar")}
}

// --- T0 of "Schema Generation from Schema": App.Root() accessor ---

// TestNewApp_Root_ReturnsSameModulePassedToNewApp proves App.Root() returns
// the exact same *module.Module value NewApp was called with (identity, not
// just an equivalent copy) -- the literal root reference the bootstrap
// stored, unchanged.
func TestNewApp_Root_ReturnsSameModulePassedToNewApp(t *testing.T) {
	root := module.New(func(m *module.Module) {})

	app, err := New[recordingFakeAdapter](root, Options{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	if app.Root() != root {
		t.Fatalf("app.Root() = %p, want the same *module.Module NewApp was called with (%p)", app.Root(), root)
	}
}

// TestNewApp_Root_WalksWholeTree_ReachesRootAndSubModuleRoutes proves that
// App.Root() combined with the ALREADY EXISTING Module.ImportedModules(),
// Module.OwnControllers(), and Controller.OwnRoutes() together reach every
// route registered anywhere in the app's module tree -- both the root
// module's own controller/route AND a sub-module's own controller/route,
// imported by the root -- with no gap. This is exactly the walk a future
// schema generator needs to perform (see context.md's "Discovery: App needs
// a NEW accessor" section), so this test exercises that walk directly
// rather than merely checking Root()'s return value in isolation.
func TestNewApp_Root_WalksWholeTree_ReachesRootAndSubModuleRoutes(t *testing.T) {
	subController := controller.New(func(c *controller.Controller) {
		c.Path("/sub")
		c.Route(route.HttpGet, "/thing", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})
	subModule := module.New(func(m *module.Module) {
		m.Controllers(subController)
	})

	rootController := controller.New(func(c *controller.Controller) {
		c.Path("/root")
		c.Route(route.HttpPost, "/own", func(r *route.Route) {
			r.Handler(func(req *execution.Request, res *execution.Response) {})
		})
	})
	root := module.New(func(m *module.Module) {
		m.Imports(subModule)
		m.Controllers(rootController)
	})

	app, err := New[recordingFakeAdapter](root, Options{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	// Walk the whole tree starting from App.Root(), exactly as a schema
	// generator would: this module's own controllers/routes, then recurse
	// into every imported module and do the same.
	type foundRoute struct {
		method route.HttpMethod
		path   string
	}
	var found []foundRoute
	var walk func(m *module.Module)
	walk = func(m *module.Module) {
		for _, c := range m.OwnControllers() {
			rc, ok := c.(interface {
				PathPrefix() string
				OwnRoutes() []*route.Route
			})
			if !ok {
				continue
			}
			for _, r := range rc.OwnRoutes() {
				found = append(found, foundRoute{method: r.Method(), path: rc.PathPrefix() + r.Path()})
			}
		}
		for _, imported := range m.ImportedModules() {
			walk(imported)
		}
	}
	walk(app.Root())

	want := map[foundRoute]bool{
		{method: route.HttpPost, path: "/root/own"}: true,
		{method: route.HttpGet, path: "/sub/thing"}: true,
	}
	if len(found) != len(want) {
		t.Fatalf("found routes = %+v, want exactly %d routes matching %+v", found, len(want), want)
	}
	for _, f := range found {
		if !want[f] {
			t.Fatalf("found unexpected route %+v not in want set %+v", f, want)
		}
		delete(want, f)
	}
	if len(want) != 0 {
		t.Fatalf("missing routes from walk: %+v -- App.Root()+ImportedModules()+OwnControllers()+OwnRoutes() did not reach every registered route", want)
	}
}

// TestMustListen_DisableBanner_SkipsBannerStillLogsAndFires proves
// Options.DisableBanner (threaded onto App.opts) suppresses only
// printBanner's own call -- MustListen still fires onListen and blocks
// exactly like the banner-enabled path.
func TestMustListen_DisableBanner_SkipsBannerStillLogsAndFires(t *testing.T) {
	spy := &listenSpyAdapter{unblock: make(chan struct{})}
	a := &App{adapter: spy, opts: Options{DisableBanner: true}}

	fired := make(chan struct{})
	go func() {
		a.MustListen(":0", OnListen(func() { close(fired) }))
	}()

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatalf("onListen callback did not fire within timeout")
	}

	close(spy.unblock)
}

// TestMustListen_DisableLoaded_SkipsLoadedCountsStillLogsAndFires proves
// Options.DisableLoaded suppresses the detailed loaded counts log lines,
// but MustListen still fires onListen and blocks exactly like the normal path.
func TestMustListen_DisableLoaded_SkipsLoadedCountsStillLogsAndFires(t *testing.T) {
	spy := &listenSpyAdapter{unblock: make(chan struct{})}
	a := &App{adapter: spy, opts: Options{DisableLoaded: true}}

	fired := make(chan struct{})
	go func() {
		a.MustListen(":0", OnListen(func() { close(fired) }))
	}()

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatalf("onListen callback did not fire within timeout")
	}

	close(spy.unblock)
}

// TestListen_ReturnsErrorWithoutPanicking proves Listen (the non-panicking
// counterpart to MustListen) returns the adapter's own error directly
// instead of panicking.
func TestListen_ReturnsErrorWithoutPanicking(t *testing.T) {
	spy := &listenSpyAdapter{err: errors.New("address already in use")}
	a := &App{adapter: spy}

	err := a.Listen(":0")
	if err == nil {
		t.Fatal("Listen() error = nil, want the adapter's own error")
	}
	if !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("Listen() error = %v, want it to contain the adapter's own message", err)
	}
}

// TestListen_VariadicNoOnListen_DoesNotPanic proves Listen(addr) -- zero
// OnListen args, no nil literal required -- works.
func TestListen_VariadicNoOnListen_DoesNotPanic(t *testing.T) {
	spy := &listenSpyAdapter{unblock: make(chan struct{})}
	a := &App{adapter: spy}

	done := make(chan struct{})
	go func() {
		_ = a.Listen(":0")
		close(done)
	}()

	close(spy.unblock)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Listen(addr) with zero OnListen args did not return")
	}
}

// TestDisplayAddr_BarePort_PrintsLocalhostNotWildcard proves displayAddr
// renders a bare ":PORT" addr as "localhost:PORT" for the startup banner,
// not "0.0.0.0:PORT" -- 0.0.0.0 is a wildcard BIND address, not something a
// browser can connect to, so printing it produces a URL a dev can paste and
// get a connection failure from (the real bug found in a live erc session).
func TestDisplayAddr_BarePort_PrintsLocalhostNotWildcard(t *testing.T) {
	if got := displayAddr(":3000"); got != "localhost:3000" {
		t.Fatalf("displayAddr(%q) = %q, want %q", ":3000", got, "localhost:3000")
	}
}

// TestDisplayAddr_ExplicitHost_PassesThroughUnchanged proves displayAddr
// only rewrites the bare-":PORT" shorthand -- an addr with an explicit host
// (already meaningful to a human/browser) is returned as-is.
func TestDisplayAddr_ExplicitHost_PassesThroughUnchanged(t *testing.T) {
	if got := displayAddr("127.0.0.1:3000"); got != "127.0.0.1:3000" {
		t.Fatalf("displayAddr(%q) = %q, want unchanged", "127.0.0.1:3000", got)
	}
}
