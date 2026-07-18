// Package fiber is the adapter that translates gonest's HTTP-agnostic
// Route/Request/Response abstractions into a real Fiber v3 application.
// Together with internal/execution, it is one of only two packages in this
// codebase allowed to import Fiber directly (see design.md's "FiberApp
// (adapter)" component) -- every other package (internal/route,
// internal/controller, internal/validate) only ever sees the Fiber-agnostic
// *execution.Request/*execution.Response.
package fiber

import (
	"bufio"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"

	"gonest.dev/gonest/internal/appoptions"
	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/graphql"
	"gonest.dev/gonest/internal/route"
)

// FiberApp wraps a real *fiber.App and satisfies the minimal httpAdapter
// contract NewApp[T] (future T8) needs: RegisterRoute and Listen. Kept as a
// single unexported field, per design.md's Data Models -- FiberApp itself
// has no other state, it is purely a translation layer.
type FiberApp struct {
	app *fiber.App
}

// New builds a FiberApp around a freshly constructed *fiber.App with default
// config (equivalent to Init(appoptions.AppOptions{})). Exported (rather than
// requiring callers to reach into Fiber themselves) so T8's NewApp[T] can
// construct one via reflection/generics without importing Fiber itself.
func New() *FiberApp {
	f := &FiberApp{}
	f.Init(appoptions.AppOptions{})
	return f
}

// Init lazily sets f.app to a freshly constructed *fiber.App if it is not
// already set. It is a no-op on any call after the first (mirroring
// Controller.Declare/Provider.Declare's idempotency pattern), so it is safe
// to call unconditionally.
//
// This exists for internal/app's generic NewApp[T HttpAdapter]: it builds
// the zero-value T via reflect.New(reflect.TypeFor[T]()).Interface(), which
// for FiberApp produces &FiberApp{app: nil} -- calling RegisterRoute on that
// would nil-panic dereferencing f.app. NewApp[T] calls Init(opts) once right
// after construction (part of the HttpAdapter contract), which for FiberApp
// fills in the real *fiber.App. New() above also calls Init() internally so
// both construction paths converge on the same lazy-init logic instead of
// duplicating "fiber.New() unless already set".
//
// opts.EnableFormStreaming, when true, sets BOTH StreamRequestBody AND
// DisablePreParseMultipartForm on the underlying fiber.Config -- confirmed
// via fasthttp@v1.72.0's own source (server.go/http.go) that BOTH are
// required together: multipart pre-parsing stays on by default even with
// StreamRequestBody alone, which would silently buffer the whole multipart
// body before any Handler runs, defeating streaming entirely (Multipart
// Form Streaming feature, AD-022 in STATE.md). fiber.Config is immutable
// once fiber.New() returns, so this MUST happen here, at construction time
// -- there is no later hook to flip it on.
func (f *FiberApp) Init(opts appoptions.AppOptions) {
	if f.app == nil {
		f.app = fiber.New(fiber.Config{
			StreamRequestBody:            opts.EnableFormStreaming,
			DisablePreParseMultipartForm: opts.EnableFormStreaming,
		})
	}
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
	case route.HttpPatch:
		return fiber.MethodPatch
	case route.HttpDelete:
		return fiber.MethodDelete
	case route.HttpHead:
		return fiber.MethodHead
	case route.HttpOptions:
		return fiber.MethodOptions
	case route.HttpTrace:
		return fiber.MethodTrace
	case route.HttpConnect:
		return fiber.MethodConnect
	case route.HttpQuery:
		return fiber.MethodQuery
	default:
		panic("gonest: unknown HttpMethod " + method.String())
	}
}

