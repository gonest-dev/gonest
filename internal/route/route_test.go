package route

import (
	"io"
	"reflect"
	"testing"
	"unsafe"

	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/schema"
)

// fakeResponder is a minimal test-only execution.Responder, mirroring the one
// in internal/execution/context_test.go (that one is unexported to its own
// package, so route's tests need their own).
type fakeResponder struct {
	params map[string]string
}

func newFakeResponder() *fakeResponder {
	return &fakeResponder{params: map[string]string{}}
}

func (f *fakeResponder) JSON(v any) error                      { return nil }
func (f *fakeResponder) SetStatus(code int)                    {}
func (f *fakeResponder) GetHeader(name string) string          { return "" }
func (f *fakeResponder) SetHeaderValue(name, value string)     {}
func (f *fakeResponder) GetParam(name string) string           { return f.params[name] }
func (f *fakeResponder) Body() []byte                          { return nil }
func (f *fakeResponder) Queries() map[string]string            { return nil }
func (f *fakeResponder) HTML(s string) error                   { return nil }
func (f *fakeResponder) SendString(s string) error             { return nil }
func (f *fakeResponder) BodyStream() (io.Reader, string, bool) { return nil, "", false }

// TestNew_RunsFnImmediately proves route.New(method, path, fn) runs fn
// synchronously at call time -- unlike Provider/Module/Controller,
// which all defer fn until a later bootstrap stage (see design.md's
// rationale: by the time Controller.Route(...) invokes route.New, it is
// already executing inside the Controller's own already-deferred fn during
// Stage 2, so there is no further stage left to defer to).
func TestNew_RunsFnImmediately(t *testing.T) {
	ran := false
	r := New(HttpGet, "/users", func(r *Route) {
		ran = true
	})

	if r == nil {
		t.Fatal("expected New to return a non-nil *Route")
	}
	if !ran {
		t.Fatal("expected fn passed to New to run immediately, not be deferred")
	}
}

// TestHttpCode_StoresStatus proves HttpCode stores the default status code.
func TestHttpCode_StoresStatus(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {
		r.HttpCode(201)
	})

	if got := r.Code(); got != 201 {
		t.Fatalf("expected HttpCode(201) to store 201, got %d", got)
	}
}

// TestHttpCode_DefaultsTo200 proves a Route that never calls HttpCode
// defaults to 200 (per design.md's Data Models comment: "default 200,
// sobrescrito por HttpCode()").
func TestHttpCode_DefaultsTo200(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	if got := r.Code(); got != 200 {
		t.Fatalf("expected default HttpCode to be 200, got %d", got)
	}
}

// TestHandler_StoresFn proves Handler stores the handler fn, retrievable
// (and callable) later.
func TestHandler_StoresFn(t *testing.T) {
	called := false
	r := New(HttpGet, "/users", func(r *Route) {
		r.Handler(func(ctx *execution.Context) {
			called = true
		})
	})

	h := r.HandlerFunc()
	if h == nil {
		t.Fatal("expected HandlerFunc() to return the stored handler, got nil")
	}
	h(execution.New(newFakeResponder()))
	if !called {
		t.Fatal("expected stored handler to be callable and run")
	}
}

// TestHasParam_TrueForDeclaredPathSegment proves HasParam checks the
// Route's own declared path pattern for a ":name" segment -- this is the
// existence-check mechanism MustParams[T]/MustQuery[T] (internal/validate)
// rely on to distinguish "genuinely absent from this route" from "present
// but empty string".
func TestHasParam_TrueForDeclaredPathSegment(t *testing.T) {
	r := New(HttpGet, "/users/:id/orders/:orderId", func(r *Route) {})

	if !r.HasParam("id") {
		t.Fatal("expected HasParam(\"id\") to be true, path declares :id")
	}
	if !r.HasParam("orderId") {
		t.Fatal("expected HasParam(\"orderId\") to be true, path declares :orderId")
	}
}

// TestHasParam_FalseForUndeclaredName proves HasParam is false for a name
// that isn't a ":name" segment in the Route's declared path.
func TestHasParam_FalseForUndeclaredName(t *testing.T) {
	r := New(HttpGet, "/users/:id", func(r *Route) {})

	if r.HasParam("nonexistent") {
		t.Fatal("expected HasParam(\"nonexistent\") to be false, path does not declare it")
	}
}

