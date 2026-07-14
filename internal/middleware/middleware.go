// Package middleware implements the declarative Middleware API: the
// reusable request-observation/mutation unit with a continuation-passing
// (ctx, next) shape, mirroring Express/Nest middleware. See design.md's
// "Next / Middleware" component.
package middleware

import "github.com/gonest-dev/gonest/internal/execution"

// Next represents the continuation of the middleware chain: calling it runs
// whatever comes after the current middleware (the next middleware, or
// eventually the route's own Handler). Its underlying type
// (func(ctx *execution.Context)) is IDENTICAL in shape to a route Handler --
// this is what lets internal/app's Stage 2.5 wrap a route's Handler as the
// innermost Next with a direct assignment, no conversion shim needed (see
// design.md's Data Models "Relationships").
type Next func(ctx *execution.Context)

// Middleware represents a single middleware unit: it holds the (ctx, next)
// handler function registered via Handler.
type Middleware struct {
	handler func(ctx *execution.Context, next Next)
}

// New creates a Middleware and runs fn on it IMMEDIATELY -- unlike
// Provider/Module/Controller/Pipe, which all defer fn until a later
// bootstrap stage (their own Declare()). Middleware.Handler(...)
// registration has no dependency on the module tree being assembled first
// -- it needs no MustInject, no Owner/module-scope knowledge at all --
// so there is no further stage left to usefully defer to. This mirrors
// route.New's own precedent (internal/route/route.go) for the same reason.
func New(fn func(*Middleware)) *Middleware {
	m := &Middleware{}
	if fn != nil {
		fn(m)
	}
	return m
}

// Handler stores h as this Middleware's (ctx, next) handler function.
func (m *Middleware) Handler(h func(ctx *execution.Context, next Next)) {
	m.handler = h
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called (mirrors pipe.Pipe.HandlerFunc()'s zero-value contract).
func (m *Middleware) HandlerFunc() func(ctx *execution.Context, next Next) {
	return m.handler
}