// RegisterRoute registers a real route on the internal *fiber.App. The
// handler Fiber invokes is a thin wrapper: it builds a *execution.Request/
// *execution.Response pair around a fiberResponder for this specific
// request/response pair, then runs the gonest Handler inside the wrapper's
// OWN recover() -- per design.md's Tech Decisions, this deliberately does
// NOT use Fiber's error-return contract (func(fiber.Ctx) error) as the
// panic-propagation path, and does NOT install Fiber's `recover` middleware.
// gonest's Handler signature is func(req *execution.Request, res
// *execution.Response) with no return value -- panic is the only way a
// Handler signals failure, consistent with how Constructor/MustInject
// already work elsewhere in this framework (see internal/provider,
// param.go).
//
// The recover branch first type-asserts the recovered value against
// exception.Exception -- an interface satisfied structurally by anything
// embedding exception.HttpException (see internal/exception's doc comment).
// Per design.md's Tech Decisions, detection is interface-based rather than a
// type switch over the framework's five built-ins: a dev-defined exception
// type (INSIGHT.md's `type FooExampleError struct { gonest.HttpException }`
// pattern) must be recognized identically to a built-in, with zero
// awareness on this package's part of what concrete types exist. When the
// assertion succeeds, the response is built via res.Json -- the SAME
// *execution.Response already constructed above for the Handler call -- so
// the Exception branch goes through the one Fiber-agnostic path every other
// gonest response does, rather than reaching for raw fiber.Ctx calls. A
// panic value that does NOT satisfy Exception (including panic(nil), which
// Go 1.21+ turns into a non-nil *runtime.PanicNilError rather than a nil
// recover() -- see https://go.dev/doc/go1.21#runtime) falls through to the
// pre-existing generic-500 fallback UNCHANGED: that fallback intentionally
// keeps using raw fiber.Ctx calls (not res.Json) because it is a
// last-resort, best-effort write for a case where we deliberately know
// nothing about the panic value and must not risk it leaking into the
// response -- it never crashes the process and leaks no internal detail.
func (f *FiberApp) RegisterRoute(method route.HttpMethod, path string, h func(req *execution.Request, res *execution.Response)) error {
	wrapped := func(c fiber.Ctx) error {
		req, res := execution.New(&fiberResponder{c: c})

		defer func() {
			if r := recover(); r != nil {
				if exc, ok := r.(exception.Exception); ok {
					res.Status(exc.Status()).Json(map[string]any{ //nolint:errcheck // best-effort write on an already-failed request
						"name":    exception.EffectiveName(exc),
						"message": exc.Message(),
						"details": exc.Details(),
					})
					return
				}
				c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error") //nolint:errcheck // best-effort write on an already-failed request
			}
		}()

		h(req, res)
		return nil
	}

	f.app.Add([]string{fiberMethod(method)}, path, wrapped)
	return nil
}