// TestMethod_ReturnsConstructedMethod proves Method returns the HttpMethod
// passed to New -- needed by Stage 2.5 (app bootstrap route collection) to
// build a method+path collision key without reaching into Route's
// unexported fields.
func TestMethod_ReturnsConstructedMethod(t *testing.T) {
	r := New(HttpPost, "/users", func(r *Route) {})

	if got := r.Method(); got != HttpPost {
		t.Fatalf("Method() = %v, want %v", got, HttpPost)
	}
}

// TestPath_ReturnsConstructedPath proves Path returns the path string passed
// to New -- needed by Stage 2.5 to build the full route path (controller
// PathPrefix + route Path) for both collision detection and adapter
// registration.
func TestPath_ReturnsConstructedPath(t *testing.T) {
	r := New(HttpGet, "/users/:id", func(r *Route) {})

	if got := r.Path(); got != "/users/:id" {
		t.Fatalf("Path() = %q, want %q", got, "/users/:id")
	}
}

// newTestSchemaFor builds a throwaway *schema.Schema for zero's own
// type, suitable for use as RequestBody/Response/Params/Query
// payloads in route documentation-builder tests -- Route only needs a
// *schema.Schema pointer identity, never inspects its contents. The
// registry (internal/schema/registry.go) keys registration by Go type and
// panics on a duplicate, so callers needing MULTIPLE distinct *Schema
// values within one test must pass distinctly-typed zero values (see
// dummyA/dummyB below).
func newTestSchemaFor(t *testing.T, zero any) *schema.Schema {
	t.Helper()
	typ := reflect.TypeOf(zero).Elem()
	t.Cleanup(func() { schema.Deregister(typ) })
	return schema.New(typ, uintptr(unsafe.Pointer(reflect.ValueOf(zero).Pointer())))
}

// newTestSchemaForRoute is a convenience wrapper over newTestSchemaFor
// for tests that only ever need ONE *schema.Schema value.
func newTestSchemaForRoute(t *testing.T) *schema.Schema {
	t.Helper()
	type dummy struct{ X int }
	return newTestSchemaFor(t, &dummy{})
}

// TestSummary_StoresAndReturnsSelf proves Summary sets the value returned by
// SummaryText, and returns r so calls can chain.
func TestSummary_StoresAndReturnsSelf(t *testing.T) {
	var got *Route
	r := New(HttpGet, "/users", func(r *Route) {
		got = r.Summary("List users")
	})

	if got != r {
		t.Fatal("Route.Summary did not return the same *Route for chaining")
	}
	if r.SummaryText() != "List users" {
		t.Errorf("SummaryText() = %q, want %q", r.SummaryText(), "List users")
	}
}

// TestSummary_DefaultsEmpty proves SummaryText returns "" before Summary is
// ever called.
func TestSummary_DefaultsEmpty(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	if r.SummaryText() != "" {
		t.Errorf("SummaryText() = %q, want \"\"", r.SummaryText())
	}
}

// TestDescription_StoresAndReturnsSelf proves Description sets the value
// returned by DescriptionText, and returns r so calls can chain.
func TestDescription_StoresAndReturnsSelf(t *testing.T) {
	var got *Route
	r := New(HttpGet, "/users", func(r *Route) {
		got = r.Description("Lists every user")
	})

	if got != r {
		t.Fatal("Route.Description did not return the same *Route for chaining")
	}
	if r.DescriptionText() != "Lists every user" {
		t.Errorf("DescriptionText() = %q, want %q", r.DescriptionText(), "Lists every user")
	}
}

// TestDescription_DefaultsEmpty proves DescriptionText returns "" before
// Description is ever called.
func TestDescription_DefaultsEmpty(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	if r.DescriptionText() != "" {
		t.Errorf("DescriptionText() = %q, want \"\"", r.DescriptionText())
	}
}

// TestOperationId_StoresAndReturnsSelf proves OperationId sets the value
// returned by OperationIdText, and returns r so calls can chain.
func TestOperationId_StoresAndReturnsSelf(t *testing.T) {
	var got *Route
	r := New(HttpGet, "/users", func(r *Route) {
		got = r.OperationId("listUsers")
	})

	if got != r {
		t.Fatal("Route.OperationId did not return the same *Route for chaining")
	}
	if r.OperationIdText() != "listUsers" {
		t.Errorf("OperationIdText() = %q, want %q", r.OperationIdText(), "listUsers")
	}
}

// TestOperationId_DefaultsEmpty proves OperationIdText returns "" before
// OperationId is ever called.
func TestOperationId_DefaultsEmpty(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	if r.OperationIdText() != "" {
		t.Errorf("OperationIdText() = %q, want \"\"", r.OperationIdText())
	}
}

