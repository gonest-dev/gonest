package schema_test

import (
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// sanitizeRefineEntity is a minimal fixture for exercising Sanitize/
// SanitizeFunc and Refine/OwnRefines in isolation (schema-sanitize-refine
// feature).
type sanitizeRefineEntity struct {
	Password        string
	ConfirmPassword string
}

func newSanitizeRefineTestSchema(t *testing.T) (*sanitizeRefineEntity, *schema.Schema) {
	t.Helper()
	zero := &sanitizeRefineEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// --- Sanitize/SanitizeFunc (PropertyBuilder) --------------------------------

func TestPropertyBuilder_SanitizeFunc_NeverCalled_ReturnsFalse(t *testing.T) {
	zero, m := newSanitizeRefineTestSchema(t)
	pb := m.Property(&zero.Password).String().PropertyBuilder

	fn, ok := pb.SanitizeFunc()
	if ok || fn != nil {
		t.Fatalf("SanitizeFunc() ok=%v fn-is-nil=%v, want (false, true)", ok, fn == nil)
	}
}

func TestPropertyBuilder_Sanitize_StoresFn_RetrievableViaSanitizeFunc(t *testing.T) {
	zero, m := newSanitizeRefineTestSchema(t)
	pb := m.Property(&zero.Password)

	sentinel := pb.Sanitize(func(raw any) any { return raw })

	if sentinel != pb {
		t.Fatal("Sanitize() should return the bare *PropertyBuilder itself, same precedent as Custom()")
	}

	fn, ok := pb.SanitizeFunc()
	if !ok || fn == nil {
		t.Fatalf("SanitizeFunc() ok=%v fn-is-nil=%v, want (true, false)", ok, fn == nil)
	}
	if got := fn("  x  "); got != "  x  " {
		t.Fatalf("fn(%q) = %v, want the same value echoed back", "  x  ", got)
	}
}

func TestPropertyBuilder_Sanitize_LastCallWins(t *testing.T) {
	zero, m := newSanitizeRefineTestSchema(t)
	pb := m.Property(&zero.Password)

	pb.Sanitize(func(raw any) any { return "first" })
	pb.Sanitize(func(raw any) any { return "second" })

	fn, ok := pb.SanitizeFunc()
	if !ok {
		t.Fatal("expected SanitizeFunc() ok=true after two Sanitize calls")
	}
	if got := fn(nil); got != "second" {
		t.Fatalf("expected the SECOND Sanitize call to win, got %v", got)
	}
}

// --- Refine/OwnRefines (Schema) ---------------------------------------------

func TestSchema_OwnRefines_EmptyByDefault(t *testing.T) {
	_, m := newSanitizeRefineTestSchema(t)
	if got := m.OwnRefines(); len(got) != 0 {
		t.Fatalf("OwnRefines() = %v, want empty", got)
	}
}

func TestSchema_Refine_AccumulatesInRegistrationOrder(t *testing.T) {
	_, m := newSanitizeRefineTestSchema(t)

	sentinel := m.Refine(func(dst any) (string, error) { return "first", nil })
	if sentinel != m {
		t.Fatal("Refine() should return m so calls can chain")
	}
	m.Refine(func(dst any) (string, error) { return "second", nil })

	refines := m.OwnRefines()
	if len(refines) != 2 {
		t.Fatalf("OwnRefines() len = %d, want 2", len(refines))
	}
	f0, _ := refines[0](nil)
	f1, _ := refines[1](nil)
	if f0 != "first" || f1 != "second" {
		t.Fatalf("OwnRefines() order = [%q, %q], want [first, second]", f0, f1)
	}
}

func TestSchema_OwnRefines_ReturnsCopyNotInternalSlice(t *testing.T) {
	_, m := newSanitizeRefineTestSchema(t)
	m.Refine(func(dst any) (string, error) { return "", nil })

	got := m.OwnRefines()
	got[0] = func(dst any) (string, error) { return "mutated", errors.New("boom") }

	got2 := m.OwnRefines()
	f, _ := got2[0](nil)
	if f == "mutated" {
		t.Fatal("OwnRefines() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}
