package filter_test

import (
	"reflect"
	"testing"

	"github.com/gonest-dev/gonest/internal/execution"
	"github.com/gonest-dev/gonest/internal/filter"
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

func (f *fakeResponder) JSON(v any) error                  { return nil }
func (f *fakeResponder) SetStatus(code int)                {}
func (f *fakeResponder) GetHeader(name string) string      { return "" }
func (f *fakeResponder) SetHeaderValue(name, value string) {}
func (f *fakeResponder) GetParam(name string) string       { return f.params[name] }
func (f *fakeResponder) Body() []byte                      { return nil }
func (f *fakeResponder) Queries() map[string]string        { return nil }

// fooExampleError and barExampleError are two distinct exemplar types used to
// prove Catch/HandlerFor dispatch on exact type identity, mirroring
// INSIGHT.md's dev-defined-exception example shape.
type fooExampleError struct {
	Code string
}

type barExampleError struct {
	Reason string
}

// TestNew_RunsFnImmediately proves filter.New(fn) runs fn synchronously at
// call time -- unlike Provider/Module/Controller/Pipe, which all defer fn
// until a later bootstrap stage. Filter.Catch registration has no dependency
// on the module tree being assembled first (no MustInject support this
// feature, see design.md's Tech Decisions: a *Filter can be attached to
// multiple controllers/modules with no clean single owner), so there is no
// further stage to usefully defer to -- same reasoning as
// middleware.New/guard.New/interceptor.New.
func TestNew_RunsFnImmediately(t *testing.T) {
	ran := false
	f := filter.New(func(f *filter.Filter) {
		ran = true
	})

	if f == nil {
		t.Fatal("expected New to return a non-nil *Filter")
	}
	if !ran {
		t.Fatal("expected fn passed to New to run immediately, not be deferred")
	}
}

// TestCatch_HandlerFor_RoundTrip proves a valid Catch registration is
// recoverable via HandlerFor using reflect.TypeOf(exemplar), ok=true.
func TestCatch_HandlerFor_RoundTrip(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(ctx *execution.Context, exc *fooExampleError) {})
	})

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
		f.Catch(&fooExampleError{}, func(ctx *execution.Context, exc *fooExampleError) {})
	})

	_, ok := f.HandlerFor(reflect.TypeOf(&barExampleError{}))
	if ok {
		t.Fatal("expected HandlerFor to report ok=false for a type never registered via Catch")
	}
}

// TestCatch_MultipleDistinctTypes proves a single Filter can Catch multiple
// distinct exemplar types, each independently recoverable via HandlerFor.
func TestCatch_MultipleDistinctTypes(t *testing.T) {
	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(ctx *execution.Context, exc *fooExampleError) {})
		f.Catch(&barExampleError{}, func(ctx *execution.Context, exc *barExampleError) {})
	})

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
// original handler body with both ctx and the typed exception value intact,
// not just "returned something non-nil".
func TestCatch_HandlerFor_GenuineCallRoundTrip(t *testing.T) {
	var gotCtx *execution.Context
	var gotExc *fooExampleError

	f := filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(ctx *execution.Context, exc *fooExampleError) {
			gotCtx = ctx
			gotExc = exc
		})
	})

	fn, ok := f.HandlerFor(reflect.TypeOf(&fooExampleError{}))
	if !ok {
		t.Fatal("expected HandlerFor to find the registered handler")
	}

	ctx := execution.New(newFakeResponder())
	exc := &fooExampleError{Code: "boom"}

	fn.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(exc)})

	if gotCtx != ctx {
		t.Fatal("expected ctx passed via reflect.Call to reach the handler body unchanged")
	}
	if gotExc != exc {
		t.Fatal("expected the typed exception value passed via reflect.Call to reach the handler body unchanged")
	}
	if gotExc.Code != "boom" {
		t.Fatalf("expected gotExc.Code to be %q, got %q", "boom", gotExc.Code)
	}
}

// TestCatch_PanicsOnWrongParamCount proves Catch panics at registration time
// (clear message) when handler doesn't take exactly (ctx, exc).
func TestCatch_PanicsOnWrongParamCount(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Catch handler signature (wrong param count), got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(ctx *execution.Context) {})
	})
}

// TestCatch_PanicsOnWrongFirstParamType proves Catch panics when the first
// parameter isn't *execution.Context.
func TestCatch_PanicsOnWrongFirstParamType(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Catch handler signature (wrong first param type), got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(exc *fooExampleError, ctx *execution.Context) {})
	})
}

// TestCatch_PanicsOnWrongSecondParamType proves Catch panics when the second
// parameter's type doesn't exactly match reflect.TypeOf(exemplar).
func TestCatch_PanicsOnWrongSecondParamType(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid Catch handler signature (wrong second param type), got none")
		}
	}()

	filter.New(func(f *filter.Filter) {
		f.Catch(&fooExampleError{}, func(ctx *execution.Context, exc *barExampleError) {})
	})
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
		f.Catch(&fooExampleError{}, func(ctx *execution.Context, exc *fooExampleError) error {
			return nil
		})
	})
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
	})
}
