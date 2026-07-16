package execution

import (
	"io"
	"testing"
)

// fakeResponder is a test-only implementation of Responder, standing in for
// the real Fiber-backed implementation that arrives in a later task (T7).
type fakeResponder struct {
	jsonValue  any
	jsonErr    error
	statusCode int
	headers    map[string]string
	params     map[string]string
	body       []byte
	queries    map[string]string
	htmlValue  string
	htmlErr    error
	method     string
	path       string
}

func newFakeResponder() *fakeResponder {
	return &fakeResponder{
		headers: map[string]string{},
		params:  map[string]string{},
	}
}

func (f *fakeResponder) JSON(v any) error {
	f.jsonValue = v
	return f.jsonErr
}

func (f *fakeResponder) SetStatus(code int) {
	f.statusCode = code
}

func (f *fakeResponder) GetStatus() int {
	return f.statusCode
}

func (f *fakeResponder) GetMethod() string {
	return f.method
}

func (f *fakeResponder) GetPath() string {
	return f.path
}

func (f *fakeResponder) GetHeader(name string) string {
	return f.headers[name]
}

func (f *fakeResponder) SetHeaderValue(name, value string) {
	f.headers[name] = value
}

func (f *fakeResponder) GetParam(name string) string {
	return f.params[name]
}

func (f *fakeResponder) Body() []byte {
	return f.body
}

func (f *fakeResponder) Queries() map[string]string {
	return f.queries
}

func (f *fakeResponder) HTML(s string) error {
	f.htmlValue = s
	return f.htmlErr
}

func (f *fakeResponder) SendString(s string) error {
	return nil
}

func (f *fakeResponder) BodyStream() (io.Reader, string, bool) {
	return nil, "", false
}

func TestContext_Json_DelegatesToResponder(t *testing.T) {
	fake := newFakeResponder()
	ctx := New(fake)

	type payload struct{ Name string }
	value := payload{Name: "gonest"}

	if err := ctx.Json(value); err != nil {
		t.Fatalf("Json() returned error: %v", err)
	}

	if fake.jsonValue != value {
		t.Fatalf("expected responder.JSON to receive %+v, got %+v", value, fake.jsonValue)
	}
}

func TestContext_Status_IsChainableAndSetsCode(t *testing.T) {
	fake := newFakeResponder()
	ctx := New(fake)

	returned := ctx.Status(201)

	if returned != ctx {
		t.Fatalf("expected Status() to return the same *Context for chaining")
	}
	if fake.statusCode != 201 {
		t.Fatalf("expected responder.SetStatus(201), got %d", fake.statusCode)
	}
}

func TestContext_Status_Json_Chained(t *testing.T) {
	fake := newFakeResponder()
	ctx := New(fake)

	if err := ctx.Status(200).Json(map[string]string{"ok": "true"}); err != nil {
		t.Fatalf("chained Status().Json() returned error: %v", err)
	}

	if fake.statusCode != 200 {
		t.Fatalf("expected status 200, got %d", fake.statusCode)
	}
	if fake.jsonValue == nil {
		t.Fatalf("expected JSON value to be set via chained call")
	}
}

func TestContext_Header_ReadsFromResponder(t *testing.T) {
	fake := newFakeResponder()
	fake.headers["X-Test"] = "abc"
	ctx := New(fake)

	if got := ctx.Header("X-Test"); got != "abc" {
		t.Fatalf("expected Header() to return %q, got %q", "abc", got)
	}
}

func TestContext_SetHeader_WritesToResponder(t *testing.T) {
	fake := newFakeResponder()
	ctx := New(fake)

	ctx.SetHeader("X-Custom", "value")

	if got := fake.headers["X-Custom"]; got != "value" {
		t.Fatalf("expected responder header %q, got %q", "value", got)
	}
}

func TestContext_Param_ReadsFromResponder(t *testing.T) {
	fake := newFakeResponder()
	fake.params["id"] = "42"
	ctx := New(fake)

	if got := ctx.Param("id"); got != "42" {
		t.Fatalf("expected Param() to return %q, got %q", "42", got)
	}
}

