package inject

import (
	"reflect"
	"strings"
	"testing"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/scope"
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

// pingable/fakePinger are placeholder target/implementation types for
// MustInjectAll[T] Provider-owner tests (findAllRefs/mustAllProvider/
// MustAll's new dispatch branch) -- same "content does not matter, only
// identity/type" role fakeService/otherService play for MustInject[T]
// above.
type pingable interface {
	Ping() string
}

type fakePinger struct {
	name string
}

func (f *fakePinger) Ping() string { return f.name }

// newDeclaredProvider returns a *provider.Provider with ctor already
// registered via Constructor and Declare()'d immediately, so
// ResolvedType() reflects ctor's return type right away -- without this,
// ResolvedType() returns nil (see Provider.ResolvedType's own doc comment:
// "returns nil if Constructor has not been called yet"), which would make
// every findAllRefs/mustAllProvider match comparison against it fail
// silently instead of matching.
func newDeclaredProvider(ctor any) *provider.Provider {
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(ctor)
	})
	p.Declare()
	return p
}

// pendingAllEdgesFor returns the pending multi-valor edges recorded for
// owner, in registration order. Test-only helper, mirrors pendingEdgesFor
// above.
func pendingAllEdgesFor(owner module.Owner) []PendingAllEdge {
	pendingAllEdgesMu.Lock()
	defer pendingAllEdgesMu.Unlock()

	var out []PendingAllEdge
	for _, e := range pendingAllEdges {
		if e.Owner == owner {
			out = append(out, e)
		}
	}
	return out
}

func TestMustInject_ReturnsNonNilPlaceholderForPointerType(t *testing.T) {
	owner := newOwner()

	got := Must[*fakeService](owner)

	if got == nil {
		t.Fatalf("MustInject[*fakeService] returned nil, want a non-nil allocated placeholder")
	}
}

func TestMustInject_PlaceholderIsUsable(t *testing.T) {
	owner := newOwner()

	got := Must[*fakeService](owner)

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

	Must[fakeService](owner)
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

	Must[otherService](owner)
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

	Must[int](owner)
}

func TestMustInject_RegistersExactlyOnePendingEdge(t *testing.T) {
	resetPendingEdges()
	owner := newOwner()

	Must[*fakeService](owner)

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

	Must[*fakeService](owner)

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

	got := Must[*fakeService](owner)

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

	Must[*fakeService](owner)
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

	Must[*fakeService](ownerA)
	Must[*otherService](ownerA)
	Must[*fakeService](ownerB)

	edgesA := pendingEdgesFor(ownerA)
	edgesB := pendingEdgesFor(ownerB)

	if len(edgesA) != 2 {
		t.Fatalf("pending edges for ownerA = %d, want 2", len(edgesA))
	}
	if len(edgesB) != 1 {
		t.Fatalf("pending edges for ownerB = %d, want 1", len(edgesB))
	}
}

// ---------------------------------------------------------------------
// T1 -- PendingAllEdge bookkeeping
// ---------------------------------------------------------------------

func TestPendingAllEdges_ReturnsCopyNotAliasingInternalState(t *testing.T) {
	resetPendingEdges()
	owner := newOwner()
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	slice := reflect.MakeSlice(reflect.SliceOf(pingerType), 1, 1)

	pendingAllEdgesMu.Lock()
	pendingAllEdges = append(pendingAllEdges, PendingAllEdge{Owner: owner, InterfaceType: pingerType, Slice: slice})
	pendingAllEdgesMu.Unlock()

	got := PendingAllEdges()
	if len(got) != 1 {
		t.Fatalf("PendingAllEdges() len = %d, want 1", len(got))
	}
	if got[0].Owner != owner {
		t.Fatalf("PendingAllEdges()[0].Owner = %v, want %v", got[0].Owner, owner)
	}

	// Mutating the returned slice must not affect internal bookkeeping.
	got[0].Owner = nil
	again := PendingAllEdges()
	if again[0].Owner != owner {
		t.Fatalf("PendingAllEdges() second call Owner = %v, want %v (mutation of first result leaked into internal state)", again[0].Owner, owner)
	}
}

func TestReset_ClearsPendingAllEdges(t *testing.T) {
	resetPendingEdges()
	owner := newOwner()
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	slice := reflect.MakeSlice(reflect.SliceOf(pingerType), 0, 0)

	pendingAllEdgesMu.Lock()
	pendingAllEdges = append(pendingAllEdges, PendingAllEdge{Owner: owner, InterfaceType: pingerType, Slice: slice})
	pendingAllEdgesMu.Unlock()

	if got := len(PendingAllEdges()); got == 0 {
		t.Fatalf("PendingAllEdges() len = 0 before Reset(), want > 0 (setup didn't record an edge)")
	}

	Reset()

	if got := len(PendingAllEdges()); got != 0 {
		t.Fatalf("PendingAllEdges() len = %d after Reset(), want 0", got)
	}
}

// ---------------------------------------------------------------------
// T2 -- findAllRefs structural search
// ---------------------------------------------------------------------

func TestFindAllRefs_OwnModuleMatch(t *testing.T) {
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	p := newDeclaredProvider(func() pingable { return &fakePinger{name: "own"} })

	m := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	got := findAllRefs(m, pingerType)
	if len(got) != 1 {
		t.Fatalf("findAllRefs() len = %d, want 1", len(got))
	}
	if got[0] != module.ProviderRef(p) {
		t.Fatalf("findAllRefs()[0] = %v, want %v", got[0], p)
	}
}

