package gonest

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gonest-dev/gonest/internal/fiberapp"
	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/route"
)

// TestNewPipe_RootAlias_TypeCheck proves NewPipe/Pipe resolve and
// type-check at the root gonest package: NewPipe builds a *Pipe, Handler
// accepts a valid func(ctx, raw string) T signature (validated via
// reflect), and route.Route.Param genuinely declares it (running the
// deferred fn) without the caller needing to call Declare manually -- see
// internal/route.Route.Param's doc comment for why that matters.
func TestNewPipe_RootAlias_TypeCheck(t *testing.T) {
	p := NewPipe(func(p *Pipe) {
		p.Handler(func(ctx *httpctx.Context, raw string) int {
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
// an int64, panicking a BadRequestException (built-in Exception, from the
// "HttpException Core" feature) with the invalid raw value as Details on
// failure.
var ParseIntPipe = NewPipe(func(pipe *Pipe) {
	pipe.Handler(func(ctx *httpctx.Context, raw string) int64 {
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
// not just at construction time -- this is exactly the path that was
// broken before internal/route.Route.Param started calling Declare
// itself). Root has no Route/HttpGet aliases yet (pre-existing gap,
// documented in STATE.md's Deferred Ideas, same precedent as
// guard_test.go/middleware_test.go), so internal/route and
// internal/httpctx are imported directly here.
func TestParseIntPipe_RootAlias_InsightCallShape(t *testing.T) {
	var gotID int64
	handlerRan := false

	controller := NewController(func(c *Controller) {
		c.Route(route.HttpGet, "/items/:id", func(r *route.Route) {
			r.Param("id", ParseIntPipe)
			r.Handler(func(ctx *httpctx.Context) {
				gotID = MustParam[int64](ctx, "id")
				handlerRan = true
				ctx.Json(map[string]int64{"id": gotID})
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