// TestTags_NeverCalled_ReportsUnset proves OwnTags returns (nil-or-empty,
// false) before Tags is ever called -- distinguishing "never called, inherit
// controller's" from "called" (design.md's Route documentation builder
// methods component).
func TestTags_NeverCalled_ReportsUnset(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	tags, set := r.OwnTags()
	if set {
		t.Fatal("OwnTags() set = true, want false before Tags() was ever called")
	}
	if len(tags) != 0 {
		t.Fatalf("OwnTags() tags = %v, want empty before Tags() was ever called", tags)
	}
}

// TestTags_Called_ReportsSetWithValue proves OwnTags returns (tags, true)
// after Tags is called, overriding the controller's own value entirely.
func TestTags_Called_ReportsSetWithValue(t *testing.T) {
	var got *Route
	r := New(HttpGet, "/users", func(r *Route) {
		got = r.Tags("admin", "internal")
	})

	if got != r {
		t.Fatal("Route.Tags did not return the same *Route for chaining")
	}

	tags, set := r.OwnTags()
	if !set {
		t.Fatal("OwnTags() set = false, want true after Tags() was called")
	}
	if len(tags) != 2 || tags[0] != "admin" || tags[1] != "internal" {
		t.Fatalf("OwnTags() tags = %v, want [admin internal]", tags)
	}
}

// TestBearerAuth_NeverCalled_ReportsUnset proves HasBearerAuth returns
// (false, false) before BearerAuth is ever called -- distinguishing "never
// called, inherit controller's" from "explicitly called".
func TestBearerAuth_NeverCalled_ReportsUnset(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	value, set := r.HasBearerAuth()
	if set {
		t.Fatal("HasBearerAuth() set = true, want false before BearerAuth() was ever called")
	}
	if value {
		t.Fatal("HasBearerAuth() value = true, want false as the zero value before BearerAuth() was ever called")
	}
}

// TestBearerAuth_Called_ReportsSetTrue proves HasBearerAuth returns
// (true, true) after BearerAuth is called, and BearerAuth returns r so calls
// can chain.
func TestBearerAuth_Called_ReportsSetTrue(t *testing.T) {
	var got *Route
	r := New(HttpGet, "/users", func(r *Route) {
		got = r.BearerAuth()
	})

	if got != r {
		t.Fatal("Route.BearerAuth did not return the same *Route for chaining")
	}

	value, set := r.HasBearerAuth()
	if !set {
		t.Fatal("HasBearerAuth() set = false, want true after BearerAuth() was called")
	}
	if !value {
		t.Fatal("HasBearerAuth() value = false, want true after BearerAuth() was called")
	}
}

// TestRequestBody_StoresAndReportsSet proves RequestBody stores the
// *schema.Schema retrievable via RequestBodySchema, and returns r so
// calls can chain.
func TestRequestBody_StoresAndReportsSet(t *testing.T) {
	m := newTestSchemaForRoute(t)

	var got *Route
	r := New(HttpPost, "/users", func(r *Route) {
		got = r.RequestBody(m)
	})

	if got != r {
		t.Fatal("Route.RequestBody did not return the same *Route for chaining")
	}

	gotMeta, set := r.RequestBodySchema()
	if !set {
		t.Fatal("RequestBodySchema() set = false, want true after RequestBody() was called")
	}
	if gotMeta != m {
		t.Fatalf("RequestBodySchema() = %v, want %v", gotMeta, m)
	}
}

// TestRequestBody_NeverCalled_ReportsUnset proves RequestBodySchema
// returns (nil, false) before RequestBody is ever called.
func TestRequestBody_NeverCalled_ReportsUnset(t *testing.T) {
	r := New(HttpPost, "/users", func(r *Route) {})

	gotMeta, set := r.RequestBodySchema()
	if set {
		t.Fatal("RequestBodySchema() set = true, want false before RequestBody() was ever called")
	}
	if gotMeta != nil {
		t.Fatalf("RequestBodySchema() = %v, want nil before RequestBody() was ever called", gotMeta)
	}
}

