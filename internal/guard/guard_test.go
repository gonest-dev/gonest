package guard

import (
	"testing"

	"github.com/gonest-dev/gonest/internal/execution"
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

func (f *fakeResponder) JSON(v any) error                  { return nil }
func (f *fakeResponder) SetStatus(code int)                {}
func (f *fakeResponder) GetHeader(name string) string      { return "" }
func (f *fakeResponder) SetHeaderValue(name, value string) {}
func (f *fakeResponder) GetParam(name string) string       { return f.params[name] }
func (f *fakeResponder) Body() []byte                      { return nil }
func (f *fakeResponder) Queries() map[string]string        { return nil }

// TestNew_RunsFnImmediately proves guard.New(fn) runs fn synchronously at
// call time -- unlike Provider/Module/Controller/Pipe, which all defer fn
// until a later bootstrap stage. Guard.Handler registration has no
// dependency on the module tree being assembled first (no MustInject
// support this feature, see design.md's Tech Decisions), so there is no
// further stage to usefully defer to (same reasoning as middleware.New).
func TestNew_RunsFnImmediately(t *testing.T) {
	ran := false
	g := New(func(g *Guard) {
		ran = true
	})

	if g == nil {
		t.Fatal("expected New to return a non-nil *Guard")
	}
	if !ran {
		t.Fatal("expected fn passed to New to run immediately, not be deferred")
	}
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

	if fn := g.HandlerFunc(); fn != nil {
		t.Fatal("expected HandlerFunc to return nil when Handler was never called")
	}
}
