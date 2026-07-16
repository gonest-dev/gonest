package schema_test

import (
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// objectEntity mirrors INSIGHT.md's UserEntity shape for the two Object()
// cases: Address (reused *Schema via Schema(ref)) and Schema
// (open/free-form via AdditionalProperties()).
type objectEntity struct {
	Address addressEntity
	Schema  map[string]any
}

// newObjectTestSchema mirrors array_test.go's newArrayTestSchema helper
// (same pattern: construct a zero value, keep it alive via the returned
// pointer, build *Schema from its address).
func newObjectTestSchema(t *testing.T) (*objectEntity, *schema.Schema) {
	t.Helper()
	zero := &objectEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// TestObject_SetsFormatAndReturnsNewObjectSchema proves OB-01: Object(fn)
// sets format="object" on the field's own PropertyBuilder and returns a
// brand new *ObjectSchema whose embedded PropertyBuilder is the SAME
// pointer as the field (pointer-identity, no synthetic secondary builder --
// unlike ArraySchema).
func TestObject_SetsFormatAndReturnsNewObjectSchema(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Address)

	om := pb.Object(func(om *schema.ObjectSchema) {})

	if om == nil {
		t.Fatal("Object() returned nil *ObjectSchema")
	}
	if pb.FormatValue() != "object" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "object")
	}
	if om.PropertyBuilder != pb {
		t.Fatal("ObjectSchema.PropertyBuilder is not the same pointer as the field's PropertyBuilder")
	}
}

// TestObject_CallbackReceivesSameObjectSchema proves OB-01: fn receives
// the exact same *ObjectSchema that Object(fn) itself returns (pointer
// identity), same test-pattern as ArraySchema.Items(fn).
func TestObject_CallbackReceivesSameObjectSchema(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Address)

	var received *schema.ObjectSchema
	returned := pb.Object(func(om *schema.ObjectSchema) {
		received = om
	})

	if received == nil {
		t.Fatal("callback was never invoked")
	}
	if received != returned {
		t.Fatal("callback received a different *ObjectSchema than Object() returned")
	}
}

// TestObjectSchema_Schema_StoresRefWithIdentity proves OB-02: Schema(ref)
// stores ref, recoverable via SchemaRef() with pointer identity preserved.
func TestObjectSchema_Schema_StoresRefWithIdentity(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Address)
	addressSchema := newAddressTestSchema(t)

	om := pb.Object(func(om *schema.ObjectSchema) {
		om.Schema(addressSchema)
	})

	got, ok := om.SchemaRef()
	if !ok {
		t.Fatal("SchemaRef() ok = false, want true")
	}
	if got != addressSchema {
		t.Fatal("SchemaRef() did not return the same pointer passed to Schema()")
	}
}

// TestObjectSchema_SchemaRef_NeverCalled proves the "never called"
// distinction: SchemaRef() returns (nil, false) when Schema() was never
// invoked -- same precedent as ArraySchema's ItemRef/MinValue/MaxValue.
func TestObjectSchema_SchemaRef_NeverCalled(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Schema)

	om := pb.Object(func(om *schema.ObjectSchema) {
		om.AdditionalProperties()
	})

	got, ok := om.SchemaRef()
	if ok {
		t.Fatal("SchemaRef() ok = true, want false (Schema() never called)")
	}
	if got != nil {
		t.Fatalf("SchemaRef() value = %v, want nil", got)
	}
}

// TestObjectSchema_AdditionalProperties_SetsFlag proves OB-04:
// AdditionalProperties() marks the field as an open schema.
func TestObjectSchema_AdditionalProperties_SetsFlag(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Schema)

	om := pb.Object(func(om *schema.ObjectSchema) {
		om.AdditionalProperties()
	})

	if !om.IsAdditionalProperties() {
		t.Fatal("IsAdditionalProperties() = false, want true")
	}
}

