// Package execution provides Request/Response, the per-request read/write
// access points exposed to route Handlers (and Guard/Middleware/Interceptor/
// Filter). Both delegate to a minimal Responder interface so the real
// Fiber-backed implementation can be wired in later without changing their
// public API or these tests.
//
// Request/Response replace the single Context type this package used to
// export (request-response-split feature) -- Request carries read-side
// concerns (params/query/headers/body), Response carries write-side
// concerns (status/headers/body), mirroring the (req, res) convention most
// Node/Express-like frameworks use, which is the ecosystem gonest's target
// devuser comes from (see context.md's "Motivação de Produto").
package execution

import "io"

// Parseable is the single contract every HTTP data source (params, query,
// headers, JSON body, form body) satisfies -- unified-parse-api feature's
// gonest.Parse[T]/gonest.MustParse[T] are source-agnostic and only ever call
// ParseInto on whatever Parseable value req.Params()/req.Query()/etc. hand
// them.
//
// SPEC_DEVIATION (context.md wanted a single UNEXPORTED method so only
// internal/validate's source structs could implement it): Go's spec makes
// two unexported identifiers "different" whenever they're declared in
// different packages, even if spelled identically -- an unexported method
// named e.g. "parse" declared HERE (package execution) can never be
// satisfied by a method also named "parse" declared in internal/validate,
// because that's a distinct identifier in Go's eyes. Since every concrete
// Parseable implementation must live in internal/validate (to reuse
// resolveSchema/validateValue/populate/etc. without duplicating that logic
// here or introducing an execution->validate cycle), the method must be
// EXPORTED so validate's types can actually satisfy this interface. The
// concrete struct types themselves (paramsSource, querySource, ...) stay
// unexported in validate, and are only ever handed out via constructor
// functions returning Parseable -- so external callers still cannot
// construct or type-assert a custom source without deliberately choosing to
// implement ParseInto themselves (context.md's Deferred Ideas already
// anticipates this exact scenario).
type Parseable interface {
	ParseInto(dst any, schema any) error
}

// BodySource is the intermediate type req.Body() returns, letting the
// request body's FORMAT be picked explicitly and discoverably
// (req.Body().Json(), req.Body().Form(onFile), req.Body().Raw(),
// req.Body().Text()) instead of one function per source.
//
// jsonFn/formFn are opaque closures rather than direct fields of a
// validate-package type: internal/validate already imports internal/execution
// (for *Request), so execution can never import validate back without an
// import cycle. The closures are populated once, per-request, by whichever
// package bridges both (internal/app, at dispatch time) via NewBodySource --
// same "opaque carrier" trick Request.WithRoute already uses for the same
// reason (see WithRoute's doc comment).
type BodySource struct {
	req    *Request
	jsonFn func() Parseable
	formFn func(onFile func(*FormFile) error) Parseable
}

// NewBodySource builds a BodySource around req and the two
// source-constructing closures. Exported so internal/app (which imports both
// execution and validate) can wire a real BodySource into a Request
// per-request without execution ever needing to know internal/validate
// exists. req is kept (rather than the raw bytes directly) so Raw()/Text()
// can read req.res.RawBody() lazily, from the SAME package as Request (the
// underlying Responder field stays unexported) -- callers outside execution
// never need to extract/pass raw bytes themselves.
func NewBodySource(req *Request, jsonFn func() Parseable, formFn func(onFile func(*FormFile) error) Parseable) BodySource {
	return BodySource{req: req, jsonFn: jsonFn, formFn: formFn}
}

// Raw returns the request body as raw bytes -- request-response-split
// feature, replaces the old Request.RawBody() so every body access sits
// behind Body(). Same "synchronous use only, no defensive copy" contract the
// old RawBody() documented (see L-009 in STATE.md). Returns nil for a zero-
// value BodySource (e.g. a Request built directly in a test without
// WithSources).
func (b BodySource) Raw() []byte {
	if b.req == nil {
		return nil
	}
	return b.req.res.RawBody()
}

// Text returns the request body decoded as a string (string(Raw())) --
// request-response-split feature, sugar for callers that just want plain
// text instead of bytes.
func (b BodySource) Text() string {
	return string(b.Raw())
}

// Json returns a Parseable for this request's JSON body.
func (b BodySource) Json() Parseable {
	return b.jsonFn()
}

// Form returns a Parseable for this request's multipart/form-data body.
// onFile is invoked for each file part encountered during parsing; nil means
// file parts are silently skipped (spec.md's Edge Cases).
func (b BodySource) Form(onFile func(*FormFile) error) Parseable {
	return b.formFn(onFile)
}

// Responder is the minimal surface Request/Response need to serve their
// request/response methods. A fake implementation is used in tests; a
// Fiber-backed implementation is added in a later task.
//
// Exported (not just "responder") so packages other than execution -- namely
// internal/route's tests -- can build a *Request/*Response around their own
// fake without needing a Fiber-backed one to exist yet (see L-004 in
// STATE.md: Go ties interface satisfaction/visibility to whether the
// interface itself is exported, not just its methods).
type Responder interface {
	JSON(v any) error
	SetStatus(code int)
	GetStatus() int
	GetMethod() string
	GetPath() string
	GetHeader(name string) string
	SetHeaderValue(name, value string)
	GetParam(name string) string
	RawBody() []byte
	Queries() map[string]string
	HTML(s string) error
	SendString(s string) error
	// BodyStream returns the raw request body as a stream, plus the
	// multipart boundary parsed from Content-Type, for
	// ParseRestFormBody/MustParseRestFormBody (internal/validate,
	// Multipart Form Streaming feature, AD-022 in STATE.md). ok is false
	// when streaming was never enabled for this app
	// (AppOptions.EnableFormStreaming) OR the current request's
	// Content-Type isn't multipart/form-data at all -- 2 different non-
	// error conditions covered by one bool, same (value, bool) convention
	// internal/schema.Lookup already uses for "not applicable" rather
	// than inventing a sentinel error.
	BodyStream() (stream io.Reader, boundary string, ok bool)
}

