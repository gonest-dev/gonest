// Package fiberapp is the adapter that translates gonest's HTTP-agnostic
// Route/Context abstractions into a real Fiber v3 application. Together with
// internal/httpctx, it is one of only two packages in this codebase allowed
// to import Fiber directly (see design.md's "FiberApp (adapter)" component)
// -- every other package (internal/route, internal/controller, internal/pipe)
// only ever sees the Fiber-agnostic *httpctx.Context.
package fiberapp

import (
	"github.com/gofiber/fiber/v3"

	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/route"
)

// FiberApp wraps a real *fiber.App and satisfies the minimal httpAdapter
// contract NewApp[T] (future T8) needs: RegisterRoute and Listen. Kept as a
// single unexported field, per design.md's Data Models -- FiberApp itself
// has no other state, it is purely a translation layer.
type FiberApp struct {
	app *fiber.App
}

// New builds a FiberApp around a freshly constructed *fiber.App with default
// config. Exported (rather than requiring callers to reach into Fiber
// themselves) so T8's NewApp[T] can construct one via reflection/generics
// without importing Fiber itself.
func New() *FiberApp {
	return &FiberApp{app: fiber.New()}
}

// FiberApp returns the underlying *fiber.App. Exported for tests (this
// package's own integration tests dispatch through app.Test(req), Fiber's
// own no-port-required test helper -- see TESTING.md) and for T8, which
// needs to call Listen indirectly via the httpAdapter contract but may need
// the raw app for anything the minimal contract doesn't cover yet.
func (f *FiberApp) FiberApp() *fiber.App {
	return f.app
}

// fiberMethod maps gonest's HttpMethod enum to the string Fiber's
// app.Add(methods []string, ...) expects. A local switch rather than
// reusing HttpMethod.String() (internal/route/method.go) because that
// String() is documented as debug-friendly output, not a promise to stay
// byte-identical to Fiber's method constants -- keeping the two mappings
// separate means a future debug-string tweak in method.go can't silently
// break route registration here.
func fiberMethod(method route.HttpMethod) string {
	switch method {
	case route.HttpGet:
		return fiber.MethodGet
	case route.HttpPost:
		return fiber.MethodPost
	case route.HttpPut:
		return fiber.MethodPut
	case route.HttpDelete:
		return fiber.MethodDelete
	case route.HttpQuery:
		return "QUERY"
	default:
		return fiber.MethodGet
	}
}

// RegisterRoute registers a real route on the internal *fiber.App. The
// handler Fiber invokes is a thin wrapper: it builds a *httpctx.Context
// around a fiberResponder for this specific request/response pair, then
// runs the gonest Handler inside the wrapper's OWN recover() -- per
// design.md's Tech Decisions, this deliberately does NOT use Fiber's
// error-return contract (func(fiber.Ctx) error) as the panic-propagation
// path, and does NOT install Fiber's `recover` middleware. gonest's Handler
// signature is func(ctx *httpctx.Context) with no return value -- panic is
// the only way a Handler signals failure, consistent with how
// Constructor/MustInject already work elsewhere in this framework (see
// internal/provider, param.go). A panic with anything that is not (yet --
// Milestone 2 introduces a structured Exception type) recognized becomes a
// generic 500, never crashes the process, and leaks no internal detail.
func (f *FiberApp) RegisterRoute(method route.HttpMethod, path string, h func(ctx *httpctx.Context)) error {
	wrapped := func(c fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error") //nolint:errcheck // best-effort write on an already-failed request
			}
		}()

		ctx := httpctx.New(&fiberResponder{c: c})
		h(ctx)
		return nil
	}

	f.app.Add([]string{fiberMethod(method)}, path, wrapped)
	return nil
}

// Listen starts the underlying Fiber app listening on addr. Only used by a
// later feature (per design.md's Interfaces note on the httpAdapter
// contract) -- included now so FiberApp already satisfies the full contract
// T8 needs.
func (f *FiberApp) Listen(addr string) error {
	return f.app.Listen(addr)
}

// fiberResponder is the real, Fiber-backed implementation of
// httpctx.Responder -- the counterpart to context_test.go's fakeResponder,
// but wired to an actual fiber.Ctx instead of in-memory maps. Constructed
// fresh per request inside RegisterRoute's wrapper, since fiber.Ctx itself
// is only valid for the lifetime of a single request.
type fiberResponder struct {
	c fiber.Ctx
}

// JSON writes v as the JSON response body via Fiber's own Ctx.JSON.
func (r *fiberResponder) JSON(v any) error {
	return r.c.JSON(v)
}

// SetStatus sets the response status code via Fiber's Ctx.Status, which
// returns the Ctx for its own chaining -- discarded here since
// httpctx.Responder's contract returns nothing.
func (r *fiberResponder) SetStatus(code int) {
	r.c.Status(code)
}

// GetHeader returns the named request header's value via Fiber's Ctx.Get.
func (r *fiberResponder) GetHeader(name string) string {
	return r.c.Get(name)
}

// SetHeaderValue sets the named response header via Fiber's Ctx.Set.
func (r *fiberResponder) SetHeaderValue(name, value string) {
	r.c.Set(name, value)
}

// GetParam returns the named route param's raw string value via Fiber's
// Ctx.Params, which itself returns "" for a param that doesn't exist on the
// matched route -- the same ambiguity route.Route.HasParam (internal/route/
// route.go) exists to disambiguate at the MustParam[T] layer.
func (r *fiberResponder) GetParam(name string) string {
	return r.c.Params(name)
}
