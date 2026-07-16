// Package execution provides Context, the single per-request access point
// exposed to route Handlers. Context delegates to a minimal Responder
// interface so the real Fiber-backed implementation can be wired in later
// (see design.md's "FiberApp" component) without changing Context's public
// API or these tests.
package execution

import "io"

// Responder is the minimal surface Context needs to serve its request/response
// methods. A fake implementation is used in tests; a Fiber-backed
// implementation is added in a later task.
//
// Exported (not just "responder") so packages other than execution -- namely
// internal/route's tests -- can build a *Context around their own fake
// without needing a Fiber-backed one to exist yet (see L-004 in STATE.md:
// Go ties interface satisfaction/visibility to whether the interface itself
// is exported, not just its methods).
type Responder interface {
	JSON(v any) error
	SetStatus(code int)
	GetStatus() int
	GetMethod() string
	GetPath() string
	GetHeader(name string) string
	SetHeaderValue(name, value string)
	GetParam(name string) string
	Body() []byte
	Queries() map[string]string
	HTML(s string) error
	SendString(s string) error
	// BodyStream returns the raw request body as a stream, plus the
	// multipart boundary parsed from Content-Type, for
	// ParseRestFormBody/MustParseRestFormBody (internal/validate,
	// Multipart Form Streaming feature, AD-022 in STATE.md). ok is false
	// when streaming was never enabled for this app
	// (AppOptions.EnableFormStreaming) OR the current request's
	// Content-Type isn't multipart/form-data at all -- 2 different non-
	// error conditions covered by one bool, same (value, bool) convention
	// internal/schema.Lookup already uses for "not applicable" rather
	// than inventing a sentinel error.
	BodyStream() (stream io.Reader, boundary string, ok bool)
}

// Context encapsulates the HTTP request/response for a single route Handler.
type Context struct {
	res   Responder
	route any
}

// New builds a Context around the given Responder. Exported so other
// packages (namely internal/route, which needs to attach a *Route to a
// Context for MustParams[T]/MustQuery[T] (internal/validate) to consult --
// see WithRoute/Route below) can construct a *Context in their own tests
// without a real Fiber-backed Responder existing yet.
func New(res Responder) *Context {
	return &Context{res: res}
}

// WithRoute attaches an opaque reference to the *route.Route that owns this
// Context to the Context, and returns ctx for chaining. It is stored as
// `any` (not *route.Route) deliberately: internal/route already imports
// internal/execution (Route.Handler takes a *Context) -- if execution
// imported route back to give WithRoute/Route a concrete type, that would be
// an import cycle. Storing `any` here keeps execution's boundary exactly as
// narrow as T2 established ("no Fiber", not "no Route"): execution never
// inspects or calls anything on the stored value, it is purely a carrier.
// internal/validate (which imports both execution and route for
// MustParams[T]/MustQuery[T]) is the only place that type-asserts this back
// to *route.Route.
func (ctx *Context) WithRoute(route any) *Context {
	ctx.route = route
	return ctx
}

// Route returns the opaque reference previously attached via WithRoute, or
// nil if none was attached (e.g. a Context built directly in a test that
// doesn't exercise MustParams[T]/MustQuery[T]'s route-lookup path).
func (ctx *Context) Route() any {
	return ctx.route
}

// Json writes value as the JSON response body.
func (ctx *Context) Json(value any) error {
	return ctx.res.JSON(value)
}

// Status sets the response status code and returns ctx for chaining.
func (ctx *Context) Status(code int) *Context {
	ctx.res.SetStatus(code)
	return ctx
}

// ResponseStatus returns the response status code currently set on this
// request -- the underlying HTTP framework's own default (200) if Status
// was never called. Used by a Logger middleware (gonest.NewLoggerMiddleware)
// to read back the status AFTER next(ctx) runs the rest of the chain, since
// Status/SetStatus are otherwise write-only.
func (ctx *Context) ResponseStatus() int {
	return ctx.res.GetStatus()
}

// Method returns the actual incoming request's HTTP method (e.g. "POST"),
// as reported by the underlying HTTP framework -- not the route's own
// declared method (route.Route.Method), so it stays correct even if a
// Middleware runs before/without a *route.Route ever being attached.
func (ctx *Context) Method() string {
	return ctx.res.GetMethod()
}

// Path returns the actual incoming request's full path (e.g.
// "/auth/register/otp"), as reported by the underlying HTTP framework -- not
// the route's own declared pattern (route.Route.Path, which is relative to
// its controller's prefix and may contain ":param" placeholders).
func (ctx *Context) Path() string {
	return ctx.res.GetPath()
}

// Header returns the value of the named request/response header.
func (ctx *Context) Header(name string) string {
	return ctx.res.GetHeader(name)
}

// SetHeader sets the named response header to value.
func (ctx *Context) SetHeader(name, value string) {
	ctx.res.SetHeaderValue(name, value)
}

// Param returns the raw string value of the named route param.
func (ctx *Context) Param(name string) string {
	return ctx.res.GetParam(name)
}

// Body returns the raw request body bytes.
//
// The returned slice must NOT be retained past synchronous use within the
// same request/handler execution -- no defensive copy is made here, unlike
// Param's GetParam (see L-009 in STATE.md, which DOES clone). This is safe
// because Body() is expected to be consumed synchronously (e.g. passed
// straight into json.Unmarshal, which copies data into the destination
// values during decode rather than retaining the input slice); a caller
// that stores the raw []byte in a struct field or otherwise reads it after
// the handler returns risks seeing it corrupted/overwritten by a later
// request reusing the same underlying buffer, exactly like L-009's bug.
func (ctx *Context) Body() []byte {
	return ctx.res.Body()
}

// Queries returns the raw query-string params as a map, exactly as reported
// by the underlying Responder -- one-line delegation, same pattern as
// Body(). Used by MustQuery[T] (internal/validate) as its source of
// presence/raw values, mirroring how MustParams[T] consults Route.HasParam/
// Context.Param instead.
func (ctx *Context) Queries() map[string]string {
	return ctx.res.Queries()
}

// HTML writes s as a raw text/html response body -- one-line delegation,
// same pattern as Json/Body/Queries above. Used by SetupSwagger
// (internal/openapi) to serve the Swagger UI's HTML page.
func (ctx *Context) HTML(s string) error {
	return ctx.res.HTML(s)
}

// SendString writes s as a raw, uninterpreted response body -- no
// JSON-encoding (unlike Json), no text/html Content-Type (unlike HTML).
// Used by health/liveness-probe style handlers (see "Terminus/health
// checks" feature) that just need to write a plain status string like
// "OK".
func (ctx *Context) SendString(s string) error {
	return ctx.res.SendString(s)
}

// FormStream returns the raw request body as a stream plus its multipart
// boundary, for ParseRestFormBody/MustParseRestFormBody (internal/validate)
// -- one-line delegation, same pattern as Body()/Queries(). ok is false when
// form streaming isn't available for this request (see Responder.BodyStream's
// own doc comment for the 2 reasons that can be true).
func (ctx *Context) FormStream() (stream io.Reader, boundary string, ok bool) {
	return ctx.res.BodyStream()
}
