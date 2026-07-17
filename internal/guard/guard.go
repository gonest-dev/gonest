// Package guard implements the declarative Guard API: the reusable
// authorization-check unit with a `func(req, res) bool` shape -- true
// continues the request, false (or a panic'd exception.Exception) stops it.
// See design.md's "Guard" component.
package guard

import (
	"reflect"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// Guard represents a single authorization-check unit: it holds the
// req/res-in/bool-out handler function registered via Handler. Unlike
// Middleware, a Guard doesn't decorate/wrap a continuation -- it gates:
// true means continue, false means stop.
type Guard struct {
	fn      func(*Guard)
	handler func(req *execution.Request, res *execution.Response) bool

	scope    []*module.Module
	declared bool
}

// New creates a Guard that defers fn until Declare(scope) runs it -- AD-008
// reversed (see .specs/features/test-app-bootstrap/design.md): a *Guard can
// now call MustInject/MustInjectAll inside fn, since it is no longer run at
// Go package-init time (the typical `var X = gonest.NewGuard(...)`
// declaration style, which always runs BEFORE NewApp), but during
// bootstrap's pipeline-stage-type phase (phase 3), once its scope (the
// union of every referencing module) is known.
func New(fn func(*Guard)) *Guard {
	return &Guard{fn: fn}
}

// Declare runs this guard's deferred fn exactly once, with scope stored
// first so ResolveDirect/ResolveDirectAll (called from WITHIN fn, via
// MustInject/MustInjectAll) know what to search. Idempotent -- a no-op on
// any call after the first (including when fn is nil) -- same contract as
// Pipe.Declare's own precedent (L-012 in STATE.md): a shared Guard value
// attached to MULTIPLE controllers/modules must only run its OWN fn once,
// not once per attachment.
func (g *Guard) Declare(scope []*module.Module) {
	if g.declared {
		return
	}
	g.declared = true
	g.scope = scope
	if g.fn != nil {
		g.fn(g)
	}
}

// ResolveDirect satisfies internal/inject's directResolver interface,
// delegating to internal/resolver's FindDirect using the scope Declare
// stored.
func (g *Guard) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect(g.scope, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// delegating to internal/resolver's FindDirectAll using the scope Declare
// stored.
func (g *Guard) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll(g.scope, t)
}

// Handler stores h as this Guard's req/res-in/bool-out handler function.
func (g *Guard) Handler(h func(req *execution.Request, res *execution.Response) bool) {
	g.handler = h
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called (mirrors middleware.Middleware.HandlerFunc()'s zero-value
// contract).
func (g *Guard) HandlerFunc() func(req *execution.Request, res *execution.Response) bool {
	return g.handler
}
