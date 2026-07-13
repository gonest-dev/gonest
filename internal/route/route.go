package route

import (
	"strings"

	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/pipe"
)

// defaultHttpCode is the status code a Route uses when HttpCode is never
// called, per design.md's Data Models comment ("default 200, sobrescrito
// por HttpCode()").
const defaultHttpCode = 200

// Route represents a single declared HTTP route -- method, path, default
// status code, handler, and any per-param custom Pipes. It is purely
// declarative: nothing here participates in the DI graph.
type Route struct {
	method HttpMethod
	path   string

	httpCode int
	handler  func(ctx *httpctx.Context)

	paramPipes map[string]*pipe.Pipe
}

// New creates a Route and runs fn on it immediately -- unlike
// Provider/Module/Controller/Pipe, which all defer fn until a later
// bootstrap stage. By the time Controller.Route(...) calls route.New, it is
// already running inside the Controller's own already-deferred fn, itself
// only invoked during Stage 2 after the module tree is fully assembled --
// there is no further stage left to usefully defer to, so New runs fn right
// away (see design.md's "Route (declaração de rota)" component).
func New(method HttpMethod, path string, fn func(*Route)) *Route {
	r := &Route{
		method:     method,
		path:       path,
		httpCode:   defaultHttpCode,
		paramPipes: map[string]*pipe.Pipe{},
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
func (r *Route) Handler(fn func(ctx *httpctx.Context)) {
	r.handler = fn
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called.
func (r *Route) HandlerFunc() func(ctx *httpctx.Context) {
	return r.handler
}

// Param registers a custom Pipe for the route param named name. When
// MustParam[T] is called for this param name on a Context attached to this
// Route, it uses p's Handler instead of the default reflect+strconv
// coercion.
//
// Calls p.Declare() before storing it: pipe.New(fn) defers fn (the call
// that registers p's own Handler) until Declare runs, mirroring
// Provider/Controller/Module's own deferred-fn pattern -- but unlike those,
// nothing in the DI bootstrap (internal/app's declareAll) ever walks
// Route's registered Pipes to declare them, since Pipes aren't tracked by
// any Module. Without this call, a Pipe attached via Param would never run
// its own Handler(...) registration, and PipeFor/HandlerFunc would return a
// zero reflect.Value the first time MustParam[T] tried to use it. Declare
// is idempotent (no-op after the first call, per pipe.Pipe's own contract),
// so calling it here is always safe even if the Pipe var is reused across
// multiple Route.Param registrations.
func (r *Route) Param(name string, p *pipe.Pipe) {
	p.Declare()
	r.paramPipes[name] = p
}

// PipeFor returns the custom Pipe registered for name via Param, and
// whether one was registered at all.
func (r *Route) PipeFor(name string) (*pipe.Pipe, bool) {
	p, ok := r.paramPipes[name]
	return p, ok
}

// HasParam reports whether this Route's declared path pattern contains a
// ":name" segment matching name. This is the existence-check mechanism
// MustParam[T] (root param.go) relies on: httpctx.Context.Param mirrors
// Fiber's own c.Params semantics and returns a bare "" for a param that
// doesn't exist on the route -- which is indistinguishable, at that layer
// alone, from a param that exists but genuinely carries an empty string
// value (an ambiguity T4's defaultCoerce explicitly documents for T=string).
// Route already knows its own declared path string at construction time, so
// checking it directly gives MustParam a ground-truth answer to "does this
// route even have a param named X" that does not depend on interpreting
// Context's raw string return value.
func (r *Route) HasParam(name string) bool {
	segment := ":" + name
	for _, part := range strings.Split(r.path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}
