// Package interceptor implements the declarative Interceptor API: the
// reusable before/after-Handler-execution unit with a continuation-passing
// (req, res, next) shape, wrapping a route's Handler for AOP-style
// pre/post-processing (timing, transformation, caching, etc.), mirroring
// Nest interceptors. See design.md's "Next / Interceptor" component.
package interceptor

import (
	"reflect"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// Next represents the continuation of the interceptor chain: calling it runs
// whatever comes after the current interceptor (the next interceptor, or
// eventually the already-guard-gated route Handler). This is a package-own
// type -- NOT reused from internal/middleware.Next -- even though both share
// the identical underlying shape (func(req *execution.Request, res
// *execution.Response)). See design.md's Tech Decisions: internal/interceptor
// and internal/middleware are parallel, conceptually independent
// pipeline-stage packages (different composition position, different
// semantic framing) that only coincide in shape today; coupling one to the
// other just to reuse a one-liner type declaration would be an artificial
// dependency for no runtime benefit (Go's structural function-type
// compatibility already makes both Next types freely interchangeable at the
// underlying-function level for composition purposes).
type Next func(req *execution.Request, res *execution.Response)

// Interceptor represents a single interceptor unit: it holds the (req, res,
// next) handler function registered via Handler.
type Interceptor struct {
	fn      func(*Interceptor)
	handler func(req *execution.Request, res *execution.Response, next Next)

	scope    []*module.Module
	declared bool
}

// New creates an Interceptor that defers fn until Declare(scope) runs it --
// AD-008 reversed (see .specs/features/test-app-bootstrap/design.md): an
// *Interceptor can now call MustInject/MustInjectAll inside fn, since it is
// no longer run at Go package-init time (the typical `var X =
// gonest.NewInterceptor(...)` declaration style, which always runs BEFORE
// NewApp), but during bootstrap's pipeline-stage-type phase (phase 3), once
// its scope (the union of every referencing module) is known.
func New(fn func(*Interceptor)) *Interceptor {
	return &Interceptor{fn: fn}
}

// Declare runs this interceptor's deferred fn exactly once, with scope
// stored first so ResolveDirect/ResolveDirectAll (called from WITHIN fn,
// via MustInject/MustInjectAll) know what to search. Idempotent -- a no-op
// on any call after the first (including when fn is nil) -- same contract
// as Pipe.Declare's own precedent (L-012 in STATE.md): a shared Interceptor
// value attached to MULTIPLE controllers/modules must only run its OWN fn
// once, not once per attachment.
func (i *Interceptor) Declare(scope []*module.Module) {
	if i.declared {
		return
	}
	i.declared = true
	i.scope = scope
	if i.fn != nil {
		i.fn(i)
	}
}

// ResolveDirect satisfies internal/inject's directResolver interface,
// delegating to internal/resolver's FindDirect using the scope Declare
// stored.
func (i *Interceptor) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect(i.scope, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// delegating to internal/resolver's FindDirectAll using the scope Declare
// stored.
func (i *Interceptor) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll(i.scope, t)
}

// Handler stores h as this Interceptor's (req, res, next) handler function.
func (i *Interceptor) Handler(h func(req *execution.Request, res *execution.Response, next Next)) {
	i.handler = h
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called (mirrors middleware.Middleware.HandlerFunc()'s zero-value
// contract).
func (i *Interceptor) HandlerFunc() func(req *execution.Request, res *execution.Response, next Next) {
	return i.handler
}
