package gonest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gonest-dev/gonest/internal/adapter/fiber"
	"github.com/gonest-dev/gonest/internal/execution"
	interceptorpkg "github.com/gonest-dev/gonest/internal/interceptor"
	"github.com/gonest-dev/gonest/internal/route"
	"github.com/gonest-dev/gonest/internal/validate"
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
//
// The old singular gonest.MustParam[T](ctx, name) (and the whole Pipe
// mechanism it could fall back to) is REMOVED per param-query-validation's
// T3 (context.md's Decisions 2/3) -- every path param access is now
// struct-based via MustParams[T]. gonest.go re-exports MustParams[T]/
// MustQuery[T] at root as of T4 (see TestMustParamsAndMustQuery_RootAlias_InsightCallShape
// below for the root-alias reproduction); the tests directly below this
// comment predate that wrapper and intentionally still reach the real
// implementation via internal/validate directly, same as internal/route's
// own tests do for HasParam -- left as-is since they still pass and add
// coverage at that layer. The Pipe-specific custom-transform coverage
// (TestMustParam_WithCustomPipe_.../TestMustParam_PanicsWhenCustomPipeHandlerPanics/
// TestNewPipe_RootAlias_TypeCheck/TestParseIntPipe_RootAlias_InsightCallShape)
// is intentionally NOT ported here -- that capability now lives in
// PropertyBuilder.Custom(fn) (context.md's Decision 4), already covered by
// internal/validate/params_test.go's TestMustParams_CustomFunc_ReceivesRawString_NotCoerced
// and TestMustParams_RealHTTPDispatch_CustomFunc (unit + real HTTP dispatch,
// same intent this file's Pipe tests proved for the old mechanism).

// idParams mirrors INSIGHT.md's settled MustParams[*UserIdParams] shape: a
// single required path param.
type idParams struct {
	ID int `param:"id"`
}

var idParamsMetadata = NewMetadata[idParams](func(t *idParams, m *Metadata) {
	m.Property(&t.ID).Integer().Required()
})

// TestMustParams_RootPackage_HappyPath proves the replacement for the old
// TestMustParam_WithoutCustomPipe_UsesDefaultCoerce: a route param, present
// and valid, populates T via MustParams[T] without panic.
func TestMustParams_RootPackage_HappyPath(t *testing.T) {

	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {})

	res := newParamFakeResponder()
	res.params["id"] = "42"
	ctx := execution.New(res).WithRoute(r)

	got := validate.MustParams[*idParams](ctx)
	if got.ID != 42 {
		t.Fatalf("MustParams[*idParams](ctx).ID = %d, want %d", got.ID, 42)
	}
}

// TestMustParams_RootPackage_PanicsWhenParamNotDeclaredOnRoute proves the
// replacement for the old TestMustParam_PanicsWhenParamNotDeclaredOnRoute:
// a required param absent from the route's own declared path (HasParam
// false) is collected as a violation and panics *BadRequestException.
func TestMustParams_RootPackage_PanicsWhenParamNotDeclaredOnRoute(t *testing.T) {

	r := route.New(route.HttpGet, "/users", func(r *route.Route) {})

	res := newParamFakeResponder()
	ctx := execution.New(res).WithRoute(r)

	defer func() {
		rec := recover()
		exc, ok := rec.(*BadRequestException)
		if !ok {
			t.Fatalf("expected panic *BadRequestException, got %T: %v", rec, rec)
		}
		if exc.Status() != http.StatusBadRequest {
			t.Fatalf("Status() = %d, want %d", exc.Status(), http.StatusBadRequest)
		}
	}()

	validate.MustParams[*idParams](ctx)
}

// TestMustParams_RootPackage_PanicsOnConversionFailure proves the
// replacement for the old TestMustParam_PanicsOnConversionFailure_DefaultCoerce:
// a param present but failing to coerce to the field's declared kind
// (integer) is collected as a violation and panics *BadRequestException.
func TestMustParams_RootPackage_PanicsOnConversionFailure(t *testing.T) {

	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {})

	res := newParamFakeResponder()
	res.params["id"] = "not-a-number"
	ctx := execution.New(res).WithRoute(r)

	defer func() {
		rec := recover()
		if _, ok := rec.(*BadRequestException); !ok {
			t.Fatalf("expected panic *BadRequestException, got %T: %v", rec, rec)
		}
	}()

	validate.MustParams[*idParams](ctx)
}

// paramFakeResponder is a minimal test-only execution.Responder for exercising
// MustParams[T] end to end (Context -> Route -> validate.MustParams).
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
func (f *paramFakeResponder) Body() []byte                      { return nil }
func (f *paramFakeResponder) Queries() map[string]string        { return nil }
func (f *paramFakeResponder) HTML(s string) error               { return nil }
func (f *paramFakeResponder) SendString(s string) error         { return nil }

