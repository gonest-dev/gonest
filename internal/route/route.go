package route

import (
	"strings"

	"github.com/gonest-dev/gonest/internal/execution"
)

// defaultHttpCode is the status code a Route uses when HttpCode is never
// called, per design.md's Data Models comment ("default 200, sobrescrito
// por HttpCode()").
const defaultHttpCode = 200

// Route represents a single declared HTTP route -- method, path, default
// status code, and handler. It is purely declarative: nothing here
// participates in the DI graph.
type Route struct {
	method HttpMethod
	path   string

	httpCode int
	handler  func(ctx *execution.Context)
}

// New creates a Route and runs fn on it immediately -- unlike
// Provider/Module/Controller, which all defer fn until a later bootstrap
// stage. By the time Controller.Route(...) calls route.New, it is already
// running inside the Controller's own already-deferred fn, itself only
// invoked during Stage 2 after the module tree is fully assembled -- there
// is no further stage left to usefully defer to, so New runs fn right away
// (see design.md's "Route (declaração de rota)" component).
func New(method HttpMethod, path string, fn func(*Route)) *Route {
	r := &Route{
		method:   method,
		path:     path,
		httpCode: defaultHttpCode,
	}
	if fn != nil {
		fn(r)
	}
	return r
}

// Method returns this Route's HTTP method, as passed to New. Used by Stage
// 2.5 of app bootstrap (internal/app) to build the method+path collision key
// and to register the route on the HttpAdapter.
func (r *Route) Method() HttpMethod {
	return r.method
}

// Path returns this Route's declared path pattern, as passed to New (not
// combined with any owning Controller's PathPrefix -- that composition is
// Stage 2.5's job, not Route's). Used alongside Method for the same
// collision-key/registration purposes.
func (r *Route) Path() string {
	return r.path
}

// HttpCode stores the default status code this Route's Handler responds
// with (unless the Handler itself overrides it via ctx.Status).
func (r *Route) HttpCode(status int) {
	r.httpCode = status
}

// Code returns the currently stored default status code (200 unless
// HttpCode was called).
func (r *Route) Code() int {
	return r.httpCode
}

// Handler stores fn as this Route's request handler.
func (r *Route) Handler(fn func(ctx *execution.Context)) {
	r.handler = fn
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called.
func (r *Route) HandlerFunc() func(ctx *execution.Context) {
	return r.handler
}

// HasParam reports whether this Route's declared path pattern contains a
// ":name" segment matching name. This is the existence-check mechanism
// MustParams[T]/MustQuery[T] (internal/validate) rely on: execution.Context.
// Param mirrors Fiber's own c.Params semantics and returns a bare "" for a
// param that doesn't exist on the route -- which is indistinguishable, at
// that layer alone, from a param that exists but genuinely carries an empty
// string value. Route already knows its own declared path string at
// construction time, so checking it directly gives callers a ground-truth
// answer to "does this route even have a param named X" that does not
// depend on interpreting Context's raw string return value.
func (r *Route) HasParam(name string) bool {
	segment := ":" + name
	for _, part := range strings.Split(r.path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}
