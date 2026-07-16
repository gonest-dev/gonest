package schema_test

import (
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// customEntity is a minimal fixture for exercising Custom/CustomFunc in
// isolation (param-query-validation feature's P0 -- context.md's Decision
// 4).
type customEntity struct {
	Code string
}

func newCustomTestSchema(t *testing.T) (*customEntity, *schema.Schema) {
	t.Helper()
	zero := &customEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

func TestPropertyBuilder_CustomFunc_NeverCalled_ReturnsFalse(t *testing.T) {
	zero, m := newCustomTestSchema(t)
	pb := m.Property(&zero.Code).String().PropertyBuilder

	fn, ok := pb.CustomFunc()
	if ok || fn != nil {
		t.Fatalf("CustomFunc() ok=%v fn-is-nil=%v, want (false, true)", ok, fn == nil)
	}
}

func TestPropertyBuilder_Custom_StoresFn_RetrievableViaCustomFunc(t *testing.T) {
	zero, m := newCustomTestSchema(t)
	pb := m.Property(&zero.Code)

	called := false
	sentinel := pb.Custom(func(raw any) (any, error) {
		called = true
		return raw, nil
	})

	if sentinel != pb {
		t.Fatal("Custom() should return the bare *PropertyBuilder itself, same precedent as Boolean()")
	}

	fn, ok := pb.CustomFunc()
	if !ok || fn == nil {
		t.Fatalf("CustomFunc() ok=%v fn-is-nil=%v, want (true, false)", ok, fn == nil)
	}

	val, err := fn("v1:42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected the stored fn to be the same closure passed to Custom")
	}
	if val != "v1:42" {
		t.Fatalf("fn(%q) = %v, want %q echoed back", "v1:42", val, "v1:42")
	}
}

func TestPropertyBuilder_Custom_LastCallWins(t *testing.T) {
	zero, m := newCustomTestSchema(t)
	pb := m.Property(&zero.Code)

	pb.Custom(func(raw any) (any, error) { return "first", nil })
	pb.Custom(func(raw any) (any, error) { return "second", errors.New("boom") })

	fn, ok := pb.CustomFunc()
	if !ok {
		t.Fatal("expected CustomFunc() ok=true after two Custom calls")
	}
	val, err := fn(nil)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected the SECOND Custom call to win, got val=%v err=%v", val, err)
	}
}
