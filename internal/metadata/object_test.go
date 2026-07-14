package metadata_test

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/gonest-dev/gonest/internal/metadata"
)

// objectEntity mirrors INSIGHT.md's UserEntity shape for the two Object()
// cases: Address (reused *Metadata via Metadata(ref)) and Metadata
// (open/free-form via AdditionalProperties()).
type objectEntity struct {
	Address  addressEntity
	Metadata map[string]any
}

// newObjectTestMetadata mirrors array_test.go's newArrayTestMetadata helper
// (same pattern: construct a zero value, keep it alive via the returned
// pointer, build *Metadata from its address).
func newObjectTestMetadata(t *testing.T) (*objectEntity, *metadata.Metadata) {
	t.Helper()
	zero := &objectEntity{}
	m := metadata.New(reflect.TypeOf(*zero), uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// TestObject_SetsFormatAndReturnsNewObjectMetadata proves OB-01: Object(fn)
// sets format="object" on the field's own PropertyBuilder and returns a
// brand new *ObjectMetadata whose embedded PropertyBuilder is the SAME
// pointer as the field (pointer-identity, no synthetic secondary builder --
// unlike ArrayMetadata).
func TestObject_SetsFormatAndReturnsNewObjectMetadata(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Address)

	om := pb.Object(func(om *metadata.ObjectMetadata) {})

	if om == nil {
		t.Fatal("Object() returned nil *ObjectMetadata")
	}
	if pb.FormatValue() != "object" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "object")
	}
	if om.PropertyBuilder != pb {
		t.Fatal("ObjectMetadata.PropertyBuilder is not the same pointer as the field's PropertyBuilder")
	}
}

// TestObject_CallbackReceivesSameObjectMetadata proves OB-01: fn receives
// the exact same *ObjectMetadata that Object(fn) itself returns (pointer
// identity), same test-pattern as ArrayMetadata.Items(fn).
func TestObject_CallbackReceivesSameObjectMetadata(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Address)

	var received *metadata.ObjectMetadata
	returned := pb.Object(func(om *metadata.ObjectMetadata) {
		received = om
	})

	if received == nil {
		t.Fatal("callback was never invoked")
	}
	if received != returned {
		t.Fatal("callback received a different *ObjectMetadata than Object() returned")
	}
}

// TestObjectMetadata_Metadata_StoresRefWithIdentity proves OB-02: Metadata(ref)
// stores ref, recoverable via MetadataRef() with pointer identity preserved.
func TestObjectMetadata_Metadata_StoresRefWithIdentity(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Address)
	addressMetadata := newAddressTestMetadata(t)

	om := pb.Object(func(om *metadata.ObjectMetadata) {
		om.Metadata(addressMetadata)
	})

	got, ok := om.MetadataRef()
	if !ok {
		t.Fatal("MetadataRef() ok = false, want true")
	}
	if got != addressMetadata {
		t.Fatal("MetadataRef() did not return the same pointer passed to Metadata()")
	}
}

// TestObjectMetadata_MetadataRef_NeverCalled proves the "never called"
// distinction: MetadataRef() returns (nil, false) when Metadata() was never
// invoked -- same precedent as ArrayMetadata's ItemRef/MinValue/MaxValue.
func TestObjectMetadata_MetadataRef_NeverCalled(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Metadata)

	om := pb.Object(func(om *metadata.ObjectMetadata) {
		om.AdditionalProperties()
	})

	got, ok := om.MetadataRef()
	if ok {
		t.Fatal("MetadataRef() ok = true, want false (Metadata() never called)")
	}
	if got != nil {
		t.Fatalf("MetadataRef() value = %v, want nil", got)
	}
}

// TestObjectMetadata_AdditionalProperties_SetsFlag proves OB-04:
// AdditionalProperties() marks the field as an open schema.
func TestObjectMetadata_AdditionalProperties_SetsFlag(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Metadata)

	om := pb.Object(func(om *metadata.ObjectMetadata) {
		om.AdditionalProperties()
	})

	if !om.IsAdditionalProperties() {
		t.Fatal("IsAdditionalProperties() = false, want true")
	}
}

// TestObjectMetadata_AdditionalProperties_DefaultFalse proves the flag
// defaults to false when AdditionalProperties() is never called.
func TestObjectMetadata_AdditionalProperties_DefaultFalse(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Address)

	om := pb.Object(func(om *metadata.ObjectMetadata) {})

	if om.IsAdditionalProperties() {
		t.Fatal("IsAdditionalProperties() = true, want false (never called)")
	}
}

