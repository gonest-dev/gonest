package resolver

import (
	"context"
	"reflect"
	"testing"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/provider"
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

func TestFindDirect_Interface_SingleImplementor_Resolves(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{})
	m := module.New(func(m *module.Module) { m.Providers(cat) })
	m.Assemble()

	v, ok := FindDirect([]*module.Module{m}, animalIfaceType())
	if !ok {
		t.Fatalf("FindDirect() ok=false, want true")
	}
	if v.Interface().(animalIface).Talk() != "meow" {
		t.Fatalf("FindDirect() resolved wrong value")
	}
}

func TestFindDirect_Interface_TwoImplementors_AmbiguousNotFoundHere(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{})
	dog := newResolved(reflect.TypeOf(dogAnimal{}), dogAnimal{})
	m := module.New(func(m *module.Module) { m.Providers(cat, dog) })
	m.Assemble()

	_, ok := FindDirect([]*module.Module{m}, animalIfaceType())
	if ok {
		t.Fatalf("FindDirect() ok=true with 2 implementors, want false (ambiguity handled by caller via FindDirectAll)")
	}
}

func TestFindDirect_Interface_ExactMatchTakesPrecedenceOverImplements(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{})
	exactIface := newResolved(animalIfaceType(), catAnimal{})
	m := module.New(func(m *module.Module) { m.Providers(cat, exactIface) })
	m.Assemble()

	matches := FindDirectAll([]*module.Module{m}, animalIfaceType())
	if len(matches) != 1 {
		t.Fatalf("FindDirectAll() = %d matches, want 1 (exact match should suppress Implements() matches)", len(matches))
	}
}

func TestFindDirectAll_Interface_ReturnsEveryImplementor(t *testing.T) {
	cat := newResolved(reflect.TypeOf(catAnimal{}), catAnimal{})
	dog := newResolved(reflect.TypeOf(dogAnimal{}), dogAnimal{})
	m := module.New(func(m *module.Module) { m.Providers(cat, dog) })
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
