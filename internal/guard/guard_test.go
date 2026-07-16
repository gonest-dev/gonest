package guard

import (
	"io"
	"testing"

	"gonest.dev/gonest/internal/execution"
)

// fakeResponder is a minimal test-only execution.Responder, mirroring the one
// in internal/middleware/middleware_test.go (execution.Responder is exported
// precisely so packages like this one can build their own fake -- see
// L-004 in STATE.md).
type fakeResponder struct {
	params map[string]string
}

func newFakeResponder() *fakeResponder {
	return &fakeResponder{params: map[string]string{}}
}

func (f *fakeResponder) JSON(v any) error                      { return nil }
func (f *fakeResponder) SetStatus(code int)                    {}
func (f *fakeResponder) GetStatus() int                        { return 200 }
func (f *fakeResponder) GetMethod() string                     { return "GET" }
func (f *fakeResponder) GetPath() string                       { return "" }
func (f *fakeResponder) GetHeader(name string) string          { return "" }
func (f *fakeResponder) SetHeaderValue(name, value string)     {}
func (f *fakeResponder) GetParam(name string) string           { return f.params[name] }
func (f *fakeResponder) Body() []byte                          { return nil }
func (f *fakeResponder) Queries() map[string]string            { return nil }
func (f *fakeResponder) HTML(s string) error                   { return nil }
func (f *fakeResponder) SendString(s string) error             { return nil }
func (f *fakeResponder) BodyStream() (io.Reader, string, bool) { return nil, "", false }

// TestNew_DoesNotExecuteFnOnCall proves guard.New(fn) defers fn until
// Declare(scope) runs it -- AD-008 reversed (see
// .specs/features/test-app-bootstrap/design.md): a *Guard can now call
// MustInject/MustInjectAll inside fn, which requires fn to run during
// bootstrap's pipeline-stage-type phase (once scope is known), not at Go
// package-init time.
func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	ran := false
	g := New(func(g *Guard) {
		ran = true
	})

	if g == nil {
		t.Fatal("expected New to return a non-nil *Guard")
	}
	if ran {
		t.Fatal("expected New(fn) to defer fn, not run it synchronously")
	}
}

func TestDeclare_ExecutesFn(t *testing.T) {
	ran := false
	g := New(func(g *Guard) {
		ran = true
	})

	g.Declare(nil)

	if !ran {
		t.Fatal("expected Declare to run the deferred fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	g := New(func(g *Guard) {
		count++
	})

	g.Declare(nil)
	g.Declare(nil)

	if count != 1 {
		t.Fatalf("expected fn to run exactly once across repeated Declare calls, ran %d times", count)
	}
}

func TestDeclare_NilFn_DoesNotPanic(t *testing.T) {
	g := New(nil)
	g.Declare(nil)
}

// TestHandler_HandlerFunc_RoundTrip_True proves Handler stores the given
// function and HandlerFunc returns exactly that function, genuinely callable
// with ctx reaching the handler body and the handler's own `true` decision
// coming back out unchanged.
func TestHandler_HandlerFunc_RoundTrip_True(t *testing.T) {
	var gotCtx *execution.Context

	g := New(func(g *Guard) {
		g.Handler(func(ctx *execution.Context) bool {
			gotCtx = ctx
			return true
		})
	})
	g.Declare(nil)

	fn := g.HandlerFunc()
	if fn == nil {
		t.Fatal("expected HandlerFunc to return the function stored via Handler, got nil")
	}

	ctx := execution.New(newFakeResponder())
	got := fn(ctx)

	if gotCtx != ctx {
		t.Fatal("expected ctx passed to the returned handler to reach the handler body unchanged")
	}
	if got != true {
		t.Fatal("expected the returned bool to genuinely reflect the handler's own true decision")
	}
}

// TestHandler_HandlerFunc_RoundTrip_False proves the same round-trip as
// above but for a handler whose own logic decides false, confirming the
// returned bool is not hardcoded/always-true.
func TestHandler_HandlerFunc_RoundTrip_False(t *testing.T) {
	var gotCtx *execution.Context

	g := New(func(g *Guard) {
		g.Handler(func(ctx *execution.Context) bool {
			gotCtx = ctx
			return false
		})
	})
	g.Declare(nil)

	fn := g.HandlerFunc()
	if fn == nil {
		t.Fatal("expected HandlerFunc to return the function stored via Handler, got nil")
	}

	ctx := execution.New(newFakeResponder())
	got := fn(ctx)

	if gotCtx != ctx {
		t.Fatal("expected ctx passed to the returned handler to reach the handler body unchanged")
	}
	if got != false {
		t.Fatal("expected the returned bool to genuinely reflect the handler's own false decision")
	}
}

// TestHandlerFunc_NilWhenNeverCalled proves HandlerFunc returns nil when
// Handler was never called, mirroring middleware.Middleware.HandlerFunc()'s
// zero-value contract.
func TestHandlerFunc_NilWhenNeverCalled(t *testing.T) {
	g := New(func(g *Guard) {})
	g.Declare(nil)

	if fn := g.HandlerFunc(); fn != nil {
		t.Fatal("expected HandlerFunc to return nil when Handler was never called")
	}
}
