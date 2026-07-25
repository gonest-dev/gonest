package route

import (
	"reflect"
	"strings"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// defaultHttpCode is the status code a Route uses when HttpCode is never
// called, per design.md's Data Models comment ("default 200, sobrescrito
// por HttpCode()").
const defaultHttpCode = 200

// Route represents a single declared HTTP route -- method, path, default
// status code, and handler. It is purely declarative: nothing here
// participates in the DI graph.
type Route struct {
	method HttpMethod
	path   string

	httpCode int
	handler  func(c *execution.HttpContext)

	// Documentation-builder state (schema-generation feature, P1) -- every
	// field here is purely declarative, read back only by the OpenAPI
	// generator (P2, out of scope here). Setter-returns-self / plain-getter
	// convention throughout, same shape as HttpCode/Code above.
	summary     string
	description string
	operationId string

	// tags/tagsSet: tagsSet distinguishes "Tags was never called on this
	// Route (inherit the owning Controller's own Tags)" from "Tags WAS
	// called" (an explicit override, replacing the controller's value
	// entirely -- even if the route passed the exact same tags). Resolution
	// of the inherit-vs-override choice happens at generation time (P2),
	// not here -- Route has no back-reference to its owning Controller.
	tags    []string
	tagsSet bool

	// bearerAuthValue/bearerAuthSet: same never-called-vs-explicit-override
	// distinction as tags/tagsSet, but for a bool value that has no natural
	// nil/unset sentinel of its own.
	bearerAuthValue bool
	bearerAuthSet   bool

	requestBody *schema.Schema
	// formBody/formBodyFileFields: multipart/form-data documentation,
	// separate from requestBody (application/json) -- see FormBody's own
	// doc comment.
	formBody           *schema.Schema
	formBodyFileFields []string
	// responses maps status code -> the *Response built for it. The KEY's
	// mere presence distinguishes "documented" from "never documented at
	// all" (spec.md AC3) -- a *Response with no Schema() call still means
	// "documented, no body".
	responses map[int]*Response

	params *schema.Schema
	query  *schema.Schema

	excluded   bool
	deprecated bool

	// owner is the value passed to New -- in practice always the
	// *controller.Controller that declared this Route (via Controller.Route),
	// or nil for every existing test call site that never needs resolution.
	// Typed any, not module.Owner or some route-local named type, for the
	// same reason internal/inject's own Must[T](owner any) is: internal/
	// controller already imports internal/route (Controller.Route builds a
	// *Route), so route cannot import controller back without an import
	// cycle -- there is no concrete type route.go could name here even if it
	// wanted to. owner is consulted only by ResolveDirect/ResolveDirectAll
	// below, both driven by structural (not named) interface satisfaction, so
	// route never needs to know controller.Controller exists at all.
	owner any
}

// resolver is a package-private structural mirror of internal/inject's own
// unexported directResolver interface (ResolveDirect/ResolveDirectAll, same
// two methods, same signatures). It exists here, duplicated rather than
// imported, for the same reason declarable/constructable/lazyScoped are
// duplicated in internal/inject itself: route cannot import inject (inject
// already imports module, and importing inject from route buys nothing route
// doesn't already get for free via Go's structural typing), and inject
// cannot import route without inverting the controller -> route -> ??? import
// direction. Naming this interface identically in shape (not name) to
// inject.directResolver is exactly what lets owner -- typed any above --
// satisfy it via *controller.Controller's ALREADY-EXISTING ResolveDirect/
// ResolveDirectAll methods (see controller.go), with zero changes required in
// either internal/controller or internal/inject: Go interfaces are satisfied
// structurally, not by declared conformance, so a *Route whose owner is a
// *controller.Controller transparently becomes a directResolver itself the
// moment Route gains the same two methods below.
type resolver interface {
	ResolveDirect(t reflect.Type) (reflect.Value, bool)
	ResolveDirectAll(t reflect.Type) []reflect.Value
}

// New creates a Route and runs fn on it immediately -- unlike
// Provider/Module/Controller, which all defer fn until a later bootstrap
// stage. By the time Controller.Route(...) calls route.New, it is already
// running inside the Controller's own already-deferred fn, itself only
// invoked during Stage 2 after the module tree is fully assembled -- there
// is no further stage left to usefully defer to, so New runs fn right away
// (see design.md's "Route (declaração de rota)" component).
//
// owner is stored on r BEFORE fn runs (not after), because fn is exactly
// where a route callback calls gonest.MustInject[T](r) -- see
// ResolveDirect/ResolveDirectAll below, which read r.owner. Passing owner as
// nil (every pre-existing call site outside internal/controller) is safe and
// unchanged: ResolveDirect/ResolveDirectAll simply report not-found instead
// of panicking, the same as before this field existed.
func New(owner any, method HttpMethod, path string, fn func(*Route)) *Route {
	r := &Route{
		owner:    owner,
		method:   method,
		path:     path,
		httpCode: defaultHttpCode,
	}
	if fn != nil {
		fn(r)
	}
	return r
}

// ResolveDirect lets *Route satisfy internal/inject's unexported
// directResolver interface (see this package's own resolver interface's doc
// comment for why the two are structurally identical without either package
// importing the other). It delegates to owner's own ResolveDirect when owner
// satisfies resolver -- for a Route built via Controller.Route, that is the
// owning *controller.Controller, so gonest.MustInject[T](r) inside a route
// callback resolves from the EXACT SAME single-module scope
// gonest.MustInject[T](c) would for that Controller (spec.md's route-
// must-inject feature, R1). Returns a zero reflect.Value and false when owner
// is nil or does not satisfy resolver -- MustInject then panics with its own
// "not a pointer/no provider registered" message, not a crash here.
func (r *Route) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	dr, ok := r.owner.(resolver)
	if !ok {
		return reflect.Value{}, false
	}
	return dr.ResolveDirect(t)
}