// Request encapsulates the READ side of an HTTP request/response cycle for a
// single route Handler -- request-response-split feature (replaces the
// read-side half of the old single Context type).
type Request struct {
	res   Responder
	route any

	paramsSource  Parseable
	querySource   Parseable
	headersSource Parseable
	bodySource    BodySource
}

// newRequest builds a Request around the given Responder. Unexported:
// New (response.go) is the only public constructor, building both Request
// and Response together so Response can hold its Request reference from the
// start.
func newRequest(res Responder) *Request {
	return &Request{res: res}
}

// WithRoute attaches an opaque reference to the *route.Route that owns this
// Request to the Request, and returns req for chaining. It is stored as
// `any` (not *route.Route) deliberately: internal/route already imports
// internal/execution (Route.Handler takes a *Request) -- if execution
// imported route back to give WithRoute/Route a concrete type, that would be
// an import cycle. Storing `any` here keeps execution's boundary exactly as
// narrow as before ("no Fiber", not "no Route"): execution never inspects or
// calls anything on the stored value, it is purely a carrier.
// internal/validate (which imports both execution and route for
// paramsSource/etc.) is the only place that type-asserts this back to
// *route.Route.
func (req *Request) WithRoute(route any) *Request {
	req.route = route
	return req
}

// Route returns the opaque reference previously attached via WithRoute, or
// nil if none was attached (e.g. a Request built directly in a test that
// doesn't exercise the params-parsing path).
func (req *Request) Route() any {
	return req.route
}

// Method returns the actual incoming request's HTTP method (e.g. "POST"),
// as reported by the underlying HTTP framework -- not the route's own
// declared method (route.Route.Method), so it stays correct even if a
// Middleware runs before/without a *route.Route ever being attached.
func (req *Request) Method() string {
	return req.res.GetMethod()
}

// Path returns the actual incoming request's full path (e.g.
// "/auth/register/otp"), as reported by the underlying HTTP framework -- not
// the route's own declared pattern (route.Route.Path, which is relative to
// its controller's prefix and may contain ":param" placeholders).
func (req *Request) Path() string {
	return req.res.GetPath()
}

// Header returns the value of the named request header.
func (req *Request) Header(name string) string {
	return req.res.GetHeader(name)
}

// Param returns the raw string value of the named route param.
func (req *Request) Param(name string) string {
	return req.res.GetParam(name)
}

// Body returns a BodySource -- an intermediate type exposing .Raw()/.Text()/
// .Json()/.Form(onFile) so the request body's FORMAT is explicit and
// autocomplete-discoverable at the call site. The returned BodySource is
// whatever was wired in via WithSources for this request; a Request built
// directly without WithSources (e.g. in a test that doesn't exercise body
// parsing) returns a zero BodySource whose Json()/Form() would nil-panic if
// called -- callers that need body parsing always go through real request
// dispatch (internal/app), which always wires this.
func (req *Request) Body() BodySource {
	return req.bodySource
}

// Params returns a Parseable for this request's path params -- wired in via
// WithSources at dispatch time.
func (req *Request) Params() Parseable {
	return req.paramsSource
}

// Query returns a Parseable for this request's query string -- wired in via
// WithSources at dispatch time.
func (req *Request) Query() Parseable {
	return req.querySource
}

// Headers returns a Parseable for this request's HTTP headers -- wired in
// via WithSources at dispatch time.
func (req *Request) Headers() Parseable {
	return req.headersSource
}

// WithSources attaches the per-request Parseable/BodySource values that
// Params()/Query()/Headers()/Body() return, and returns req for chaining --
// same pattern as WithRoute. Called once per request by internal/app (the
// package that bridges execution and validate) right after WithRoute.
func (req *Request) WithSources(params, query, headers Parseable, body BodySource) *Request {
	req.paramsSource = params
	req.querySource = query
	req.headersSource = headers
	req.bodySource = body
	return req
}

// Queries returns the raw query-string params as a map, exactly as reported
// by the underlying Responder -- one-line delegation, same pattern as
// Body(). Used by querySource (internal/validate) as its source of
// presence/raw values, mirroring how paramsSource consults Route.HasParam/
// Request.Param instead.
func (req *Request) Queries() map[string]string {
	return req.res.Queries()
}

// FormStream returns the raw request body as a stream plus its multipart
// boundary, for formBodySource (internal/validate) -- one-line delegation,
// same pattern as Body()/Queries(). ok is false when form streaming isn't
// available for this request (see Responder.BodyStream's own doc comment
// for the 2 reasons that can be true).
func (req *Request) FormStream() (stream io.Reader, boundary string, ok bool) {
	return req.res.BodyStream()
}
