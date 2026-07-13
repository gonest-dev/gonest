package gonest

import (
	"testing"

	"github.com/gonest-dev/gonest/internal/httpctx"
	"github.com/gonest-dev/gonest/internal/pipe"
	"github.com/gonest-dev/gonest/internal/route"
)

// paramFakeResponder is a minimal test-only httpctx.Responder for exercising
// MustParam[T] end to end (Context -> Route -> Pipe/defaultCoerce).
type paramFakeResponder struct {
	params map[string]string
}

func newParamFakeResponder() *paramFakeResponder {
	return &paramFakeResponder{params: map[string]string{}}
}

func (f *paramFakeResponder) JSON(v any) error                  { return nil }
func (f *paramFakeResponder) SetStatus(code int)                {}
func (f *paramFakeResponder) GetHeader(name string) string      { return "" }
func (f *paramFakeResponder) SetHeaderValue(name, value string) {}
func (f *paramFakeResponder) GetParam(name string) string       { return f.params[name] }

// TestMustParam_WithoutCustomPipe_UsesDefaultCoerce proves that when a
// Route has no custom Pipe registered for a param name, MustParam[T] falls
// back to the default reflect+strconv coercion (T4's defaultCoerce).
func TestMustParam_WithoutCustomPipe_UsesDefaultCoerce(t *testing.T) {
	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {})

	res := newParamFakeResponder()
	res.params["id"] = "42"
	ctx := httpctx.New(res).WithRoute(r)

	got := MustParam[int](ctx, "id")
	if got != 42 {
		t.Fatalf("MustParam[int](ctx, \"id\") = %d, want %d", got, 42)
	}
}

// TestMustParam_WithCustomPipe_UsesCustomPipeInsteadOfDefault proves that
// when the current Route has a custom Pipe registered for a param name (via
// Route.Param), MustParam[T] runs that Pipe's Handler instead of
// defaultCoerce.
func TestMustParam_WithCustomPipe_UsesCustomPipeInsteadOfDefault(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *httpctx.Context, raw string) int {
			// Deliberately does NOT match defaultCoerce's behavior (would
			// return 42 for raw "42") -- proves the custom Pipe ran, not
			// the default coercion.
			return 999
		})
	})
	p.Declare()

	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {
		r.Param("id", p)
	})

	res := newParamFakeResponder()
	res.params["id"] = "42"
	ctx := httpctx.New(res).WithRoute(r)

	got := MustParam[int](ctx, "id")
	if got != 999 {
		t.Fatalf("MustParam[int](ctx, \"id\") = %d, want %d (from custom Pipe)", got, 999)
	}
}

// TestMustParam_PanicsWhenParamNotDeclaredOnRoute proves MustParam[T] panics
// with the distinct "no param named" message when the current Route's
// declared path doesn't have a ":name" segment for the requested name --
// this is the existence check that resolves T4's documented string
// ambiguity (ctx.Param returning "" cannot alone distinguish "absent" from
// "present but empty" for T=string).
func TestMustParam_PanicsWhenParamNotDeclaredOnRoute(t *testing.T) {
	r := route.New(route.HttpGet, "/users", func(r *route.Route) {})

	res := newParamFakeResponder()
	ctx := httpctx.New(res).WithRoute(r)

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected MustParam to panic for a param not declared on the route, got no panic")
		}
		msg, ok := rec.(string)
		if !ok {
			t.Fatalf("expected panic value to be a string, got %T: %v", rec, rec)
		}
		want := `gonest: no param named "id" on this route`
		if msg != want {
			t.Fatalf("panic message = %q, want %q", msg, want)
		}
	}()

	MustParam[int](ctx, "id")
}

// TestMustParam_PanicsOnConversionFailure_DefaultCoerce proves MustParam[T]
// panics with the distinct "could not be converted" message when the raw
// value exists but fails to convert to T via defaultCoerce.
func TestMustParam_PanicsOnConversionFailure_DefaultCoerce(t *testing.T) {
	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {})

	res := newParamFakeResponder()
	res.params["id"] = "not-a-number"
	ctx := httpctx.New(res).WithRoute(r)

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected MustParam to panic on conversion failure, got no panic")
		}
		msg, ok := rec.(string)
		if !ok {
			t.Fatalf("expected panic value to be a string, got %T: %v", rec, rec)
		}
		if !stringsContains(msg, `gonest: param "id" could not be converted to int`) {
			t.Fatalf("panic message = %q, want prefix %q", msg, `gonest: param "id" could not be converted to int`)
		}
	}()

	MustParam[int](ctx, "id")
}

// TestMustParam_PanicsWhenCustomPipeHandlerPanics proves that if the custom
// Pipe's Handler itself panics, MustParam lets that panic propagate as-is
// (pass-through, not caught/rewrapped) -- expected behavior per the task
// spec, not something MustParam should swallow.
func TestMustParam_PanicsWhenCustomPipeHandlerPanics(t *testing.T) {
	p := pipe.New(func(p *pipe.Pipe) {
		p.Handler(func(ctx *httpctx.Context, raw string) int {
			panic("custom pipe exploded")
		})
	})
	p.Declare()

	r := route.New(route.HttpGet, "/users/:id", func(r *route.Route) {
		r.Param("id", p)
	})

	res := newParamFakeResponder()
	res.params["id"] = "42"
	ctx := httpctx.New(res).WithRoute(r)

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected the custom Pipe's panic to propagate, got no panic")
		}
		msg, ok := rec.(string)
		if !ok || msg != "custom pipe exploded" {
			t.Fatalf("expected panic value %q to pass through unchanged, got %v", "custom pipe exploded", rec)
		}
	}()

	MustParam[int](ctx, "id")
}

// TestMustParam_WithoutAttachedRoute_UsesDefaultCoerce proves MustParam[T]
// works even when Context has no attached Route (Route() returns nil) --
// falls back straight to defaultCoerce, no existence check possible without
// a Route to consult, mirrors defaultCoerce's own pre-T5 behavior for a
// non-string T where an empty raw fails to parse.
func TestMustParam_WithoutAttachedRoute_UsesDefaultCoerce(t *testing.T) {
	res := newParamFakeResponder()
	res.params["id"] = "7"
	ctx := httpctx.New(res)

	got := MustParam[int](ctx, "id")
	if got != 7 {
		t.Fatalf("MustParam[int](ctx, \"id\") = %d, want %d", got, 7)
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
