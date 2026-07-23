package resolver

import (
	"context"
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
)

// fakeResolvedProvider extends fakeProvider (resolver_test.go) with a
// resolved value, mirroring what *provider.Provider looks like once Stage 3
// has called SetResolvedValue on it -- direct.go's FindDirect/FindDirectAll
// only ever operate on providers past that point.
type fakeResolvedProvider struct {
	fakeProvider
	value any
}

func (p *fakeResolvedProvider) ResolvedValue() (reflect.Value, bool) {
	if p.value == nil {
		return reflect.Value{}, false
	}
	return reflect.ValueOf(p.value), true
}

func newResolved(t reflect.Type, value any) *fakeResolvedProvider {
	return &fakeResolvedProvider{fakeProvider: fakeProvider{resolved: t}, value: value}
}

type animalIface interface{ Talk() string }
type catAnimal struct{}

func (catAnimal) Talk() string { return "meow" }

type dogAnimal struct{}

func (dogAnimal) Talk() string { return "woof" }

func animalIfaceType() reflect.Type {
	return reflect.TypeOf((*animalIface)(nil)).Elem()
}

func TestFindDirect_Pointer_ExactMatch(t *testing.T) {
	p := newResolved(fooType(), &fooService{})
	m := module.New(func(m *module.Module) { m.Providers(p) })
	m.Assemble()

	v, ok := FindDirect([]*module.Module{m}, fooType())
	if !ok {
		t.Fatalf("FindDirect() ok=false, want true")
	}
	if v.Interface().(*fooService) != p.value {
		t.Fatalf("FindDirect() = %v, want %v", v.Interface(), p.value)
	}
}

func TestFindDirect_Pointer_NoMatch(t *testing.T) {
	m := module.New(func(m *module.Module) {})
	m.Assemble()

	_, ok := FindDirect([]*module.Module{m}, fooType())
	if ok {
		t.Fatalf("FindDirect() ok=true, want false (no providers registered)")
	}
}

func TestFindDirect_UnionScope_FindsProviderFromEitherModule(t *testing.T) {
	pA := newResolved(fooType(), &fooService{})
	pB := newResolved(barType(), &barService{})
	mA := module.New(func(m *module.Module) { m.Providers(pA) })
	mB := module.New(func(m *module.Module) { m.Providers(pB) })
	mA.Assemble()
	mB.Assemble()

	scope := []*module.Module{mA, mB}

	if _, ok := FindDirect(scope, fooType()); !ok {
		t.Fatalf("FindDirect() did not find provider from mA in union scope")
	}
	if _, ok := FindDirect(scope, barType()); !ok {
		t.Fatalf("FindDirect() did not find provider from mB in union scope")
	}
}

// TestFindDirect_Interface_SingleImplementor_Resolves proves interface
// resolution now goes exclusively through an explicit module.ProviderAs
// registration -- cat's concrete provider is registered AND wrapped via
// ProviderAs[animalIface], both passed to Providers (see
// module.ProviderAs's own doc comment: the wrapper never drives Stage 3
// construction, only the concrete registration does -- this test only
// exercises FindDirect directly, no Stage 3 involved, so registering both
// is enough to prove the exact-match path finds the wrapper).
func TestFindDirect_Interface_SingleImplementor_Resolves(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{})
	catAsAnimal := module.ProviderAs[animalIface](cat)
	m := module.New(func(m *module.Module) { m.Providers(cat, catAsAnimal) })
	m.Assemble()

	v, ok := FindDirect([]*module.Module{m}, animalIfaceType())
	if !ok {
		t.Fatalf("FindDirect() ok=false, want true")
	}
	if v.Interface().(animalIface).Talk() != "meow" {
		t.Fatalf("FindDirect() resolved wrong value")
	}
}

// TestFindDirect_Interface_TwoImplementors_AmbiguousNotFoundHere proves 2
// DIFFERENT concrete providers, each explicitly wrapped via
// ProviderAs[animalIface], are still ambiguous in one scope -- same
// ambiguity-via-exact-match-count contract as before, just reached via 2
// explicit registrations instead of 2 structural matches (PROVAS-06).
func TestFindDirect_Interface_TwoImplementors_AmbiguousNotFoundHere(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{})
	dog := newResolved(reflect.TypeOf(dogAnimal{}), dogAnimal{})
	catAsAnimal := module.ProviderAs[animalIface](cat)
	dogAsAnimal := module.ProviderAs[animalIface](dog)
	m := module.New(func(m *module.Module) { m.Providers(cat, dog, catAsAnimal, dogAsAnimal) })
	m.Assemble()

	_, ok := FindDirect([]*module.Module{m}, animalIfaceType())
	if ok {
		t.Fatalf("FindDirect() ok=true with 2 implementors, want false (ambiguity handled by caller via FindDirectAll)")
	}
}