// ResolveDirectAll mirrors ResolveDirect, for the interface-kind /
// MustInjectAll[T](r) path (spec.md's route-must-inject feature, R2).
// Returns nil, not a panic, when owner is nil or does not satisfy resolver --
// MustInjectAll's own dispatch then reports zero matches instead of crashing
// here.
func (r *Route) ResolveDirectAll(t reflect.Type) []reflect.Value {
	dr, ok := r.owner.(resolver)
	if !ok {
		return nil
	}
	return dr.ResolveDirectAll(t)
}

// Method returns this Route's HTTP method, as passed to New. Used by Stage
// 2.5 of app bootstrap (internal/app) to build the method+path collision key
// and to register the route on the HttpAdapter.
func (r *Route) Method() HttpMethod {
	return r.method
}

// Path returns this Route's declared path pattern, as passed to New (not
// combined with any owning Controller's PathPrefix -- that composition is
// Stage 2.5's job, not Route's). Used alongside Method for the same
// collision-key/registration purposes.
func (r *Route) Path() string {
	return r.path
}

// HttpCode stores the default status code this Route's Handler responds
// with (unless the Handler itself overrides it via ctx.Status).
func (r *Route) HttpCode(status int) {
	r.httpCode = status
}

// Code returns the currently stored default status code (200 unless
// HttpCode was called).
func (r *Route) Code() int {
	return r.httpCode
}

