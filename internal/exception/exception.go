// Package exception is the foundational error vocabulary for gonest -- it
// defines what a "structured HTTP exception" is (Exception) and the one
// concrete type (HttpException) meant to be embedded by both framework
// built-ins and dev-defined exception types alike. It intentionally has zero
// HTTP-adapter knowledge (no request/response types, no Fiber import): a
// future panic-recovery feature will import this package to decide whether a
// recovered panic value is a structured exception, never the other way
// around (see design.md's Tech Decisions table).
package exception

import (
	"encoding/json"
	"net/http"
	"reflect"
)

// Exception is the single assertion point any code -- today just this
// package's own tests, eventually panic recovery and Filter -- uses to ask
// "is this value a structured HTTP exception". It is satisfied purely
// structurally: any type that embeds HttpException gets these four methods
// promoted automatically, with no explicit "implements Exception" needed.
// This mirrors Go's usual embedding-promotes-methods behavior and matches
// INSIGHT.md's dev-defined-exception pattern, where a type like
// `type FooExampleError struct { gonest.HttpException }` satisfies Exception
// without ever naming it.
type Exception interface {
	Status() int
	Name() string
	Message() string
	Details() any
}

// HttpException is the concrete carrier of an exception's four pieces of
// data. It is not meant to be used bare -- both the framework's built-in
// exceptions (NotFoundException etc., added in a later task) and any
// dev-defined exception type embed it BY VALUE, which is what promotes
// Status/Name/Message/Details onto the embedding type and lets that type
// satisfy Exception for free.
type HttpException struct {
	status  int
	name    string
	message string
	details any
}

// NewHttpException builds a zero-arg HttpException, defaulting status to
// http.StatusInternalServerError (500) -- name/message default to "",
// details defaults to nil. Configure it via the chainable Set* methods
// below (SetStatus/SetName/SetMessage/SetDetails), e.g.
// `gonest.NewHttpException().SetStatus(http.StatusConflict).SetName("DuplicateEmailException")`.
// It returns a VALUE, not a pointer: INSIGHT.md's own dev-defined-exception
// example assigns the result directly into a struct-literal field
// (`HttpException: gonest.NewHttpException()...`), which requires a value.
// Embedding a value field also sidesteps the nil-pointer-embed footgun a
// *HttpException field would introduce. Each Set* method also has a value
// receiver and returns a new HttpException value (immutable-builder style,
// idiomatic for a small value type) -- no method mutates its receiver in
// place.
func NewHttpException() HttpException {
	return HttpException{status: http.StatusInternalServerError}
}

// SetStatus returns a copy of e with status set to code.
func (e HttpException) SetStatus(code int) HttpException {
	e.status = code
	return e
}

// SetName returns a copy of e with name set. If never called, Name()
// returns "" -- EffectiveName (below) is the framework's own fallback for
// formatting an exception whose name was never explicitly set.
func (e HttpException) SetName(name string) HttpException {
	e.name = name
	return e
}

// SetMessage returns a copy of e with message set.
func (e HttpException) SetMessage(message string) HttpException {
	e.message = message
	return e
}

// SetDetails returns a copy of e with details set.
func (e HttpException) SetDetails(details any) HttpException {
	e.details = details
	return e
}

// Status returns the HTTP status code this exception carries, exactly as
// passed to NewHttpException.
func (e HttpException) Status() int {
	return e.status
}

// Name returns this exception's name, exactly as passed to NewHttpException.
func (e HttpException) Name() string {
	return e.name
}

// Message returns this exception's human-readable message, exactly as
// passed to NewHttpException.
func (e HttpException) Message() string {
	return e.message
}

// Details returns this exception's arbitrary details payload, exactly as
// passed to NewHttpException -- including a bare nil if that is what was
// passed, never a synthesized zero value.
func (e HttpException) Details() any {
	return e.details
}

// Error satisfies Go's stdlib error interface -- promoted onto every type
// embedding HttpException by value (built-in and dev-defined alike), same
// mechanism as Status/Name/Message/Details/MarshalJSON. Lets the non-
// panicking ParseRest* family (internal/validate) return a *BadRequestException
// etc. directly as a plain `error`, with no separate wrapper type needed.
func (e HttpException) Error() string {
	return e.message
}

// MarshalJSON encodes e as {"name","message","details"} -- deliberately
// omitting status, which belongs on the HTTP response's status line/code,
// not the JSON body. Because Go's encoding/json invokes a promoted
// MarshalJSON exactly like any other promoted method, any type embedding
// HttpException by value (every built-in exception, every dev-defined
// exception type) automatically gets this exact JSON shape from a plain
// `json.Marshal(exc)`/`ctx.Json(exc)` call, with zero manual field mapping
// -- see .examples/blog-api/shared/exception.go's own Filter for a real
// consumer of this. Note: since this method only ever runs with e itself
// as the receiver (Go's method promotion does not hand the OUTER embedding
// type's identity down into an embedded field's own method), it cannot
// auto-default an empty name to the outer concrete type's own name -- see
// EffectiveName below for where that fallback actually lives.
func (e HttpException) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name    string `json:"name"`
		Message string `json:"message"`
		Details any    `json:"details"`
	}{Name: e.name, Message: e.message, Details: e.details})
}

// EffectiveName returns exc.Name() if non-empty, otherwise a default
// derived from exc's own concrete (dynamic) type via reflect -- e.g. a
// dev-defined `*FooError` that never called SetName still gets a
// reasonable name ("FooError") instead of an empty string. This fallback
// cannot live inside HttpException.Name()/MarshalJSON() itself (a method
// promoted from an embedded field has no way to see the OUTER struct's own
// type identity) -- it needs the full Exception value, which is exactly
// what a panic-recovery site (internal/adapter/fiber's own default
// formatting) or a Filter always has on hand. Used by the framework's own
// default (unhandled) exception formatting; re-exported at root
// (gonest.ExceptionName) for any dev-defined Filter that wants the same
// fallback behavior.
func EffectiveName(exc Exception) string {
	if name := exc.Name(); name != "" {
		return name
	}
	t := reflect.TypeOf(exc)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}