// TestFindDirect_Interface_ExactMatchTakesPrecedenceOverImplements proves
// structural implementation ALONE (cat implements animalIface but is never
// wrapped via ProviderAs) is not enough to resolve -- only the explicitly
// wrapped provider (exact) is found, confirming there is no
// reflect.Type.Implements() fallback for a provider that merely happens to
// satisfy the interface (PROVAS-02).
func TestFindDirect_Interface_ExactMatchTakesPrecedenceOverImplements(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{}) // implements animalIface structurally, never wrapped
	exact := newResolved(reflect.TypeOf(dogAnimal{}), dogAnimal{})
	exactAsAnimal := module.ProviderAs[animalIface](exact)
	m := module.New(func(m *module.Module) { m.Providers(cat, exact, exactAsAnimal) })
	m.Assemble()

	matches := FindDirectAll([]*module.Module{m}, animalIfaceType())
	if len(matches) != 1 {
		t.Fatalf("FindDirectAll() = %d matches, want 1 (only the explicitly ProviderAs-wrapped provider should resolve)", len(matches))
	}
}

// TestFindDirectAll_Interface_ReturnsEveryImplementor proves every
// explicitly ProviderAs-wrapped provider in scope is returned (PROVAS-07's
// multi-binding contract).
func TestFindDirectAll_Interface_ReturnsEveryImplementor(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{})
	dog := newResolved(reflect.TypeOf(dogAnimal{}), dogAnimal{})
	catAsAnimal := module.ProviderAs[animalIface](cat)
	dogAsAnimal := module.ProviderAs[animalIface](dog)
	m := module.New(func(m *module.Module) { m.Providers(cat, dog, catAsAnimal, dogAsAnimal) })
	m.Assemble()

	matches := FindDirectAll([]*module.Module{m}, animalIfaceType())
	if len(matches) != 2 {
		t.Fatalf("FindDirectAll() = %d matches, want 2", len(matches))
	}
}

func TestFindDirectAll_Interface_ZeroImplementors_ReturnsEmpty(t *testing.T) {
	m := module.New(func(m *module.Module) {})
	m.Assemble()

	matches := FindDirectAll([]*module.Module{m}, animalIfaceType())
	if len(matches) != 0 {
		t.Fatalf("FindDirectAll() = %d matches, want 0", len(matches))
	}
}

func TestFindDirect_UnexportedImports_NotVisible(t *testing.T) {
	// a provider registered in an imported module but NOT exported must not
	// be found via union scope -- same encapsulation Find() already enforces
	// for single-owner resolution.
	unexported := newResolved(fooType(), &fooService{})
	child := module.New(func(m *module.Module) { m.Providers(unexported) })
	root := module.New(func(m *module.Module) { m.Imports(child) })
	child.Assemble()
	root.Assemble()

	if _, ok := FindDirect([]*module.Module{root}, fooType()); ok {
		t.Fatalf("FindDirect() found an unexported provider from an imported module, want not visible")
	}
}

// TestFindDirect_ReExportedModule_ResolvesTransitivelyThroughEffectiveExports
// is direct.go's counterpart to resolver_test.go's re-export test: C
// owns+exports a resolved provider, B imports C and re-exports it via
// Exports (contributing no providers of its own), and A imports ONLY
// B. candidateProviders now walks imported.EffectiveExports() instead of
// imported.ExportedProviders(), so FindDirect on a scope containing only A
// must still find C's provider.
func TestFindDirect_ReExportedModule_ResolvesTransitivelyThroughEffectiveExports(t *testing.T) {
	pc := newResolved(fooType(), &fooService{})
	c := module.New(func(m *module.Module) {
		m.Providers(pc)
		m.Exports(pc)
	})
	b := module.New(func(m *module.Module) {
		m.Imports(c)
		m.Exports(c)
	})
	a := module.New(func(m *module.Module) {
		m.Imports(b)
	})
	a.Assemble()

	v, ok := FindDirect([]*module.Module{a}, fooType())
	if !ok {
		t.Fatalf("FindDirect() ok=false, want true (C's provider reached transitively via B's re-export of C)")
	}
	if v.Interface().(*fooService) != pc.value {
		t.Fatalf("FindDirect() = %v, want %v", v.Interface(), pc.value)
	}
}

