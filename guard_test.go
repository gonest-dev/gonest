package gonest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gonest-dev/gonest/internal/fiberapp"
	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/route"
)

// TestNewGuard_RootAlias_TypeCheck proves NewGuard/Guard resolve and
// type-check at the root gonest package: NewGuard builds a *Guard, Handler
// accepts a func(ctx) bool, and the resulting HandlerFunc genuinely reaches
// ctx through to the handler body and returns its own decision -- the same
// smoke-test shape middleware_test.go uses for its own root-aliased
// constructor (NewMiddleware).
func TestNewGuard_RootAlias_TypeCheck(t *testing.T) {
	var gotCtx *httpctx.Context

	g := NewGuard(func(g *Guard) {
		g.Handler(func(ctx *httpctx.Context) bool {
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

	ctx := httpctx.New(nil)
	result := fn(ctx)

	if gotCtx != ctx {
		t.Fatal("ctx passed to the stored handler did not reach the handler body unchanged")
	}
	if !result {
		t.Fatal("HandlerFunc() result did not reach back out as the handler's own decision")
	}
}

// authService is a stand-in for INSIGHT.md's AuthGuard example's
// AuthService, adapted per this feature's spec.md's Out of Scope: no
// MustInject support (Guard.New(fn) runs fn immediately, before any
// module-tree/DI context exists -- see internal/guard.New's doc comment).
// Instead of injecting it, AuthGuard below closes over an already-
// constructed instance directly, same as any other plain Go closure would.
type authService struct{}

// Validate reports whether token is considered valid. In this stand-in,
// any non-empty token other than the literal "invalid-token" is valid --
// enough to prove both the false->403 path and the true->Handler-runs path.
func (a *authService) Validate(token string) bool {
	return token != "invalid-token"
}

var stubAuthService = &authService{}

// AuthGuard reproduces INSIGHT.md's AuthGuard example, adapted per
// spec.md's Out of Scope (no MustInject): rather than
// gonest.MustInject[*AuthService](guard), it closes over the
// package-level stubAuthService directly. The rest of the example is
// faithful: missing Authorization header panics a custom
// gonest.NewUnauthorizedException(nil) (proving the custom-exception-on-
// invalid path), otherwise it returns authService.Validate(token) as a
// plain bool (proving the false->automatic 403 path and the true->Handler-
// runs path).
var AuthGuard = NewGuard(func(guard *Guard) {
	guard.Handler(func(ctx *httpctx.Context) bool {
		token := ctx.Header("Authorization")
		if token == "" {
			panic(NewUnauthorizedException(nil))
		}
		return stubAuthService.Validate(token)
	})
})

// TestAuthGuard_RootAlias_InsightCallShape proves INSIGHT.md's AuthGuard
// example (adapted per spec.md's Out of Scope, see AuthGuard's own doc
// comment) compiles and works end-to-end through the root gonest package's
// Guard/NewGuard aliases, attached via controller.Guards(AuthGuard) through
// the root Controller/Module/NewApp aliases, and dispatched via REAL
// app.Test requests covering all 3 cases. Root has no Route/HttpGet
// aliases yet (a pre-existing gap, documented in STATE.md's Deferred
// Ideas -- see middleware_test.go's own precedent for the same reasoning),
// so internal/route and internal/httpctx are imported directly here.
func TestAuthGuard_RootAlias_InsightCallShape(t *testing.T) {
	handlerRan := false

	controller := NewController(func(c *Controller) {
		c.Guards(AuthGuard)
		c.Route(route.HttpGet, "/secure", func(r *route.Route) {
			r.Handler(func(ctx *httpctx.Context) {
				handlerRan = true
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
