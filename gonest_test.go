package gonest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gonest-dev/gonest/internal/adapter/fiber"
	"github.com/gonest-dev/gonest/internal/execution"
	interceptorpkg "github.com/gonest-dev/gonest/internal/interceptor"
	"github.com/gonest-dev/gonest/internal/pipe"
	"github.com/gonest-dev/gonest/internal/route"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// App / bootstrap
// ---------------------------------------------------------------------------

// TestNewApp_RootAlias_InsightCallShape proves the exact INSIGHT.md call
// shape gonest.NewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})
// compiles and works through the root gonest package. gonest.FiberApp does
// not exist as a root alias yet (a pre-existing gap from an earlier
// feature) -- fiber.FiberApp is used directly here via import instead.
func TestNewApp_RootAlias_InsightCallShape(t *testing.T) {
	root := NewModule(func(m *Module) {})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if app == nil {
		t.Fatalf("NewApp() returned nil *App")
	}
}

// TestMustNewApp_RootAlias_InsightCallShape proves
// gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})
// compiles and works through the root gonest package.
func TestMustNewApp_RootAlias_InsightCallShape(t *testing.T) {
	root := NewModule(func(m *Module) {})

	app := MustNewApp[fiber.FiberApp](root, AppOptions{
		BufferLogs: true,
		LogLevels:  []LogLevel{LogLevelWarn, LogLevelError},
	})
	if app == nil {
		t.Fatalf("MustNewApp() returned nil *App")
	}
}

// TestApp_MustListen_PromotedThroughRootAlias proves App.MustListen (added
// on internal/app.App) is automatically visible on the root gonest.App
// alias with zero extra wrapper code, and that both
// app.MustListen(addr, gonest.OnListen(fn)) and
// app.MustListen(addr, nil) compile and work through the root alias.
func TestApp_MustListen_PromotedThroughRootAlias(t *testing.T) {
	root := NewModule(func(m *Module) {})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	const addr = "127.0.0.1:34589"

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

// TestApp_MustListen_NilOnListen_ThroughRootAlias proves
// app.MustListen(addr, nil) compiles and works through the root alias.
func TestApp_MustListen_NilOnListen_ThroughRootAlias(t *testing.T) {
	root := NewModule(func(m *Module) {})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	const addr = "127.0.0.1:34590"

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.MustListen(addr, nil)
	}()

	// Give MustListen a moment to bind, then confirm it's serving without
	// having panicked -- a nil OnListen must be safe end-to-end.
	var resp *http.Response
	var err2 error
	for i := 0; i < 50; i++ {
		resp, err2 = http.Get("http://" + addr + "/")
		if err2 == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err2 != nil {
		t.Fatalf("http.Get error = %v", err2)
	}
	resp.Body.Close()

	select {
	case <-done:
		t.Fatalf("MustListen() returned unexpectedly before shutdown")
	default:
	}
}

// ---------------------------------------------------------------------------
// Route params
// ---------------------------------------------------------------------------

// paramFakeResponder is a minimal test-only execution.Responder for exercising
// MustParam[T] end to end (Context -> Route -> Pipe/defaultCoerce).
type paramFakeResponder struct {
	params map[string]string
}

func newParamFakeResponder() *paramFakeResponder {
	return &paramFakeResponder{params: map[string]string{}}
}

func (f *paramFakeResponder) JSON(v any) error                  { return nil }
func (f *paramFakeResponder) SetStatus(code int)                {}
func (f *paramFakeResponder) GetHeader(name string) string      { return "" }
func (f *paramFakeResponder) SetHeaderValue(name, value string) {}
func (f *paramFakeResponder) GetParam(name string) string       { return f.params[name] }

// TestMustParam_WithoutCustomPipe_UsesDefaultCoerce proves that when a
// Route has no custom Pipe registered for a param name, MustParam[T] falls
// back to the default reflect+strconv coercion.
func TestMustParam_WithoutCustomPipe_UsesDefaultCoerce(t *testing.T) {
	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {})

	res := newParamFakeResponder()
	res.params["id"] = "42"
	ctx := execution.New(res).WithRoute(r)

	got := MustParam[int](ctx, "id")
	if got != 42 {
		t.Fatalf("MustParam[int](ctx, \"id\") = %d, want %d", got, 42)
	}
}

// TestMustParam_WithCustomPipe_UsesCustomPipeInsteadOfDefault proves that
// when the current Route has a custom Pipe registered for a param name (via
// Route.Param), MustParam[T] runs that Pipe's Handler instead of
// defaultCoerce.
func TestMustParam_WithCustomPipe_UsesCustomPipeInsteadOfDefault(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *execution.Context, raw string) int {
			// Deliberately does NOT match defaultCoerce's behavior (would
			// return 42 for raw "42") -- proves the custom Pipe ran, not
			// the default coercion.
			return 999
		})
	})
	p.Declare()

	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {
		r.Param("id", p)
	})

	res := newParamFakeResponder()
	res.params["id"] = "42"
	ctx := execution.New(res).WithRoute(r)

	got := MustParam[int](ctx, "id")
	if got != 999 {
		t.Fatalf("MustParam[int](ctx, \"id\") = %d, want %d (from custom Pipe)", got, 999)
	}
}

// TestMustParam_PanicsWhenParamNotDeclaredOnRoute proves MustParam[T] panics
// with the distinct "no param named" message when the current Route's
// declared path doesn't have a ":name" segment for the requested name.
func TestMustParam_PanicsWhenParamNotDeclaredOnRoute(t *testing.T) {
	r := route.New(route.HttpGet, "/users", func(r *route.Route) {})

	res := newParamFakeResponder()
	ctx := execution.New(res).WithRoute(r)

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected MustParam to panic for a param not declared on the route, got no panic")
		}
		msg, ok := rec.(string)
		if !ok {
			t.Fatalf("expected panic value to be a string, got %T: %v", rec, rec)
		}
		want := `gonest: no param named "id" on this route`
		if msg != want {
			t.Fatalf("panic message = %q, want %q", msg, want)
		}
	}()

	MustParam[int](ctx, "id")
}

// TestMustParam_PanicsOnConversionFailure_DefaultCoerce proves MustParam[T]
// panics with the distinct "could not be converted" message when the raw
// value exists but fails to convert to T via defaultCoerce.
func TestMustParam_PanicsOnConversionFailure_DefaultCoerce(t *testing.T) {
	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {})

	res := newParamFakeResponder()
	res.params["id"] = "not-a-number"
	ctx := execution.New(res).WithRoute(r)

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected MustParam to panic on conversion failure, got no panic")
		}
		msg, ok := rec.(string)
		if !ok {
			t.Fatalf("expected panic value to be a string, got %T: %v", rec, rec)
		}
		if !stringsContains(msg, `gonest: param "id" could not be converted to int`) {
			t.Fatalf("panic message = %q, want prefix %q", msg, `gonest: param "id" could not be converted to int`)
		}
	}()

	MustParam[int](ctx, "id")
}

// TestMustParam_PanicsWhenCustomPipeHandlerPanics proves that if the custom
// Pipe's Handler itself panics, MustParam lets that panic propagate as-is
// (pass-through, not caught/rewrapped).
func TestMustParam_PanicsWhenCustomPipeHandlerPanics(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *execution.Context, raw string) int {
			panic("custom pipe exploded")
		})
	})
	p.Declare()

	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {
		r.Param("id", p)
	})

	res := newParamFakeResponder()
	res.params["id"] = "42"
	ctx := execution.New(res).WithRoute(r)

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected the custom Pipe's panic to propagate, got no panic")
		}
		msg, ok := rec.(string)
		if !ok || msg != "custom pipe exploded" {
			t.Fatalf("expected panic value %q to pass through unchanged, got %v", "custom pipe exploded", rec)
		}
	}()

	MustParam[int](ctx, "id")
}

