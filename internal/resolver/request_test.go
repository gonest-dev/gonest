package resolver

import (
	"context"
	"testing"
)

// --- fixtures -----------------------------------------------------------

type requestScopedThing struct{ ID int }

// --- interface/type existence -------------------------------------------

// TestNewRequestScope_ReturnsUsableCache proves a RequestScope value can be
// constructed and satisfies the Get/Set contract this package's future
// Pipeline consumer (Milestone 3, not part of this feature) will rely on --
// see request.go's package-level doc comment for why no real HTTP wiring
// exists yet.
func TestNewRequestScope_ReturnsUsableCache(t *testing.T) {
	rs := NewRequestScope()
	if rs == nil {
		t.Fatal("NewRequestScope() returned nil")
	}

	ctx := context.Background()
	if _, ok := rs.Get(ctx, "some-key"); ok {
		t.Fatal("Get() on empty RequestScope should report ok=false")
	}
}

// --- same request context -> same instance -------------------------------

// TestRequestScope_SameContext_ReturnsSameInstance proves that two
// "resolutions" (Get calls) within the same mocked request context, after a
// single Set, observe the identical instance -- the core Request-scope
// guarantee (shared instance within one request).
func TestRequestScope_SameContext_ReturnsSameInstance(t *testing.T) {
	rs := NewRequestScope()
	ctx := WithRequestID(context.Background(), "req-1")

	want := &requestScopedThing{ID: 42}
	rs.Set(ctx, "thing", want)

	got1, ok1 := rs.Get(ctx, "thing")
	if !ok1 {
		t.Fatal("Get() ok = false, want true")
	}
	got2, ok2 := rs.Get(ctx, "thing")
	if !ok2 {
		t.Fatal("Get() ok = false, want true")
	}

	p1, ok := got1.(*requestScopedThing)
	if !ok {
		t.Fatalf("got1 type = %T, want *requestScopedThing", got1)
	}
	p2, ok := got2.(*requestScopedThing)
	if !ok {
		t.Fatalf("got2 type = %T, want *requestScopedThing", got2)
	}

	if p1 != want || p2 != want {
		t.Fatalf("Get() pointers = %p, %p, want both == %p (same instance, same context)", p1, p2, want)
	}
}

// --- different request contexts -> different instances -------------------

// TestRequestScope_DifferentContexts_ReturnDifferentInstances proves that
// the same provider key resolved (Set then Get) in two DIFFERENT mocked
// request contexts observes two independent instances -- no leakage across
// requests.
func TestRequestScope_DifferentContexts_ReturnDifferentInstances(t *testing.T) {
	rs := NewRequestScope()
	ctxA := WithRequestID(context.Background(), "req-A")
	ctxB := WithRequestID(context.Background(), "req-B")

	instA := &requestScopedThing{ID: 1}
	instB := &requestScopedThing{ID: 2}

	rs.Set(ctxA, "thing", instA)
	rs.Set(ctxB, "thing", instB)

	gotA, okA := rs.Get(ctxA, "thing")
	if !okA {
		t.Fatal("Get(ctxA) ok = false, want true")
	}
	gotB, okB := rs.Get(ctxB, "thing")
	if !okB {
		t.Fatal("Get(ctxB) ok = false, want true")
	}

	if gotA == gotB {
		t.Fatalf("Get(ctxA) and Get(ctxB) returned the same value %v, want different instances across request contexts", gotA)
	}
	if gotA.(*requestScopedThing) != instA {
		t.Fatalf("Get(ctxA) = %v, want %v", gotA, instA)
	}
	if gotB.(*requestScopedThing) != instB {
		t.Fatalf("Get(ctxB) = %v, want %v", gotB, instB)
	}
}

// --- missing request ID on context ---------------------------------------

// TestRequestScope_Get_NoRequestIDOnContext_ReportsNotFound proves that a
// ctx never tagged via WithRequestID is treated as "no request context",
// not as a crash or a shared/global bucket -- Get simply reports not found.
func TestRequestScope_Get_NoRequestIDOnContext_ReportsNotFound(t *testing.T) {
	rs := NewRequestScope()

	if _, ok := rs.Get(context.Background(), "thing"); ok {
		t.Fatal("Get() with no request ID on ctx should report ok=false")
	}
}
