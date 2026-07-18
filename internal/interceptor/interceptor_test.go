package interceptor

import (
	"bufio"
	"io"
	"testing"

	"gonest.dev/gonest/internal/execution"
)

// fakeResponder is a minimal test-only execution.Responder, mirroring the one
// in internal/middleware/middleware_test.go (execution.Responder is exported
// precisely so packages like this one can build their own fake -- see L-004
// in STATE.md).
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
func (f *fakeResponder) RawBody() []byte                       { return nil }
func (f *fakeResponder) Queries() map[string]string            { return nil }
func (f *fakeResponder) HTML(s string) error                   { return nil }
func (f *fakeResponder) SendString(s string) error             { return nil }
func (f *fakeResponder) WriteStream(fn func(w *bufio.Writer)) {}
func (f *fakeResponder) BodyStream() (io.Reader, string, bool) { return nil, "", false }

// TestNew_DoesNotExecuteFnOnCall proves interceptor.New(fn) defers fn until
// Declare(scope) runs it -- AD-008 reversed (see
// .specs/features/test-app-bootstrap/design.md): an *Interceptor can now
// call MustInject/MustInjectAll inside fn, which requires fn to run during
// bootstrap's pipeline-stage-type phase (once scope is known), not at Go
// package-init time.
func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	ran := false
	i := New(func(i *Interceptor) {
		ran = true
	})

	if i == nil {
		t.Fatal("expected New to return a non-nil *Interceptor")
	}
	if ran {
		t.Fatal("expected New(fn) to defer fn, not run it synchronously")
	}
}

func TestDeclare_ExecutesFn(t *testing.T) {
	ran := false
	i := New(func(i *Interceptor) {
		ran = true
	})

	i.Declare(nil)

	if !ran {
		t.Fatal("expected Declare to run the deferred fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	i := New(func(i *Interceptor) {
		count++
	})

	i.Declare(nil)
	i.Declare(nil)

	if count != 1 {
		t.Fatalf("expected fn to run exactly once across repeated Declare calls, ran %d times", count)
	}
}

func TestDeclare_NilFn_DoesNotPanic(t *testing.T) {
	i := New(nil)
	i.Declare(nil)
}

// TestHandler_HandlerFunc_RoundTrip proves Handler stores the given function
// and HandlerFunc returns exactly that function, genuinely callable with
// req, res, and next parameters reaching the handler body correctly.
func TestHandler_HandlerFunc_RoundTrip(t *testing.T) {
	var gotReq *execution.Request
	var gotRes *execution.Response
	nextCalled := false

	i := New(func(i *Interceptor) {
		i.Handler(func(req *execution.Request, res *execution.Response, next Next) {
			gotReq = req
			gotRes = res
			next(req, res)
		})
	})
	i.Declare(nil)

	fn := i.HandlerFunc()
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
// Handler was never called, mirroring middleware.Middleware.HandlerFunc()'s
// zero-value contract.
func TestHandlerFunc_NilWhenNeverCalled(t *testing.T) {
	i := New(func(i *Interceptor) {})
	i.Declare(nil)

	if fn := i.HandlerFunc(); fn != nil {
		t.Fatal("expected HandlerFunc to return nil when Handler was never called")
	}
}

// TestNext_TypeIdentityWithRouteHandlerSignature proves a plain
// func(req *execution.Request, res *execution.Response) value is directly
// assignable to a Next variable with zero conversion code -- the
// type-identity design.md relies on for composing interceptedHandler out of
// gatedHandler with no adapter code (same proof internal/middleware's own T1
// already made for its own Next).
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
