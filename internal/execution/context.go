// Package execution provides Context, the single per-request access point
// exposed to route Handlers. Context delegates to a minimal Responder
// interface so the real Fiber-backed implementation can be wired in later
// (see design.md's "FiberApp" component) without changing Context's public
// API or these tests.
package execution

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
	GetHeader(name string) string
	SetHeaderValue(name, value string)
	GetParam(name string) string
}

// Context encapsulates the HTTP request/response for a single route Handler.
type Context struct {
	res   Responder
	route any
}

// New builds a Context around the given Responder. Exported so other
// packages (namely internal/route, which needs to attach a *Route to a
// Context for MustParam[T] to consult -- see WithRoute/Route below) can
// construct a *Context in their own tests without a real Fiber-backed
// Responder existing yet.
func New(res Responder) *Context {
	return &Context{res: res}
}

// WithRoute attaches an opaque reference to the *route.Route that owns this
// Context to the Context, and returns ctx for chaining. It is stored as
// `any` (not *route.Route) deliberately: internal/route already imports
// internal/execution (Route.Handler takes a *Context, Route.Param takes a
// *pipe.Pipe whose Handler also takes a *Context) -- if execution imported
// route back to give WithRoute/Route a concrete type, that would be an
// import cycle. Storing `any` here keeps execution's boundary exactly as
// narrow as T2 established ("no Fiber", not "no Route"): execution never
// inspects or calls anything on the stored value, it is purely a carrier.
// The root param.go wrapper (which imports both execution and route) is the
// only place that type-asserts this back to *route.Route.
func (ctx *Context) WithRoute(route any) *Context {
	ctx.route = route
	return ctx
}

// Route returns the opaque reference previously attached via WithRoute, or
// nil if none was attached (e.g. a Context built directly in a test that
// doesn't exercise MustParam's custom-Pipe path).
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
