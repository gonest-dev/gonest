package gonest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gonest-dev/gonest/internal/fiberapp"
	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/route"
	"github.com/google/uuid"
)

// TestNewMiddleware_RootAlias_TypeCheck proves NewMiddleware/Middleware/Next
// all resolve and type-check at the root gonest package: NewMiddleware
// builds a *Middleware, Handler accepts a func(ctx, next Next), and the
// resulting HandlerFunc genuinely reaches ctx/next through to the handler
// body -- the same smoke-test shape exception_test.go uses for its own
// root-aliased constructor (NewHttpException).
func TestNewMiddleware_RootAlias_TypeCheck(t *testing.T) {
	var gotCtx *httpctx.Context
	nextCalled := false

	m := NewMiddleware(func(m *Middleware) {
		m.Handler(func(ctx *httpctx.Context, next Next) {
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

	ctx := httpctx.New(nil)
	fn(ctx, func(ctx *httpctx.Context) {
		nextCalled = true
	})

	if gotCtx != ctx {
		t.Fatal("ctx passed to the stored handler did not reach the handler body unchanged")
	}
	if !nextCalled {
		t.Fatal("next passed to the stored handler was not called/did not reach the handler body")
	}
}

// RequestIdMiddleware reproduces INSIGHT.md's Middleware example (around
// line 169) verbatim, through the root-aliased NewMiddleware/Middleware/Next.
// It generates a UUID and sets it as the X-Request-Id response header before
// calling next(ctx).
var RequestIdMiddleware = NewMiddleware(func(middleware *Middleware) {
	middleware.Handler(func(ctx *httpctx.Context, next Next) {
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
// X-Request-Id header genuinely lands in the real HTTP response. Root has no
// Route/HttpGet aliases yet (a pre-existing gap from an earlier feature, out
// of scope for T5 -- same reasoning app_test.go already notes for the
// missing gonest.FiberApp alias), so internal/route and internal/httpctx are
// imported directly here, same as app_test.go's own precedent.
func TestRequestIdMiddleware_RootAlias_InsightCallShape(t *testing.T) {
	controller := NewController(func(c *Controller) {
		c.Use(RequestIdMiddleware)
		c.Route(route.HttpGet, "/ping", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				ctx.Json(map[string]string{"ok": "true"})
			})
		})
	})

	root := NewModule(func(m *Module) {
		m.Controllers(controller)
	})

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
