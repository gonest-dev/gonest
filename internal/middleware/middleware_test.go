package middleware

import (
	"bufio"
	"io"
	"testing"

	"gonest.dev/gonest/internal/execution"
)

// fakeResponder is a minimal test-only execution.Responder, mirroring the one
// in internal/route/route_test.go (execution.Responder is exported precisely
// so packages like this one can build their own fake -- see L-004 in
// STATE.md).
type fakeResponder struct {
	params map[string]string
}

func newFakeResponder() *fakeResponder {
	return &fakeResponder{params: map[string]string{}}
}

func (f *fakeResponder) JSON(v any) error                            { return nil }
func (f *fakeResponder) SetStatus(code int)                          {}
func (f *fakeResponder) GetStatus() int                              { return 200 }
func (f *fakeResponder) GetMethod() string                           { return "GET" }
func (f *fakeResponder) GetPath() string                             { return "" }
func (f *fakeResponder) GetHeader(name string) string                { return "" }
func (f *fakeResponder) SetHeaderValue(name, value string)           {}
func (f *fakeResponder) GetParam(name string) string                 { return f.params[name] }
func (f *fakeResponder) RawBody() []byte                             { return nil }
func (f *fakeResponder) Queries() map[string]string                  { return nil }
func (f *fakeResponder) HTML(s string) error                         { return nil }
func (f *fakeResponder) SendString(s string) error                   { return nil }
func (f *fakeResponder) WriteStream(fn func(w *bufio.Writer))        {}
func (f *fakeResponder) BodyStream() (io.Reader, string, bool)       { return nil, "", false }
func (f *fakeResponder) IsUpgradeRequest() bool                      { return false }
func (f *fakeResponder) Upgrade(handler func(conn execution.WSConn)) {}

// TestNew_DoesNotExecuteFnOnCall proves middleware.New(fn) defers fn until
// Declare(scope) runs it -- AD-008 reversed (see
// .specs/features/test-app-bootstrap/design.md): a *Middleware can now call
// MustInject/MustInjectAll inside fn, which requires fn to run during
// bootstrap's pipeline-stage-type phase (once scope is known), not at Go
// package-init time. Mirrors Provider/Controller/Module's own
// TestNew_DoesNotExecuteFnOnCall precedent.
func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	ran := false
	m := New(func(m *Middleware) {
		ran = true
	})

	if m == nil {
		t.Fatal("expected New to return a non-nil *Middleware")
	}
	if ran {
		t.Fatal("expected New(fn) to defer fn, not run it synchronously")
	}
}

// TestDeclare_ExecutesFn proves Declare runs the deferred fn, and
// TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls proves it does so exactly
// once even across multiple calls -- same idempotent contract Pipe.Declare
// established (L-012 in STATE.md), now shared by Middleware/Guard/
// Interceptor/Filter.
func TestDeclare_ExecutesFn(t *testing.T) {
	ran := false
	m := New(func(m *Middleware) {
		ran = true
	})

	m.Declare(nil)

	if !ran {
		t.Fatal("expected Declare to run the deferred fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	m := New(func(m *Middleware) {
		count++
	})

	m.Declare(nil)
	m.Declare(nil)

	if count != 1 {
		t.Fatalf("expected fn to run exactly once across repeated Declare calls, ran %d times", count)
	}
}

func TestDeclare_NilFn_DoesNotPanic(t *testing.T) {
	m := New(nil)
	m.Declare(nil)
}

// TestHandler_HandlerFunc_RoundTrip proves Handler stores the given function
// and HandlerFunc returns exactly that function, genuinely callable with
// both req/res and next parameters reaching the handler body correctly.
func TestHandler_HandlerFunc_RoundTrip(t *testing.T) {
	var gotReq *execution.Request
	var gotRes *execution.Response
	nextCalled := false

	m := New(func(m *Middleware) {
		m.Handler(func(req *execution.Request, res *execution.Response, next Next) {
			gotReq = req
			gotRes = res
			next(req, res)
		})
	})
	m.Declare(nil)

	fn := m.HandlerFunc()
	if fn == nil {
		t.Fatal("expected HandlerFunc to return the function stored via Handler, got nil")
	}

	req, res := execution.New(newFakeResponder())
	fn(req, res, func(req *execution.Request, res *execution.Response) {
		nextCalled = true
	})

	if gotReq != req {
		t.Fatal("expected req passed to the returned handler to reach the handler body unchanged")
	}
	if gotRes != res {
		t.Fatal("expected res passed to the returned handler to reach the handler body unchanged")
	}
	if !nextCalled {
		t.Fatal("expected next passed to the returned handler to be callable and reach the handler body")
	}
}

// TestHandlerFunc_NilWhenNeverCalled proves HandlerFunc returns nil when
// Handler was never called, mirroring pipe.Pipe.HandlerFunc()'s zero-value
// contract.
func TestHandlerFunc_NilWhenNeverCalled(t *testing.T) {
	m := New(func(m *Middleware) {})
	m.Declare(nil)

	if fn := m.HandlerFunc(); fn != nil {
		t.Fatal("expected HandlerFunc to return nil when Handler was never called")
	}
}

// TestNext_TypeIdentityWithRouteHandlerSignature proves a plain
// func(req *execution.Request, res *execution.Response) value is directly
// assignable to a Next variable with zero conversion code -- the
// type-identity design.md relies on for composing route Handlers as the
// innermost Next.
func TestNext_TypeIdentityWithRouteHandlerSignature(t *testing.T) {
	called := false
	someFunc := func(req *execution.Request, res *execution.Response) {
		called = true
	}

	var n Next = someFunc

	req, res := execution.New(newFakeResponder())
	n(req, res)

	if !called {
		t.Fatal("expected Next-typed variable assigned from a plain func(req *execution.Request, res *execution.Response) to be callable")
	}
}