// TestResponse_ZeroArgs_DocumentsNoBodyButKeepsStatusKey proves Response
// called with zero callback args documents status with no body -- but the
// STATUS KEY itself exists in the returned map (an empty *Response),
// distinguishing "documented, no body" from "never documented at all"
// (spec.md AC3).
func TestResponse_ZeroArgs_DocumentsNoBodyButKeepsStatusKey(t *testing.T) {
	var got *Route
	r := New(HttpDelete, "/users/:id", func(r *Route) {
		got = r.Response(204)
	})

	if got != r {
		t.Fatal("Route.Response did not return the same *Route for chaining")
	}

	responses := r.Responses()
	resp, exists := responses[204]
	if !exists {
		t.Fatal("Responses()[204] key does not exist, want it to exist (documented, no body)")
	}
	if _, hasSchema := resp.SchemaValue(); hasSchema {
		t.Fatalf("Responses()[204].SchemaValue() = ok, want no body documented")
	}

	// A status never passed to Response at all must NOT have a key.
	if _, undocumented := responses[404]; undocumented {
		t.Fatal("Responses()[404] key exists, want it absent -- 404 was never documented")
	}
}

// TestResponse_OneArg_StoresBody proves Response called with one callback
// arg stores that callback's Schema for the given status.
func TestResponse_OneArg_StoresBody(t *testing.T) {
	m := newTestSchemaForRoute(t)

	r := New(HttpGet, "/users/:id", func(r *Route) {
		r.Response(200, func(response *Response) {
			response.Schema(m)
		})
	})

	responses := r.Responses()
	resp, exists := responses[200]
	if !exists {
		t.Fatal("Responses()[200] key does not exist, want it to exist")
	}
	body, ok := resp.SchemaValue()
	if !ok || body != m {
		t.Fatalf("Responses()[200].SchemaValue() = %v, %v, want %v, true", body, ok, m)
	}
}

// TestResponse_DifferentStatuses_Accumulates proves calling Response
// multiple times for DIFFERENT statuses accumulates all of them, rather than
// overwriting.
func TestResponse_DifferentStatuses_Accumulates(t *testing.T) {
	type okBody struct{ X int }
	type errBody struct{ Y int }
	okMeta := newTestSchemaFor(t, &okBody{})
	errMeta := newTestSchemaFor(t, &errBody{})

	r := New(HttpGet, "/users/:id", func(r *Route) {
		r.Response(200, func(response *Response) { response.Schema(okMeta) })
		r.Response(404, func(response *Response) { response.Schema(errMeta) })
	})

	responses := r.Responses()
	if len(responses) != 2 {
		t.Fatalf("Responses() has %d entries, want 2", len(responses))
	}
	if body, _ := responses[200].SchemaValue(); body != okMeta {
		t.Fatalf("Responses()[200].SchemaValue() = %v, want %v", body, okMeta)
	}
	if body, _ := responses[404].SchemaValue(); body != errMeta {
		t.Fatalf("Responses()[404].SchemaValue() = %v, want %v", body, errMeta)
	}
}

// TestResponse_SameStatusTwice_Overwrites proves calling Response twice for
// the SAME status overwrites the earlier entry (spec.md's Edge Cases:
// last-write-wins for a repeated slot).
func TestResponse_SameStatusTwice_Overwrites(t *testing.T) {
	type firstBody struct{ X int }
	type secondBody struct{ Y int }
	firstMeta := newTestSchemaFor(t, &firstBody{})
	secondMeta := newTestSchemaFor(t, &secondBody{})

	r := New(HttpGet, "/users/:id", func(r *Route) {
		r.Response(200, func(response *Response) { response.Schema(firstMeta) })
		r.Response(200, func(response *Response) { response.Schema(secondMeta) })
	})

	responses := r.Responses()
	if len(responses) != 1 {
		t.Fatalf("Responses() has %d entries, want 1", len(responses))
	}
	if body, _ := responses[200].SchemaValue(); body != secondMeta {
		t.Fatalf("Responses()[200].SchemaValue() = %v, want %v (last write wins)", body, secondMeta)
	}
}

// TestResponses_ReturnsCopyNotInternalMap proves Responses is a defensive
// copy -- mutating the returned map must not affect the Route's own state.
func TestResponses_ReturnsCopyNotInternalMap(t *testing.T) {
	m := newTestSchemaForRoute(t)

	r := New(HttpGet, "/users/:id", func(r *Route) {
		r.Response(200, func(response *Response) { response.Schema(m) })
	})

	got := r.Responses()
	got[200] = nil
	got[999] = &Response{}

	got2 := r.Responses()
	if body, _ := got2[200].SchemaValue(); body != m {
		t.Fatal("Responses() leaked mutable internal map: mutation of returned map affected subsequent call")
	}
	if _, exists := got2[999]; exists {
		t.Fatal("Responses() leaked mutable internal map: added key affected subsequent call")
	}
}

