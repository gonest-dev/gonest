package execution

import "testing"

// fakeResponder is a test-only implementation of Responder, standing in for
// the real Fiber-backed implementation that arrives in a later task (T7).
type fakeResponder struct {
	jsonValue  any
	jsonErr    error
	statusCode int
	headers    map[string]string
	params     map[string]string
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

func (f *fakeResponder) GetHeader(name string) string {
	return f.headers[name]
}

func (f *fakeResponder) SetHeaderValue(name, value string) {
	f.headers[name] = value
}

func (f *fakeResponder) GetParam(name string) string {
	return f.params[name]
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