// Handler stores fn as this Route's request handler.
func (r *Route) Handler(fn func(c *execution.HttpContext)) {
	r.handler = fn
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called.
func (r *Route) HandlerFunc() func(c *execution.HttpContext) {
	return r.handler
}

// HasParam reports whether this Route's declared path pattern contains a
// ":name" segment matching name. This is the existence-check mechanism
// paramsSource/querySource (internal/validate) rely on: execution.Request.
// Param mirrors Fiber's own c.Params semantics and returns a bare "" for a
// param that doesn't exist on the route -- which is indistinguishable, at
// that layer alone, from a param that exists but genuinely carries an empty
// string value. Route already knows its own declared path string at
// construction time, so checking it directly gives callers a ground-truth
// answer to "does this route even have a param named X" that does not
// depend on interpreting Context's raw string return value.
func (r *Route) HasParam(name string) bool {
	segment := ":" + name
	for _, part := range strings.Split(r.path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}

// Summary sets this Route's OpenAPI operation summary and returns r so calls
// can chain. Takes at least one word (word, words...) rather than a bare
// `...string` -- a zero-arg call would compile silently and set an empty
// summary, indistinguishable from a caller mistake; requiring one argument
// makes that state unreachable instead of guessing at it. Multiple words are
// joined with a single space, letting a long summary split across several
// Go string literals -- one per call-site line -- without manual "..." +
// "..." concatenation.
func (r *Route) Summary(word string, words ...string) *Route {
	r.summary = strings.Join(append([]string{word}, words...), " ")
	return r
}

// SummaryText returns the summary set via Summary, or "" if it was never
// called.
func (r *Route) SummaryText() string {
	return r.summary
}

// Description sets this Route's OpenAPI operation description and returns r
// so calls can chain. See Summary's own doc comment for why this takes at
// least one word (word, words...) instead of a bare `...string`.
func (r *Route) Description(word string, words ...string) *Route {
	r.description = strings.Join(append([]string{word}, words...), " ")
	return r
}

// DescriptionText returns the description set via Description, or "" if it
// was never called.
func (r *Route) DescriptionText() string {
	return r.description
}

// OperationId sets this Route's OpenAPI operationId and returns r so calls
// can chain.
func (r *Route) OperationId(s string) *Route {
	r.operationId = s
	return r
}

// OperationIdText returns the operationId set via OperationId, or "" if it
// was never called.
func (r *Route) OperationIdText() string {
	return r.operationId
}

// Tags stores tags as this Route's OWN OpenAPI tags, overriding (not
// merging with) whatever the owning Controller's own Tags declared. Returns
// r so calls can chain.
func (r *Route) Tags(tags ...string) *Route {
	r.tags = append([]string(nil), tags...)
	r.tagsSet = true
	return r
}

// OwnTags returns the tags set via Tags, and whether Tags was ever called on
// this Route -- the bool distinguishes "never called, inherit the owning
// Controller's own Tags" (generation-time resolution, P2) from "called,
// explicit override" (spec.md AC2).
func (r *Route) OwnTags() ([]string, bool) {
	return append([]string(nil), r.tags...), r.tagsSet
}

// BearerAuth marks this Route as requiring bearer authentication, overriding
// (not merging with) whatever the owning Controller's own BearerAuth
// declared. Returns r so calls can chain.
func (r *Route) BearerAuth() *Route {
	r.bearerAuthValue = true
	r.bearerAuthSet = true
	return r
}

// HasBearerAuth returns the value set via BearerAuth, and whether
// BearerAuth was ever called on this Route -- the second bool distinguishes
// "never called, inherit the owning Controller's own BearerAuth" from
// "called, explicit override" (spec.md AC2, same shape as OwnTags).
func (r *Route) HasBearerAuth() (bool, bool) {
	return r.bearerAuthValue, r.bearerAuthSet
}

// RequestBody stores m as this Route's documented request body schema and
// returns r so calls can chain. Calling RequestBody more than once
// overwrites the previous value (last-write-wins, spec.md's Edge Cases).
func (r *Route) RequestBody(m *schema.Schema) *Route {
	r.requestBody = m
	return r
}

// RequestBodySchema returns the *schema.Schema set via RequestBody,
// and whether RequestBody was ever called -- the bool distinguishes "never
// documented" from "documented".
func (r *Route) RequestBodySchema() (*schema.Schema, bool) {
	return r.requestBody, r.requestBody != nil
}

// FormBody stores m as this Route's documented multipart/form-data request
// body schema (for MustParseRestFormBody routes, Multipart Form Streaming
// feature), plus fileFields -- the form field names that are FILES, not
// covered by m's own Property declarations (a file part has no primitive
// value to validate via Schema, only name/size/content-type -- see
// FormFile/MustParseRestFormBody's own onFile callback). Returns r so
// calls can chain. Calling FormBody more than once overwrites the previous
// value (last-write-wins, same convention as RequestBody).
func (r *Route) FormBody(m *schema.Schema, fileFields ...string) *Route {
	r.formBody = m
	r.formBodyFileFields = fileFields
	return r
}

// FormBodySchema returns the *schema.Schema and file-field names set via
// FormBody, and whether FormBody was ever called -- the bool distinguishes
// "never documented" from "documented".
func (r *Route) FormBodySchema() (*schema.Schema, []string, bool) {
	return r.formBody, r.formBodyFileFields, r.formBody != nil
}

// Response documents status as one of this Route's possible responses. With
// zero callback args, status is documented as having no body (the map key
// still exists, pointing at an empty *Response, distinguishing "documented,
// no body" from "never documented" -- spec.md AC3). With one callback arg,
// it runs against a fresh *Response so the route can set that status's
// Schema/Description in one place. Calling Response again for the SAME
// status overwrites that status's entry entirely; calling it for a
// DIFFERENT status accumulates alongside any previously documented
// statuses. Returns r so calls can chain.
func (r *Route) Response(status int, fn ...func(response *Response)) *Route {
	if r.responses == nil {
		r.responses = map[int]*Response{}
	}
	resp := &Response{}
	if len(fn) > 0 && fn[0] != nil {
		fn[0](resp)
	}
	r.responses[status] = resp
	return r
}

// Responses returns a copy of the status -> *Response map built via
// Response. Read-only: mutating the returned map does not affect this
// Route's internal state (same defensive-copy pattern as
// Controller.OwnMiddleware/OwnTags).
func (r *Route) Responses() map[int]*Response {
	out := make(map[int]*Response, len(r.responses))
	for status, resp := range r.responses {
		out[status] = resp
	}
	return out
}

// Params stores m as this Route's documented path-parameters schema and
// returns r so calls can chain (named after NestJS's `@Param()`, not
// `@PathParam()` -- matches MustParams' own naming, which reads path
// params at runtime). Calling Params more than once overwrites the
// previous value (last-write-wins, spec.md's Edge Cases).
func (r *Route) Params(m *schema.Schema) *Route {
	r.params = m
	return r
}

// ParamsSchema returns the *schema.Schema set via Params, and whether
// Params was ever called -- the bool distinguishes "never documented" from
// "documented".
func (r *Route) ParamsSchema() (*schema.Schema, bool) {
	return r.params, r.params != nil
}

// Query stores m as this Route's documented query-parameters schema and
// returns r so calls can chain (matches MustQuery's own naming, which reads
// query params at runtime). Calling Query more than once overwrites the
// previous value (last-write-wins, spec.md's Edge Cases).
func (r *Route) Query(m *schema.Schema) *Route {
	r.query = m
	return r
}

// QuerySchema returns the *schema.Schema set via Query, and whether Query
// was ever called -- the bool distinguishes "never documented" from
// "documented".
func (r *Route) QuerySchema() (*schema.Schema, bool) {
	return r.query, r.query != nil
}

// ExcludeFromDocs marks this Route to be omitted entirely from OpenAPI
// generation (P2/P4) and returns r so calls can chain.
func (r *Route) ExcludeFromDocs() *Route {
	r.excluded = true
	return r
}

// IsExcludedFromDocs reports whether ExcludeFromDocs was called.
func (r *Route) IsExcludedFromDocs() bool {
	return r.excluded
}

// Deprecated marks this Route as deprecated in OpenAPI generation and
// returns r so calls can chain.
func (r *Route) Deprecated() *Route {
	r.deprecated = true
	return r
}

// IsDeprecated reports whether Deprecated was called.
func (r *Route) IsDeprecated() bool {
	return r.deprecated
}
