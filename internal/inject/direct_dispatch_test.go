package inject

import (
	"reflect"
	"strings"
	"testing"

	"gonest.dev/gonest/internal/module"
)

// fakeDirectOwner is a minimal module.Owner + directResolver test double,
// standing in for what *controller.Controller/*middleware.Middleware/
// *guard.Guard/*interceptor.Interceptor/*filter.Filter all look like once
// their scope is known -- direct.go's real FindDirect/FindDirectAll are
// exercised by internal/resolver's own tests; this package only needs to
// prove MustInject/MustInjectAll dispatch correctly to whatever a
// directResolver-satisfying owner returns.
type fakeDirectOwner struct {
	single      reflect.Value
	singleOK    bool
	all         []reflect.Value
	ownerModule *module.Module
}

func (f *fakeDirectOwner) OwnerModule() *module.Module { return f.ownerModule }

func (f *fakeDirectOwner) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return f.single, f.singleOK
}

func (f *fakeDirectOwner) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return f.all
}

type directIface interface{ Name() string }
type directImpl struct{ name string }

func (d directImpl) Name() string { return d.name }

func TestMustInject_DirectResolver_Pointer_Found(t *testing.T) {
	real := &fakeService{Name: "real"}
	owner := &fakeDirectOwner{single: reflect.ValueOf(real), singleOK: true}

	got := MustInject[*fakeService](owner)
	if got != real {
		t.Fatalf("MustInject() = %v, want %v", got, real)
	}
}

func TestMustInject_DirectResolver_Pointer_NotFound_Panics(t *testing.T) {
	owner := &fakeDirectOwner{singleOK: false}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustInject() did not panic, want panic (no provider found)")
		}
		if !strings.Contains(r.(string), "no provider registered for type") {
			t.Fatalf("panic message = %q, missing expected substring", r)
		}
	}()
	MustInject[*fakeService](owner)
}

func TestMustInject_DirectResolver_Interface_SingleMatch_Resolves(t *testing.T) {
	owner := &fakeDirectOwner{all: []reflect.Value{reflect.ValueOf(directImpl{name: "x"})}}

	got := MustInject[directIface](owner)
	if got.Name() != "x" {
		t.Fatalf("MustInject() = %v, want Name()==x", got)
	}
}

func TestMustInject_DirectResolver_Interface_ZeroMatches_Panics(t *testing.T) {
	owner := &fakeDirectOwner{all: nil}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustInject() did not panic, want panic (zero matches)")
		}
		if !strings.Contains(r.(string), "no provider implements interface") {
			t.Fatalf("panic message = %q, missing expected substring", r)
		}
	}()
	MustInject[directIface](owner)
}

func TestMustInject_DirectResolver_Interface_TwoMatches_PanicsAmbiguous(t *testing.T) {
	owner := &fakeDirectOwner{all: []reflect.Value{
		reflect.ValueOf(directImpl{name: "a"}),
		reflect.ValueOf(directImpl{name: "b"}),
	}}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustInject() did not panic, want panic (ambiguous)")
		}
		msg := r.(string)
		if !strings.Contains(msg, "ambiguous") || !strings.Contains(msg, "MustInjectAll") {
			t.Fatalf("panic message = %q, missing expected substrings", msg)
		}
	}()
	MustInject[directIface](owner)
}

func TestMustInjectAll_DirectResolver_ReturnsAllMatches(t *testing.T) {
	owner := &fakeDirectOwner{all: []reflect.Value{
		reflect.ValueOf(directImpl{name: "a"}),
		reflect.ValueOf(directImpl{name: "b"}),
	}}

	got := MustInjectAll[directIface](owner)
	if len(got) != 2 {
		t.Fatalf("MustInjectAll() len = %d, want 2", len(got))
	}
	if got[0].Name() != "a" || got[1].Name() != "b" {
		t.Fatalf("MustInjectAll() = %v, unexpected order/content", got)
	}
}

func TestMustInjectAll_DirectResolver_ZeroMatches_ReturnsEmptyNotPanic(t *testing.T) {
	owner := &fakeDirectOwner{all: nil}

	got := MustInjectAll[directIface](owner)
	if len(got) != 0 {
		t.Fatalf("MustInjectAll() len = %d, want 0", len(got))
	}
}

func TestMustInjectAll_PointerType_Panics(t *testing.T) {
	owner := &fakeDirectOwner{all: nil}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustInjectAll() did not panic, want panic (pointer T not allowed)")
		}
		if !strings.Contains(r.(string), "requires T to be an interface type") {
			t.Fatalf("panic message = %q, missing expected substring", r)
		}
	}()
	MustInjectAll[*fakeService](owner)
}

func TestMustInjectAll_FromProviderOwner_Panics(t *testing.T) {
	owner := newOwner() // *provider.Provider, does not satisfy directResolver

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustInjectAll() did not panic, want panic (Provider unsupported)")
		}
		if !strings.Contains(r.(string), "not supported from this owner") {
			t.Fatalf("panic message = %q, missing expected substring", r)
		}
	}()
	MustInjectAll[directIface](owner)
}
