package schema_test

import (
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// defaultEntity is a minimal fixture for exercising Default/DefaultValue in
// isolation (env-schema-binding feature).
type defaultEntity struct {
	Port int
}

func newDefaultTestSchema(t *testing.T) (*defaultEntity, *schema.Schema) {
	t.Helper()
	zero := &defaultEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

func TestPropertyBuilder_Default_SetsDefaultValue(t *testing.T) {
	zero, m := newDefaultTestSchema(t)
	pb := m.Property(&zero.Port).Integer().PropertyBuilder

	pb.Default(5432)

	val, ok := pb.DefaultValue()
	if !ok || val != 5432 {
		t.Fatalf("DefaultValue() = (%v, %v), want (5432, true)", val, ok)
	}
}

func TestPropertyBuilder_DefaultValue_NeverCalled_ReturnsFalse(t *testing.T) {
	zero, m := newDefaultTestSchema(t)
	pb := m.Property(&zero.Port).Integer().PropertyBuilder

	val, ok := pb.DefaultValue()
	if ok || val != nil {
		t.Fatalf("DefaultValue() = (%v, %v), want (nil, false)", val, ok)
	}
}

func TestPropertyBuilder_Default_LastCallWins(t *testing.T) {
	zero, m := newDefaultTestSchema(t)
	pb := m.Property(&zero.Port).Integer().PropertyBuilder

	pb.Default(1)
	pb.Default(2)

	val, ok := pb.DefaultValue()
	if !ok || val != 2 {
		t.Fatalf("DefaultValue() = (%v, %v), want (2, true) -- last call should win", val, ok)
	}
}

func TestPropertyBuilder_Default_ReturnsSelfForChaining(t *testing.T) {
	zero, m := newDefaultTestSchema(t)
	pb := m.Property(&zero.Port).Integer().PropertyBuilder.Required()

	sentinel := pb.Default(5432)

	if sentinel != pb {
		t.Fatal("Default() should return the bare *PropertyBuilder itself, same precedent as Custom()")
	}
	val, ok := sentinel.DefaultValue()
	if !ok || val != 5432 {
		t.Fatalf("DefaultValue() after chained Required().Default() = (%v, %v), want (5432, true)", val, ok)
	}
}
