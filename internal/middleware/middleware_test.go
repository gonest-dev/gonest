package middleware

import (
	"testing"

	"github.com/gonest-dev/gonest/internal/execution"
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

func (f *fakeResponder) JSON(v any) error                  { return nil }
func (f *fakeResponder) SetStatus(code int)                {}
func (f *fakeResponder) GetHeader(name string) string      { return "" }
func (f *fakeResponder) SetHeaderValue(name, value string) {}
func (f *fakeResponder) GetParam(name string) string       { return f.params[name] }
func (f *fakeResponder) Body() []byte                      { return nil }
func (f *fakeResponder) Queries() map[string]string        { return nil }
func (f *fakeResponder) HTML(s string) error               { return nil }

// TestNew_RunsFnImmediately proves middleware.New(fn) runs fn synchronously
// at call time -- unlike Provider/Module/Controller/Pipe, which all defer fn
// until a later bootstrap stage. Middleware.Handler registration has no
// dependency on the module tree being assembled first, so there is no
// further stage to usefully defer to (same reasoning as route.New).
func TestNew_RunsFnImmediately(t *testing.T) {
	ran := false
	m := New(func(m *Middleware) {
		ran = true
	})

	if m == nil {
		t.Fatal("expected New to return a non-nil *Middleware")
	}
	if !ran {
		t.Fatal("expected fn passed to New to run immediately, not be deferred")
	}
}

// TestHandler_HandlerFunc_RoundTrip proves Handler stores the given function
// and HandlerFunc returns exactly that function, genuinely callable with
// both ctx and next parameters reaching the handler body correctly.
func TestHandler_HandlerFunc_RoundTrip(t *testing.T) {
	var gotCtx *execution.Context
	nextCalled := false

	m := New(func(m *Middleware) {
		m.Handler(func(ctx *execution.Context, next Next) {
			gotCtx = ctx
			next(ctx)
		})
	})

	fn := m.HandlerFunc()
	if fn == nil {
		t.Fatal("expected HandlerFunc to return the function stored via Handler, got nil")
	}

	ctx := execution.New(newFakeResponder())
	fn(ctx, func(ctx *execution.Context) {
		nextCalled = true
	})

	if gotCtx != ctx {
		t.Fatal("expected ctx passed to the returned handler to reach the handler body unchanged")
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

	if fn := m.HandlerFunc(); fn != nil {
		t.Fatal("expected HandlerFunc to return nil when Handler was never called")
	}
}

// TestNext_TypeIdentityWithRouteHandlerSignature proves a plain
// func(ctx *execution.Context) value is directly assignable to a Next
// variable with zero conversion code -- the type-identity design.md relies
// on for composing route Handlers as the innermost Next.
func TestNext_TypeIdentityWithRouteHandlerSignature(t *testing.T) {
	called := false
	someFunc := func(ctx *execution.Context) {
		called = true
	}

	var n Next = someFunc

	n(execution.New(newFakeResponder()))

	if !called {
		t.Fatal("expected Next-typed variable assigned from a plain func(ctx *execution.Context) to be callable")
	}
}
