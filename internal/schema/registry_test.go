package schema_test

import (
	"reflect"
	"sync"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// registryUserEntity is a type PRIVATE to this test file (distinct
// reflect.Type from userEntity in schema_test.go and every other type
// used elsewhere in this package's test suite) -- necessary because
// Register panics on duplicate registration of the SAME type, and this
// file's own tests each register their own single-use types to stay
// independent of one another and of every other test in the package.
type registryUserEntity struct {
	Id   int64
	Name string
}

// TestRegister_RecoverableViaLookup proves the core round-trip: a type
// registered via New (NewSchema[T]'s internal call) is recoverable via
// Lookup, keyed by reflect.Type (spec.md JV-01/AC1).
func TestRegister_RecoverableViaLookup(t *testing.T) {
	type localEntity struct {
		Id int64
	}
	zero := &localEntity{}
	typ := reflect.TypeOf(*zero)
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))

	got, ok := schema.Lookup(typ)
	if !ok {
		t.Fatalf("Lookup(%v) ok = false, want true", typ)
	}
	if got != m {
		t.Fatalf("Lookup(%v) = %p, want %p (same *Schema New returned)", typ, got, m)
	}
}

// TestNew_CalledTwiceForSameType_Panics proves one schema declaration per
// type is enforced (spec.md JV-01/AC2), same precedent as Property's own
// double-registration panic.
func TestNew_CalledTwiceForSameType_Panics(t *testing.T) {
	zero := &registryUserEntity{}
	typ := reflect.TypeOf(*zero)
	schema.New(typ, uintptr(unsafe.Pointer(zero)))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected New to panic on second registration for the same type, it did not")
		}
	}()
	schema.New(typ, uintptr(unsafe.Pointer(zero)))
}

// TestLookup_NeverRegisteredType_ReturnsFalseNoPanic proves an unregistered
// type reports "not found" cleanly (spec.md JV-02) instead of a nil-pointer
// crash -- this is what lets MustJsonBody panic a CLEAR error instead.
func TestLookup_NeverRegisteredType_ReturnsFalseNoPanic(t *testing.T) {
	type neverRegisteredEntity struct {
		X int
	}
	typ := reflect.TypeOf(neverRegisteredEntity{})

	got, ok := schema.Lookup(typ)
	if ok {
		t.Fatalf("Lookup(%v) ok = true, want false (never registered)", typ)
	}
	if got != nil {
		t.Fatalf("Lookup(%v) = %v, want nil", typ, got)
	}
}

// TestRegister_ConcurrentDistinctTypes_NoRace registers several DISTINCT
// types from concurrent goroutines -- design.md flags this as plausible if
// multiple init() functions across packages register concurrently during
// startup; -race is what would catch an unguarded map access here.
func TestRegister_ConcurrentDistinctTypes_NoRace(t *testing.T) {
	type concurrentEntityA struct{ A int }
	type concurrentEntityB struct{ B int }
	type concurrentEntityC struct{ C int }
	type concurrentEntityD struct{ D int }
	type concurrentEntityE struct{ E int }

	zeros := []any{
		&concurrentEntityA{}, &concurrentEntityB{}, &concurrentEntityC{},
		&concurrentEntityD{}, &concurrentEntityE{},
	}

	var wg sync.WaitGroup
	for _, z := range zeros {
		wg.Add(1)
		go func(zero any) {
			defer wg.Done()
			v := reflect.ValueOf(zero).Elem()
			typ := v.Type()
			addr := v.Addr().Pointer()
			schema.New(typ, addr)
		}(z)
	}
	wg.Wait()

	for _, z := range zeros {
		typ := reflect.ValueOf(z).Elem().Type()
		if _, ok := schema.Lookup(typ); !ok {
			t.Errorf("Lookup(%v) ok = false after concurrent registration, want true", typ)
		}
	}
}