// TestMustParams_RootPackage_RealHTTPDispatch proves the replacement for the
// old TestParseIntPipe_RootAlias_InsightCallShape: a route param round-trips
// through a REAL app.Test HTTP dispatch, covering both the valid-int and
// invalid-int paths, now via validate.MustParams[*idParams] instead of
// MustParam[int64] + a custom Pipe.
func TestMustParams_RootPackage_RealHTTPDispatch(t *testing.T) {

	var gotID int
	handlerRan := false

	controller := NewController(func(c *Controller) {
		c.Route(route.HttpGet, "/items/:id", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				p := validate.MustParams[*idParams](ctx)
				gotID = p.ID
				handlerRan = true
				ctx.Json(map[string]int{"id": gotID})
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

	t.Run("valid int -> 200, MustParams decodes", func(t *testing.T) {
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

// insightUserIdParams mirrors INSIGHT.md's settled "exemplo de Param/Query
// Validation" section's UserIdParams: a single required path param.
type insightUserIdParams struct {
	UserId int64 `param:"user_id"`
}

var insightUserIdParamsMetadata = NewMetadata[insightUserIdParams](func(t *insightUserIdParams, m *Metadata) {
	m.Property(&t.UserId).Integer().Min(1).Required()
})

// insightListUsersQuery mirrors INSIGHT.md's settled ListUsersQuery: two
// required query params.
type insightListUsersQuery struct {
	Page  int `query:"page"`
	Limit int `query:"limit"`
}

var insightListUsersQueryMetadata = NewMetadata[insightListUsersQuery](func(t *insightListUsersQuery, m *Metadata) {
	m.Property(&t.Page).Integer().Min(1).Required()
	m.Property(&t.Limit).Integer().Min(1).Max(100).Required()
})

// TestMustParamsAndMustQuery_RootAlias_InsightCallShape reproduces INSIGHT.md's
// settled "exemplo de Param/Query Validation" example end to end through the
// root gonest package: a single route combining a path param
// (gonest.MustParams[T]) AND a query string (gonest.MustQuery[T]),
// dispatched via a REAL app.Test HTTP request (T4 of
// param-query-validation -- confirms gonest.MustParams/gonest.MustQuery
// resolve at root and behave identically to the internal/validate
// implementations exercised directly by the tests above).
func TestMustParamsAndMustQuery_RootAlias_InsightCallShape(t *testing.T) {

	var gotUserId int64
	var gotPage, gotLimit int
	handlerRan := false

	controller := NewController(func(c *Controller) {
		c.Route(route.HttpGet, "/users/:user_id/orders", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				params := MustParams[*insightUserIdParams](ctx)
				query := MustQuery[*insightListUsersQuery](ctx)
				gotUserId = params.UserId
				gotPage = query.Page
				gotLimit = query.Limit
				handlerRan = true
				ctx.Json(map[string]any{
					"userId": params.UserId,
					"page":   query.Page,
					"limit":  query.Limit,
				})
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

	t.Run("happy path: valid path param + valid query -> 200, both populated", func(t *testing.T) {
		handlerRan = false
		req := httptest.NewRequest(http.MethodGet, "/users/42/orders?page=2&limit=10", nil)
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
		if gotUserId != 42 || gotPage != 2 || gotLimit != 10 {
			t.Fatalf("got userId=%d page=%d limit=%d, want userId=42 page=2 limit=10", gotUserId, gotPage, gotLimit)
		}
	})

	t.Run("violation: query limit exceeds Max(100) -> 400 BadRequestException, Handler does not run", func(t *testing.T) {
		handlerRan = false
		req := httptest.NewRequest(http.MethodGet, "/users/42/orders?page=1&limit=500", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
		if handlerRan {
			t.Fatal("route Handler ran, want it NOT to run when a query param violates its constraint")
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
	m.Declare(nil)

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
	g.Declare(nil)

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
	i.Declare(nil)

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
	f.Declare(nil)

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

// ---------------------------------------------------------------------------
// Validation (JSON Body Validation feature)
// ---------------------------------------------------------------------------

// jsonBodyAddressEntity mirrors INSIGHT.md's AddressEntity ("exemplo de
// Array e Object aninhados").
type jsonBodyAddressEntity struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

// jsonBodyUserEntity mirrors INSIGHT.md's UserEntity, merging BOTH
// "exemplo para definição de metadados em estruturas" (Id/Name/Email/
// IsActive/CreatedAt/UpdatedAt/DeletedAt) and "exemplo de Array e Object
// aninhados" (Tags/Scores/Addresses/Address/Metadata) into a single struct,
// as this task's T4 Done-when explicitly asks for ("incluindo Tags/
// Addresses/Address aninhados").
type jsonBodyUserEntity struct {
	Id        int64                   `json:"id"`
	Name      string                  `json:"name"`
	Email     string                  `json:"email"`
	IsActive  bool                    `json:"isActive"`
	CreatedAt time.Time               `json:"createdAt"`
	UpdatedAt time.Time               `json:"updatedAt"`
	DeletedAt *time.Time              `json:"deletedAt"`
	Tags      []string                `json:"tags"`
	Scores    []int                   `json:"scores"`
	Addresses []jsonBodyAddressEntity `json:"addresses"`
	Address   jsonBodyAddressEntity   `json:"address"`
	Metadata  map[string]any          `json:"metadata"`
}

// jsonBodyAddressMetadata/jsonBodyUserMetadata register jsonBodyUserEntity's
// full metadata exactly once (the registry panics on duplicate
// registration for the same reflect.Type -- T1), via a package-level init
// mirroring INSIGHT.md's own top-level `var _ = gonest.NewMetadata[...]`
// call shape.
var jsonBodyAddressMetadata = NewMetadata[jsonBodyAddressEntity](func(t *jsonBodyAddressEntity, m *Metadata) {
	m.Description("Endereço")
	m.Property(&t.Street).String().Required().Description("Logradouro").Examples("Rua A, 123")
	m.Property(&t.City).String().Required().Description("Cidade").Examples("São Paulo")
	m.Property(&t.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`).Description("CEP").Examples("01310-100")
})

var jsonBodyUserMetadata = NewMetadata[jsonBodyUserEntity](func(t *jsonBodyUserEntity, m *Metadata) {
	m.Description("Entidade de usuário com campos aninhados")
	m.Property(&t.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))
	m.Property(&t.Name).String().Required().Description("Nome do usuário").Examples("John Doe")
	m.Property(&t.Email).Email().Required().Description("Email do usuário").Examples("user@example.com")
	m.Property(&t.IsActive).Boolean().Required().Description("Status do usuário").Examples(true)
	m.Property(&t.CreatedAt).DateTime().Required().Description("Data de criação do usuário").Examples(time.Now())
	m.Property(&t.UpdatedAt).DateTime().Required().Description("Data de atualização do usuário").Examples(time.Now())
	m.Property(&t.DeletedAt).DateTime().Nullable().Description("Data de exclusão do usuário").Examples(nil, time.Now())

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
		m.Object(jsonBodyAddressMetadata)
		m.Required()
		m.Min(1)
		m.Description("Endereços do usuário")
	})

	m.Property(&t.Address).Object(func(om *ObjectMetadata) {
		om.Metadata(jsonBodyAddressMetadata)
		om.Required()
		om.Description("Endereço principal")
	})

	m.Property(&t.Metadata).Object(func(om *ObjectMetadata) {
		om.AdditionalProperties()
	}).Nullable().Description("Metadados abertos do usuário")
})

// TestMustJsonBody_RootAlias_UserEntityInsightCallShape proves
// gonest.MustJsonBody[*jsonBodyUserEntity] resolves and works end-to-end
// through the root gonest package, reproducing INSIGHT.md's full UserEntity
// shape (both metadata-definition sections combined) via REAL HTTP dispatch
// (app.Test, same pattern as every other *_RootAlias_InsightCallShape test
// in this file): one happy-path case (fully valid body, handler receives a
// correctly populated value, response reflects it) and one multi-violation
// case (a bad array-item AND a bad nested-object field in the SAME
// request, response is 400 with BOTH violations present in the body).
func TestMustJsonBody_RootAlias_UserEntityInsightCallShape(t *testing.T) {
	var gotUser *jsonBodyUserEntity

	controller := NewController(func(c *Controller) {
		c.Route(route.HttpPost, "/users", func(r *route.Route) {
			r.Handler(func(ctx *execution.Context) {
				gotUser = MustJsonBody[*jsonBodyUserEntity](ctx)
				ctx.Json(gotUser)
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

	t.Run("happy path -> 200, handler receives populated value", func(t *testing.T) {
		gotUser = nil
		payload := map[string]any{
			"id":        int64(1),
			"name":      "John Doe",
			"email":     "john.doe@example.com",
			"isActive":  true,
			"createdAt": time.Now().Format(time.RFC3339),
			"updatedAt": time.Now().Format(time.RFC3339),
			"deletedAt": nil,
			"tags":      []string{"admin", "beta"},
			"scores":    []int{80, 95},
			"addresses": []map[string]any{
				{"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000"},
			},
			"address": map[string]any{
				"street": "Rua A, 123", "city": "São Paulo", "zip": "01310-100",
			},
			"metadata": map[string]any{"source": "insight"},
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if gotUser == nil {
			t.Fatal("handler did not receive a populated *jsonBodyUserEntity")
		}
		if gotUser.Name != "John Doe" || gotUser.Email != "john.doe@example.com" {
			t.Fatalf("gotUser = %+v, want Name=John Doe Email=john.doe@example.com", gotUser)
		}
		if len(gotUser.Addresses) != 1 || gotUser.Addresses[0].Zip != "22000-000" {
			t.Fatalf("gotUser.Addresses = %+v, want 1 address with Zip=22000-000", gotUser.Addresses)
		}
		if gotUser.Address.City != "São Paulo" {
			t.Fatalf("gotUser.Address.City = %q, want %q", gotUser.Address.City, "São Paulo")
		}

		var respBody map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			t.Fatalf("decode response body error = %v", err)
		}
		if respBody["name"] != "John Doe" {
			t.Fatalf("response body = %v, want name=John Doe", respBody)
		}
	})

	t.Run("multi-violation: bad array item AND bad nested object -> 400 with BOTH violations", func(t *testing.T) {
		gotUser = nil
		payload := map[string]any{
			"id":        int64(2),
			"name":      "Jane Doe",
			"email":     "jane.doe@example.com",
			"isActive":  true,
			"createdAt": time.Now().Format(time.RFC3339),
			"updatedAt": time.Now().Format(time.RFC3339),
			"tags":      []string{""}, // violates item Min(1) -> "tags[0]"
			"scores":    []int{80},
			"addresses": []map[string]any{
				{"street": "Rua B, 456", "city": "Rio de Janeiro", "zip": "22000-000"},
			},
			"address": map[string]any{
				// bad Zip pattern -> "address.zip"
				"street": "Rua A, 123", "city": "São Paulo", "zip": "not-a-zip",
			},
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
		if gotUser != nil {
			t.Fatal("handler ran past MustJsonBody, want it to panic before assigning gotUser")
		}

		var respBody struct {
			Name    string `json:"name"`
			Details []struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			} `json:"details"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			t.Fatalf("decode response body error = %v", err)
		}
		if respBody.Name != "BadRequestException" {
			t.Fatalf("response name = %q, want %q", respBody.Name, "BadRequestException")
		}

		var hasArrayItemViolation, hasNestedObjectViolation bool
		for _, v := range respBody.Details {
			if v.Field == "tags[0]" {
				hasArrayItemViolation = true
			}
			if v.Field == "address.zip" {
				hasNestedObjectViolation = true
			}
		}
		if !hasArrayItemViolation {
			t.Fatalf("response details = %+v, want a violation for field %q", respBody.Details, "tags[0]")
		}
		if !hasNestedObjectViolation {
			t.Fatalf("response details = %+v, want a violation for field %q", respBody.Details, "address.zip")
		}
	})
}

// ---------------------------------------------------------------------------
// OpenAPI (OpenAPI Document Builder feature)
// ---------------------------------------------------------------------------

// TestNewOpenApiDocument_RootAlias_InsightBootstrapExample reproduces
// INSIGHT.md's own bootstrap example verbatim (the "# exemplo de bootstrap
// completo" section's gonest.NewOpenApiDocument("3.1.0", func (b
// *gonest.OpenApiDocument) {...}) call shape) through the root gonest
// package's aliases, asserting every field round-trips correctly.
func TestNewOpenApiDocument_RootAlias_InsightBootstrapExample(t *testing.T) {
	type contactConfig struct {
		Name  string
		Url   string
		Email string
	}
	type licenseConfig struct {
		Name string
		Url  string
	}
	config := struct {
		OpenApi struct {
			Title       string
			Description string
			Version     string
			Contact     contactConfig
			License     licenseConfig
		}
	}{}
	config.OpenApi.Title = "Example API"
	config.OpenApi.Description = "An example API"
	config.OpenApi.Version = "1.2.3"
	config.OpenApi.Contact = contactConfig{Name: "Support Team", Url: "https://example.com", Email: "support@example.com"}
	config.OpenApi.License = licenseConfig{Name: "MIT", Url: "https://opensource.org/licenses/MIT"}

	doc := NewOpenApiDocument("3.1.0", func(b *OpenApiDocument) {
		b.Title(config.OpenApi.Title)
		b.Description(config.OpenApi.Description)
		b.Version(config.OpenApi.Version)
		b.Contact(config.OpenApi.Contact.Name, config.OpenApi.Contact.Url, config.OpenApi.Contact.Email)
		b.License(config.OpenApi.License.Name, config.OpenApi.License.Url)
		b.BearerAuth()
	})

	if doc == nil {
		t.Fatal("NewOpenApiDocument() returned nil")
	}
	if got := doc.SpecVersion(); got != "3.1.0" {
		t.Fatalf("SpecVersion() = %q, want %q", got, "3.1.0")
	}
	if got := doc.TitleText(); got != config.OpenApi.Title {
		t.Fatalf("TitleText() = %q, want %q", got, config.OpenApi.Title)
	}
	if got := doc.DescriptionText(); got != config.OpenApi.Description {
		t.Fatalf("DescriptionText() = %q, want %q", got, config.OpenApi.Description)
	}
	if got := doc.VersionText(); got != config.OpenApi.Version {
		t.Fatalf("VersionText() = %q, want %q", got, config.OpenApi.Version)
	}
	name, url, email := doc.ContactInfo()
	if name != config.OpenApi.Contact.Name || url != config.OpenApi.Contact.Url || email != config.OpenApi.Contact.Email {
		t.Fatalf("ContactInfo() = (%q, %q, %q), want (%q, %q, %q)",
			name, url, email, config.OpenApi.Contact.Name, config.OpenApi.Contact.Url, config.OpenApi.Contact.Email)
	}
	licName, licUrl := doc.LicenseInfo()
	if licName != config.OpenApi.License.Name || licUrl != config.OpenApi.License.Url {
		t.Fatalf("LicenseInfo() = (%q, %q), want (%q, %q)",
			licName, licUrl, config.OpenApi.License.Name, config.OpenApi.License.Url)
	}
	if !doc.HasBearerAuth() {
		t.Fatal("HasBearerAuth() = false, want true")
	}
}

// TestGenerateOpenApiSchema_RootAlias_InsightExample reproduces INSIGHT.md's
// settled "Schema Generation from Metadata" example (UserEntity/AddressEntity,
// Controller.Tags/BearerAuth, Route.Summary/RequestBody/Response/PathParams/
// ExcludeFromDocs) entirely through root gonest aliases, then confirms
// GenerateOpenApiSchema(app, doc) produces the expected paths/
// components.schemas shape: the excluded route is absent, a documented
// route has the right method/summary, and the nested AddressEntity schema
// (reused by both UserEntity.Address and the array field) appears exactly
// once.
func TestGenerateOpenApiSchema_RootAlias_InsightExample(t *testing.T) {
	type AddressEntity struct {
		City string
		Zip  string
	}
	type UserIdParams struct {
		UserId string
	}
	type UserEntity struct {
		Id        string
		Name      string
		Address   AddressEntity
		Addresses []AddressEntity
	}

	addressMetadata := NewMetadata[AddressEntity](func(t *AddressEntity, m *Metadata) {
		m.Property(&t.City).String().Required()
		m.Property(&t.Zip).String().Required()
	})

	userIdParamsMetadata := NewMetadata[UserIdParams](func(t *UserIdParams, m *Metadata) {
		m.Property(&t.UserId).String().Required()
	})

	userEntityMetadata := NewMetadata[UserEntity](func(t *UserEntity, m *Metadata) {
		m.Title("UserEntity")
		m.Property(&t.Id).String().Required()
		m.Property(&t.Name).String().Required()
		m.Property(&t.Address).Object(func(om *ObjectMetadata) {
			om.Metadata(addressMetadata)
		}).Required()
		m.Property(&t.Addresses).Array().Items(func(am *ArrayMetadata) {
			am.Object(addressMetadata)
		})
	})

	userController := NewController(func(c *Controller) {
		c.Path("/user")
		c.Tags("users")
		c.BearerAuth()

		c.Route(route.HttpGet, "/:user_id", func(r *Route) {
			r.Summary("Busca um usuario por ID")
			r.PathParams(userIdParamsMetadata)
			r.Response(http.StatusOK, userEntityMetadata)
			r.Response(http.StatusNotFound)
			r.HttpCode(http.StatusOK)
			r.Handler(func(ctx *execution.Context) {
				ctx.Json(map[string]any{"ok": true})
			})
		})

		c.Route(route.HttpGet, "/_internal/debug", func(r *Route) {
			r.ExcludeFromDocs()
			r.HttpCode(http.StatusOK)
			r.Handler(func(ctx *execution.Context) {
				ctx.Json(map[string]any{"ok": true})
			})
		})
	})

	root := NewModule(func(m *Module) {
		m.Controllers(userController)
	})

	app, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	doc := NewOpenApiDocument("3.1.0", func(b *OpenApiDocument) {
		b.Title("Example API")
		b.Version("1.0.0")
	})

	GenerateOpenApiSchema(app, doc)

	document := doc.Document()

	paths, ok := document["paths"].(map[string]any)
	if !ok {
		t.Fatalf("Document()[\"paths\"] type = %T, want map[string]any", document["paths"])
	}

	if _, excluded := paths["/user/_internal/debug"]; excluded {
		t.Fatalf("paths contains excluded route %q, want absent", "/user/_internal/debug")
	}

	item, ok := paths["/user/:user_id"].(map[string]any)
	if !ok {
		t.Fatalf("paths[\"/user/:user_id\"] type = %T, want map[string]any", paths["/user/:user_id"])
	}
	opAny, ok := item["get"]
	if !ok {
		t.Fatalf("paths[\"/user/:user_id\"] missing \"get\" method, got keys %v", item)
	}
	op, ok := opAny.(map[string]any)
	if !ok {
		t.Fatalf("paths[\"/user/:user_id\"][\"get\"] type = %T, want map[string]any", opAny)
	}
	if got := op["summary"]; got != "Busca um usuario por ID" {
		t.Fatalf("summary = %v, want %q", got, "Busca um usuario por ID")
	}
	tags, ok := op["tags"].([]string)
	if !ok || len(tags) != 1 || tags[0] != "users" {
		t.Fatalf("tags = %v, want [\"users\"] (inherited from Controller.Tags)", op["tags"])
	}
	if _, ok := op["security"]; !ok {
		t.Fatalf("operation missing security, want inherited Controller.BearerAuth()")
	}

	components, ok := document["components"].(map[string]any)
	if !ok {
		t.Fatalf("Document()[\"components\"] type = %T, want map[string]any", document["components"])
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("components[\"schemas\"] type = %T, want map[string]any", components["schemas"])
	}

	if _, ok := schemas["UserEntity"]; !ok {
		t.Fatalf("schemas missing %q, got keys %v", "UserEntity", mapKeys(schemas))
	}
	addressCount := 0
	for name := range schemas {
		if name == "AddressEntity" {
			addressCount++
		}
	}
	if addressCount != 1 {
		t.Fatalf("schemas contains %d entries named %q, want exactly 1 (dedup via $ref reuse)", addressCount, "AddressEntity")
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// SetupSwagger (Swagger UI Setup feature)
// ---------------------------------------------------------------------------

// TestSetupSwagger_RootAlias_InsightBootstrapCallShape reproduces INSIGHT.md's
// own bootstrap example verbatim (the "# exemplo de bootstrap completo"
// section's gonest.SetupSwagger(app, config.OpenApi.UiPath, doc,
// gonest.SwaggerOptions{JsonDocumentUrl, PersistAuth, DocExpansion}) call
// shape) through the root gonest package, dispatched via REAL app.Test HTTP
// requests to both routes it registers.
func TestSetupSwagger_RootAlias_InsightBootstrapCallShape(t *testing.T) {
	root := NewModule(func(m *Module) {})

	appInstance, err := NewApp[fiber.FiberApp](root, AppOptions{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	fiberAdapter, ok := appInstance.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("appInstance.Adapter() is not a *fiber.FiberApp: %T", appInstance.Adapter())
	}
	t.Cleanup(func() {
		_ = fiberAdapter.FiberApp().Shutdown()
	})

	doc := NewOpenApiDocument("3.1.0", func(b *OpenApiDocument) {
		b.Title("Example API")
		b.Version("1.0.0")
	})

	if err := SetupSwagger(appInstance, "/docs", doc, SwaggerOptions{
		JsonDocumentUrl: "/openapi.json",
		PersistAuth:     true,
		DocExpansion:    "none",
	}); err != nil {
		t.Fatalf("SetupSwagger() error = %v", err)
	}

	t.Run("GET /openapi.json returns doc.Document() as JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode error = %v", err)
		}
		info, ok := body["info"].(map[string]any)
		if !ok {
			t.Fatalf("body[info] is not a map: %v", body["info"])
		}
		if info["title"] != "Example API" {
			t.Fatalf("body[info][title] = %v, want %q", info["title"], "Example API")
		}
	})

	t.Run("GET /docs returns Swagger UI HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		resp, err := fiberAdapter.FiberApp().Test(req)
		if err != nil {
			t.Fatalf("app.Test error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("Content-Type = %q, want prefix %q", ct, "text/html")
		}

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		html := buf.String()

		if !strings.Contains(html, "/openapi.json") {
			t.Fatalf("expected HTML body to reference JsonDocumentUrl %q, got:\n%s", "/openapi.json", html)
		}
		if !strings.Contains(html, "none") {
			t.Fatalf("expected HTML body to reference configured docExpansion %q, got:\n%s", "none", html)
		}
	})
}

// ---------------------------------------------------------------------------
// MustInjectAll (multi-binding por interface)
// ---------------------------------------------------------------------------

// insightConnectable/insightPostgres/insightRedis/insightConnectableService/
// insightSystemController/insightSystemModule reproduce INSIGHT.md's
// "exemplo de MustInjectAll (multi-binding por interface)" section
// verbatim: two distinct Providers (Postgres, Redis) both satisfying the
// same Connectable interface, injected as a whole slice via MustInjectAll,
// resolved ONCE inside the Controller's own builder closure (phase 2, not
// per-request).
type insightConnectable interface{ Ping() bool }

type insightPostgres struct{}

var _ insightConnectable = (*insightPostgres)(nil)

func (c *insightPostgres) Ping() bool { return true }

var insightPostgresProvider = NewProvider(func(provider *Provider) {
	provider.Constructor(func() *insightPostgres { return &insightPostgres{} })
})

type insightRedis struct{}

var _ insightConnectable = (*insightRedis)(nil)

func (d *insightRedis) Ping() bool { return true }

var insightRedisProvider = NewProvider(func(provider *Provider) {
	provider.Constructor(func() *insightRedis { return &insightRedis{} })
})

type insightConnectableService struct {
	connectables []insightConnectable
}

func (t *insightConnectableService) PingAll() []bool {
	out := make([]bool, 0, len(t.connectables))
	for _, a := range t.connectables {
		out = append(out, a.Ping())
	}
	return out
}

// TestMustInjectAll_RootAlias_InsightConnectableExample reproduces
// INSIGHT.md's Postgres/Redis/Connectable example end to end via a REAL
// app.Test HTTP dispatch: MustInjectAll[Connectable] resolves both
// registered providers as a []Connectable, and the route handler (closing
// over the already-built ConnectableService, no per-request resolution)
// returns both Ping() results.
func TestMustInjectAll_RootAlias_InsightConnectableExample(t *testing.T) {
	systemController := NewController(func(controller *Controller) {
		controller.Path("/health")

		connectables := MustInjectAll[insightConnectable](controller)
		service := &insightConnectableService{connectables: connectables}

		controller.Route(route.HttpGet, "/ping", func(r *Route) {
			r.HttpCode(http.StatusOK)
			r.Handler(func(ctx *execution.Context) {
				ctx.Json(service.PingAll())
			})
		})
	})

	systemModule := NewModule(func(module *Module) {
		module.Providers(insightPostgresProvider, insightRedisProvider)
		module.Controllers(systemController)
	})

	app, err := NewApp[fiber.FiberApp](systemModule, AppOptions{})
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

	req := httptest.NewRequest(http.MethodGet, "/health/ping", nil)
	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got []bool
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if len(got) != 2 || !got[0] || !got[1] {
		t.Fatalf("body = %v, want [true, true] (2 Connectable providers, both Ping()==true)", got)
	}
}

// TestMustInjectAll_ZeroMatches_ReturnsEmptySlice proves MustInjectAll[T]
// returns an empty (not nil-panicking) slice, never panics, when zero
// providers implement T -- spec.md P2 AC2.
func TestMustInjectAll_ZeroMatches_ReturnsEmptySlice(t *testing.T) {
	var got []insightConnectable

	c := NewController(func(controller *Controller) {
		got = MustInjectAll[insightConnectable](controller)
	})

	root := NewModule(func(m *Module) {
		m.Controllers(c)
	})

	if _, err := NewApp[fiber.FiberApp](root, AppOptions{}); err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("MustInjectAll() = %v, want empty slice", got)
	}
}

// TestMustInjectAll_PointerType_RootAlias_Panics proves the root re-export
// preserves MustInjectAll's pointer-type panic (spec.md P2 AC3).
func TestMustInjectAll_PointerType_RootAlias_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected MustInjectAll[*insightPostgres] to panic (pointer type, not interface)")
		}
	}()

	c := NewController(func(controller *Controller) {
		MustInjectAll[*insightPostgres](controller)
	})

	root := NewModule(func(m *Module) {
		m.Providers(insightPostgresProvider)
		m.Controllers(c)
	})

	MustNewApp[fiber.FiberApp](root, AppOptions{})
}

// ---------------------------------------------------------------------------
// Testing (Test App Bootstrap feature -- MustNewTestApp/TestBuilder/MustOverride)
// ---------------------------------------------------------------------------

// insightTestUserEntity/IUserService/UserService/UserServiceMock/
// UserController/UserModule reproduce INSIGHT.md's "exemplo de Testing"
// section: UserController depends on IUserService (an interface, the
// precondition for MustOverride to have anything to intercept -- Go has no
// runtime vtable swap for a concrete struct), UserService is the real
// implementation, UserServiceMock is a hand-written test double.
type insightTestUserEntity struct {
	ID int64 `json:"id"`
}

type insightTestIUserService interface {
	Get(userID int64) *insightTestUserEntity
}

type insightTestUserService struct {
	list []*insightTestUserEntity
}

var _ insightTestIUserService = (*insightTestUserService)(nil)

func (s *insightTestUserService) Get(userID int64) *insightTestUserEntity {
	for _, u := range s.list {
		if u.ID == userID {
			return u
		}
	}
	panic(NewNotFoundException(nil))
}

type insightTestUserServiceMock struct {
	GetFn func(userID int64) *insightTestUserEntity
}

func (m *insightTestUserServiceMock) Get(userID int64) *insightTestUserEntity {
	return m.GetFn(userID)
}

type insightTestUserIDParam struct {
	ID int64 `param:"id"`
}

var insightTestUserIDParamMetadata = NewMetadata[insightTestUserIDParam](func(t *insightTestUserIDParam, m *Metadata) {
	m.Property(&t.ID).Integer().Required()
})

// newInsightTestUserModule builds a FRESH *Module (+ Provider + Controller)
// per call -- a *Module's own builder fn is not idempotent across multiple
// bootstrap calls (Assemble/MustNewTestApp re-running it a second time
// would re-register providers/controllers, producing duplicate routes),
// the same constraint plain NewApp already has (every other test in this
// file that calls NewApp/MustNewApp also builds its own root fresh, never
// reusing a package-level *Module across multiple bootstrap calls).
func newInsightTestUserModule() *Module {
	provider := NewProvider(func(provider *Provider) {
		provider.Constructor(func() *insightTestUserService {
			return &insightTestUserService{list: []*insightTestUserEntity{{ID: 42}}}
		})
	})

	controller := NewController(func(controller *Controller) {
		controller.Path("/user")

		userService := MustInject[insightTestIUserService](controller)

		controller.Route(route.HttpGet, "/:id", func(r *route.Route) {
			r.HttpCode(http.StatusOK)
			r.Handler(func(ctx *execution.Context) {
				p := validate.MustParams[*insightTestUserIDParam](ctx)
				ctx.Json(userService.Get(p.ID))
			})
		})
	})

	return NewModule(func(module *Module) {
		module.Providers(provider)
		module.Controllers(controller)
	})
}

// TestMustNewTestApp_OverrideByInterface_RealHTTPDispatch reproduces
// INSIGHT.md's TestUserController_Get verbatim (adapted: MustRequest/
// AssertStatus/AssertJsonPath belong to the separate, not-yet-built "HTTP
// Test Client" feature -- spec.md's own Out of Scope -- so dispatch here
// uses the same real app.Test mechanism every other HTTP test in this file
// already uses, via tester.Adapter()).
func TestMustNewTestApp_OverrideByInterface_RealHTTPDispatch(t *testing.T) {
	mock := &insightTestUserServiceMock{
		GetFn: func(userID int64) *insightTestUserEntity {
			return &insightTestUserEntity{ID: userID}
		},
	}

	tester := MustNewTestApp(newInsightTestUserModule(), func(b *TestBuilder) {
		MustOverride[insightTestIUserService](b, mock)
	})
	defer tester.Close()

	fiberAdapter, ok := tester.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("tester.Adapter() is not a *fiber.FiberApp: %T", tester.Adapter())
	}
	t.Cleanup(func() { _ = fiberAdapter.FiberApp().Shutdown() })

	req := httptest.NewRequest(http.MethodGet, "/user/42", nil)
	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got insightTestUserEntity
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if got.ID != 42 {
		t.Fatalf("body.ID = %d, want 42 (from the OVERRIDE mock, not the real UserService's seeded user)", got.ID)
	}
}

// TestMustNewTestApp_NoOverride_DirectMustInject_UnitStyle reproduces
// INSIGHT.md's TestUserService_Get_NotFound verbatim: no override,
// MustInject[*UserService](tester) resolves the REAL provider directly
// (unit-test style, no HTTP dispatch at all), and calling Get for a
// missing ID panics the real NotFoundException.
func TestMustNewTestApp_NoOverride_DirectMustInject_UnitStyle(t *testing.T) {
	tester := MustNewTestApp(newInsightTestUserModule(), nil)
	defer tester.Close()

	service := MustInject[*insightTestUserService](tester)

	defer func() {
		exc, ok := recover().(*NotFoundException)
		if !ok {
			t.Fatal("expected a *NotFoundException panic")
		}
		_ = exc
	}()
	service.Get(999)
}

// TestMustNewTestApp_RealProviderConstructor_NeverRunsWhenOverridden proves
// spec.md P3 AC2: the real Provider's Constructor never runs for an
// overridden provider (observable via a side effect -- a counter -- that
// would only increment if the real Constructor executed).
func TestMustNewTestApp_RealProviderConstructor_NeverRunsWhenOverridden(t *testing.T) {
	realConstructorRuns := 0
	p := NewProvider(func(provider *Provider) {
		provider.Constructor(func() *insightTestUserService {
			realConstructorRuns++
			return &insightTestUserService{}
		})
	})
	c := NewController(func(controller *Controller) {
		MustInject[insightTestIUserService](controller)
	})
	m := NewModule(func(module *Module) {
		module.Providers(p)
		module.Controllers(c)
	})

	mock := &insightTestUserServiceMock{GetFn: func(int64) *insightTestUserEntity { return nil }}
	tester := MustNewTestApp(m, func(b *TestBuilder) {
		MustOverride[insightTestIUserService](b, mock)
	})
	defer tester.Close()

	if realConstructorRuns != 0 {
		t.Fatalf("real Constructor ran %d times, want 0 (overridden provider must never invoke its real Constructor)", realConstructorRuns)
	}
}

// TestMustNewTestApp_NilConfigure_BehavesLikeNewAppMinusListen proves
// spec.md P3 AC4: MustNewTestApp(module, nil) bootstraps identically to
// NewApp in every observable way (routes registered, DI resolved) except
// not starting a real HTTP listener.
func TestMustNewTestApp_NilConfigure_BehavesLikeNewAppMinusListen(t *testing.T) {
	tester := MustNewTestApp(newInsightTestUserModule(), nil)
	defer tester.Close()

	fiberAdapter, ok := tester.Adapter().(*fiber.FiberApp)
	if !ok {
		t.Fatalf("tester.Adapter() is not a *fiber.FiberApp: %T", tester.Adapter())
	}
	t.Cleanup(func() { _ = fiberAdapter.FiberApp().Shutdown() })

	req := httptest.NewRequest(http.MethodGet, "/user/42", nil)
	resp, err := fiberAdapter.FiberApp().Test(req)
	if err != nil {
		t.Fatalf("app.Test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (real, non-overridden UserService resolving user 42)", resp.StatusCode, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// HTTP Test Client (MustRequest / AssertStatus / AssertJsonPath)
// ---------------------------------------------------------------------------

// TestMustRequest_RootAlias_InsightTestUserControllerExample reproduces
// INSIGHT.md's TestUserController_Get VERBATIM (unlike
// TestMustNewTestApp_OverrideByInterface_RealHTTPDispatch above, which
// predates this feature and dispatches through the concrete Fiber adapter
// directly) -- tester.MustRequest + res.AssertStatus + res.AssertJsonPath.
func TestMustRequest_RootAlias_InsightTestUserControllerExample(t *testing.T) {
	mock := &insightTestUserServiceMock{
		GetFn: func(userID int64) *insightTestUserEntity {
			return &insightTestUserEntity{ID: userID}
		},
	}

	tester := MustNewTestApp(newInsightTestUserModule(), func(b *TestBuilder) {
		MustOverride[insightTestIUserService](b, mock)
	})
	defer tester.Close()

	res := tester.MustRequest(HttpGet, "/user/42", nil)
	res.AssertStatus(t, http.StatusOK)
	res.AssertJsonPath(t, "id", int64(42))
}

// TestMustRequest_NotFound_StatusPropagatesGenericException proves a
// MustRequest dispatch against a route whose Handler panics an Exception
// (NotFoundException, from the real UserService for a missing ID) produces
// the expected non-2xx status through the SAME MustRequest/AssertStatus
// path -- confirming AssertStatus reads the genuine dispatched status, not
// a hardcoded 200.
func TestMustRequest_NotFound_StatusPropagatesGenericException(t *testing.T) {
	tester := MustNewTestApp(newInsightTestUserModule(), nil)
	defer tester.Close()

	res := tester.MustRequest(HttpGet, "/user/999", nil)
	res.AssertStatus(t, http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// Emitter & Listener (Milestone 9)
// ---------------------------------------------------------------------------

// insightUserCreatedEvent/insightLoggerService/insightUserCreatedListener/
// insightEmitterUserService/insightEmitterUserProvider reproduce INSIGHT.md's
// "exemplo de Emitter" section: a typed event (not a bare string), a
// Listener registered via NewListener+MustOn (itself depending on a
// LoggerService via MustInject, proving Listener's own builder resolves
// direct dependencies too), and a Provider that resolves the framework's
// global Emitter singleton (via MustInject[*Emitter], no explicit
// registration) BEFORE calling its own Constructor -- the real
// Provider-to-Provider/Provider-to-framework-singleton dependency pattern.
type insightUserCreatedEvent struct {
	UserID int64
}

type insightLoggerService struct {
	mu      sync.Mutex
	message string
	userID  int64
}

func (l *insightLoggerService) Log(message string, userID int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.message = message
	l.userID = userID
}

func (l *insightLoggerService) snapshot() (string, int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.message, l.userID
}

type insightEmitterUserService struct {
	emitter *Emitter
}

func (s *insightEmitterUserService) Create(userID int64) {
	s.emitter.Emit(insightUserCreatedEvent{UserID: userID})
}

// TestEmitter_RootAlias_InsightUserCreatedExample reproduces INSIGHT.md's
// "exemplo de Emitter" end to end: emits a typed event from a Provider's
// service (via the framework's global Emitter singleton, no registration),
// confirms the Listener (itself depending on a LoggerService via
// MustInject) runs asynchronously with the correct payload.
func TestEmitter_RootAlias_InsightUserCreatedExample(t *testing.T) {
	logger := &insightLoggerService{}
	loggerProvider := NewProvider(func(provider *Provider) {
		provider.Constructor(func() *insightLoggerService { return logger })
	})

	ranCh := make(chan struct{})
	userCreatedListener := NewListener(func(listener *Listener) {
		loggerDep := MustInject[*insightLoggerService](listener)
		MustOn[insightUserCreatedEvent](listener, func(ctx context.Context, event insightUserCreatedEvent) {
			loggerDep.Log("user created", event.UserID)
			close(ranCh)
		})
	})

	var userService *insightEmitterUserService
	userProvider := NewProvider(func(provider *Provider) {
		em := MustInject[*Emitter](provider)
		provider.Constructor(func() *insightEmitterUserService {
			userService = &insightEmitterUserService{emitter: em}
			return userService
		})
	})

	userModule := NewModule(func(module *Module) {
		module.Providers(loggerProvider, userProvider)
		module.Listeners(userCreatedListener)
	})

	tester := MustNewTestApp(userModule, nil)
	defer tester.Close()

	resolvedService := MustInject[*insightEmitterUserService](tester)
	resolvedService.Create(42)

	select {
	case <-ranCh:
	case <-time.After(2 * time.Second):
		t.Fatal("listener did not run within 2s")
	}

	message, userID := logger.snapshot()
	if message != "user created" || userID != 42 {
		t.Fatalf("logger.snapshot() = (%q, %d), want (\"user created\", 42)", message, userID)
	}
}

// TestMustInject_Emitter_ResolvesFromAnyModule_NoRegistration proves
// spec.md EM-01: MustInject[*Emitter] resolves successfully from ANY
// module, with zero explicit registration anywhere.
func TestMustInject_Emitter_ResolvesFromAnyModule_NoRegistration(t *testing.T) {
	var resolved *Emitter
	c := NewController(func(controller *Controller) {
		resolved = MustInject[*Emitter](controller)
	})

	root := NewModule(func(m *Module) {
		m.Controllers(c)
	})

	if _, err := NewApp[fiber.FiberApp](root, AppOptions{}); err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	if resolved == nil {
		t.Fatal("MustInject[*Emitter] returned nil, want a real Emitter singleton")
	}
}

// ---------------------------------------------------------------------------
// Terminus/health checks (Milestone 11 -- plain Controller, no new type)
// ---------------------------------------------------------------------------

// insightHealthConnectable/insightHealthDb/insightHealthRedis reproduce
// INSIGHT.md's "exemplo de Probes / health" section: Connectable providers
// controllable per-test (toggle up/down), a HealthController built via
// gonest.NewController (no new bootstrap type at all), exposing /readyz
// (aggregates every Connectable via MustInjectAll) and /livez (static OK
// via Context.SendString).
type insightHealthConnectable interface {
	Name() string
	Ping(ctx context.Context) error
}

type insightHealthDb struct {
	mu   sync.Mutex
	down bool
}

func (d *insightHealthDb) Name() string { return "database" }
func (d *insightHealthDb) Ping(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.down {
		return fmt.Errorf("connection refused")
	}
	return nil
}

type insightHealthRedis struct{}

func (r *insightHealthRedis) Name() string                   { return "redis" }
func (r *insightHealthRedis) Ping(ctx context.Context) error { return nil }

func newInsightHealthModule(db *insightHealthDb) *Module {
	dbProvider := NewProvider(func(provider *Provider) {
		provider.Constructor(func() *insightHealthDb { return db })
	})
	redisProvider := NewProvider(func(provider *Provider) {
		provider.Constructor(func() *insightHealthRedis { return &insightHealthRedis{} })
	})

	healthController := NewController(func(controller *Controller) {
		controller.Path("/health")

		connectableList := MustInjectAll[insightHealthConnectable](controller)

		controller.Route(HttpGet, "/readyz", func(r *Route) {
			r.Handler(func(ctx *execution.Context) {
				results, status := make(map[string]string), HttpStatusOk

				for _, c := range connectableList {
					name := c.Name()
					if err := c.Ping(context.Background()); err != nil {
						results[name], status = "down", HttpStatusServiceUnavailable
					} else {
						results[name] = "up"
					}
				}

				ctx.Status(status).Json(map[string]any{"status": "ok", "checks": results})
			})
		})

		controller.Route(HttpGet, "/livez", func(r *Route) {
			r.HttpCode(HttpStatusOk)
			r.Handler(func(ctx *execution.Context) {
				ctx.Status(HttpStatusOk).SendString("OK")
			})
		})
	})

	return NewModule(func(module *Module) {
		module.Providers(dbProvider, redisProvider)
		module.Controllers(healthController)
	})
}

func TestHealthController_RootAlias_InsightExample_Readyz_AllUp(t *testing.T) {
	db := &insightHealthDb{}
	tester := MustNewTestApp(newInsightHealthModule(db), nil)
	defer tester.Close()

	res := tester.MustRequest(HttpGet, "/health/readyz", nil)
	res.AssertStatus(t, http.StatusOK)
	res.AssertJsonPath(t, "checks.database", "up")
	res.AssertJsonPath(t, "checks.redis", "up")
}

func TestHealthController_RootAlias_InsightExample_Readyz_OneDown(t *testing.T) {
	db := &insightHealthDb{down: true}
	tester := MustNewTestApp(newInsightHealthModule(db), nil)
	defer tester.Close()

	res := tester.MustRequest(HttpGet, "/health/readyz", nil)
	res.AssertStatus(t, http.StatusServiceUnavailable)
	res.AssertJsonPath(t, "checks.database", "down")
	res.AssertJsonPath(t, "checks.redis", "up")
}

func TestHealthController_RootAlias_InsightExample_Livez_AlwaysOk(t *testing.T) {
	db := &insightHealthDb{}
	tester := MustNewTestApp(newInsightHealthModule(db), nil)
	defer tester.Close()

	res := tester.MustRequest(HttpGet, "/health/livez", nil)
	res.AssertStatus(t, http.StatusOK)
}

// ---------------------------------------------------------------------------
// Scheduler (Milestone 10)
// ---------------------------------------------------------------------------

// insightSchedulerUserService/insightCleanupScheduler reproduce INSIGHT.md's
// "exemplo de Schedule" section: a Scheduler depending on a service via
// MustInject (same builder-time resolution as Controller/Listener), with
// Cron/Interval/Timeout jobs -- test uses millisecond-scale durations
// instead of INSIGHT.md's real-world ones (time.Minute etc), same
// adaptation this feature's own spec.md's Independent Test calls for.
type insightSchedulerUserService struct {
	purgeCh  chan struct{}
	pingCh   chan struct{}
	warmupCh chan struct{}
}

func (s *insightSchedulerUserService) PurgeExpired(ctx context.Context) {
	select {
	case s.purgeCh <- struct{}{}:
	default:
	}
}
func (s *insightSchedulerUserService) Ping(ctx context.Context) {
	select {
	case s.pingCh <- struct{}{}:
	default:
	}
}
func (s *insightSchedulerUserService) WarmupCache(ctx context.Context) {
	close(s.warmupCh)
}

func TestScheduler_RootAlias_InsightCleanupSchedulerExample(t *testing.T) {
	userService := &insightSchedulerUserService{
		purgeCh:  make(chan struct{}, 1),
		pingCh:   make(chan struct{}, 1),
		warmupCh: make(chan struct{}),
	}
	userProvider := NewProvider(func(provider *Provider) {
		provider.Constructor(func() *insightSchedulerUserService { return userService })
	})

	cleanupScheduler := NewScheduler(func(scheduler *Scheduler) {
		us := MustInject[*insightSchedulerUserService](scheduler)
		scheduler.Cron("cleanup-expired-users", "0 0 * * *", func(ctx context.Context) { us.PurgeExpired(ctx) })
		scheduler.Interval("healthcheck-ping", 10*time.Millisecond, func(ctx context.Context) { us.Ping(ctx) })
		scheduler.Timeout("warmup-cache", 20*time.Millisecond, func(ctx context.Context) { us.WarmupCache(ctx) })
	})

	appModule := NewModule(func(module *Module) {
		module.Providers(userProvider)
		module.Schedulers(cleanupScheduler)
	})

	tester := MustNewTestApp(appModule, nil)
	defer tester.Close()

	select {
	case <-userService.pingCh:
	case <-time.After(time.Second):
		t.Fatal("Interval job (healthcheck-ping) did not fire within 1s")
	}

	select {
	case <-userService.warmupCh:
	case <-time.After(time.Second):
		t.Fatal("Timeout job (warmup-cache) did not fire within 1s")
	}
}