// TestObjectSchema_AdditionalProperties_DefaultFalse proves the flag
// defaults to false when AdditionalProperties() is never called.
func TestObjectSchema_AdditionalProperties_DefaultFalse(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Address)

	om := pb.Object(func(om *schema.ObjectSchema) {})

	if om.IsAdditionalProperties() {
		t.Fatal("IsAdditionalProperties() = true, want false (never called)")
	}
}

// TestObjectSchema_RequiredNullableDescriptionExamples_InsideVsOutside
// proves OB-03 / design.md's key point: Required/Nullable/Description/
// Examples produce EXACTLY the same result whether called INSIDE the
// callback or chained OUTSIDE (on Object(fn)'s own return value) -- no
// inside/outside distinction exists here, unlike ArraySchema's field-vs-
// item routing.
func TestObjectSchema_RequiredNullableDescriptionExamples_InsideVsOutside(t *testing.T) {
	t.Run("inside callback", func(t *testing.T) {
		zero, m := newObjectTestSchema(t)
		pb := m.Property(&zero.Address)

		om := pb.Object(func(om *schema.ObjectSchema) {
			om.Required()
			om.Nullable()
			om.Description("Endereço principal")
			om.Examples("ex1", "ex2")
		})

		if !om.PropertyBuilder.IsRequired() {
			t.Fatal("IsRequired() = false, want true")
		}
		if !om.PropertyBuilder.IsNullable() {
			t.Fatal("IsNullable() = false, want true")
		}
		if om.PropertyBuilder.DescriptionText() != "Endereço principal" {
			t.Fatalf("DescriptionText() = %q, want %q", om.PropertyBuilder.DescriptionText(), "Endereço principal")
		}
		if !reflect.DeepEqual(om.PropertyBuilder.ExamplesList(), []any{"ex1", "ex2"}) {
			t.Fatalf("ExamplesList() = %v, want [ex1 ex2]", om.PropertyBuilder.ExamplesList())
		}
	})

	t.Run("outside callback (chained on Object(fn)'s return)", func(t *testing.T) {
		zero, m := newObjectTestSchema(t)
		pb := m.Property(&zero.Address)

		om := pb.Object(func(om *schema.ObjectSchema) {}).
			Required().
			Nullable().
			Description("Endereço principal").
			Examples("ex1", "ex2")

		if !om.PropertyBuilder.IsRequired() {
			t.Fatal("IsRequired() = false, want true")
		}
		if !om.PropertyBuilder.IsNullable() {
			t.Fatal("IsNullable() = false, want true")
		}
		if om.PropertyBuilder.DescriptionText() != "Endereço principal" {
			t.Fatalf("DescriptionText() = %q, want %q", om.PropertyBuilder.DescriptionText(), "Endereço principal")
		}
		if !reflect.DeepEqual(om.PropertyBuilder.ExamplesList(), []any{"ex1", "ex2"}) {
			t.Fatalf("ExamplesList() = %v, want [ex1 ex2]", om.PropertyBuilder.ExamplesList())
		}
	})
}

// TestObjectSchema_Required_ReturnsObjectSchemaForChaining proves
// Required()/Nullable()/Description()/Examples() are redeclared on
// *ObjectSchema (not just promoted from *PropertyBuilder) so calls stay
// chainable as *ObjectSchema -- same precedent as ArraySchema's own
// redeclarations.
func TestObjectSchema_Required_ReturnsObjectSchemaForChaining(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Address)

	om := pb.Object(func(om *schema.ObjectSchema) {})

	var chained *schema.ObjectSchema = om.Required().Nullable().Description("d").Examples(1, 2)
	if chained != om {
		t.Fatal("chained Required/Nullable/Description/Examples did not return the same *ObjectSchema")
	}
}