// RegisterWebSocket wires one WebSocket upgrade endpoint onto f.app, via
// github.com/gofiber/contrib/v3/websocket (confirmed real API through
// Context7: `app.Use(path, upgradeCheckMiddleware)` followed by
// `app.Get(path, websocket.New(func(c *websocket.Conn) {...}))` -- the
// middleware step is required, per that package's own README, so a
// non-upgrade request to the same path gets fiber.ErrUpgradeRequired
// instead of falling through to the WebSocket handler).
func (f *FiberApp) RegisterWebSocket(path string, h func(conn graphql.WSConn)) error {
	f.app.Use(path, func(c fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	f.app.Get(path, websocket.New(func(c *websocket.Conn) {
		h(&fiberWSConn{c: c})
	}))
	return nil
}

// fiberWSConn adapts *websocket.Conn to graphql.WSConn -- same
// adapter-agnostic-wrapper rationale as fiberResponder for
// execution.Responder.
type fiberWSConn struct {
	c *websocket.Conn
}

func (w *fiberWSConn) ReadMessage() (int, []byte, error) {
	return w.c.ReadMessage()
}

func (w *fiberWSConn) WriteMessage(messageType int, data []byte) error {
	return w.c.WriteMessage(messageType, data)
}

func (w *fiberWSConn) Close() error {
	return w.c.Close()
}

// CloseWithCode is a temporary stub satisfying the execution.WSConn
// method added in T2 (graphql-realtime-protocols feature, Milestone
// 18) -- ignores code/reason and just delegates to Close(); real
// close-code handling (graphql-transport-ws's own 4400/4401/4408/4409/
// 4429) lands in T4, out of this task's scope.
func (w *fiberWSConn) CloseWithCode(code int, reason string) error {
	return w.c.Close()
}

func (w *fiberWSConn) Params(name string) string {
	return w.c.Params(name)
}

func (w *fiberWSConn) Query(name string) string {
	return w.c.Query(name)
}

// Listen starts the underlying Fiber app listening on addr, blocking until
// it stops. If onListen is non-nil, it is registered via Fiber's own
// Hooks().OnListen BEFORE the blocking f.app.Listen(addr) call -- per
// design.md's "HttpAdapter.Listen (extended contract)" component, Fiber
// runs its onListen hooks synchronously right after its own bind succeeds
// (app.go's startupProcess/runOnListenHooks, called from Listen's own
// goroutine setup, right before the accept loop's app.server.Serve(ln)
// takes over) -- exactly the "signal bind succeeded before blocking for
// good" semantics this contract needs, without gonest reimplementing that
// synchronization itself with a goroutine+channel (see design.md's Tech
// Decisions table). onListen being nil means "no hook wanted": Fiber's own
// Hooks().OnListen never gets called, so nothing is registered and no
// nil-func-call panic risk exists.
func (f *FiberApp) Listen(addr string, onListen func()) error {
	if onListen != nil {
		f.app.Hooks().OnListen(func(fiber.ListenData) error {
			onListen()
			return nil
		})
	}
	// DisableStartupMessage suppresses Fiber's own ASCII-art banner --
	// gonest presents its own adapter-agnostic startup log instead (see
	// internal/app.App.MustListen), so every adapter (present or future)
	// looks identical to a caller regardless of which HTTP engine actually
	// answers requests underneath.
	return f.app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
}

// Test dispatches req against f.app in-memory, via Fiber's own *fiber.App.Test
// (no real network port bound) -- satisfies internal/app's HttpAdapter
// contract for MustNewTestApp's MustRequest ("HTTP Test Client" feature).
// Every test in this codebase already reaches this same underlying Fiber
// method directly (via FiberApp() above); this is simply the generic,
// HttpAdapter-interface-level path to it.
func (f *FiberApp) Test(req *http.Request) (*http.Response, error) {
	return f.app.Test(req)
}

// fiberResponder is the real, Fiber-backed implementation of
// execution.Responder -- the counterpart to context_test.go's fakeResponder,
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
// execution.Responder's contract returns nothing.
func (r *fiberResponder) SetStatus(code int) {
	r.c.Status(code)
}

// GetStatus returns the response status code currently set for this request
// via Fiber's own Ctx.Response().StatusCode() -- fasthttp defaults this to
// 200 until SetStatus/Ctx.Status changes it, so this is always meaningful
// even if the Handler never calls Status explicitly.
func (r *fiberResponder) GetStatus() int {
	return r.c.Response().StatusCode()
}

// GetMethod returns the incoming request's HTTP method via Fiber's own
// Ctx.Method.
func (r *fiberResponder) GetMethod() string {
	return r.c.Method()
}

// GetPath returns the incoming request's full path via Fiber's own Ctx.Path.
func (r *fiberResponder) GetPath() string {
	return r.c.Path()
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
// route.go) exists to disambiguate at the MustParams[T]/MustQuery[T] layer
// (internal/validate).
//
// Fiber's own Ctx.Params doc comment warns the returned string is "only
// valid within the handler" and to "make copies... to use the value outside
// the Handler" -- it is a zero-copy view into a fasthttp request buffer
// that gets reused for the NEXT request once this handler returns. gonest's
// MustParams[T] contract gives callers a plain typed struct with no such
// caveat -- e.g. a Handler doing
// `p := gonest.MustParams[*NameParams](ctx); entity.Name = p.Name` and
// persisting entity beyond the request is the expected, supported usage,
// not a misuse of an internal API. Without this copy, that string's bytes
// can be silently overwritten by a later request sharing the same
// underlying buffer (observed as flaky corrupted param values under
// sequential app.Test calls in internal/app's UserController end-to-end
// test). strings.Clone forces a fresh, independently-backed copy.
func (r *fiberResponder) GetParam(name string) string {
	return strings.Clone(r.c.Params(name))
}

// RawBody returns the raw request body bytes via Fiber's own Ctx.Body.
//
// Unlike GetParam above, this deliberately does NOT defensively copy (see
// L-009 in STATE.md, which is why GetParam clones): L-009's bug was about a
// value RETAINED past the request (stored in a struct field, read later by
// some other request). BodySource.Raw()'s own doc comment documents the
// same constraint on ITS callers -- jsonBodySource calls json.Unmarshal on
// the body synchronously, within the same handler execution RawBody() is
// called in, and encoding/json copies string/byte data into the destination
// values during decode rather than retaining the input slice. As long as no
// future caller stores the raw []byte past the synchronous validation call,
// there is no reuse-corruption risk.
func (r *fiberResponder) RawBody() []byte {
	return r.c.Body()
}

// Queries returns the request's query-string params via Fiber's own
// Ctx.Queries. Although Fiber allocates a FRESH map on every call (unlike
// GetParam's single string lookup), its keys/values are built via the
// app's configured toString conversion, which defaults to
// utils.UnsafeString -- an unsafe, zero-copy view into fasthttp's
// underlying request buffer (same class of footgun L-009 in STATE.md
// documents for GetParam). That buffer gets reused for the NEXT request
// once this handler returns, so each string is defensively cloned here,
// same rationale/precedent as GetParam's strings.Clone.
func (r *fiberResponder) Queries() map[string]string {
	raw := r.c.Queries()
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[strings.Clone(k)] = strings.Clone(v)
	}
	return out
}

// HTML sets the response Content-Type to text/html (via Fiber's own
// Ctx.Type, mirroring the extension-based convention Fiber's own static/
// render paths use internally) and writes s as the raw response body via
// Ctx.SendString -- SetupSwagger's (internal/openapi) HTML endpoint is the
// only caller today.
func (r *fiberResponder) HTML(s string) error {
	r.c.Type("html")
	return r.c.SendString(s)
}

// SendString writes s as the raw response body via Fiber's own
// Ctx.SendString, with NO Content-Type override -- unlike HTML above, which
// deliberately sets text/html first.
func (r *fiberResponder) SendString(s string) error {
	return r.c.SendString(s)
}

// BodyStream returns the raw request body stream plus its multipart
// boundary, when BOTH are true: (1) the request's own Content-Type is
// multipart/form-data with a boundary, and (2) this app was built with
// AppOptions.EnableFormStreaming (which sets fasthttp's StreamRequestBody +
// DisablePreParseMultipartForm together, see FiberApp.Init) -- without (2),
// fasthttp has already fully parsed/buffered the multipart form before this
// method is ever called, so RequestBodyStream() returns nil (confirmed via
// fasthttp@v1.72.0 source: Request.bodyStream is only populated by
// ContinueReadBodyStream, which readBodyStream/StreamRequestBody's dispatch
// path calls -- see design.md's Tech Decisions).
func (r *fiberResponder) BodyStream() (io.Reader, string, bool) {
	contentType := r.c.Get(fiber.HeaderContentType)
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, "", false
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, "", false
	}

	stream := r.c.RequestCtx().RequestBodyStream()
	if stream == nil {
		return nil, "", false
	}
	return stream, boundary, true
}

// WriteStream registers fn as this response's body-streaming writer, via
// fasthttp's own RequestCtx.SetBodyStreamWriter (confirmed real API,
// fasthttp@v1.72.0's server.go/stream.go: `type StreamWriter func(w
// *bufio.Writer)`, `func (ctx *RequestCtx) SetBodyStreamWriter(sw
// StreamWriter)`) -- graphql-support feature, Milestone 17's SSE
// transport. fasthttp runs fn in its OWN goroutine, on a connection kept
// open until fn returns; fasthttp's own doc comment on SetBodyStreamWriter
// forbids touching RequestCtx/its members from inside fn, so fn must only
// ever write to (and Flush) the *bufio.Writer it's given -- gonest's own
// fn (internal/graphql's SSE handler) already only does that.
func (r *fiberResponder) WriteStream(fn func(w *bufio.Writer)) {
	r.c.RequestCtx().SetBodyStreamWriter(fn)
}
