package inject

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/provider"
)

// fakeService/otherService are placeholder targets for MustInject[T] in
// tests -- their content does not matter, only their pointer identity.
type fakeService struct {
	Name string
}

type otherService struct{}

func newOwner() module.Owner {
	return provider.New(func(p *provider.Provider) {})
}

func TestMustInject_ReturnsNonNilPlaceholderForPointerType(t *testing.T) {
	owner := newOwner()

	got := MustInject[*fakeService](owner)

	if got == nil {
		t.Fatalf("MustInject[*fakeService] returned nil, want a non-nil allocated placeholder")
	}
}

func TestMustInject_PlaceholderIsUsable(t *testing.T) {
	owner := newOwner()

	got := MustInject[*fakeService](owner)

	// Confirm it's a genuinely usable *fakeService, not just a non-nil
	// interface{} smuggled through -- write to a field and read it back.
	got.Name = "hello"
	if got.Name != "hello" {
		t.Fatalf("placeholder field write/read failed, got.Name = %q", got.Name)
	}
}

func TestMustInject_NonPointerType_PanicsWithDynamicTypeName(t *testing.T) {
	owner := newOwner()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustInject[fakeService] (non-pointer) did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want string", r, r)
		}
		want := "gonest: MustInject[T] requires T to be a pointer type, got inject.fakeService"
		if msg != want {
			t.Fatalf("panic message = %q, want %q", msg, want)
		}
	}()

	MustInject[fakeService](owner)
}

func TestMustInject_NonPointerType_MessageIsDynamicNotHardcoded(t *testing.T) {
	owner := newOwner()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustInject[otherService] (non-pointer) did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want string", r, r)
		}
		if strings.Contains(msg, "fakeService") {
			t.Fatalf("panic message = %q, mentions fakeService for a call with otherService -- message is hardcoded, not dynamic", msg)
		}
		want := "gonest: MustInject[T] requires T to be a pointer type, got inject.otherService"
		if msg != want {
			t.Fatalf("panic message = %q, want %q", msg, want)
		}
	}()

	MustInject[otherService](owner)
}

func TestMustInject_NonPointerBuiltinType_PanicsWithDynamicTypeName(t *testing.T) {
	owner := newOwner()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustInject[int] (non-pointer) did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want string", r, r)
		}
		want := "gonest: MustInject[T] requires T to be a pointer type, got int"
		if msg != want {
			t.Fatalf("panic message = %q, want %q", msg, want)
		}
	}()

	MustInject[int](owner)
}

func TestMustInject_RegistersExactlyOnePendingEdge(t *testing.T) {
	resetPendingEdges()
	owner := newOwner()

	MustInject[*fakeService](owner)

	edges := pendingEdgesFor(owner)
	if len(edges) != 1 {
		t.Fatalf("pending edges for owner = %d, want 1", len(edges))
	}
	wantType := "*inject.fakeService"
	if got := edges[0].TargetType.String(); got != wantType {
		t.Fatalf("pending edge targetType = %q, want %q", got, wantType)
	}
}

func TestPendingEdges_ReturnsCopyNotAliasingInternalState(t *testing.T) {
	resetPendingEdges()
	owner := newOwner()

	MustInject[*fakeService](owner)

	got := PendingEdges()
	if len(got) != 1 {
		t.Fatalf("PendingEdges() len = %d, want 1", len(got))
	}
	if got[0].Owner != owner {
		t.Fatalf("PendingEdges()[0].Owner = %v, want %v", got[0].Owner, owner)
	}
	wantType := "*inject.fakeService"
	if gotType := got[0].TargetType.String(); gotType != wantType {
		t.Fatalf("PendingEdges()[0].TargetType = %q, want %q", gotType, wantType)
	}

	// Mutating the returned slice must not affect internal bookkeeping.
	got[0].Owner = nil
	again := PendingEdges()
	if again[0].Owner != owner {
		t.Fatalf("PendingEdges() second call Owner = %v, want %v (mutation of first result leaked into internal state)", again[0].Owner, owner)
	}
}

func TestPendingEdge_RetainsPlaceholderUsableForCopyInPlace(t *testing.T) {
	resetPendingEdges()
	owner := newOwner()

	got := MustInject[*fakeService](owner)

	edges := pendingEdgesFor(owner)
	if len(edges) != 1 {
		t.Fatalf("pending edges for owner = %d, want 1", len(edges))
	}
	if !edges[0].Placeholder.IsValid() {
		t.Fatalf("PendingEdge.Placeholder is zero reflect.Value, want the allocated placeholder")
	}

	// Simulate Stage 3's copy-in-place: write through the retained
	// reflect.Value and confirm it's the SAME underlying allocation the
	// caller already holds a pointer to (got), not a separate copy.
	real := &fakeService{Name: "resolved"}
	edges[0].Placeholder.Elem().Set(reflect.ValueOf(real).Elem())

	if got.Name != "resolved" {
		t.Fatalf("got.Name = %q after copy-in-place via retained Placeholder, want %q -- Placeholder does not alias the value MustInject returned", got.Name, "resolved")
	}
}

func TestReset_ClearsAllPendingEdges(t *testing.T) {
	resetPendingEdges()
	owner := newOwner()

	MustInject[*fakeService](owner)
	if got := len(PendingEdges()); got == 0 {
		t.Fatalf("PendingEdges() len = 0 before Reset(), want > 0 (setup didn't record an edge)")
	}

	Reset()

	if got := len(PendingEdges()); got != 0 {
		t.Fatalf("PendingEdges() len = %d after Reset(), want 0", got)
	}
}

func TestMustInject_MultipleCallsDoNotCollide(t *testing.T) {
	resetPendingEdges()
	ownerA := newOwner()
	ownerB := newOwner()

	MustInject[*fakeService](ownerA)
	MustInject[*otherService](ownerA)
	MustInject[*fakeService](ownerB)

	edgesA := pendingEdgesFor(ownerA)
	edgesB := pendingEdgesFor(ownerB)

	if len(edgesA) != 2 {
		t.Fatalf("pending edges for ownerA = %d, want 2", len(edgesA))
	}
	if len(edgesB) != 1 {
		t.Fatalf("pending edges for ownerB = %d, want 1", len(edgesB))
	}
}
