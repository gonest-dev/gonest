package resolve

import (
	"strings"
	"testing"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/provider"
)

// fakeService/otherService are placeholder targets for MustResolve[T] in
// tests -- their content does not matter, only their pointer identity.
type fakeService struct {
	Name string
}

type otherService struct{}

func newOwner() module.Owner {
	return provider.New(func(p *provider.Provider) {})
}

func TestMustResolve_ReturnsNonNilPlaceholderForPointerType(t *testing.T) {
	owner := newOwner()

	got := MustResolve[*fakeService](owner)

	if got == nil {
		t.Fatalf("MustResolve[*fakeService] returned nil, want a non-nil allocated placeholder")
	}
}

func TestMustResolve_PlaceholderIsUsable(t *testing.T) {
	owner := newOwner()

	got := MustResolve[*fakeService](owner)

	// Confirm it's a genuinely usable *fakeService, not just a non-nil
	// interface{} smuggled through -- write to a field and read it back.
	got.Name = "hello"
	if got.Name != "hello" {
		t.Fatalf("placeholder field write/read failed, got.Name = %q", got.Name)
	}
}

func TestMustResolve_NonPointerType_PanicsWithDynamicTypeName(t *testing.T) {
	owner := newOwner()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustResolve[fakeService] (non-pointer) did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want string", r, r)
		}
		want := "gonest: MustResolve[T] requires T to be a pointer type, got resolve.fakeService"
		if msg != want {
			t.Fatalf("panic message = %q, want %q", msg, want)
		}
	}()

	MustResolve[fakeService](owner)
}

func TestMustResolve_NonPointerType_MessageIsDynamicNotHardcoded(t *testing.T) {
	owner := newOwner()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustResolve[otherService] (non-pointer) did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want string", r, r)
		}
		if strings.Contains(msg, "fakeService") {
			t.Fatalf("panic message = %q, mentions fakeService for a call with otherService -- message is hardcoded, not dynamic", msg)
		}
		want := "gonest: MustResolve[T] requires T to be a pointer type, got resolve.otherService"
		if msg != want {
			t.Fatalf("panic message = %q, want %q", msg, want)
		}
	}()

	MustResolve[otherService](owner)
}

func TestMustResolve_NonPointerBuiltinType_PanicsWithDynamicTypeName(t *testing.T) {
	owner := newOwner()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustResolve[int] (non-pointer) did not panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want string", r, r)
		}
		want := "gonest: MustResolve[T] requires T to be a pointer type, got int"
		if msg != want {
			t.Fatalf("panic message = %q, want %q", msg, want)
		}
	}()

	MustResolve[int](owner)
}

func TestMustResolve_RegistersExactlyOnePendingEdge(t *testing.T) {
	resetPendingEdges()
	owner := newOwner()

	MustResolve[*fakeService](owner)

	edges := pendingEdgesFor(owner)
	if len(edges) != 1 {
		t.Fatalf("pending edges for owner = %d, want 1", len(edges))
	}
	wantType := "*resolve.fakeService"
	if got := edges[0].targetType.String(); got != wantType {
		t.Fatalf("pending edge targetType = %q, want %q", got, wantType)
	}
}

func TestMustResolve_MultipleCallsDoNotCollide(t *testing.T) {
	resetPendingEdges()
	ownerA := newOwner()
	ownerB := newOwner()

	MustResolve[*fakeService](ownerA)
	MustResolve[*otherService](ownerA)
	MustResolve[*fakeService](ownerB)

	edgesA := pendingEdgesFor(ownerA)
	edgesB := pendingEdgesFor(ownerB)

	if len(edgesA) != 2 {
		t.Fatalf("pending edges for ownerA = %d, want 2", len(edgesA))
	}
	if len(edgesB) != 1 {
		t.Fatalf("pending edges for ownerB = %d, want 1", len(edgesB))
	}
}
