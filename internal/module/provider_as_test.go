package module

import (
	"reflect"
	"testing"
)

// providerAsAnimal/providerAsCat mirror internal/resolver/direct_test.go's
// animalIface/catAnimal fixtures -- a minimal interface + implementor pair,
// used here to exercise ProviderAs's own reporting/panic contract in
// isolation, without needing internal/resolver's exact-match machinery.
type providerAsAnimal interface{ Talk() string }

type providerAsCat struct{}

func (providerAsCat) Talk() string { return "meow" }

// fakeResolvedProvider extends fakeProvider (module_test.go) with a
// resolved value, mirroring internal/resolver/direct_test.go's own
// fakeResolvedProvider -- needed here to prove ResolvedValue delegation.
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

func TestProviderAs_ReportsTAsResolvedType(t *testing.T) {
	inner := &fakeProvider{resolved: reflect.TypeOf(providerAsCat{})}

	view := ProviderAs[providerAsAnimal](inner)

	want := reflect.TypeOf((*providerAsAnimal)(nil)).Elem()
	if got := view.ResolvedType(); got != want {
		t.Fatalf("ResolvedType() = %v, want %v", got, want)
	}
}

func TestProviderAs_NonInterfaceT_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected ProviderAs[*providerAsCat] to panic (T is not an interface kind)")
		}
	}()

	inner := &fakeProvider{resolved: reflect.TypeOf(providerAsCat{})}
	ProviderAs[*providerAsCat](inner)
}

func TestProviderAs_ChainingAnotherProviderAsView_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected ProviderAs wrapping another ProviderAs view to panic")
		}
	}()

	inner := &fakeProvider{resolved: reflect.TypeOf(providerAsCat{})}
	view := ProviderAs[providerAsAnimal](inner)
	ProviderAs[providerAsAnimal](view)
}

func TestProviderAs_ResolvedValue_DelegatesToWrappedRef(t *testing.T) {
	cat := providerAsCat{}
	inner := &fakeResolvedProvider{
		fakeProvider: fakeProvider{resolved: reflect.TypeOf(cat)},
		value:        cat,
	}

	view := ProviderAs[providerAsAnimal](inner)

	rv, ok := view.(hasResolvedValue)
	if !ok {
		t.Fatal("providerAsRef does not expose ResolvedValue")
	}
	v, ok := rv.ResolvedValue()
	if !ok {
		t.Fatal("ResolvedValue() ok=false, want true (wrapped ref has a value)")
	}
	if v.Interface().(providerAsAnimal).Talk() != "meow" {
		t.Fatalf("ResolvedValue() = %v, want wrapped ref's value", v.Interface())
	}
}

func TestProviderAs_ResolvedValue_NoDelegate_ReturnsFalse(t *testing.T) {
	inner := &fakeProvider{resolved: reflect.TypeOf(providerAsCat{})}
	view := ProviderAs[providerAsAnimal](inner)

	rv, ok := view.(hasResolvedValue)
	if !ok {
		t.Fatal("providerAsRef does not expose ResolvedValue")
	}
	if _, ok := rv.ResolvedValue(); ok {
		t.Fatal("ResolvedValue() ok=true, want false (wrapped ref has no ResolvedValue method)")
	}
}

func TestProviderAs_SetOwnerModule_OwnStorage(t *testing.T) {
	inner := &fakeProvider{resolved: reflect.TypeOf(providerAsCat{})}
	view := ProviderAs[providerAsAnimal](inner)

	m := &Module{}
	view.SetOwnerModule(m)

	owner, ok := view.(Owner)
	if !ok {
		t.Fatal("providerAsRef does not implement module.Owner")
	}
	if got := owner.OwnerModule(); got != m {
		t.Fatalf("OwnerModule() = %v, want %v", got, m)
	}
	if inner.ownerModule != nil {
		t.Fatal("SetOwnerModule on the view mutated the wrapped ref's own ownerModule, want independent storage")
	}
}