func TestFindAllRefs_ImportExportedMatch(t *testing.T) {
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	p := newDeclaredProvider(func() pingable { return &fakePinger{name: "imported"} })

	imported := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Exports(p)
	})
	owner := module.New(func(m *module.Module) {
		m.Imports(imported)
	})
	if _, err := owner.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	got := findAllRefs(owner, pingerType)
	if len(got) != 1 {
		t.Fatalf("findAllRefs() len = %d, want 1", len(got))
	}
	if got[0] != module.ProviderRef(p) {
		t.Fatalf("findAllRefs()[0] = %v, want %v", got[0], p)
	}
}

func TestFindAllRefs_UnexportedImportProvider_NotMatched(t *testing.T) {
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	p := newDeclaredProvider(func() pingable { return &fakePinger{name: "hidden"} })

	imported := module.New(func(m *module.Module) {
		m.Providers(p) // registered, but never Exported
	})
	owner := module.New(func(m *module.Module) {
		m.Imports(imported)
	})
	if _, err := owner.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	got := findAllRefs(owner, pingerType)
	if len(got) != 0 {
		t.Fatalf("findAllRefs() len = %d, want 0 (unexported import provider must not match)", len(got))
	}
}

func TestFindAllRefs_DiamondImport_Dedups(t *testing.T) {
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	p := newDeclaredProvider(func() pingable { return &fakePinger{name: "shared"} })

	shared := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Exports(p)
	})
	a := module.New(func(m *module.Module) {
		m.Imports(shared)
		m.Exports(shared)
	})
	b := module.New(func(m *module.Module) {
		m.Imports(shared)
		m.Exports(shared)
	})
	owner := module.New(func(m *module.Module) {
		m.Imports(a, b)
	})
	if _, err := owner.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	got := findAllRefs(owner, pingerType)
	if len(got) != 1 {
		t.Fatalf("findAllRefs() len = %d, want 1 (diamond-visible provider must be deduped)", len(got))
	}
	if got[0] != module.ProviderRef(p) {
		t.Fatalf("findAllRefs()[0] = %v, want %v", got[0], p)
	}
}

func TestFindAllRefs_ProviderAsRef_UnwrappedToConcreteRef(t *testing.T) {
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	concrete := newDeclaredProvider(func() *fakePinger { return &fakePinger{name: "wrapped"} })
	view := module.ProviderAs[pingable](concrete)

	m := module.New(func(m *module.Module) {
		m.Providers(view)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	got := findAllRefs(m, pingerType)
	if len(got) != 1 {
		t.Fatalf("findAllRefs() len = %d, want 1", len(got))
	}
	if got[0] != module.ProviderRef(concrete) {
		t.Fatalf("findAllRefs()[0] = %v, want the unwrapped concrete ref %v (got the providerAsRef view instead)", got[0], concrete)
	}
}

// ---------------------------------------------------------------------
// T3 -- mustAllProvider[T]
// ---------------------------------------------------------------------

func TestMustAllProvider_ZeroMatches_ReturnsEmptyNonNilSlice_NoPanic(t *testing.T) {
	resetPendingEdges()
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	owner := provider.New(func(p *provider.Provider) {})

	m := module.New(func(m *module.Module) {
		m.Providers(owner)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	got := mustAllProvider[pingable](owner, pingerType)
	if got == nil {
		t.Fatalf("mustAllProvider() returned nil, want a non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("mustAllProvider() len = %d, want 0", len(got))
	}
}

func TestMustAllProvider_SingletonMatches_ReturnsSliceAndRecordsEdge(t *testing.T) {
	resetPendingEdges()
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	owner := provider.New(func(p *provider.Provider) {})
	p1 := newDeclaredProvider(func() pingable { return &fakePinger{name: "a"} })
	p2 := newDeclaredProvider(func() pingable { return &fakePinger{name: "b"} })

	m := module.New(func(m *module.Module) {
		m.Providers(owner, p1, p2)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	got := mustAllProvider[pingable](owner, pingerType)
	if len(got) != 2 {
		t.Fatalf("mustAllProvider() len = %d, want 2", len(got))
	}

	edges := pendingAllEdgesFor(owner)
	if len(edges) != 1 {
		t.Fatalf("pending all edges for owner = %d, want 1", len(edges))
	}
	if edges[0].InterfaceType != pingerType {
		t.Fatalf("edge.InterfaceType = %v, want %v", edges[0].InterfaceType, pingerType)
	}
	if len(edges[0].Matches) != 2 {
		t.Fatalf("edge.Matches len = %d, want 2", len(edges[0].Matches))
	}
}

func TestMustAllProvider_TransientMatch_PanicsBeforeRecordingEdge(t *testing.T) {
	resetPendingEdges()
	pingerType := reflect.TypeOf((*pingable)(nil)).Elem()
	owner := provider.New(func(p *provider.Provider) {})
	transientP := provider.New(func(p *provider.Provider) {
		p.Scope(scope.Transient)
		p.Constructor(func() pingable { return &fakePinger{name: "transient"} })
	})
	transientP.Declare()

	m := module.New(func(m *module.Module) {
		m.Providers(owner, transientP)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("mustAllProvider() did not panic for a Transient match")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want string", r, r)
		}
		if !strings.Contains(msg, "only ScopeSingleton providers can be members of a Provider-owned MustInjectAll slice") {
			t.Fatalf("panic message = %q, missing expected substring", msg)
		}
		if got := len(pendingAllEdgesFor(owner)); got != 0 {
			t.Fatalf("pending all edges for owner = %d after panic, want 0 (edge recorded despite fail-fast)", got)
		}
	}()

	mustAllProvider[pingable](owner, pingerType)
}