// TestContext_Body_ReadsFromResponder proves Body() returns exactly the
// bytes the underlying Responder provides, unchanged -- same one-line
// delegation pattern as every other Context method.
func TestContext_Body_ReadsFromResponder(t *testing.T) {
	fake := newFakeResponder()
	fake.body = []byte(`{"name":"gonest"}`)
	ctx := New(fake)

	got := ctx.Body()

	if string(got) != `{"name":"gonest"}` {
		t.Fatalf("expected Body() to return %q, got %q", fake.body, got)
	}
}

// TestContext_Body_EmptyByDefault proves Body() returns whatever the
// Responder reports for an unset body (nil/empty), not a panic -- the fake's
// zero value for body is nil.
func TestContext_Body_EmptyByDefault(t *testing.T) {
	fake := newFakeResponder()
	ctx := New(fake)

	if got := ctx.Body(); len(got) != 0 {
		t.Fatalf("expected Body() to be empty by default, got %q", got)
	}
}

// TestContext_Queries_ReadsFromResponder proves Queries() returns exactly
// the map the underlying Responder provides -- same one-line delegation
// pattern as Body()/Param().
func TestContext_Queries_ReadsFromResponder(t *testing.T) {
	fake := newFakeResponder()
	fake.queries = map[string]string{"page": "2", "limit": "10"}
	ctx := New(fake)

	got := ctx.Queries()

	if len(got) != 2 || got["page"] != "2" || got["limit"] != "10" {
		t.Fatalf("expected Queries() to return %+v, got %+v", fake.queries, got)
	}
}

// TestContext_Queries_EmptyByDefault proves Queries() returns whatever the
// Responder reports for an unset query map (nil/empty), not a panic.
func TestContext_Queries_EmptyByDefault(t *testing.T) {
	fake := newFakeResponder()
	ctx := New(fake)

	if got := ctx.Queries(); len(got) != 0 {
		t.Fatalf("expected Queries() to be empty by default, got %v", got)
	}
}

// TestContext_Route_NilByDefault proves a Context built without WithRoute
// has no attached route reference -- the zero value case exercised by any
// existing/future test that builds a Context directly and never needs
// MustParam's custom-Pipe lookup.
func TestContext_Route_NilByDefault(t *testing.T) {
	ctx := New(newFakeResponder())

	if got := ctx.Route(); got != nil {
		t.Fatalf("expected Route() to be nil by default, got %v", got)
	}
}

// TestContext_HTML_DelegatesToResponder proves HTML(s) is a one-line
// delegation to the underlying Responder's own HTML method -- same pattern
// as Json/Body/Queries above.
func TestContext_HTML_DelegatesToResponder(t *testing.T) {
	fake := newFakeResponder()
	ctx := New(fake)

	if err := ctx.HTML("<h1>hello</h1>"); err != nil {
		t.Fatalf("HTML() returned error: %v", err)
	}

	if fake.htmlValue != "<h1>hello</h1>" {
		t.Fatalf("expected responder.HTML to receive %q, got %q", "<h1>hello</h1>", fake.htmlValue)
	}
}

// TestContext_WithRoute_IsChainableAndStoresRoute proves WithRoute attaches
// an opaque reference and returns ctx for chaining, and Route() returns
// exactly what was attached (round-trip, no transformation).
func TestContext_WithRoute_IsChainableAndStoresRoute(t *testing.T) {
	ctx := New(newFakeResponder())

	type fakeRoute struct{ name string }
	r := &fakeRoute{name: "user-route"}

	returned := ctx.WithRoute(r)

	if returned != ctx {
		t.Fatalf("expected WithRoute() to return the same *Context for chaining")
	}
	got, ok := ctx.Route().(*fakeRoute)
	if !ok {
		t.Fatalf("expected Route() to return the attached *fakeRoute, got %T", ctx.Route())
	}
	if got != r {
		t.Fatalf("expected Route() to return the exact attached pointer")
	}
}