// TestObject_InsightAddressCase reproduces INSIGHT.md's Address field
// verbatim: Object(fn) with Schema(addressSchema) + Required() +
// Description() all called INSIDE the callback.
func TestObject_InsightAddressCase(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	addressSchema := newAddressTestSchema(t)

	pb := m.Property(&zero.Address)
	om := pb.Object(func(om *schema.ObjectSchema) {
		om.Schema(addressSchema)
		om.Required()
		om.Description("Endereço principal")
	})

	if pb.FormatValue() != "object" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "object")
	}
	ref, ok := om.SchemaRef()
	if !ok || ref != addressSchema {
		t.Fatal("SchemaRef() did not return addressSchema with identity preserved")
	}
	if !om.PropertyBuilder.IsRequired() {
		t.Fatal("IsRequired() = false, want true")
	}
	if om.PropertyBuilder.DescriptionText() != "Endereço principal" {
		t.Fatalf("DescriptionText() = %q, want %q", om.PropertyBuilder.DescriptionText(), "Endereço principal")
	}
}

// TestObject_InsightSchemaCase reproduces INSIGHT.md's Schema
// (map[string]any) field verbatim: Object(fn) with AdditionalProperties()
// INSIDE the callback, then .Nullable().Description(...) chained OUTSIDE
// the callback on Object(fn)'s own return value.
func TestObject_InsightSchemaCase(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Schema)

	om := pb.Object(func(om *schema.ObjectSchema) {
		om.AdditionalProperties()
	}).Nullable().Description("Metadados abertos do usuário")

	if pb.FormatValue() != "object" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "object")
	}
	if !om.IsAdditionalProperties() {
		t.Fatal("IsAdditionalProperties() = false, want true")
	}
	if _, ok := om.SchemaRef(); ok {
		t.Fatal("SchemaRef() ok = true, want false (Schema() never called)")
	}
	if !om.PropertyBuilder.IsNullable() {
		t.Fatal("IsNullable() = false, want true")
	}
	if om.PropertyBuilder.DescriptionText() != "Metadados abertos do usuário" {
		t.Fatalf("DescriptionText() = %q, want %q", om.PropertyBuilder.DescriptionText(), "Metadados abertos do usuário")
	}
}

// TestObject_CalledTwice_SecondObjectSchemaIsIndependent proves the Edge
// Case / Done-when: calling Object(fn) twice on the same *PropertyBuilder
// does not panic, and the second *ObjectSchema's ref/additionalProperties
// are NOT carried over from the first (fresh state every call, since ref/
// additionalProperties live on the *ObjectSchema wrapper, not on p).
func TestObject_CalledTwice_SecondObjectSchemaIsIndependent(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Address)
	addressSchema := newAddressTestSchema(t)

	om1 := pb.Object(func(om *schema.ObjectSchema) {
		om.Schema(addressSchema)
		om.AdditionalProperties()
	})
	if _, ok := om1.SchemaRef(); !ok {
		t.Fatal("first ObjectSchema: SchemaRef() ok = false, want true")
	}

	om2 := pb.Object(func(om *schema.ObjectSchema) {})

	if _, ok := om2.SchemaRef(); ok {
		t.Fatal("second ObjectSchema: SchemaRef() ok = true, want false (fresh state)")
	}
	if om2.IsAdditionalProperties() {
		t.Fatal("second ObjectSchema: IsAdditionalProperties() = true, want false (fresh state)")
	}
	if pb.FormatValue() != "object" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "object")
	}
}

// TestObjectSchema_BothRefAndAdditionalProperties_NoPanic proves the Edge
// Case: calling both Schema(ref) and AdditionalProperties() in the same
// callback is accepted without panic -- both stored independently.
func TestObjectSchema_BothRefAndAdditionalProperties_NoPanic(t *testing.T) {
	zero, m := newObjectTestSchema(t)
	pb := m.Property(&zero.Address)
	addressSchema := newAddressTestSchema(t)

	om := pb.Object(func(om *schema.ObjectSchema) {
		om.Schema(addressSchema)
		om.AdditionalProperties()
	})

	ref, ok := om.SchemaRef()
	if !ok || ref != addressSchema {
		t.Fatal("SchemaRef() did not preserve ref alongside AdditionalProperties()")
	}
	if !om.IsAdditionalProperties() {
		t.Fatal("IsAdditionalProperties() = false, want true")
	}
}
