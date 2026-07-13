// Package exception is the foundational error vocabulary for gonest -- it
// defines what a "structured HTTP exception" is (Exception) and the one
// concrete type (HttpException) meant to be embedded by both framework
// built-ins and dev-defined exception types alike. It intentionally has zero
// HTTP-adapter knowledge (no request/response types, no Fiber import): a
// future panic-recovery feature will import this package to decide whether a
// recovered panic value is a structured exception, never the other way
// around (see design.md's Tech Decisions table).
package exception

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

// NewHttpException builds an HttpException from its four parts. It returns a
// VALUE, not a pointer: INSIGHT.md's own dev-defined-exception example
// assigns the result directly into a struct-literal field
// (`HttpException: gonest.NewHttpException(...)`), which requires a value.
// Embedding a value field also sidesteps the nil-pointer-embed footgun a
// *HttpException field would introduce. No validation is performed on any
// argument -- a zero or out-of-range status, an empty name/message, or a nil
// details are all accepted as-is; that is the caller's concern, not this
// constructor's.
func NewHttpException(status int, name, message string, details any) HttpException {
	return HttpException{
		status:  status,
		name:    name,
		message: message,
		details: details,
	}
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
