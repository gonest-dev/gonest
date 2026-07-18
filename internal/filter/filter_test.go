package filter_test

import (
	"bufio"
	"io"
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/filter"
)

// fakeResponder is a minimal test-only execution.Responder, mirroring the one
// in internal/guard/guard_test.go and internal/middleware/middleware_test.go
// (execution.Responder is exported precisely so packages like this one can
// build their own fake -- see L-004 in STATE.md).
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

// fooExampleError and barExampleError are two distinct exemplar types used to
// prove Catch/HandlerFor dispatch on exact type identity, mirroring
// INSIGHT.md's dev-defined-exception example shape.
type fooExampleError struct {
	Code string
}

type barExampleError struct {
	Reason string
}

// TestNew_DoesNotExecuteFnOnCall proves filter.New(fn) defers fn until
// Declare(scope) runs it -- AD-008 reversed (see
// .specs/features/test-app-bootstrap/design.md): a *Filter can now call
// MustInject/MustInjectAll inside fn, which requires fn to run during
// bootstrap's pipeline-stage-type phase (once scope is known), not at Go
// package-init time.
func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	ran := false
	f := filter.New(func(f *filter.Filter) {
		ran = true
	})

	if f == nil {
		t.Fatal("expected New to return a non-nil *Filter")
	}
	if ran {
		t.Fatal("expected New(fn) to defer fn, not run it synchronously")
	}
}

func TestDeclare_ExecutesFn(t *testing.T) {
	ran := false
	f := filter.New(func(f *filter.Filter) {
		ran = true
	})

	f.Declare(nil)

	if !ran {
		t.Fatal("expected Declare to run the deferred fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	f := filter.New(func(f *filter.Filter) {
		count++
	})

	f.Declare(nil)
	f.Declare(nil)

	if count != 1 {
		t.Fatalf("expected fn to run exactly once across repeated Declare calls, ran %d times", count)
	}
}

func TestDeclare_NilFn_DoesNotPanic(t *testing.T) {
	f := filter.New(nil)
	f.Declare(nil)
}

// TestCatch_HandlerFor_RoundTrip proves a valid Catch registration is
// recoverable via HandlerFor using reflect.TypeOf(exemplar), ok=true.
func TestCatch_HandlerFor_RoundTrip(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(req *execution.Request, res *execution.Response, exc *fooExampleError) {})
	})
	f.Declare(nil)

	fn, ok := f.HandlerFor(reflect.TypeOf(&fooExampleError{}))
	if !ok {
		t.Fatal("expected HandlerFor to report ok=true for a type registered via Catch")
	}
	if !fn.IsValid() {
		t.Fatal("expected HandlerFor to return a valid reflect.Value for a registered type")
	}
}

// TestHandlerFor_MissReturnsFalse proves HandlerFor reports ok=false for a
// type that was never registered via Catch on this Filter.
func TestHandlerFor_MissReturnsFalse(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(req *execution.Request, res *execution.Response, exc *fooExampleError) {})
	})
	f.Declare(nil)

	_, ok := f.HandlerFor(reflect.TypeOf(&barExampleError{}))
	if ok {
		t.Fatal("expected HandlerFor to report ok=false for a type never registered via Catch")
	}
}

// TestCatch_MultipleDistinctTypes proves a single Filter can Catch multiple
// distinct exemplar types, each independently recoverable via HandlerFor.
func TestCatch_MultipleDistinctTypes(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(req *execution.Request, res *execution.Response, exc *fooExampleError) {})
		f.Catch(&barExampleError{}, func(req *execution.Request, res *execution.Response, exc *barExampleError) {})
	})
	f.Declare(nil)

	fooFn, fooOK := f.HandlerFor(reflect.TypeOf(&fooExampleError{}))
	if !fooOK || !fooFn.IsValid() {
		t.Fatal("expected HandlerFor to find the fooExampleError handler")
	}

	barFn, barOK := f.HandlerFor(reflect.TypeOf(&barExampleError{}))
	if !barOK || !barFn.IsValid() {
		t.Fatal("expected HandlerFor to find the barExampleError handler")
	}
}

// TestCatch_HandlerFor_GenuineCallRoundTrip proves the handler recovered via
// HandlerFor is genuinely callable via reflect.Call -- the call reaches the
// original handler body with req, res, and the typed exception value intact,
// not just "returned something non-nil".
func TestCatch_HandlerFor_GenuineCallRoundTrip(t *testing.T) {
	var gotReq *execution.Request
	var gotRes *execution.Response
	var gotExc *fooExampleError

	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(req *execution.Request, res *execution.Response, exc *fooExampleError) {
			gotReq = req
			gotRes = res
			gotExc = exc
		})
	})
	f.Declare(nil)

	fn, ok := f.HandlerFor(reflect.TypeOf(&fooExampleError{}))
	if !ok {
		t.Fatal("expected HandlerFor to find the registered handler")
	}

	req, res := execution.New(newFakeResponder())
	exc := &fooExampleError{Code: "boom"}

	fn.Call([]reflect.Value{reflect.ValueOf(req), reflect.ValueOf(res), reflect.ValueOf(exc)})

	if gotReq != req {
		t.Fatal("expected req passed via reflect.Call to reach the handler body unchanged")
	}
	if gotRes != res {
		t.Fatal("expected res passed via reflect.Call to reach the handler body unchanged")
	}
	if gotExc != exc {
		t.Fatal("expected the typed exception value passed via reflect.Call to reach the handler body unchanged")
	}
	if gotExc.Code != "boom" {
		t.Fatalf("expected gotExc.Code to be %q, got %q", "boom", gotExc.Code)
	}
}

// TestCatch_PanicsOnWrongParamCount proves Catch panics at registration time
// (clear message) when handler doesn't take exactly (req, res, exc).
func TestCatch_PanicsOnWrongParamCount(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Catch handler signature (wrong param count), got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(req *execution.Request) {})
	}).Declare(nil)
}

// TestCatch_PanicsOnWrongFirstParamType proves Catch panics when the first
// parameter isn't *execution.Request.
func TestCatch_PanicsOnWrongFirstParamType(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Catch handler signature (wrong first param type), got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(exc *fooExampleError, req *execution.Request, res *execution.Response) {})
	}).Declare(nil)
}

// TestCatch_PanicsOnWrongSecondParamType proves Catch panics when the second
// parameter isn't *execution.Response.
func TestCatch_PanicsOnWrongSecondParamType(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Catch handler signature (wrong second param type), got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(req *execution.Request, exc *fooExampleError, res *execution.Response) {})
	}).Declare(nil)
}

// TestCatch_PanicsOnWrongThirdParamType proves Catch panics when the third
// parameter's type doesn't exactly match reflect.TypeOf(exemplar).
func TestCatch_PanicsOnWrongThirdParamType(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Catch handler signature (wrong third param type), got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(req *execution.Request, res *execution.Response, exc *barExampleError) {})
	}).Declare(nil)
}

// TestCatch_PanicsOnWrongReturnCount proves Catch panics when handler returns
// anything at all -- unlike Pipe.Handler (which returns T), a Filter's Catch
// handler must return nothing.
func TestCatch_PanicsOnWrongReturnCount(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Catch handler signature (non-zero return count), got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(req *execution.Request, res *execution.Response, exc *fooExampleError) error {
			return nil
		})
	}).Declare(nil)
}

// TestCatch_PanicsOnNonFunc proves Catch panics when handler isn't a func at
// all.
func TestCatch_PanicsOnNonFunc(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for non-func Catch handler argument, got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, 42)
	}).Declare(nil)
}
