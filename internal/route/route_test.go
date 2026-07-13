package route

import (
	"testing"

	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/pipe"
)

// fakeResponder is a minimal test-only httpctx.Responder, mirroring the one
// in internal/httpctx/context_test.go (that one is unexported to its own
// package, so route's tests need their own).
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

// TestNew_RunsFnImmediately proves route.New(method, path, fn) runs fn
// synchronously at call time -- unlike Provider/Module/Controller/Pipe,
// which all defer fn until a later bootstrap stage (see design.md's
// rationale: by the time Controller.Route(...) invokes route.New, it is
// already executing inside the Controller's own already-deferred fn during
// Stage 2, so there is no further stage left to defer to).
func TestNew_RunsFnImmediately(t *testing.T) {
	ran := false
	r := New(HttpGet, "/users", func(r *Route) {
		ran = true
	})

	if r == nil {
		t.Fatal("expected New to return a non-nil *Route")
	}
	if !ran {
		t.Fatal("expected fn passed to New to run immediately, not be deferred")
	}
}

// TestHttpCode_StoresStatus proves HttpCode stores the default status code.
func TestHttpCode_StoresStatus(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {
		r.HttpCode(201)
	})

	if got := r.Code(); got != 201 {
		t.Fatalf("expected HttpCode(201) to store 201, got %d", got)
	}
}

// TestHttpCode_DefaultsTo200 proves a Route that never calls HttpCode
// defaults to 200 (per design.md's Data Models comment: "default 200,
// sobrescrito por HttpCode()").
func TestHttpCode_DefaultsTo200(t *testing.T) {
	r := New(HttpGet, "/users", func(r *Route) {})

	if got := r.Code(); got != 200 {
		t.Fatalf("expected default HttpCode to be 200, got %d", got)
	}
}

// TestHandler_StoresFn proves Handler stores the handler fn, retrievable
// (and callable) later.
func TestHandler_StoresFn(t *testing.T) {
	called := false
	r := New(HttpGet, "/users", func(r *Route) {
		r.Handler(func(ctx *httpctx.Context) {
			called = true
		})
	})

	h := r.HandlerFunc()
	if h == nil {
		t.Fatal("expected HandlerFunc() to return the stored handler, got nil")
	}
	h(httpctx.New(newFakeResponder()))
	if !called {
		t.Fatal("expected stored handler to be callable and run")
	}
}

// TestParam_RegistersCustomPipe proves Route.Param(name, pipe) registers a
// custom Pipe for that param name, retrievable via PipeFor.
func TestParam_RegistersCustomPipe(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *httpctx.Context, raw string) int {
			return 99
		})
	})
	p.Declare()

	r := New(HttpGet, "/users/:id", func(r *Route) {
		r.Param("id", p)
	})

	got, ok := r.PipeFor("id")
	if !ok {
		t.Fatal("expected PipeFor(\"id\") to report ok=true after Param registered a custom Pipe")
	}
	if got != p {
		t.Fatal("expected PipeFor(\"id\") to return the exact *pipe.Pipe registered via Param")
	}
}

// TestParam_UnregisteredNameReportsNotOk proves PipeFor reports ok=false for
// a param name that never had a custom Pipe registered via Route.Param.
func TestParam_UnregisteredNameReportsNotOk(t *testing.T) {
	r := New(HttpGet, "/users/:id", func(r *Route) {})

	_, ok := r.PipeFor("id")
	if ok {
		t.Fatal("expected PipeFor(\"id\") to report ok=false when no custom Pipe was registered")
	}
}

// TestHasParam_TrueForDeclaredPathSegment proves HasParam checks the
// Route's own declared path pattern for a ":name" segment -- this is the
// existence-check mechanism MustParam[T] (root param.go) relies on to
// distinguish "genuinely absent from this route" from "present but empty
// string" (the T4-documented ambiguity in defaultCoerce for T=string).
func TestHasParam_TrueForDeclaredPathSegment(t *testing.T) {
	r := New(HttpGet, "/users/:id/orders/:orderId", func(r *Route) {})

	if !r.HasParam("id") {
		t.Fatal("expected HasParam(\"id\") to be true, path declares :id")
	}
	if !r.HasParam("orderId") {
		t.Fatal("expected HasParam(\"orderId\") to be true, path declares :orderId")
	}
}

// TestHasParam_FalseForUndeclaredName proves HasParam is false for a name
// that isn't a ":name" segment in the Route's declared path.
func TestHasParam_FalseForUndeclaredName(t *testing.T) {
	r := New(HttpGet, "/users/:id", func(r *Route) {})

	if r.HasParam("nonexistent") {
		t.Fatal("expected HasParam(\"nonexistent\") to be false, path does not declare it")
	}
}

// TestMethod_ReturnsConstructedMethod proves Method returns the HttpMethod
// passed to New -- needed by Stage 2.5 (app bootstrap route collection) to
// build a method+path collision key without reaching into Route's
// unexported fields.
func TestMethod_ReturnsConstructedMethod(t *testing.T) {
	r := New(HttpPost, "/users", func(r *Route) {})

	if got := r.Method(); got != HttpPost {
		t.Fatalf("Method() = %v, want %v", got, HttpPost)
	}
}

// TestPath_ReturnsConstructedPath proves Path returns the path string passed
// to New -- needed by Stage 2.5 to build the full route path (controller
// PathPrefix + route Path) for both collision detection and adapter
// registration.
func TestPath_ReturnsConstructedPath(t *testing.T) {
	r := New(HttpGet, "/users/:id", func(r *Route) {})

	if got := r.Path(); got != "/users/:id" {
		t.Fatalf("Path() = %q, want %q", got, "/users/:id")
	}
}
