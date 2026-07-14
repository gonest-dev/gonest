// Package interceptor implements the declarative Interceptor API: the
// reusable before/after-Handler-execution unit with a continuation-passing
// (ctx, next) shape, wrapping a route's Handler for AOP-style
// pre/post-processing (timing, transformation, caching, etc.), mirroring
// Nest interceptors. See design.md's "Next / Interceptor" component.
package interceptor

import "github.com/gonest-dev/gonest/internal/execution"

// Next represents the continuation of the interceptor chain: calling it runs
// whatever comes after the current interceptor (the next interceptor, or
// eventually the already-guard-gated route Handler). This is a package-own
// type -- NOT reused from internal/middleware.Next -- even though both share
// the identical underlying shape (func(ctx *execution.Context)). See
// design.md's Tech Decisions: internal/interceptor and internal/middleware
// are parallel, conceptually independent pipeline-stage packages (different
// composition position, different semantic framing) that only coincide in
// shape today; coupling one to the other just to reuse a 12-character
// one-liner type declaration would be an artificial dependency for no
// runtime benefit (Go's structural function-type compatibility already
// makes both Next types freely interchangeable at the underlying-function
// level for composition purposes).
type Next func(ctx *execution.Context)

// Interceptor represents a single interceptor unit: it holds the (ctx, next)
// handler function registered via Handler.
type Interceptor struct {
	handler func(ctx *execution.Context, next Next)
}

// New creates an Interceptor and runs fn on it IMMEDIATELY -- unlike
// Provider/Module/Controller/Pipe, which all defer fn until a later
// bootstrap stage (their own Declare()). Interceptor.Handler(...)
// registration has no dependency on the module tree being assembled first
// -- it needs no MustInject, no Owner/module-scope knowledge at all --
// so there is no further stage left to usefully defer to (AD-008 in
// STATE.md: pipeline-stage types don't support MustInject in v1, same
// reasoning as middleware.New/guard.New). This mirrors route.New's own
// precedent (internal/route/route.go) for the same reason.
func New(fn func(*Interceptor)) *Interceptor {
	i := &Interceptor{}
	if fn != nil {
		fn(i)
	}
	return i
}

// Handler stores h as this Interceptor's (ctx, next) handler function.
func (i *Interceptor) Handler(h func(ctx *execution.Context, next Next)) {
	i.handler = h
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called (mirrors middleware.Middleware.HandlerFunc()'s zero-value
// contract).
func (i *Interceptor) HandlerFunc() func(ctx *execution.Context, next Next) {
	return i.handler
}
