// Package filter implements the declarative Filter API: a reusable,
// per-exception-type custom response handler unit, reusable across
// controllers and modules. See design.md's "Filter" component.
package filter

import (
	"reflect"

	"github.com/gonest-dev/gonest/internal/execution"
)

// contextType is used to validate Catch's accepted handler signature via
// reflect.
var contextType = reflect.TypeOf((*execution.Context)(nil))

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
	catches map[reflect.Type]reflect.Value
}

// New creates a Filter and runs fn on it IMMEDIATELY -- unlike
// Provider/Module/Controller/Pipe, which all defer fn until a later
// bootstrap stage (their own Declare()). Filter.Catch registration has no
// dependency on the module tree being assembled first -- this feature
// deliberately has no MustInject support (see design.md's Tech Decisions: a
// *Filter can be attached to multiple controllers/modules across the app,
// with no clean single "owner" to resolve MustInject against) -- so there is
// no further stage left to usefully defer to. This mirrors
// middleware.New/guard.New/interceptor.New's own precedent for the same
// reason.
func New(fn func(*Filter)) *Filter {
	f := &Filter{catches: make(map[reflect.Type]reflect.Value)}
	if fn != nil {
		fn(f)
	}
	return f
}

// Catch registers handler as the response handler for exceptions whose
// concrete type is EXACTLY reflect.TypeOf(exemplar). handler must have the
// signature:
//
//	func(ctx *execution.Context, exc T)
//
// where T is exactly reflect.TypeOf(exemplar) -- for any other signature,
// Catch panics with a clear message at registration time (reflect-validated,
// same "fail fast with a clear message" convention as Pipe.Handler). Unlike
// Pipe.Handler (which returns T), a Catch handler returns nothing: it acts
// by mutating ctx (writing the response), not by producing a value for a
// caller to use.
func (f *Filter) Catch(exemplar any, handler any) {
	excType := reflect.TypeOf(exemplar)

	v := reflect.ValueOf(handler)
	if v.Kind() != reflect.Func || !isValidCatchSignature(v.Type(), excType) {
		panic("gonest: invalid Filter.Catch handler signature, expected func(ctx *execution.Context, exc " + excType.String() + ")")
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
// handler signature: func(ctx *execution.Context, exc excType), returning
// nothing.
func isValidCatchSignature(t reflect.Type, excType reflect.Type) bool {
	if t.NumIn() != 2 {
		return false
	}
	if t.In(0) != contextType || t.In(1) != excType {
		return false
	}
	return t.NumOut() == 0
}
