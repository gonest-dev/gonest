// Package middleware implements the declarative Middleware API: the
// reusable request-observation/mutation unit with a continuation-passing
// (ctx, next) shape, mirroring Express/Nest middleware. See design.md's
// "Next / Middleware" component.
package middleware

import (
	"reflect"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// Next represents the continuation of the middleware chain: calling it runs
// whatever comes after the current middleware (the next middleware, or
// eventually the route's own Handler). Its underlying type
// (func(c *execution.HttpContext)) is IDENTICAL in shape to a route Handler
// -- this is what lets internal/app's Stage 2.5 wrap a route's Handler as
// the innermost Next with a direct assignment, no conversion shim needed
// (see design.md's Data Models "Relationships").
type Next func(c *execution.HttpContext)

// Middleware represents a single middleware unit: it holds the (context,
// next) handler function registered via Handler.
type Middleware struct {
	fn      func(*Middleware)
	handler func(c *execution.HttpContext, next Next)

	scope    []*module.Module
	declared bool
}

// New creates a Middleware that defers fn until Declare(scope) runs it --
// AD-008 reversed (see .specs/features/test-app-bootstrap/design.md): a
// *Middleware can now call MustInject/MustInjectAll inside fn, since it is
// no longer run at Go package-init time (the typical `var X =
// gonest.NewMiddleware(...)` declaration style, which always runs BEFORE
// NewApp), but during bootstrap's pipeline-stage-type phase (phase 3),
// once its scope (the union of every referencing module) is known.
func New(fn func(*Middleware)) *Middleware {
	return &Middleware{fn: fn}
}

// Declare runs this middleware's deferred fn exactly once, with scope
// stored first so ResolveDirect/ResolveDirectAll (called from WITHIN fn,
// via MustInject/MustInjectAll) know what to search. Idempotent -- a no-op
// on any call after the first (including when fn is nil) -- same contract
// as Pipe.Declare's own precedent (L-012 in STATE.md): a shared Middleware
// value attached to MULTIPLE controllers/modules must only run its OWN fn
// once, not once per attachment.
func (m *Middleware) Declare(scope []*module.Module) {
	if m.declared {
		return
	}
	m.declared = true
	m.scope = scope
	if m.fn != nil {
		m.fn(m)
	}
}

// IsMiddleware is the marker method that satisfies module.MiddlewareRef, so
// *Middleware can be passed to (*module.Module).Use without module needing
// to import this package (avoiding an import cycle -- this package imports
// module for Declare(scope []*module.Module), so module cannot import this
// package back). Exported: Go ties unexported interface methods to the
// declaring package, so an unexported marker here could never satisfy
// module's interface across packages.
func (m *Middleware) IsMiddleware() {}

// IsToken is a marker method that satisfies module.TokenRef (via
// module.MiddlewareRef, which embeds it), so *Middleware can be passed to
// any of Module's builder methods.
func (m *Middleware) IsToken() {}

// ResolveDirect satisfies internal/inject's directResolver interface,
// delegating to internal/resolver's FindDirect using the scope Declare
// stored.
func (m *Middleware) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect(m.scope, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// delegating to internal/resolver's FindDirectAll using the scope Declare
// stored.
func (m *Middleware) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll(m.scope, t)
}

// Handler stores h as this Middleware's (context, next) handler function.
func (m *Middleware) Handler(h func(c *execution.HttpContext, next Next)) {
	m.handler = h
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called (mirrors pipe.Pipe.HandlerFunc()'s zero-value contract).
func (m *Middleware) HandlerFunc() func(c *execution.HttpContext, next Next) {
	return m.handler
}
