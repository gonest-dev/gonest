package schema_test

import (
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/schema"
)

// newTestValueSchema builds a *Schema via NewValue for typ, deregistering
// it at test cleanup (same precedent as newTestSchema's t.Cleanup, avoiding
// Register's own duplicate-registration panic across this file's many Test
// functions sharing the same handful of primitive types).
func newTestValueSchema(t *testing.T, typ reflect.Type) (*schema.Schema, *schema.PropertyBuilder) {
	t.Helper()
	t.Cleanup(func() { schema.Deregister(typ) })
	return schema.NewValue(typ)
}

func TestNewValue_StringMinMaxPattern(t *testing.T) {
	m, pb := newTestValueSchema(t, reflect.TypeOf(""))

	pb.String().Min(11).Max(11).Pattern(`^\d{11}$`).Required()

	if !m.IsValue() {
		t.Fatal("IsValue(): want true for a NewValue-built Schema")
	}
	if got := m.ValueProperty(); got != pb {
		t.Fatalf("ValueProperty(): want the same *PropertyBuilder returned by NewValue, got a different pointer")
	}
	if pb.KindValue() != "string" {
		t.Fatalf("KindValue(): want %q, got %q", "string", pb.KindValue())
	}
	if min, ok := pb.MinValue(); !ok || min != 11 {
		t.Fatalf("MinValue(): want (11, true), got (%d, %v)", min, ok)
	}
	if max, ok := pb.MaxValue(); !ok || max != 11 {
		t.Fatalf("MaxValue(): want (11, true), got (%d, %v)", max, ok)
	}
	if pb.PatternValue() != `^\d{11}$` {
		t.Fatalf("PatternValue(): want %q, got %q", `^\d{11}$`, pb.PatternValue())
	}
	if !pb.IsRequired() {
		t.Fatal("IsRequired(): want true")
	}
}

func TestNewValue_IntegerMinMax(t *testing.T) {
	m, pb := newTestValueSchema(t, reflect.TypeOf(0))

	pb.Integer().Min(1).Max(100)

	if !m.IsValue() {
		t.Fatal("IsValue(): want true")
	}
	if pb.KindValue() != "integer" {
		t.Fatalf("KindValue(): want %q, got %q", "integer", pb.KindValue())
	}
	if min, ok := pb.MinValue(); !ok || min != 1 {
		t.Fatalf("MinValue(): want (1, true), got (%d, %v)", min, ok)
	}
	if max, ok := pb.MaxValue(); !ok || max != 100 {
		t.Fatalf("MaxValue(): want (100, true), got (%d, %v)", max, ok)
	}
}

func TestNewValue_RegistersInSameRegistryAsNew(t *testing.T) {
	typ := reflect.TypeOf(int64(0))
	m, _ := newTestValueSchema(t, typ)

	got, ok := schema.Lookup(typ)
	if !ok {
		t.Fatal("Lookup: want the Value-schema to be found in the registry")
	}
	if got != m {
		t.Fatal("Lookup: want the same *Schema returned by NewValue")
	}
}

func TestValueProperty_PanicsOnStructShapedSchema(t *testing.T) {
	zero := &userEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, 0)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ValueProperty(): want panic on a struct-shaped Schema, got none")
		}
	}()
	m.ValueProperty()
}

func TestIsValue_FalseForStructShapedSchema(t *testing.T) {
	_, m := newTestSchema(t)
	if m.IsValue() {
		t.Fatal("IsValue(): want false for a New-built (struct) Schema")
	}
}
