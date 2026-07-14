// Package guard implements the declarative Guard API: the reusable
// authorization-check unit with a `func(ctx) bool` shape -- true continues
// the request, false (or a panic'd exception.Exception) stops it. See
// design.md's "Guard" component.
package guard

import "github.com/gonest-dev/gonest/internal/execution"

// Guard represents a single authorization-check unit: it holds the
// ctx-in/bool-out handler function registered via Handler. Unlike
// Middleware, a Guard doesn't decorate/wrap a continuation -- it gates:
// true means continue, false means stop.
type Guard struct {
	handler func(ctx *execution.Context) bool
}

// New creates a Guard and runs fn on it IMMEDIATELY -- unlike
// Provider/Module/Controller/Pipe, which all defer fn until a later
// bootstrap stage (their own Declare()). Guard.Handler(...) registration has
// no dependency on the module tree being assembled first -- this feature
// deliberately has no MustInject support (see design.md's Tech Decisions: a
// *Guard can be attached to multiple controllers across different modules,
// with no clean single "owner" to resolve MustInject against) -- so there
// is no further stage left to usefully defer to. This mirrors
// middleware.New's own precedent (internal/middleware/middleware.go) for
// the same reason.
func New(fn func(*Guard)) *Guard {
	g := &Guard{}
	if fn != nil {
		fn(g)
	}
	return g
}

// Handler stores h as this Guard's ctx-in/bool-out handler function.
func (g *Guard) Handler(h func(ctx *execution.Context) bool) {
	g.handler = h
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called (mirrors middleware.Middleware.HandlerFunc()'s zero-value
// contract).
func (g *Guard) HandlerFunc() func(ctx *execution.Context) bool {
	return g.handler
}
