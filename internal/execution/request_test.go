package execution

import (
	"bufio"
	"io"
	"testing"
)

// fakeResponder is a test-only implementation of Responder, standing in for
// the real Fiber-backed implementation.
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
	sentString string

	isUpgradeRequest    bool
	isUpgradeCalled     bool
	upgradeCalled       bool
	upgradeHandler      func(conn WSConn)
	upgradeSubprotocols []string
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

func (f *fakeResponder) RawBody() []byte {
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
	f.sentString = s
	return nil
}

func (f *fakeResponder) BodyStream() (io.Reader, string, bool) {
	return nil, "", false
}

func (f *fakeResponder) WriteStream(fn func(w *bufio.Writer)) {}

func (f *fakeResponder) IsUpgradeRequest() bool {
	f.isUpgradeCalled = true
	return f.isUpgradeRequest
}

func (f *fakeResponder) Upgrade(handler func(conn WSConn), subprotocols ...string) {
	f.upgradeCalled = true
	f.upgradeHandler = handler
	f.upgradeSubprotocols = subprotocols
}

func TestRequest_Header_ReadsFromResponder(t *testing.T) {
	fake := newFakeResponder()
	fake.headers["X-Test"] = "abc"
	req, _ := New(fake)

	if got := req.Header("X-Test"); got != "abc" {
		t.Fatalf("expected Header() to return %q, got %q", "abc", got)
	}
}

func TestRequest_Param_ReadsFromResponder(t *testing.T) {
	fake := newFakeResponder()
	fake.params["id"] = "42"
	req, _ := New(fake)

	if got := req.Param("id"); got != "42" {
		t.Fatalf("expected Param() to return %q, got %q", "42", got)
	}
}

// TestRequest_Body_ReturnsBodySource proves Body() returns a BodySource
// rather than raw bytes.
func TestRequest_Body_ReturnsBodySource(t *testing.T) {
	fake := newFakeResponder()
	req, _ := New(fake)

	var _ BodySource = req.Body()
}

// TestBodySource_Raw_ReturnsRawBytes proves BodySource.Raw() returns exactly
// the bytes the underlying Responder reports.
func TestBodySource_Raw_ReturnsRawBytes(t *testing.T) {
	fake := newFakeResponder()
	fake.body = []byte(`{"name":"gonest"}`)
	req, _ := New(fake)
	req.WithSources(nil, nil, nil, NewBodySource(req, nil, nil))

	if got := req.Body().Raw(); string(got) != `{"name":"gonest"}` {
		t.Fatalf("expected Raw() to return %q, got %q", `{"name":"gonest"}`, got)
	}
}

// TestBodySource_Text_ReturnsStringOfRaw proves BodySource.Text() is
// string(Raw()).
func TestBodySource_Text_ReturnsStringOfRaw(t *testing.T) {
	fake := newFakeResponder()
	fake.body = []byte("hello")
	req, _ := New(fake)
	req.WithSources(nil, nil, nil, NewBodySource(req, nil, nil))

	if got := req.Body().Text(); got != "hello" {
		t.Fatalf("expected Text() to return %q, got %q", "hello", got)
	}
}

// TestBodySource_Raw_EmptyByDefault proves a zero-value BodySource's Raw()
// is empty, not a panic.
func TestBodySource_Raw_EmptyByDefault(t *testing.T) {
	req, _ := New(newFakeResponder())

	if got := req.Body().Raw(); len(got) != 0 {
		t.Fatalf("expected Raw() to be empty by default, got %q", got)
	}
}

// TestRequest_Queries_ReadsFromResponder proves Queries() returns exactly
// the map the underlying Responder provides -- same one-line delegation
// pattern as Body()/Param().
func TestRequest_Queries_ReadsFromResponder(t *testing.T) {
	fake := newFakeResponder()
	fake.queries = map[string]string{"page": "2", "limit": "10"}
	req, _ := New(fake)

	got := req.Queries()

	if len(got) != 2 || got["page"] != "2" || got["limit"] != "10" {
		t.Fatalf("expected Queries() to return %+v, got %+v", fake.queries, got)
	}
}