// TestParams_StoresAndReportsSet proves Params stores the *schema.Schema
// retrievable via ParamsSchema, and returns r so calls can chain.
func TestParams_StoresAndReportsSet(t *testing.T) {
	m := newTestSchemaForRoute(t)

	var got *Route
	r := New(HttpGet, "/users/:id", func(r *Route) {
		got = r.Params(m)
	})

	if got != r {
		t.Fatal("Route.Params did not return the same *Route for chaining")
	}

	gotMeta, set := r.ParamsSchema()
	if !set {
		t.Fatal("ParamsSchema() set = false, want true after Params() was called")
	}
	if gotMeta != m {
		t.Fatalf("ParamsSchema() = %v, want %v", gotMeta, m)
	}
}

// TestParams_NeverCalled_ReportsUnset proves ParamsSchema returns
// (nil, false) before Params is ever called.
func TestParams_NeverCalled_ReportsUnset(t *testing.T) {
	r := New(HttpGet, "/users/:id", func(r *Route) {})

	gotMeta, set := r.ParamsSchema()
	if set {
		t.Fatal("ParamsSchema() set = true, want false before Params() was ever called")
	}
	if gotMeta != nil {
		t.Fatalf("ParamsSchema() = %v, want nil", gotMeta)
	}
}

// TestQuery_StoresAndReportsSet proves Query stores the *schema.Schema
// retrievable via QuerySchema, and returns r so calls can chain.
func TestQuery_StoresAndReportsSet(t *testing.T) {
	m := newTestSchemaForRoute(t)

	var got *Route
	r := New(HttpGet, "/users", func(r *Route) {
		got = r.Query(m)
	})

	if got != r {
		t.Fatal("Route.Query did not return the same *Route for chaining")
	}

	gotMeta, set := r.QuerySchema()
	if !set {
		t.Fatal("QuerySchema() set = false, want true after Query() was called")
	}
	if gotMeta != m {
		t.Fatalf("QuerySchema() = %v, want %v", gotMeta, m)
	}
}

// TestQuery_NeverCalled_ReportsUnset proves QuerySchema returns (nil,
// false) before Query is ever called.
func TestQuery_NeverCalled_ReportsUnset(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	gotMeta, set := r.QuerySchema()
	if set {
		t.Fatal("QuerySchema() set = true, want false before Query() was ever called")
	}
	if gotMeta != nil {
		t.Fatalf("QuerySchema() = %v, want nil", gotMeta)
	}
}

// TestExcludeFromDocs_SetsFlag proves ExcludeFromDocs sets the flag returned
// by IsExcludedFromDocs, and returns r so calls can chain.
func TestExcludeFromDocs_SetsFlag(t *testing.T) {
	var got *Route
	r := New(HttpGet, "/internal/health", func(r *Route) {
		got = r.ExcludeFromDocs()
	})

	if got != r {
		t.Fatal("Route.ExcludeFromDocs did not return the same *Route for chaining")
	}
	if !r.IsExcludedFromDocs() {
		t.Fatal("IsExcludedFromDocs() = false, want true after ExcludeFromDocs() was called")
	}
}

// TestIsExcludedFromDocs_DefaultsFalse proves IsExcludedFromDocs returns
// false before ExcludeFromDocs is ever called.
func TestIsExcludedFromDocs_DefaultsFalse(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	if r.IsExcludedFromDocs() {
		t.Fatal("IsExcludedFromDocs() = true, want false before ExcludeFromDocs() was ever called")
	}
}

// TestDeprecated_SetsFlag proves Deprecated sets the flag returned by
// IsDeprecated, and returns r so calls can chain.
func TestDeprecated_SetsFlag(t *testing.T) {
	var got *Route
	r := New(HttpGet, "/users/legacy", func(r *Route) {
		got = r.Deprecated()
	})

	if got != r {
		t.Fatal("Route.Deprecated did not return the same *Route for chaining")
	}
	if !r.IsDeprecated() {
		t.Fatal("IsDeprecated() = false, want true after Deprecated() was called")
	}
}

// TestIsDeprecated_DefaultsFalse proves IsDeprecated returns false before
// Deprecated is ever called.
func TestIsDeprecated_DefaultsFalse(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	if r.IsDeprecated() {
		t.Fatal("IsDeprecated() = true, want false before Deprecated() was ever called")
	}
}
