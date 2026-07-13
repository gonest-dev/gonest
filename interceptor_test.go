package gonest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gonest-dev/gonest/internal/fiberapp"
	"github.com/gonest-dev/gonest/internal/httpctx"
	interceptorpkg "github.com/gonest-dev/gonest/internal/interceptor"
	"github.com/gonest-dev/gonest/internal/route"
)

// TestNewInterceptor_RootAlias_TypeCheck proves NewInterceptor/Interceptor
// resolve and type-check at the root gonest package: NewInterceptor builds
// a *Interceptor, Handler accepts a func(ctx, next interceptor.Next), and
// the resulting HandlerFunc genuinely reaches ctx/next through to the
// handler body -- the same smoke-test shape guard_test.go/middleware_test.go
// use for their own root-aliased constructors (NewGuard/NewMiddleware).
// interceptor.Next itself has no root alias yet (only Interceptor/
// NewInterceptor are in this feature's scope, see interceptor.go's doc
// comment and design.md's Tech Decisions on why interceptor.Next is
// deliberately its own type, not reused from middleware.Next), so
// internal/interceptor is imported directly here for the Next parameter
// type, same precedent as internal/httpctx/internal/route being imported
// directly for pieces with no root alias yet.
func TestNewInterceptor_RootAlias_TypeCheck(t *testing.T) {
	var gotCtx *httpctx.Context
	nextCalled := false

	i := NewInterceptor(func(i *Interceptor) {
		i.Handler(func(ctx *httpctx.Context, next interceptorpkg.Next) {
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

// timingLog is a stand-in for INSIGHT.md's TimingInterceptor example's
// LoggerService, adapted per this feature's spec.md's Out of Scope: no
// MustInject support (Interceptor.New(fn) runs fn immediately, before any
// module/DI context exists -- see internal/interceptor.New's doc comment).
// Instead of injecting a logger, TimingInterceptor below closes over an
// already-constructed package-level slice directly, same as any other plain
// Go closure would, and records each logged entry so the test can assert
// on before/after ordering.
var timingLog []string

// TimingInterceptor reproduces INSIGHT.md's TimingInterceptor example,
// adapted per spec.md's Out of Scope (no MustInject): rather than
// gonest.MustInject[*LoggerService](interceptor), it closes over the
// package-level timingLog directly. The rest of the example is faithful:
// start := time.Now(), next(ctx) runs the rest of the chain/Handler, then
// it logs something time-related -- proving the log call happens AFTER
// next(ctx) returns via observable ordering (timingLog's contents), not by
// measuring real elapsed time precisely.
var TimingInterceptor = NewInterceptor(func(interceptor *Interceptor) {
	interceptor.Handler(func(ctx *httpctx.Context, next interceptorpkg.Next) {
		start := time.Now()
		timingLog = append(timingLog, "before")
		next(ctx)
		timingLog = append(timingLog, "request took "+time.Since(start).String())
	})
})

// TestTimingInterceptor_RootAlias_InsightCallShape proves INSIGHT.md's
// TimingInterceptor example (adapted per spec.md's Out of Scope, see
// TimingInterceptor's own doc comment) compiles and works end-to-end
// through the root gonest package's Interceptor/Next/NewInterceptor
// aliases, attached via controller.Interceptors(TimingInterceptor) through
// the root Controller/Module/NewApp aliases, and dispatched via a REAL
// app.Test request -- confirming both the before-Handler and after-Handler
// logic ran, in the right order (before logged first, then the route
// Handler ran, then after logged last). Root has no Route/HttpGet aliases
// yet (a pre-existing gap, documented in STATE.md's Deferred Ideas -- see
// guard_test.go/middleware_test.go's own precedent for the same reasoning),
// so internal/route and internal/httpctx are imported directly here.
func TestTimingInterceptor_RootAlias_InsightCallShape(t *testing.T) {
	timingLog = nil
	handlerRan := false

	controller := NewController(func(c *Controller) {
		c.Interceptors(TimingInterceptor)
		c.Route(route.HttpGet, "/timed", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
				timingLog = append(timingLog, "handler")
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
