package route

import (
	"strings"

	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/schema"
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
	handler  func(ctx *execution.Context)

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
	// responses maps status code -> the *Response built for it. The KEY's
	// mere presence distinguishes "documented" from "never documented at
	// all" (spec.md AC3) -- a *Response with no Schema() call still means
	// "documented, no body".
	responses map[int]*Response

	params *schema.Schema
	query  *schema.Schema

	excluded   bool
	deprecated bool
}

// New creates a Route and runs fn on it immediately -- unlike
// Provider/Module/Controller, which all defer fn until a later bootstrap
// stage. By the time Controller.Route(...) calls route.New, it is already
// running inside the Controller's own already-deferred fn, itself only
// invoked during Stage 2 after the module tree is fully assembled -- there
// is no further stage left to usefully defer to, so New runs fn right away
// (see design.md's "Route (declaração de rota)" component).
func New(method HttpMethod, path string, fn func(*Route)) *Route {
	r := &Route{
		method:   method,
		path:     path,
		httpCode: defaultHttpCode,
	}
	if fn != nil {
		fn(r)
	}
	return r
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
func (r *Route) Handler(fn func(ctx *execution.Context)) {
	r.handler = fn
}

// HandlerFunc returns the handler stored via Handler, or nil if Handler was
// never called.
func (r *Route) HandlerFunc() func(ctx *execution.Context) {
	return r.handler
}

// HasParam reports whether this Route's declared path pattern contains a
// ":name" segment matching name. This is the existence-check mechanism
// MustParams[T]/MustQuery[T] (internal/validate) rely on: execution.Context.
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
// can chain.
func (r *Route) Summary(s string) *Route {
	r.summary = s
	return r
}

// SummaryText returns the summary set via Summary, or "" if it was never
// called.
func (r *Route) SummaryText() string {
	return r.summary
}

// Description sets this Route's OpenAPI operation description and returns r
// so calls can chain.
func (r *Route) Description(s string) *Route {
	r.description = s
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