// TestRequest_Queries_EmptyByDefault proves Queries() returns whatever the
// Responder reports for an unset query map (nil/empty), not a panic.
func TestRequest_Queries_EmptyByDefault(t *testing.T) {
	req, _ := New(newFakeResponder())

	if got := req.Queries(); len(got) != 0 {
		t.Fatalf("expected Queries() to be empty by default, got %v", got)
	}
}

// TestRequest_Route_NilByDefault proves a Request built without WithRoute
// has no attached route reference -- the zero value case exercised by any
// existing/future test that builds a Request directly and never needs the
// route-lookup path.
func TestRequest_Route_NilByDefault(t *testing.T) {
	req, _ := New(newFakeResponder())

	if got := req.Route(); got != nil {
		t.Fatalf("expected Route() to be nil by default, got %v", got)
	}
}

// TestRequest_WithRoute_IsChainableAndStoresRoute proves WithRoute attaches
// an opaque reference and returns req for chaining, and Route() returns
// exactly what was attached (round-trip, no transformation).
func TestRequest_WithRoute_IsChainableAndStoresRoute(t *testing.T) {
	req, _ := New(newFakeResponder())

	type fakeRoute struct{ name string }
	r := &fakeRoute{name: "user-route"}

	returned := req.WithRoute(r)

	if returned != req {
		t.Fatalf("expected WithRoute() to return the same *Request for chaining")
	}
	got, ok := req.Route().(*fakeRoute)
	if !ok {
		t.Fatalf("expected Route() to return the attached *fakeRoute, got %T", req.Route())
	}
	if got != r {
		t.Fatalf("expected Route() to return the exact attached pointer")
	}
}

// TestRequest_IsWebSocketUpgrade_DelegatesToResponder proves
// Request.IsWebSocketUpgrade is a one-line delegation to the underlying
// Responder.IsUpgradeRequest, both invoking it and returning its result
// unchanged.
func TestRequest_IsWebSocketUpgrade_DelegatesToResponder(t *testing.T) {
	fake := newFakeResponder()
	fake.isUpgradeRequest = true
	req, _ := New(fake)

	if got := req.IsWebSocketUpgrade(); got != true {
		t.Fatalf("expected IsWebSocketUpgrade() to return true, got %v", got)
	}
	if !fake.isUpgradeCalled {
		t.Fatalf("expected IsWebSocketUpgrade() to delegate to Responder.IsUpgradeRequest()")
	}
}

// TestResponse_UpgradeWebSocket_DelegatesToResponder proves
// Response.UpgradeWebSocket is a one-line delegation to the underlying
// Responder.Upgrade, passing the exact handler through unchanged.
func TestResponse_UpgradeWebSocket_DelegatesToResponder(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	var called bool
	handler := func(conn WSConn) { called = true }

	res.UpgradeWebSocket(handler)

	if !fake.upgradeCalled {
		t.Fatalf("expected UpgradeWebSocket() to delegate to Responder.Upgrade()")
	}
	if fake.upgradeHandler == nil {
		t.Fatalf("expected UpgradeWebSocket() to pass the handler through to Responder.Upgrade()")
	}
	fake.upgradeHandler(nil)
	if !called {
		t.Fatalf("expected the handler passed to Responder.Upgrade() to be the exact handler given to UpgradeWebSocket()")
	}
}

// TestResponse_UpgradeWebSocket_PassesSubprotocolsThrough proves
// Response.UpgradeWebSocket forwards its variadic subprotocols argument to
// Responder.Upgrade unchanged -- this is what lets a caller (internal/app's
// graphql GET dispatcher) request that the underlying HTTP engine negotiate
// and echo back a Sec-WebSocket-Protocol response header such as
// "graphql-transport-ws" during the handshake.
func TestResponse_UpgradeWebSocket_PassesSubprotocolsThrough(t *testing.T) {
	fake := newFakeResponder()
	_, res := New(fake)

	res.UpgradeWebSocket(func(conn WSConn) {}, "graphql-transport-ws", "graphql-ws")

	if got := fake.upgradeSubprotocols; len(got) != 2 || got[0] != "graphql-transport-ws" || got[1] != "graphql-ws" {
		t.Fatalf("expected UpgradeWebSocket() to pass subprotocols through to Responder.Upgrade(), got %v", got)
	}
}