// TestFindDirectAll_ReExportedModule_ResolvesTransitivelyThroughEffectiveExports
// is the FindDirectAll/interface-kind variant of the above, since
// internal/inject's MustInject[T] for interface T goes through
// FindDirectAll specifically, a genuinely different call path than
// FindDirect worth covering separately.
func TestFindDirectAll_ReExportedModule_ResolvesTransitivelyThroughEffectiveExports(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{})
	catAsAnimal := module.ProviderAs[animalIface](cat)
	c := module.New(func(m *module.Module) {
		m.Providers(cat, catAsAnimal)
		m.Exports(catAsAnimal)
	})
	b := module.New(func(m *module.Module) {
		m.Imports(c)
		m.Exports(c)
	})
	a := module.New(func(m *module.Module) {
		m.Imports(b)
	})
	a.Assemble()

	matches := FindDirectAll([]*module.Module{a}, animalIfaceType())
	if len(matches) != 1 {
		t.Fatalf("FindDirectAll() = %d matches, want 1 (C's implementor reached transitively via B's re-export of C)", len(matches))
	}
	if matches[0].Interface().(animalIface).Talk() != "meow" {
		t.Fatalf("FindDirectAll() resolved wrong value")
	}
}

// TestFindDirect_ImportedNotReExported_StillInvisible is the regression
// counterpart: B imports C but never calls Exports(C), so C's
// exported provider must stay invisible to a scope built from A alone --
// re-export stays strictly opt-in for the direct-injection path too, not
// just Find's own path.
func TestFindDirect_ImportedNotReExported_StillInvisible(t *testing.T) {
	pc := newResolved(fooType(), &fooService{})
	c := module.New(func(m *module.Module) {
		m.Providers(pc)
		m.Exports(pc)
	})
	b := module.New(func(m *module.Module) {
		m.Imports(c) // no Exports(c): re-export not opted in
	})
	a := module.New(func(m *module.Module) {
		m.Imports(b)
	})
	a.Assemble()

	if _, ok := FindDirect([]*module.Module{a}, fooType()); ok {
		t.Fatalf("FindDirect() found C's provider though B never re-exported C, want not visible")
	}
}

func TestResolveWithOverrides_MatchingPointerOverride_SkipsRealConstructor(t *testing.T) {
	realConstructorRan := false
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *overrideFooService {
			realConstructorRan = true
			return &overrideFooService{}
		})
	})
	p.Declare()

	m := module.New(func(m *module.Module) { m.Providers(p) })
	modules, err := m.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	mock := &overrideFooService{Name: "mock"}
	overrides := map[reflect.Type]reflect.Value{
		reflect.TypeOf(&overrideFooService{}): reflect.ValueOf(mock),
	}

	if err := ResolveWithOverrides(context.Background(), modules, overrides); err != nil {
		t.Fatalf("ResolveWithOverrides() error = %v", err)
	}

	if realConstructorRan {
		t.Fatal("real Constructor ran for an overridden provider, want it to never run")
	}

	v, ok := p.ResolvedValue()
	if !ok {
		t.Fatal("ResolvedValue() ok=false after ResolveWithOverrides, want true")
	}
	if v.Interface().(*overrideFooService) != mock {
		t.Fatalf("ResolvedValue() = %v, want the override's mock value %v", v.Interface(), mock)
	}
}

func TestResolveWithOverrides_InterfaceOverride_SkipsRealConstructor(t *testing.T) {
	realConstructorRan := false
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *overrideFooService {
			realConstructorRan = true
			return &overrideFooService{}
		})
	})
	p.Declare()

	m := module.New(func(m *module.Module) { m.Providers(p) })
	modules, err := m.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	mock := &overrideFooServiceMock{}
	overrides := map[reflect.Type]reflect.Value{
		reflect.TypeOf((*overrideFooIface)(nil)).Elem(): reflect.ValueOf(mock),
	}

	if err := ResolveWithOverrides(context.Background(), modules, overrides); err != nil {
		t.Fatalf("ResolveWithOverrides() error = %v", err)
	}

	if realConstructorRan {
		t.Fatal("real Constructor ran for an overridden provider, want it to never run")
	}

	v, ok := p.ResolvedValue()
	if !ok {
		t.Fatal("ResolvedValue() ok=false after ResolveWithOverrides, want true")
	}
	if v.Interface().(*overrideFooServiceMock) != mock {
		t.Fatalf("ResolvedValue() = %v, want the override's mock value %v", v.Interface(), mock)
	}
}

func TestResolveWithOverrides_NoMatch_RunsRealConstructor(t *testing.T) {
	realConstructorRan := false
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *overrideFooService {
			realConstructorRan = true
			return &overrideFooService{}
		})
	})
	p.Declare()

	m := module.New(func(m *module.Module) { m.Providers(p) })
	modules, err := m.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	if err := ResolveWithOverrides(context.Background(), modules, nil); err != nil {
		t.Fatalf("ResolveWithOverrides() error = %v", err)
	}

	if !realConstructorRan {
		t.Fatal("real Constructor did not run when no override matched, want it to run")
	}
}

type overrideFooService struct{ Name string }
type overrideFooIface interface{ isOverrideFoo() }

func (*overrideFooService) isOverrideFoo() {}

type overrideFooServiceMock struct{}

func (*overrideFooServiceMock) isOverrideFoo() {}
