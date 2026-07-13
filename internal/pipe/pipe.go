// Package pipe implements the declarative Pipe API: Pipe.New(fn) defers fn
// until Declare runs (same deferred-fn pattern as provider.New/
// controller.New/module.New), and Pipe.Handler registers a reflect-validated
// transform function used to coerce a raw route param string into a typed
// value.
package pipe

import (
	"reflect"

	"github.com/gonest-dev/gonest/internal/httpctx"
)

// contextType is used to validate Handler's accepted signature via reflect.
var contextType = reflect.TypeOf((*httpctx.Context)(nil))

// stringType is used to validate Handler's accepted signature via reflect.
var stringType = reflect.TypeOf("")

// Pipe represents a param transform: it holds the deferred configuration fn
// and (once Declare runs it) the validated Handler.
//
// New(fn) does not execute fn at call time. fn is only invoked when Declare
// is called, mirroring provider.New/controller.New/module.New.
type Pipe struct {
	fn func(*Pipe)

	handler  reflect.Value
	declared bool
}

// New creates a Pipe that defers fn until Declare runs. fn is expected to
// call Handler on the *Pipe passed to it -- no validation happens here.
func New(fn func(*Pipe)) *Pipe {
	return &Pipe{fn: fn}
}

// Declare runs this pipe's deferred fn exactly once. It is a no-op on any
// call after the first (including when fn is nil), matching
// provider.Provider's Declare semantics.
func (p *Pipe) Declare() {
	if p.declared {
		return
	}
	p.declared = true
	if p.fn != nil {
		p.fn(p)
	}
}

// Handler registers the transform function used to coerce a raw route param
// string into a typed value. It accepts exactly one signature (validated via
// reflect immediately):
//
//	func(ctx *httpctx.Context, raw string) T
//
// for any T. Any other signature panics with a clear message at declaration
// time. Actual invocation of the stored handler happens in a later task --
// this method only validates and stores it.
func (p *Pipe) Handler(fn any) {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func || !isValidHandlerSignature(v.Type()) {
		panic("gonest: invalid Pipe.Handler signature, expected func(ctx *httpctx.Context, raw string) T")
	}
	p.handler = v
}

// HandlerFunc returns the reflect.Value of the Handler stored via
// Handler(fn any), ready to be invoked directly (fn.Call(...)) by a future
// param-resolution task. Returns an invalid (zero) reflect.Value if Handler
// has not been called yet.
func (p *Pipe) HandlerFunc() reflect.Value {
	return p.handler
}

// isValidHandlerSignature reports whether t matches the single accepted
// Handler signature: func(ctx *httpctx.Context, raw string) T, for any T.
func isValidHandlerSignature(t reflect.Type) bool {
	if t.NumIn() != 2 {
		return false
	}
	if t.In(0) != contextType || t.In(1) != stringType {
		return false
	}
	return t.NumOut() == 1
}
