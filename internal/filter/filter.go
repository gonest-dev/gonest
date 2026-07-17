// Package filter implements the declarative Filter API: a reusable,
// per-exception-type custom response handler unit, reusable across
// controllers and modules. See design.md's "Filter" component.
package filter

import (
	"reflect"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// requestType and responseType are used to validate Catch's accepted handler
// signature via reflect.
var (
	requestType  = reflect.TypeOf((*execution.Request)(nil))
	responseType = reflect.TypeOf((*execution.Response)(nil))
)

// Filter represents a reusable set of per-exception-type response handlers,
// registered via Catch and keyed by the exact exception type via
// reflect.Type. Unlike Middleware/Guard/Interceptor's typed-func-parameter
// registration style, Catch's exemplar-plus-reflect-validated-handler shape
// is deliberate: the accepted handler signature depends on a value only
// known at runtime (exemplar's concrete type), which a clean generic method
// can't express without forcing callers to explicitly instantiate a type
// parameter (`f.Catch[*FooExampleError](...)`) -- a shape that doesn't match
// INSIGHT.md's own call convention (exemplar value first, type inferred from
// it). Catch instead mirrors Pipe.Handler's precedent: validate a
// caller-supplied func's signature via reflect at registration time, panic
// clearly if it doesn't match (see design.md's Tech Decisions).
type Filter struct {
	fn      func(*Filter)
	catches map[reflect.Type]reflect.Value

	scope    []*module.Module
	declared bool
}

// New creates a Filter that defers fn until Declare(scope) runs it -- AD-008
// reversed (see .specs/features/test-app-bootstrap/design.md): a *Filter
// can now call MustInject/MustInjectAll inside fn, since it is no longer
// run at Go package-init time (the typical `var X =
// gonest.NewFilter(...)` declaration style, which always runs BEFORE
// NewApp), but during bootstrap's pipeline-stage-type phase (phase 3),
// once its scope (the union of every referencing module) is known.
func New(fn func(*Filter)) *Filter {
	return &Filter{fn: fn, catches: make(map[reflect.Type]reflect.Value)}
}

// Declare runs this filter's deferred fn exactly once, with scope stored
// first so ResolveDirect/ResolveDirectAll (called from WITHIN fn, via
// MustInject/MustInjectAll) know what to search. Idempotent -- a no-op on
// any call after the first (including when fn is nil) -- same contract as
// Pipe.Declare's own precedent (L-012 in STATE.md): a shared Filter value
// attached to MULTIPLE controllers/modules must only run its OWN fn once,
// not once per attachment.
func (f *Filter) Declare(scope []*module.Module) {
	if f.declared {
		return
	}
	f.declared = true
	f.scope = scope
	if f.fn != nil {
		f.fn(f)
	}
}

// IsFilter is the marker method that satisfies module.FilterRef, so *Filter
// can be passed to (*module.Module).Filters without module needing to
// import this package (avoiding an import cycle -- this package imports
// module for Declare(scope []*module.Module), so module cannot import this
// package back). Exported: Go ties unexported interface methods to the
// declaring package, so an unexported marker here could never satisfy
// module's interface across packages.
func (f *Filter) IsFilter() {}

// ResolveDirect satisfies internal/inject's directResolver interface,
// delegating to internal/resolver's FindDirect using the scope Declare
// stored.
func (f *Filter) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect(f.scope, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// delegating to internal/resolver's FindDirectAll using the scope Declare
// stored.
func (f *Filter) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll(f.scope, t)
}

// Catch registers handler as the response handler for exceptions whose
// concrete type is EXACTLY reflect.TypeOf(exemplar). handler must have the
// signature:
//
//	func(req *execution.Request, res *execution.Response, exc T)
//
// where T is exactly reflect.TypeOf(exemplar) -- for any other signature,
// Catch panics with a clear message at registration time (reflect-validated,
// same "fail fast with a clear message" convention as Pipe.Handler). Unlike
// Pipe.Handler (which returns T), a Catch handler returns nothing: it acts
// by mutating res (writing the response), not by producing a value for a
// caller to use.
func (f *Filter) Catch(exemplar any, handler any) {
	excType := reflect.TypeOf(exemplar)

	v := reflect.ValueOf(handler)
	if v.Kind() != reflect.Func || !isValidCatchSignature(v.Type(), excType) {
		panic("gonest: invalid Filter.Catch handler signature, expected func(req *execution.Request, res *execution.Response, exc " + excType.String() + ")")
	}

	f.catches[excType] = v
}

// HandlerFor returns the reflect.Value of the handler registered via
// Catch(exemplar, handler) for excType, and whether one was found. It is an
// exact map lookup -- no supertype/interface matching -- matching Catch's
// own exact-type registration.
func (f *Filter) HandlerFor(excType reflect.Type) (reflect.Value, bool) {
	v, ok := f.catches[excType]
	return v, ok
}

// isValidCatchSignature reports whether t matches the single accepted Catch
// handler signature: func(req *execution.Request, res *execution.Response,
// exc excType), returning nothing.
func isValidCatchSignature(t reflect.Type, excType reflect.Type) bool {
	if t.NumIn() != 3 {
		return false
	}
	if t.In(0) != requestType || t.In(1) != responseType || t.In(2) != excType {
		return false
	}
	return t.NumOut() == 0
}