// TestObjectMetadata_RequiredNullableDescriptionExamples_InsideVsOutside
// proves OB-03 / design.md's key point: Required/Nullable/Description/
// Examples produce EXACTLY the same result whether called INSIDE the
// callback or chained OUTSIDE (on Object(fn)'s own return value) -- no
// inside/outside distinction exists here, unlike ArrayMetadata's field-vs-
// item routing.
func TestObjectMetadata_RequiredNullableDescriptionExamples_InsideVsOutside(t *testing.T) {
	t.Run("inside callback", func(t *testing.T) {
		zero, m := newObjectTestMetadata(t)
		pb := m.Property(&zero.Address)

		om := pb.Object(func(om *metadata.ObjectMetadata) {
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
		zero, m := newObjectTestMetadata(t)
		pb := m.Property(&zero.Address)

		om := pb.Object(func(om *metadata.ObjectMetadata) {}).
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

// TestObjectMetadata_Required_ReturnsObjectMetadataForChaining proves
// Required()/Nullable()/Description()/Examples() are redeclared on
// *ObjectMetadata (not just promoted from *PropertyBuilder) so calls stay
// chainable as *ObjectMetadata -- same precedent as ArrayMetadata's own
// redeclarations.
func TestObjectMetadata_Required_ReturnsObjectMetadataForChaining(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Address)

	om := pb.Object(func(om *metadata.ObjectMetadata) {})

	var chained *metadata.ObjectMetadata = om.Required().Nullable().Description("d").Examples(1, 2)
	if chained != om {
		t.Fatal("chained Required/Nullable/Description/Examples did not return the same *ObjectMetadata")
	}
}

// TestObject_InsightAddressCase reproduces INSIGHT.md's Address field
// verbatim: Object(fn) with Metadata(addressMetadata) + Required() +
// Description() all called INSIDE the callback.
func TestObject_InsightAddressCase(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	addressMetadata := newAddressTestMetadata(t)

	pb := m.Property(&zero.Address)
	om := pb.Object(func(om *metadata.ObjectMetadata) {
		om.Metadata(addressMetadata)
		om.Required()
		om.Description("Endereço principal")
	})

	if pb.FormatValue() != "object" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "object")
	}
	ref, ok := om.MetadataRef()
	if !ok || ref != addressMetadata {
		t.Fatal("MetadataRef() did not return addressMetadata with identity preserved")
	}
	if !om.PropertyBuilder.IsRequired() {
		t.Fatal("IsRequired() = false, want true")
	}
	if om.PropertyBuilder.DescriptionText() != "Endereço principal" {
		t.Fatalf("DescriptionText() = %q, want %q", om.PropertyBuilder.DescriptionText(), "Endereço principal")
	}
}

// TestObject_InsightMetadataCase reproduces INSIGHT.md's Metadata
// (map[string]any) field verbatim: Object(fn) with AdditionalProperties()
// INSIDE the callback, then .Nullable().Description(...) chained OUTSIDE
// the callback on Object(fn)'s own return value.
func TestObject_InsightMetadataCase(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Metadata)

	om := pb.Object(func(om *metadata.ObjectMetadata) {
		om.AdditionalProperties()
	}).Nullable().Description("Metadados abertos do usuário")

	if pb.FormatValue() != "object" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "object")
	}
	if !om.IsAdditionalProperties() {
		t.Fatal("IsAdditionalProperties() = false, want true")
	}
	if _, ok := om.MetadataRef(); ok {
		t.Fatal("MetadataRef() ok = true, want false (Metadata() never called)")
	}
	if !om.PropertyBuilder.IsNullable() {
		t.Fatal("IsNullable() = false, want true")
	}
	if om.PropertyBuilder.DescriptionText() != "Metadados abertos do usuário" {
		t.Fatalf("DescriptionText() = %q, want %q", om.PropertyBuilder.DescriptionText(), "Metadados abertos do usuário")
	}
}

// TestObject_CalledTwice_SecondObjectMetadataIsIndependent proves the Edge
// Case / Done-when: calling Object(fn) twice on the same *PropertyBuilder
// does not panic, and the second *ObjectMetadata's ref/additionalProperties
// are NOT carried over from the first (fresh state every call, since ref/
// additionalProperties live on the *ObjectMetadata wrapper, not on p).
func TestObject_CalledTwice_SecondObjectMetadataIsIndependent(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Address)
	addressMetadata := newAddressTestMetadata(t)

	om1 := pb.Object(func(om *metadata.ObjectMetadata) {
		om.Metadata(addressMetadata)
		om.AdditionalProperties()
	})
	if _, ok := om1.MetadataRef(); !ok {
		t.Fatal("first ObjectMetadata: MetadataRef() ok = false, want true")
	}

	om2 := pb.Object(func(om *metadata.ObjectMetadata) {})

	if _, ok := om2.MetadataRef(); ok {
		t.Fatal("second ObjectMetadata: MetadataRef() ok = true, want false (fresh state)")
	}
	if om2.IsAdditionalProperties() {
		t.Fatal("second ObjectMetadata: IsAdditionalProperties() = true, want false (fresh state)")
	}
	if pb.FormatValue() != "object" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "object")
	}
}

// TestObjectMetadata_BothRefAndAdditionalProperties_NoPanic proves the Edge
// Case: calling both Metadata(ref) and AdditionalProperties() in the same
// callback is accepted without panic -- both stored independently.
func TestObjectMetadata_BothRefAndAdditionalProperties_NoPanic(t *testing.T) {
	zero, m := newObjectTestMetadata(t)
	pb := m.Property(&zero.Address)
	addressMetadata := newAddressTestMetadata(t)

	om := pb.Object(func(om *metadata.ObjectMetadata) {
		om.Metadata(addressMetadata)
		om.AdditionalProperties()
	})

	ref, ok := om.MetadataRef()
	if !ok || ref != addressMetadata {
		t.Fatal("MetadataRef() did not preserve ref alongside AdditionalProperties()")
	}
	if !om.IsAdditionalProperties() {
		t.Fatal("IsAdditionalProperties() = false, want true")
	}
}