// TestMustParam_WithoutAttachedRoute_UsesDefaultCoerce proves MustParam[T]
// works even when Context has no attached Route (Route() returns nil) --
// falls back straight to defaultCoerce.
func TestMustParam_WithoutAttachedRoute_UsesDefaultCoerce(t *testing.T) {
	res := newParamFakeResponder()
	res.params["id"] = "7"
	ctx := execution.New(res)

	got := MustParam[int](ctx, "id")
	if got != 7 {
		t.Fatalf("MustParam[int](ctx, \"id\") = %d, want %d", got, 7)
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// TestNewPipe_RootAlias_TypeCheck proves NewPipe/Pipe resolve and
// type-check at the root gonest package: NewPipe builds a *Pipe, Handler
// accepts a valid func(ctx, raw string) T signature (validated via
// reflect), and route.Route.Param genuinely declares it (running the
// deferred fn) without the caller needing to call Declare manually.
func TestNewPipe_RootAlias_TypeCheck(t *testing.T) {
	p := NewPipe(func(p *Pipe) {
		p.Handler(func(ctx *execution.Context, raw string) int {
			return 0
		})
	})
	if p == nil {
		t.Fatal("NewPipe() returned nil *Pipe")
	}

	r := route.New(route.HttpGet, "/x/:n", func(r *route.Route) {
		r.Param("n", p)
	})

	got, ok := r.PipeFor("n")
	if !ok {
		t.Fatal("expected PipeFor(\"n\") to report ok=true")
	}
	if !got.HandlerFunc().IsValid() {
		t.Fatal("expected Route.Param to have declared the Pipe (HandlerFunc() valid) without a manual Declare() call")
	}
}

// ParseIntPipe reproduces INSIGHT.md's own ParseIntPipe example verbatim
// through the root gonest package's Pipe/NewPipe aliases: parses raw into
// an int64, panicking a BadRequestException with the invalid raw value as
// Details on failure.
var ParseIntPipe = NewPipe(func(pipe *Pipe) {
	pipe.Handler(func(ctx *execution.Context, raw string) int64 {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			panic(NewBadRequestException(map[string]any{"raw": raw}))
		}
		return value
	})
})

// TestParseIntPipe_RootAlias_InsightCallShape proves INSIGHT.md's
// ParseIntPipe example compiles and works end-to-end through the root
// gonest package's Pipe/NewPipe aliases, attached via
// route.Param("id", ParseIntPipe) through the root Controller/Module/
// NewApp aliases, dispatched via REAL app.Test requests covering both the
// valid-int and invalid-int paths (proving MustParam[T] genuinely reaches
// the custom Pipe's Handler through the whole real HTTP dispatch chain,
// not just at construction time).
func TestParseIntPipe_RootAlias_InsightCallShape(t *testing.T) {
	var gotID int64
	handlerRan := false

	controller := NewController(func(c *Controller) {
		c.Route(route.HttpGet, "/items/:id", func(r *route.Route) {
			r.Param("id", ParseIntPipe)
			r.Handler(func(ctx *execution.Context) {
				gotID = MustParam[int64](ctx, "id")
				handlerRan = true
				ctx.Json(map[string]int64{"id": gotID})
			})
		})
	})

	root := NewModule(func(m *Module) {
		m.Controllers(controller)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	t.Run("valid int -> 200, MustParam decodes via ParseIntPipe", func(t *testing.T) {
		handlerRan = false
		req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if !handlerRan {
			t.Fatal("route Handler did not run")
		}
		if gotID != 42 {
			t.Fatalf("gotID = %d, want 42", gotID)
		}
	})

	t.Run("invalid int -> 400 BadRequestException, Handler does not run", func(t *testing.T) {
		handlerRan = false
		req := httptest.NewRequest(http.MethodGet, "/items/not-a-number", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
		if handlerRan {
			t.Fatal("route Handler ran, want it NOT to run when the param fails to parse")
		}
	})
}

// ---------------------------------------------------------------------------
// Exceptions
// ---------------------------------------------------------------------------

// FooExampleError reproduces INSIGHT.md's dev-defined-exception example
// verbatim, adapted per SPEC_DEVIATION: INSIGHT.md uses
// gonest.HttpStatusBadRequest, a named HttpStatus constant that was
// explicitly scoped OUT of the "HttpException Core" feature, so this test
// uses the equivalent net/http.StatusBadRequest int literal instead.
type FooExampleError struct {
	HttpException
}

// NewFooExampleError mirrors INSIGHT.md's constructor shape exactly, using
// the root-aliased HttpException/NewHttpException.
func NewFooExampleError(details any) *FooExampleError {
	return &FooExampleError{
		HttpException: NewHttpException(http.StatusBadRequest, "FooExampleError", "lorem ipsum dolor met", details),
	}
}

// TestFooExampleError_RootAlias_InsightCallShape proves INSIGHT.md's
// dev-defined-exception example compiles and works end-to-end through the
// root gonest package's Exception/HttpException/NewHttpException aliases.
func TestFooExampleError_RootAlias_InsightCallShape(t *testing.T) {
	err := NewFooExampleError(map[string]any{"field": "bar"})

	if err.Status() != http.StatusBadRequest {
		t.Fatalf("Status() = %d, want %d", err.Status(), http.StatusBadRequest)
	}
	if err.Name() != "FooExampleError" {
		t.Fatalf("Name() = %q, want %q", err.Name(), "FooExampleError")
	}
	if err.Message() != "lorem ipsum dolor met" {
		t.Fatalf("Message() = %q, want %q", err.Message(), "lorem ipsum dolor met")
	}

	var _ Exception = err
}

// TestNewNotFoundException_RootAlias_PanicRecoverRoundTrip proves a
// root-aliased built-in exception constructor (NewNotFoundException) can be
// panicked with and recovered via a type assertion back to
// *NotFoundException through the root gonest package.
func TestNewNotFoundException_RootAlias_PanicRecoverRoundTrip(t *testing.T) {
	defer func() {
		r := recover()
		exc, ok := r.(*NotFoundException)
		if !ok {
			t.Fatalf("recover() type assertion to *NotFoundException failed, got %T", r)
		}
		if exc.Status() != http.StatusNotFound {
			t.Fatalf("Status() = %d, want %d", exc.Status(), http.StatusNotFound)
		}
	}()

	panic(NewNotFoundException(map[string]any{"userId": "abc123"}))
}

// TestHttpException_RootAlias_SatisfiesException proves the root-aliased
// HttpException/Exception types keep their structural-satisfaction
// relationship: a type embedding gonest.HttpException satisfies
// gonest.Exception without ever naming it.
func TestHttpException_RootAlias_SatisfiesException(t *testing.T) {
	var exc Exception = NewFooExampleError(nil)

	if exc.Details() != nil {
		t.Fatalf("Details() = %v, want nil", exc.Details())
	}
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

// TestNewMiddleware_RootAlias_TypeCheck proves NewMiddleware/Middleware/Next
// all resolve and type-check at the root gonest package: NewMiddleware
// builds a *Middleware, Handler accepts a func(ctx, next Next), and the
// resulting HandlerFunc genuinely reaches ctx/next through to the handler
// body.
func TestNewMiddleware_RootAlias_TypeCheck(t *testing.T) {
	var gotCtx *execution.Context
	nextCalled := false

	m := NewMiddleware(func(m *Middleware) {
		m.Handler(func(ctx *execution.Context, next Next) {
			gotCtx = ctx
			next(ctx)
		})
	})
	if m == nil {
		t.Fatal("NewMiddleware() returned nil *Middleware")
	}

	fn := m.HandlerFunc()
	if fn == nil {
		t.Fatal("HandlerFunc() returned nil after Handler was called")
	}

	ctx := execution.New(nil)
	fn(ctx, func(ctx *execution.Context) {
		nextCalled = true
	})

	if gotCtx != ctx {
		t.Fatal("ctx passed to the stored handler did not reach the handler body unchanged")
	}
	if !nextCalled {
		t.Fatal("next passed to the stored handler was not called/did not reach the handler body")
	}
}

// RequestIdMiddleware reproduces INSIGHT.md's Middleware example verbatim,
// through the root-aliased NewMiddleware/Middleware/Next. It generates a
// UUID and sets it as the X-Request-Id response header before calling
// next(ctx).
var RequestIdMiddleware = NewMiddleware(func(middleware *Middleware) {
	middleware.Handler(func(ctx *execution.Context, next Next) {
		requestId, _ := uuid.NewV7()
		ctx.SetHeader("X-Request-Id", requestId.String())
		next(ctx)
	})
})

// TestRequestIdMiddleware_RootAlias_InsightCallShape proves INSIGHT.md's
// RequestIdMiddleware example compiles and works end-to-end through the root
// gonest package's Middleware/Next/NewMiddleware aliases, attached via
// controller.Use(RequestIdMiddleware) through the root Controller/Module
// aliases, and dispatched via a REAL app.Test request -- confirming the
// X-Request-Id header genuinely lands in the real HTTP response.
func TestRequestIdMiddleware_RootAlias_InsightCallShape(t *testing.T) {
	controller := NewController(func(c *Controller) {
		c.Use(RequestIdMiddleware)
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := NewModule(func(m *Module) {
		m.Controllers(controller)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	requestId := resp.Header.Get("X-Request-Id")
	if requestId == "" {
		t.Fatal("X-Request-Id response header is empty, want a generated UUID")
	}
	if _, err := uuid.Parse(requestId); err != nil {
		t.Fatalf("X-Request-Id header = %q is not a valid UUID: %v", requestId, err)
	}
}

// ---------------------------------------------------------------------------
// Guard
// ---------------------------------------------------------------------------

// TestNewGuard_RootAlias_TypeCheck proves NewGuard/Guard resolve and
// type-check at the root gonest package: NewGuard builds a *Guard, Handler
// accepts a func(ctx) bool, and the resulting HandlerFunc genuinely reaches
// ctx through to the handler body and returns its own decision.
func TestNewGuard_RootAlias_TypeCheck(t *testing.T) {
	var gotCtx *execution.Context

	g := NewGuard(func(g *Guard) {
		g.Handler(func(ctx *execution.Context) bool {
			gotCtx = ctx
			return true
		})
	})
	if g == nil {
		t.Fatal("NewGuard() returned nil *Guard")
	}

	fn := g.HandlerFunc()
	if fn == nil {
		t.Fatal("HandlerFunc() returned nil after Handler was called")
	}

	ctx := execution.New(nil)
	result := fn(ctx)

	if gotCtx != ctx {
		t.Fatal("ctx passed to the stored handler did not reach the handler body unchanged")
	}
	if !result {
		t.Fatal("HandlerFunc() result did not reach back out as the handler's own decision")
	}
}

// authService is a stand-in for INSIGHT.md's AuthGuard example's
// AuthService, adapted per AD-008: no MustInject support (Guard.New(fn)
// runs fn immediately, before any module-tree/DI context exists). Instead
// of injecting it, AuthGuard below closes over an already-constructed
// instance directly, same as any other plain Go closure would.
type authService struct{}

// Validate reports whether token is considered valid. In this stand-in,
// any non-empty token other than the literal "invalid-token" is valid --
// enough to prove both the false->403 path and the true->Handler-runs path.
func (a *authService) Validate(token string) bool {
	return token != "invalid-token"
}

var stubAuthService = &authService{}

// AuthGuard reproduces INSIGHT.md's AuthGuard example, adapted per AD-008
// (no MustInject): rather than gonest.MustInject[*AuthService](guard), it
// closes over the package-level stubAuthService directly. The rest of the
// example is faithful: missing Authorization header panics a custom
// gonest.NewUnauthorizedException(nil) (proving the custom-exception-on-
// invalid path), otherwise it returns authService.Validate(token) as a
// plain bool (proving the false->automatic 403 path and the true->Handler-
// runs path).
var AuthGuard = NewGuard(func(guard *Guard) {
	guard.Handler(func(ctx *execution.Context) bool {
		token := ctx.Header("Authorization")
		if token == "" {
			panic(NewUnauthorizedException(nil))
		}
		return stubAuthService.Validate(token)
	})
})

// TestAuthGuard_RootAlias_InsightCallShape proves INSIGHT.md's AuthGuard
// example (adapted per AD-008, see AuthGuard's own doc comment) compiles
// and works end-to-end through the root gonest package's Guard/NewGuard
// aliases, attached via controller.Guards(AuthGuard) through the root
// Controller/Module/NewApp aliases, and dispatched via REAL app.Test
// requests covering all 3 cases.
func TestAuthGuard_RootAlias_InsightCallShape(t *testing.T) {
	handlerRan := false

	controller := NewController(func(c *Controller) {
		c.Guards(AuthGuard)
		c.Route(route.HttpGet, "/secure", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				handlerRan = true
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := NewModule(func(m *Module) {
		m.Controllers(controller)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	t.Run("no Authorization header -> 401 UnauthorizedException", func(t *testing.T) {
		handlerRan = false
		req := httptest.NewRequest(http.MethodGet, "/secure", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
		if handlerRan {
			t.Fatal("route Handler ran, want it NOT to run when Authorization header is missing")
		}
	})

	t.Run("invalid token -> 403 ForbiddenException", func(t *testing.T) {
		handlerRan = false
		req := httptest.NewRequest(http.MethodGet, "/secure", nil)
		req.Header.Set("Authorization", "invalid-token")
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
		if handlerRan {
			t.Fatal("route Handler ran, want it NOT to run when token is invalid")
		}
	})

	t.Run("valid token -> 200, Handler runs", func(t *testing.T) {
		handlerRan = false
		req := httptest.NewRequest(http.MethodGet, "/secure", nil)
		req.Header.Set("Authorization", "valid-token")
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if !handlerRan {
			t.Fatal("route Handler did not run, want it to run when token is valid")
		}
	})
}

// ---------------------------------------------------------------------------
// Interceptor
// ---------------------------------------------------------------------------

// TestNewInterceptor_RootAlias_TypeCheck proves NewInterceptor/Interceptor
// resolve and type-check at the root gonest package: NewInterceptor builds
// a *Interceptor, Handler accepts a func(ctx, next interceptor.Next), and
// the resulting HandlerFunc genuinely reaches ctx/next through to the
// handler body. interceptor.Next itself has no root alias yet (only
// Interceptor/NewInterceptor are in the "Interceptor" feature's scope --
// design.md's Tech Decisions explain why interceptor.Next is deliberately
// its own type, not reused from middleware.Next), so internal/interceptor
// is imported directly here for the Next parameter type.
func TestNewInterceptor_RootAlias_TypeCheck(t *testing.T) {
	var gotCtx *execution.Context
	nextCalled := false

	i := NewInterceptor(func(i *Interceptor) {
		i.Handler(func(ctx *execution.Context, next interceptorpkg.Next) {
			gotCtx = ctx
			next(ctx)
		})
	})
	if i == nil {
		t.Fatal("NewInterceptor() returned nil *Interceptor")
	}

	fn := i.HandlerFunc()
	if fn == nil {
		t.Fatal("HandlerFunc() returned nil after Handler was called")
	}

	ctx := execution.New(nil)
	fn(ctx, func(ctx *execution.Context) {
		nextCalled = true
	})

	if gotCtx != ctx {
		t.Fatal("ctx passed to the stored handler did not reach the handler body unchanged")
	}
	if !nextCalled {
		t.Fatal("next passed to the stored handler was not called/did not reach the handler body")
	}
}

// timingLog is a stand-in for INSIGHT.md's TimingInterceptor example's
// LoggerService, adapted per AD-008: no MustInject support
// (Interceptor.New(fn) runs fn immediately, before any module/DI context
// exists). Instead of injecting a logger, TimingInterceptor below closes
// over an already-constructed package-level slice directly, and records
// each logged entry so the test can assert on before/after ordering.
var timingLog []string

// TimingInterceptor reproduces INSIGHT.md's TimingInterceptor example,
// adapted per AD-008 (no MustInject): rather than
// gonest.MustInject[*LoggerService](interceptor), it closes over the
// package-level timingLog directly. The rest of the example is faithful:
// start := time.Now(), next(ctx) runs the rest of the chain/Handler, then
// it logs something time-related -- proving the log call happens AFTER
// next(ctx) returns via observable ordering (timingLog's contents), not by
// measuring real elapsed time precisely.
var TimingInterceptor = NewInterceptor(func(interceptor *Interceptor) {
	interceptor.Handler(func(ctx *execution.Context, next interceptorpkg.Next) {
		start := time.Now()
		timingLog = append(timingLog, "before")
		next(ctx)
		timingLog = append(timingLog, "request took "+time.Since(start).String())
	})
})

// TestTimingInterceptor_RootAlias_InsightCallShape proves INSIGHT.md's
// TimingInterceptor example (adapted per AD-008, see TimingInterceptor's
// own doc comment) compiles and works end-to-end through the root gonest
// package's Interceptor/Next/NewInterceptor aliases, attached via
// controller.Interceptors(TimingInterceptor) through the root
// Controller/Module/NewApp aliases, and dispatched via a REAL app.Test
// request -- confirming both the before-Handler and after-Handler logic
// ran, in the right order.
func TestTimingInterceptor_RootAlias_InsightCallShape(t *testing.T) {
	timingLog = nil
	handlerRan := false

	controller := NewController(func(c *Controller) {
		c.Interceptors(TimingInterceptor)
		c.Route(route.HttpGet, "/timed", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				handlerRan = true
				timingLog = append(timingLog, "handler")
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := NewModule(func(m *Module) {
		m.Controllers(controller)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	req := httptest.NewRequest(http.MethodGet, "/timed", nil)
	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !handlerRan {
		t.Fatal("route Handler did not run, want it to run")
	}

	if len(timingLog) != 3 {
		t.Fatalf("timingLog = %v, want 3 entries (before, handler, after)", timingLog)
	}
	if timingLog[0] != "before" {
		t.Fatalf("timingLog[0] = %q, want %q (before-Handler logic must run before next(ctx))", timingLog[0], "before")
	}
	if timingLog[1] != "handler" {
		t.Fatalf("timingLog[1] = %q, want %q (route Handler must run inside next(ctx))", timingLog[1], "handler")
	}
	if timingLog[2] == "before" || timingLog[2] == "handler" {
		t.Fatalf("timingLog[2] = %q, want an after-Handler log entry (must run after next(ctx) returns)", timingLog[2])
	}
}

// ---------------------------------------------------------------------------
// Filter (Filter feature)
// ---------------------------------------------------------------------------

// TestNewFilter_RootAlias_TypeCheck proves NewFilter/Filter resolve and
// type-check at the root gonest package: NewFilter builds a *Filter, and
// Catch(exemplar, handler) genuinely registers a reflect-validated handler
// findable via HandlerFor keyed by the exemplar's exact reflect.Type.
func TestNewFilter_RootAlias_TypeCheck(t *testing.T) {
	var gotCtx *execution.Context
	var gotExc *FooExampleError

	f := NewFilter(func(f *Filter) {
		f.Catch(&FooExampleError{}, func(ctx *execution.Context, exc *FooExampleError) {
			gotCtx = ctx
			gotExc = exc
		})
	})
	if f == nil {
		t.Fatal("NewFilter() returned nil *Filter")
	}

	excType := reflect.TypeOf(&FooExampleError{})
	fn, ok := f.HandlerFor(excType)
	if !ok {
		t.Fatal("expected HandlerFor(reflect.TypeOf(&FooExampleError{})) to report ok=true")
	}

	ctx := execution.New(nil)
	exc := NewFooExampleError(nil)
	fn.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(exc)})

	if gotCtx != ctx {
		t.Fatal("ctx passed to the stored handler did not reach the handler body unchanged")
	}
	if gotExc != exc {
		t.Fatal("exc passed to the stored handler did not reach the handler body unchanged")
	}
}

// FooExampleFilter reproduces INSIGHT.md's Filter example, adapted per
// SPEC_DEVIATION: INSIGHT.md's example uses gonest.HttpStatusTeapot, a named
// HttpStatus constant that was explicitly scoped OUT of the "HttpException
// Core" feature (see FooExampleError's own doc comment for the same
// deviation elsewhere in this file), so this uses the equivalent int literal
// 418 instead. It reuses FooExampleError (declared above in the Exceptions
// section) rather than redeclaring a new exception type.
var FooExampleFilter = NewFilter(func(filter *Filter) {
	filter.Catch(&FooExampleError{}, func(ctx *execution.Context, exc *FooExampleError) {
		ctx.Status(418).Json(map[string]any{
			"custom": true,
			"name":   exc.Name(),
		})
	})
})

// TestFooExampleFilter_RootAlias_InsightCallShape proves INSIGHT.md's
// FooExampleFilter example (adapted per SPEC_DEVIATION, see
// FooExampleFilter's own doc comment) compiles and works end-to-end through
// the root gonest package's Filter/NewFilter aliases, attached via
// controller.Filters(FooExampleFilter) through the root Controller/Module/
// NewApp aliases, dispatched via REAL app.Test requests covering both: (a) a
// panic with the caught *FooExampleError type -> the Filter's own custom 418
// response, and (b) a panic with an uncaught exception type
// (*NotFoundException) -> the EXISTING default {name,message,details}
// response, unmodified -- proving the Filter does not interfere with
// exceptions it did not register a Catch for.
func TestFooExampleFilter_RootAlias_InsightCallShape(t *testing.T) {
	controller := NewController(func(c *Controller) {
		c.Filters(FooExampleFilter)
		c.Route(route.HttpGet, "/caught", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				panic(NewFooExampleError(nil))
			})
		})
		c.Route(route.HttpGet, "/uncaught", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				panic(NewNotFoundException(nil))
			})
		})
	})

	root := NewModule(func(m *Module) {
		m.Controllers(controller)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := app.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("app.Adapter() is not a *fiber.FiberApp: %T", app.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	t.Run("caught *FooExampleError -> Filter's own custom 418 response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/caught", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 418 {
			t.Fatalf("status = %d, want 418", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode error = %v", err)
		}
		if body["custom"] != true {
			t.Fatalf("body = %v, want custom=true", body)
		}
		if body["name"] != "FooExampleError" {
			t.Fatalf("body = %v, want name=FooExampleError", body)
		}
	})

	t.Run("uncaught *NotFoundException -> unchanged default {name,message,details} response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/uncaught", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode error = %v", err)
		}
		if body["name"] != "NotFoundException" {
			t.Fatalf("body = %v, want name=NotFoundException (default {name,message,details} shape, Filter did not interfere)", body)
		}
	})
}

// ---------------------------------------------------------------------------
// Metadata (Metadata Registration Core feature)
// ---------------------------------------------------------------------------

// TestNewMetadata_RootAlias_TypeCheck proves NewMetadata/Metadata/
// PropertyBuilder resolve and type-check at the root gonest package:
// NewMetadata[T] builds a *Metadata, m.Property(&t.Field) returns a
// *PropertyBuilder, and the whole call shape compiles and runs without
// panicking for a minimal one-field struct.
func TestNewMetadata_RootAlias_TypeCheck(t *testing.T) {
	type minimalEntity struct {
		Id int64
	}

	m := NewMetadata[minimalEntity](func(t *minimalEntity, m *Metadata) {
		var _ *PropertyBuilder = m.Property(&t.Id)
	})
	if m == nil {
		t.Fatal("NewMetadata() returned nil *Metadata")
	}
}

// TestNewMetadata_RootAlias_UserEntityInsightCallShape reproduces INSIGHT.md's
// UserEntity metadata example (lines ~379-402) verbatim through the root
// gonest package's NewMetadata/Metadata/PropertyBuilder aliases, adapted per
// this feature's Out of Scope (spec.md/design.md): the type+format branch
// calls (.Integer()/.String()/.Email()/.Boolean()/.DateTime()) do not exist
// yet (future features), so only the base PropertyBuilder methods --
// Required/Nullable/Description/Examples -- are used here, confirmed field
// by field.
func TestNewMetadata_RootAlias_UserEntityInsightCallShape(t *testing.T) {
	type UserEntity struct {
		Id        int64      `json:"id"`
		Name      string     `json:"name"`
		Email     string     `json:"email"`
		IsActive  bool       `json:"isActive"`
		CreatedAt time.Time  `json:"createdAt"`
		UpdatedAt time.Time  `json:"updatedAt"`
		DeletedAt *time.Time `json:"deletedAt"`
	}

	now := time.Now()

	m := NewMetadata[UserEntity](func(t *UserEntity, m *Metadata) {
		m.Description("Entidade de usuário")
		m.Property(&t.Id).Required().Description("ID do usuário").Examples(int64(1))
		m.Property(&t.Name).Required().Description("Nome do usuário").Examples("John Doe")
		m.Property(&t.Email).Required().Description("Email do usuário").Examples("john@example.com")
		m.Property(&t.IsActive).Required().Description("Status do usuário").Examples(true)
		m.Property(&t.CreatedAt).Required().Description("Data de criação do usuário").Examples(now)
		m.Property(&t.UpdatedAt).Required().Description("Data de atualização do usuário").Examples(now)
		m.Property(&t.DeletedAt).Nullable().Description("Data de exclusão do usuário").Examples(nil, now)
	})

	if m == nil {
		t.Fatal("NewMetadata() returned nil *Metadata")
	}
	if m.DescriptionText() != "Entidade de usuário" {
		t.Fatalf("m.DescriptionText() = %q, want %q", m.DescriptionText(), "Entidade de usuário")
	}

	props := m.OwnProperties()
	if len(props) != 7 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 7", len(props))
	}

	byName := map[string]*PropertyBuilder{}
	for _, p := range props {
		byName[p.Field().Name] = p
	}

	checkBase := func(name string, wantRequired, wantNullable bool, wantDescription string, wantExamples []any) {
		t.Helper()
		p, ok := byName[name]
		if !ok {
			t.Fatalf("field %q was not registered via Property", name)
		}
		if p.IsRequired() != wantRequired {
			t.Fatalf("field %q: IsRequired() = %v, want %v", name, p.IsRequired(), wantRequired)
		}
		if p.IsNullable() != wantNullable {
			t.Fatalf("field %q: IsNullable() = %v, want %v", name, p.IsNullable(), wantNullable)
		}
		if p.DescriptionText() != wantDescription {
			t.Fatalf("field %q: DescriptionText() = %q, want %q", name, p.DescriptionText(), wantDescription)
		}
		gotExamples := p.ExamplesList()
		if len(gotExamples) != len(wantExamples) {
			t.Fatalf("field %q: ExamplesList() = %v, want %v", name, gotExamples, wantExamples)
		}
		for i := range wantExamples {
			if gotExamples[i] != wantExamples[i] {
				t.Fatalf("field %q: ExamplesList()[%d] = %v, want %v", name, i, gotExamples[i], wantExamples[i])
			}
		}
	}

	checkBase("Id", true, false, "ID do usuário", []any{int64(1)})
	checkBase("Name", true, false, "Nome do usuário", []any{"John Doe"})
	checkBase("Email", true, false, "Email do usuário", []any{"john@example.com"})
	checkBase("IsActive", true, false, "Status do usuário", []any{true})
	checkBase("CreatedAt", true, false, "Data de criação do usuário", []any{now})
	checkBase("UpdatedAt", true, false, "Data de atualização do usuário", []any{now})
	checkBase("DeletedAt", false, true, "Data de exclusão do usuário", []any{nil, now})

	// Confirm the field-identification-via-pointer technique (T1's core
	// mechanism) genuinely distinguishes each of the 7 fields by their own
	// Go type, not just by registration order.
	wantTypes := map[string]string{
		"Id":        "int64",
		"Name":      "string",
		"Email":     "string",
		"IsActive":  "bool",
		"CreatedAt": "time.Time",
		"UpdatedAt": "time.Time",
		"DeletedAt": "*time.Time",
	}
	for name, wantType := range wantTypes {
		p := byName[name]
		if got := p.Field().Type.String(); got != wantType {
			t.Fatalf("field %q: Field().Type.String() = %q, want %q", name, got, wantType)
		}
	}
}

// ---------------------------------------------------------------------------
// StringMetadata (String-family Branches feature)
// ---------------------------------------------------------------------------

// TestStringMetadata_RootAlias_TypeCheck proves gonest.StringMetadata
// resolves and type-checks at the root gonest package: PropertyBuilder.
// String() returns a *StringMetadata, and the base chain methods
// (Required/Description/Examples) plus the string-specific ones
// (Min/Max/Pattern) all compile and mutate the SAME underlying
// PropertyBuilder (per internal/metadata.StringMetadata's own doc comment on
// embedding a POINTER, not a copy).
func TestStringMetadata_RootAlias_TypeCheck(t *testing.T) {
	type minimalEntity struct {
		Name string
	}

	var sm *StringMetadata
	m := NewMetadata[minimalEntity](func(t *minimalEntity, m *Metadata) {
		sm = m.Property(&t.Name).String().Required().Min(1).Max(50).Pattern(`^\w+$`).
			Description("a name").Examples("John")
	})
	if m == nil {
		t.Fatal("NewMetadata() returned nil *Metadata")
	}
	if sm == nil {
		t.Fatal("expected *StringMetadata, got nil")
	}

	props := m.OwnProperties()
	if len(props) != 1 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 1", len(props))
	}
	p := props[0]

	if !p.IsRequired() {
		t.Fatal("IsRequired() = false, want true")
	}
	if p.FormatValue() != "" {
		t.Fatalf("FormatValue() = %q, want %q (String() sets no format)", p.FormatValue(), "")
	}
	if min, ok := sm.MinValue(); !ok || min != 1 {
		t.Fatalf("MinValue() = (%d, %v), want (1, true)", min, ok)
	}
	if max, ok := sm.MaxValue(); !ok || max != 50 {
		t.Fatalf("MaxValue() = (%d, %v), want (50, true)", max, ok)
	}
	if sm.PatternValue() != `^\w+$` {
		t.Fatalf("PatternValue() = %q, want %q", sm.PatternValue(), `^\w+$`)
	}
	if p.DescriptionText() != "a name" {
		t.Fatalf("DescriptionText() = %q, want %q", p.DescriptionText(), "a name")
	}
	examples := p.ExamplesList()
	if len(examples) != 1 || examples[0] != "John" {
		t.Fatalf("ExamplesList() = %v, want [John]", examples)
	}
}

// TestStringMetadata_RootAlias_AddressEntityInsightCallShape reproduces
// INSIGHT.md's AddressEntity example (lines ~431-456, "exemplo de Array e
// Object aninhados") verbatim for its String()/Pattern() field declarations
// through the root gonest package's StringMetadata alias, adapted per this
// feature's scope: only Street/City/Zip (all String()-branch fields) are
// reproduced here -- the surrounding Array()/Object()/nested-metadata parts
// of that example belong to a separate, not-yet-implemented feature (see
// ROADMAP.md).
func TestStringMetadata_RootAlias_AddressEntityInsightCallShape(t *testing.T) {
	type AddressEntity struct {
		Street string `json:"street"`
		City   string `json:"city"`
		Zip    string `json:"zip"`
	}

	m := NewMetadata[AddressEntity](func(t *AddressEntity, m *Metadata) {
		m.Description("Endereço")
		m.Property(&t.Street).String().Required().Description("Logradouro").Examples("Rua A, 123")
		m.Property(&t.City).String().Required().Description("Cidade").Examples("São Paulo")
		m.Property(&t.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`).Description("CEP").Examples("01310-100")
	})

	if m.DescriptionText() != "Endereço" {
		t.Fatalf("m.DescriptionText() = %q, want %q", m.DescriptionText(), "Endereço")
	}

	props := m.OwnProperties()
	if len(props) != 3 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 3", len(props))
	}
	byName := map[string]*PropertyBuilder{}
	for _, p := range props {
		byName[p.Field().Name] = p
	}

	street, ok := byName["Street"]
	if !ok {
		t.Fatal("field \"Street\" was not registered via Property")
	}
	if !street.IsRequired() {
		t.Fatal("Street: IsRequired() = false, want true")
	}
	if street.FormatValue() != "" {
		t.Fatalf("Street: FormatValue() = %q, want %q", street.FormatValue(), "")
	}
	if street.DescriptionText() != "Logradouro" {
		t.Fatalf("Street: DescriptionText() = %q, want %q", street.DescriptionText(), "Logradouro")
	}
	if examples := street.ExamplesList(); len(examples) != 1 || examples[0] != "Rua A, 123" {
		t.Fatalf("Street: ExamplesList() = %v, want [Rua A, 123]", examples)
	}

	city, ok := byName["City"]
	if !ok {
		t.Fatal("field \"City\" was not registered via Property")
	}
	if !city.IsRequired() {
		t.Fatal("City: IsRequired() = false, want true")
	}
	if city.FormatValue() != "" {
		t.Fatalf("City: FormatValue() = %q, want %q", city.FormatValue(), "")
	}
	if city.DescriptionText() != "Cidade" {
		t.Fatalf("City: DescriptionText() = %q, want %q", city.DescriptionText(), "Cidade")
	}
	if examples := city.ExamplesList(); len(examples) != 1 || examples[0] != "São Paulo" {
		t.Fatalf("City: ExamplesList() = %v, want [São Paulo]", examples)
	}

	zip, ok := byName["Zip"]
	if !ok {
		t.Fatal("field \"Zip\" was not registered via Property")
	}
	if !zip.IsRequired() {
		t.Fatal("Zip: IsRequired() = false, want true")
	}
	if zip.FormatValue() != "" {
		t.Fatalf("Zip: FormatValue() = %q, want %q (Pattern doesn't set format)", zip.FormatValue(), "")
	}
	if zip.DescriptionText() != "CEP" {
		t.Fatalf("Zip: DescriptionText() = %q, want %q", zip.DescriptionText(), "CEP")
	}
	if examples := zip.ExamplesList(); len(examples) != 1 || examples[0] != "01310-100" {
		t.Fatalf("Zip: ExamplesList() = %v, want [01310-100]", examples)
	}
}

// TestPropertyBuilder_RootAlias_EmailInsightCallShape reproduces
// INSIGHT.md's UserEntity.Email field (line ~397, "exemplo para definição de
// metadados em estruturas") through the root gonest package's
// PropertyBuilder.Email()/StringMetadata aliases, confirming
// FormatValue()=="email".
func TestPropertyBuilder_RootAlias_EmailInsightCallShape(t *testing.T) {
	type UserEntity struct {
		Email string `json:"email"`
	}

	m := NewMetadata[UserEntity](func(t *UserEntity, m *Metadata) {
		m.Property(&t.Email).Email().Required().Description("Email do usuário").Examples("john@example.com")
	})

	props := m.OwnProperties()
	if len(props) != 1 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 1", len(props))
	}
	p := props[0]
	if p.FormatValue() != "email" {
		t.Fatalf("FormatValue() = %q, want %q", p.FormatValue(), "email")
	}
	if !p.IsRequired() {
		t.Fatal("IsRequired() = false, want true")
	}
}

// TestStringMetadata_RootAlias_RemainingSevenBranches exercises the 7
// string-family branch methods not shown explicitly in INSIGHT.md's examples
// (Uuid/Uri/Hostname/Ipv4/Ipv6/Password/Byte/Binary minus Email/String
// already covered above) through the root gonest package's PropertyBuilder/
// StringMetadata aliases, confirming each sets its own distinct
// FormatValue().
func TestStringMetadata_RootAlias_RemainingSevenBranches(t *testing.T) {
	type entity struct {
		Uuid     string
		Uri      string
		Hostname string
		Ipv4     string
		Ipv6     string
		Password string
		Byte     string
		Binary   string
	}

	var m *Metadata
	m = NewMetadata[entity](func(t *entity, m *Metadata) {
		m.Property(&t.Uuid).Uuid()
		m.Property(&t.Uri).Uri()
		m.Property(&t.Hostname).Hostname()
		m.Property(&t.Ipv4).Ipv4()
		m.Property(&t.Ipv6).Ipv6()
		m.Property(&t.Password).Password()
		m.Property(&t.Byte).Byte()
		m.Property(&t.Binary).Binary()
	})

	byName := map[string]*PropertyBuilder{}
	for _, p := range m.OwnProperties() {
		byName[p.Field().Name] = p
	}

	wantFormats := map[string]string{
		"Uuid":     "uuid",
		"Uri":      "uri",
		"Hostname": "hostname",
		"Ipv4":     "ipv4",
		"Ipv6":     "ipv6",
		"Password": "password",
		"Byte":     "byte",
		"Binary":   "binary",
	}
	for name, want := range wantFormats {
		p, ok := byName[name]
		if !ok {
			t.Fatalf("field %q was not registered via Property", name)
		}
		if got := p.FormatValue(); got != want {
			t.Fatalf("field %q: FormatValue() = %q, want %q", name, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// NumericMetadata (Numeric & Boolean Branches feature)
// ---------------------------------------------------------------------------

// TestNumericMetadata_RootAlias_TypeCheck proves gonest.NumericMetadata
// resolves and type-checks at the root gonest package: PropertyBuilder.
// Integer() returns a *NumericMetadata, and the base chain methods
// (Required/Description/Examples) plus the numeric-specific ones (Min/Max)
// all compile and mutate the SAME underlying PropertyBuilder (per
// internal/metadata.NumericMetadata's own doc comment on embedding a
// POINTER, not a copy).
func TestNumericMetadata_RootAlias_TypeCheck(t *testing.T) {
	type minimalEntity struct {
		Age int64
	}

	var nm *NumericMetadata
	m := NewMetadata[minimalEntity](func(t *minimalEntity, m *Metadata) {
		nm = m.Property(&t.Age).Integer().Required().Min(0).Max(150).
			Description("an age").Examples(int64(30))
	})
	if m == nil {
		t.Fatal("NewMetadata() returned nil *Metadata")
	}
	if nm == nil {
		t.Fatal("expected *NumericMetadata, got nil")
	}

	props := m.OwnProperties()
	if len(props) != 1 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 1", len(props))
	}
	p := props[0]

	if !p.IsRequired() {
		t.Fatal("IsRequired() = false, want true")
	}
	if p.FormatValue() != "int64" {
		t.Fatalf("FormatValue() = %q, want %q (Integer() sets format int64)", p.FormatValue(), "int64")
	}
	if min, ok := nm.MinValue(); !ok || min != 0 {
		t.Fatalf("MinValue() = (%d, %v), want (0, true)", min, ok)
	}
	if max, ok := nm.MaxValue(); !ok || max != 150 {
		t.Fatalf("MaxValue() = (%d, %v), want (150, true)", max, ok)
	}
	if p.DescriptionText() != "an age" {
		t.Fatalf("DescriptionText() = %q, want %q", p.DescriptionText(), "an age")
	}
	examples := p.ExamplesList()
	if len(examples) != 1 || examples[0] != int64(30) {
		t.Fatalf("ExamplesList() = %v, want [30]", examples)
	}
}

// TestNumericMetadata_RootAlias_UserEntityInsightCallShape reproduces
// INSIGHT.md's UserEntity metadata example (lines ~393-401) verbatim for its
// Id (.Integer()) and IsActive (.Boolean()) fields through the root gonest
// package's PropertyBuilder.Integer()/Boolean()/NumericMetadata aliases,
// confirmed field by field.
func TestNumericMetadata_RootAlias_UserEntityInsightCallShape(t *testing.T) {
	type UserEntity struct {
		Id       int64 `json:"id"`
		IsActive bool  `json:"isActive"`
	}

	m := NewMetadata[UserEntity](func(t *UserEntity, m *Metadata) {
		m.Description("Entidade de usuário")
		m.Property(&t.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))
		m.Property(&t.IsActive).Boolean().Required().Description("Status do usuário").Examples(true)
	})

	if m.DescriptionText() != "Entidade de usuário" {
		t.Fatalf("m.DescriptionText() = %q, want %q", m.DescriptionText(), "Entidade de usuário")
	}

	props := m.OwnProperties()
	if len(props) != 2 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 2", len(props))
	}
	byName := map[string]*PropertyBuilder{}
	for _, p := range props {
		byName[p.Field().Name] = p
	}

	id, ok := byName["Id"]
	if !ok {
		t.Fatal("field \"Id\" was not registered via Property")
	}
	if !id.IsRequired() {
		t.Fatal("Id: IsRequired() = false, want true")
	}
	if id.FormatValue() != "int64" {
		t.Fatalf("Id: FormatValue() = %q, want %q", id.FormatValue(), "int64")
	}
	if id.DescriptionText() != "ID do usuário" {
		t.Fatalf("Id: DescriptionText() = %q, want %q", id.DescriptionText(), "ID do usuário")
	}
	if examples := id.ExamplesList(); len(examples) != 1 || examples[0] != int64(1) {
		t.Fatalf("Id: ExamplesList() = %v, want [1]", examples)
	}

	isActive, ok := byName["IsActive"]
	if !ok {
		t.Fatal("field \"IsActive\" was not registered via Property")
	}
	if !isActive.IsRequired() {
		t.Fatal("IsActive: IsRequired() = false, want true")
	}
	if isActive.FormatValue() != "" {
		t.Fatalf("IsActive: FormatValue() = %q, want %q (Boolean() sets no format)", isActive.FormatValue(), "")
	}
	if isActive.DescriptionText() != "Status do usuário" {
		t.Fatalf("IsActive: DescriptionText() = %q, want %q", isActive.DescriptionText(), "Status do usuário")
	}
	if examples := isActive.ExamplesList(); len(examples) != 1 || examples[0] != true {
		t.Fatalf("IsActive: ExamplesList() = %v, want [true]", examples)
	}
}

// TestNumericMetadata_RootAlias_RemainingThreeBranches exercises the 3
// numeric-family branch methods not shown explicitly in INSIGHT.md's
// examples (Int32/Float/Double -- Integer already covered above) through the
// root gonest package's PropertyBuilder/NumericMetadata aliases, confirming
// each sets its own distinct FormatValue().
func TestNumericMetadata_RootAlias_RemainingThreeBranches(t *testing.T) {
	type entity struct {
		Int32Field  int32
		FloatField  float32
		DoubleField float64
	}

	m := NewMetadata[entity](func(t *entity, m *Metadata) {
		m.Property(&t.Int32Field).Int32()
		m.Property(&t.FloatField).Float()
		m.Property(&t.DoubleField).Double()
	})

	byName := map[string]*PropertyBuilder{}
	for _, p := range m.OwnProperties() {
		byName[p.Field().Name] = p
	}

	wantFormats := map[string]string{
		"Int32Field":  "int32",
		"FloatField":  "float",
		"DoubleField": "double",
	}
	for name, want := range wantFormats {
		p, ok := byName[name]
		if !ok {
			t.Fatalf("field %q was not registered via Property", name)
		}
		if got := p.FormatValue(); got != want {
			t.Fatalf("field %q: FormatValue() = %q, want %q", name, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// DateTime/Date (wrapper-less PropertyBuilder branches)
// ---------------------------------------------------------------------------

// TestDateTime_RootAlias_UserEntityInsightCallShape reproduces INSIGHT.md's
// UserEntity CreatedAt/UpdatedAt/DeletedAt chains (lines ~399-401) verbatim
// through the root gonest package's NewMetadata/Metadata/PropertyBuilder
// aliases, confirming DateTime()/Date() need no alias of their own -- they
// return the bare *PropertyBuilder, already re-exported since Metadata
// Registration Core (same reasoning gonest.go's NumericMetadata doc comment
// already spells out for Boolean()).
func TestDateTime_RootAlias_UserEntityInsightCallShape(t *testing.T) {
	type UserEntity struct {
		CreatedAt time.Time  `json:"createdAt"`
		UpdatedAt time.Time  `json:"updatedAt"`
		DeletedAt *time.Time `json:"deletedAt"`
	}

	now := time.Now()

	m := NewMetadata[UserEntity](func(t *UserEntity, m *Metadata) {
		m.Property(&t.CreatedAt).DateTime().Required().Description("Data de criação do usuário").Examples(now)
		m.Property(&t.UpdatedAt).DateTime().Required().Description("Data de atualização do usuário").Examples(now)
		m.Property(&t.DeletedAt).DateTime().Nullable().Description("Data de exclusão do usuário").Examples(nil, now)
	})

	byName := map[string]*PropertyBuilder{}
	for _, p := range m.OwnProperties() {
		byName[p.Field().Name] = p
	}

	createdAt, ok := byName["CreatedAt"]
	if !ok {
		t.Fatal("CreatedAt was not registered via Property")
	}
	if createdAt.FormatValue() != "date-time" {
		t.Fatalf("CreatedAt: FormatValue() = %q, want %q", createdAt.FormatValue(), "date-time")
	}
	if !createdAt.IsRequired() {
		t.Fatal("CreatedAt: IsRequired() = false, want true")
	}

	deletedAt, ok := byName["DeletedAt"]
	if !ok {
		t.Fatal("DeletedAt was not registered via Property")
	}
	if deletedAt.FormatValue() != "date-time" {
		t.Fatalf("DeletedAt: FormatValue() = %q, want %q", deletedAt.FormatValue(), "date-time")
	}
	if !deletedAt.IsNullable() {
		t.Fatal("DeletedAt: IsNullable() = false, want true")
	}
	examples := deletedAt.ExamplesList()
	if len(examples) != 2 || examples[0] != nil || examples[1] != now {
		t.Fatalf("DeletedAt: ExamplesList() = %v, want [nil %v]", examples, now)
	}
}

// TestDate_RootAlias_TypeCheck proves gonest.PropertyBuilder.Date() resolves
// and type-checks at the root gonest package -- Date() needs no alias of its
// own for the same reason DateTime() doesn't (see
// TestDateTime_RootAlias_UserEntityInsightCallShape's doc comment).
func TestDate_RootAlias_TypeCheck(t *testing.T) {
	type entity struct {
		BirthDate time.Time
	}

	m := NewMetadata[entity](func(t *entity, m *Metadata) {
		m.Property(&t.BirthDate).Date().Required()
	})

	props := m.OwnProperties()
	if len(props) != 1 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 1", len(props))
	}
	if got := props[0].FormatValue(); got != "date" {
		t.Fatalf("FormatValue() = %q, want %q", got, "date")
	}
	if !props[0].IsRequired() {
		t.Fatal("IsRequired() = false, want true")
	}
}

// ---------------------------------------------------------------------------
// ArrayMetadata (Array Builder feature)
// ---------------------------------------------------------------------------

// TestArrayMetadata_RootAlias_TypeCheck proves gonest.ArrayMetadata resolves
// and type-checks at the root gonest package: PropertyBuilder.Array()
// returns a *ArrayMetadata, Items(fn) hands the same *ArrayMetadata into the
// callback (pointer identity), the item-branch methods
// (String/Integer/Object/etc) mutate the synthetic item builder, and
// Required/Nullable/Description/Examples plus the array's own Min/Max mutate
// the field itself -- all reachable purely through root aliases, no
// internal/metadata import.
func TestArrayMetadata_RootAlias_TypeCheck(t *testing.T) {
	type entity struct {
		Tags []string
	}

	var identity *ArrayMetadata
	m := NewMetadata[entity](func(t *entity, m *Metadata) {
		am := m.Property(&t.Tags).Array()
		var _ *ArrayMetadata = am
		am.Items(func(m *ArrayMetadata) {
			identity = m
			m.String().Min(1).Max(50)
			m.Required()
			m.Description("Tags")
			m.Examples("admin", "beta")
		}).Min(1).Max(10)
	})
	if m == nil {
		t.Fatal("NewMetadata() returned nil *Metadata")
	}
	if identity == nil {
		t.Fatal("Items(fn) callback never invoked")
	}

	props := m.OwnProperties()
	if len(props) != 1 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 1", len(props))
	}
	p := props[0]
	if p.FormatValue() != "array" {
		t.Fatalf("FormatValue() = %q, want %q", p.FormatValue(), "array")
	}
	if !p.IsRequired() {
		t.Fatal("IsRequired() = false, want true")
	}
	if p.DescriptionText() != "Tags" {
		t.Fatalf("DescriptionText() = %q, want %q", p.DescriptionText(), "Tags")
	}
	examples := p.ExamplesList()
	if len(examples) != 2 || examples[0] != "admin" || examples[1] != "beta" {
		t.Fatalf("ExamplesList() = %v, want [admin beta]", examples)
	}
	if min, ok := identity.MinValue(); !ok || min != 1 {
		t.Fatalf("array MinValue() = (%d, %v), want (1, true)", min, ok)
	}
	if max, ok := identity.MaxValue(); !ok || max != 10 {
		t.Fatalf("array MaxValue() = (%d, %v), want (10, true)", max, ok)
	}

	item := identity.ItemBuilder()
	if item.FormatValue() != "" {
		t.Fatalf("item FormatValue() = %q, want %q (String() sets no format)", item.FormatValue(), "")
	}
}

// TestArrayMetadata_RootAlias_UserEntityInsightCallShape reproduces
// INSIGHT.md's UserEntity.Tags/Scores/Addresses example (lines ~437-486,
// "exemplo de Array e Object aninhados") verbatim through the root gonest
// package's NewMetadata/Metadata/PropertyBuilder/ArrayMetadata aliases --
// confirms the 3 array shapes (string item, int item, and Object() item
// referencing an already-registered gonest.NewMetadata[AddressEntity])
// all resolve and behave correctly at the root package, no
// internal/metadata import required.
func TestArrayMetadata_RootAlias_UserEntityInsightCallShape(t *testing.T) {
	type AddressEntity struct {
		Street string `json:"street"`
		City   string `json:"city"`
		Zip    string `json:"zip"`
	}

	type UserEntity struct {
		Id        int64           `json:"id"`
		Tags      []string        `json:"tags"`
		Scores    []int           `json:"scores"`
		Addresses []AddressEntity `json:"addresses"`
	}

	addressMetadata := NewMetadata[AddressEntity](func(t *AddressEntity, m *Metadata) {
		m.Description("Endereço")
		m.Property(&t.Street).String().Required().Description("Logradouro").Examples("Rua A, 123")
		m.Property(&t.City).String().Required().Description("Cidade").Examples("São Paulo")
		m.Property(&t.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`).Description("CEP").Examples("01310-100")
	})

	m := NewMetadata[UserEntity](func(t *UserEntity, m *Metadata) {
		m.Description("Entidade de usuário com campos aninhados")
		m.Property(&t.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))

		m.Property(&t.Tags).Array().Items(func(m *ArrayMetadata) {
			m.String().Min(1).Max(50)
			m.Required()
			m.Description("Tags do usuário")
			m.Examples("admin", "beta")
		})

		m.Property(&t.Scores).Array().Items(func(m *ArrayMetadata) {
			m.Integer().Min(0).Max(100)
			m.Required()
			m.Description("Notas do usuário")
			m.Examples(80, 95)
		})

		m.Property(&t.Addresses).Array().Items(func(m *ArrayMetadata) {
			m.Object(addressMetadata)
			m.Required()
			m.Min(1)
			m.Description("Endereços do usuário")
			m.Examples("admin", "beta")
		})
	})

	props := m.OwnProperties()
	if len(props) != 4 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 4", len(props))
	}
	byName := map[string]*PropertyBuilder{}
	for _, p := range props {
		byName[p.Field().Name] = p
	}

	// Tags: array of string item, Min(1)/Max(50) on the ITEM (not the array).
	tags, ok := byName["Tags"]
	if !ok {
		t.Fatal("Tags was not registered via Property")
	}
	if tags.FormatValue() != "array" {
		t.Fatalf("Tags: FormatValue() = %q, want %q", tags.FormatValue(), "array")
	}
	if !tags.IsRequired() {
		t.Fatal("Tags: IsRequired() = false, want true")
	}
	if tags.DescriptionText() != "Tags do usuário" {
		t.Fatalf("Tags: DescriptionText() = %q, want %q", tags.DescriptionText(), "Tags do usuário")
	}

	// Scores: array of int item, Min(0)/Max(100) on the ITEM.
	scores, ok := byName["Scores"]
	if !ok {
		t.Fatal("Scores was not registered via Property")
	}
	if scores.FormatValue() != "array" {
		t.Fatalf("Scores: FormatValue() = %q, want %q", scores.FormatValue(), "array")
	}
	if !scores.IsRequired() {
		t.Fatal("Scores: IsRequired() = false, want true")
	}
	examples := scores.ExamplesList()
	if len(examples) != 2 || examples[0] != 80 || examples[1] != 95 {
		t.Fatalf("Scores: ExamplesList() = %v, want [80 95]", examples)
	}

	// Addresses: array of Object(addressMetadata) item, Min(1) on the ARRAY
	// itself (item count, not item constraints).
	addresses, ok := byName["Addresses"]
	if !ok {
		t.Fatal("Addresses was not registered via Property")
	}
	if addresses.FormatValue() != "array" {
		t.Fatalf("Addresses: FormatValue() = %q, want %q", addresses.FormatValue(), "array")
	}
	if !addresses.IsRequired() {
		t.Fatal("Addresses: IsRequired() = false, want true")
	}
	if addresses.DescriptionText() != "Endereços do usuário" {
		t.Fatalf("Addresses: DescriptionText() = %q, want %q", addresses.DescriptionText(), "Endereços do usuário")
	}
}

// TestObjectMetadata_RootAlias_TypeCheck proves gonest.ObjectMetadata
// resolves and type-checks at the root gonest package: PropertyBuilder.
// Object() returns a *ObjectMetadata, Object(fn) hands the same
// *ObjectMetadata into the callback (pointer identity), and
// Required/Nullable/Description/Examples called INSIDE the callback vs.
// chained OUTSIDE it (on Object(fn)'s own return value) mutate the exact
// same *PropertyBuilder either way -- all reachable purely through root
// aliases, no internal/metadata import.
func TestObjectMetadata_RootAlias_TypeCheck(t *testing.T) {
	type entity struct {
		Inside  map[string]any
		Outside map[string]any
	}

	var insideIdentity *ObjectMetadata
	m := NewMetadata[entity](func(t *entity, m *Metadata) {
		om := m.Property(&t.Inside).Object(func(m *ObjectMetadata) {
			insideIdentity = m
			m.AdditionalProperties()
			m.Required()
			m.Description("Inside")
			m.Examples("a", "b")
		})
		var _ *ObjectMetadata = om

		m.Property(&t.Outside).Object(func(m *ObjectMetadata) {
			m.AdditionalProperties()
		}).Required().Description("Outside").Examples("c", "d")
	})
	if m == nil {
		t.Fatal("NewMetadata() returned nil *Metadata")
	}
	if insideIdentity == nil {
		t.Fatal("Object(fn) callback never invoked")
	}
	if !insideIdentity.IsAdditionalProperties() {
		t.Fatal("insideIdentity.IsAdditionalProperties() = false, want true")
	}

	props := m.OwnProperties()
	if len(props) != 2 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 2", len(props))
	}
	byName := map[string]*PropertyBuilder{}
	for _, p := range props {
		byName[p.Field().Name] = p
	}

	inside, ok := byName["Inside"]
	if !ok {
		t.Fatal("Inside was not registered via Property")
	}
	outside, ok := byName["Outside"]
	if !ok {
		t.Fatal("Outside was not registered via Property")
	}

	// Both fields must produce IDENTICAL results regardless of whether
	// Required/Description/Examples were called inside the callback or
	// chained outside it -- proving there is no dual-scope distinction
	// the way there is for ArrayMetadata.
	for name, p := range map[string]*PropertyBuilder{"Inside": inside, "Outside": outside} {
		if p.FormatValue() != "object" {
			t.Fatalf("%s: FormatValue() = %q, want %q", name, p.FormatValue(), "object")
		}
		if !p.IsRequired() {
			t.Fatalf("%s: IsRequired() = false, want true", name)
		}
	}
	if inside.DescriptionText() != "Inside" {
		t.Fatalf("Inside: DescriptionText() = %q, want %q", inside.DescriptionText(), "Inside")
	}
	if outside.DescriptionText() != "Outside" {
		t.Fatalf("Outside: DescriptionText() = %q, want %q", outside.DescriptionText(), "Outside")
	}
	insideExamples := inside.ExamplesList()
	if len(insideExamples) != 2 || insideExamples[0] != "a" || insideExamples[1] != "b" {
		t.Fatalf("Inside: ExamplesList() = %v, want [a b]", insideExamples)
	}
	outsideExamples := outside.ExamplesList()
	if len(outsideExamples) != 2 || outsideExamples[0] != "c" || outsideExamples[1] != "d" {
		t.Fatalf("Outside: ExamplesList() = %v, want [c d]", outsideExamples)
	}
}

// TestObjectMetadata_RootAlias_UserEntityInsightCallShape reproduces
// INSIGHT.md's UserEntity.Address/Metadata example (lines ~488-499,
// "exemplo de Array e Object aninhados") verbatim through the root gonest
// package's NewMetadata/Metadata/PropertyBuilder/ObjectMetadata aliases --
// confirms both object shapes (a $ref-like reuse of an already-registered
// gonest.NewMetadata[AddressEntity] via om.Metadata(ref), and a free-form
// open schema via om.AdditionalProperties() with Nullable/Description
// chained outside the callback) resolve and behave correctly at the root
// package, no internal/metadata import required.
func TestObjectMetadata_RootAlias_UserEntityInsightCallShape(t *testing.T) {
	type AddressEntity struct {
		Street string `json:"street"`
		City   string `json:"city"`
		Zip    string `json:"zip"`
	}

	type UserEntity struct {
		Id       int64          `json:"id"`
		Address  AddressEntity  `json:"address"`
		Metadata map[string]any `json:"metadata"`
	}

	addressMetadata := NewMetadata[AddressEntity](func(t *AddressEntity, m *Metadata) {
		m.Description("Endereço")
		m.Property(&t.Street).String().Required().Description("Logradouro").Examples("Rua A, 123")
		m.Property(&t.City).String().Required().Description("Cidade").Examples("São Paulo")
		m.Property(&t.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`).Description("CEP").Examples("01310-100")
	})

	m := NewMetadata[UserEntity](func(t *UserEntity, m *Metadata) {
		m.Description("Entidade de usuário com campos aninhados")
		m.Property(&t.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))

		// Object() direto (não-array) -- mesma reutilização via valor, sem reflect.
		m.Property(&t.Address).Object(func(om *ObjectMetadata) {
			om.Metadata(addressMetadata)
			om.Required()
			om.Description("Endereço principal")
		})

		// Object() livre (schema aberto, tipo map[string]any) -- sem struct Go aninhada
		// pra reusar, por isso recebe callback em vez de metadata já registrada.
		m.Property(&t.Metadata).Object(func(om *ObjectMetadata) {
			om.AdditionalProperties()
		}).Nullable().Description("Metadados abertos do usuário")
	})

	props := m.OwnProperties()
	if len(props) != 3 {
		t.Fatalf("len(m.OwnProperties()) = %d, want 3", len(props))
	}
	byName := map[string]*PropertyBuilder{}
	for _, p := range props {
		byName[p.Field().Name] = p
	}

	// Address: object referencing addressMetadata via Metadata(ref), same
	// pointer identity preserved, Required/Description set inside the callback.
	address, ok := byName["Address"]
	if !ok {
		t.Fatal("Address was not registered via Property")
	}
	if address.FormatValue() != "object" {
		t.Fatalf("Address: FormatValue() = %q, want %q", address.FormatValue(), "object")
	}
	if !address.IsRequired() {
		t.Fatal("Address: IsRequired() = false, want true")
	}
	if address.DescriptionText() != "Endereço principal" {
		t.Fatalf("Address: DescriptionText() = %q, want %q", address.DescriptionText(), "Endereço principal")
	}

	// Metadata: free-form object via AdditionalProperties() inside the
	// callback, Nullable/Description chained OUTSIDE on Object(fn)'s return.
	meta, ok := byName["Metadata"]
	if !ok {
		t.Fatal("Metadata was not registered via Property")
	}
	if meta.FormatValue() != "object" {
		t.Fatalf("Metadata: FormatValue() = %q, want %q", meta.FormatValue(), "object")
	}
	if !meta.IsNullable() {
		t.Fatal("Metadata: IsNullable() = false, want true")
	}
	if meta.DescriptionText() != "Metadados abertos do usuário" {
		t.Fatalf("Metadata: DescriptionText() = %q, want %q", meta.DescriptionText(), "Metadados abertos do usuário")
	}
}
