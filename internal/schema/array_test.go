package schema_test

import (
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// arrayEntity exercises Array() against real slice-typed fields, plus a
// struct-typed field standing in for the "Address" (non-array) shape reused
// as an Object(ref) item -- mirrors INSIGHT.md's UserEntity/AddressEntity
// shapes (Tags []string, Scores []int, Addresses []AddressEntity).
type arrayEntity struct {
	Tags      []string
	Scores    []int
	Addresses []addressEntity
}

type addressEntity struct {
	Street string
	City   string
	Zip    string
}

// newArrayTestSchema mirrors numeric_test.go's newNumericTestSchema /
// string_test.go's newStringTestSchema helper (same pattern: construct a
// zero value, keep it alive via the returned pointer, build *Schema from
// its address).
func newArrayTestSchema(t *testing.T) (*arrayEntity, *schema.Schema) {
	t.Helper()
	zero := &arrayEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// newAddressTestSchema builds a standalone *Schema for addressEntity,
// standing in for INSIGHT.md's `addressSchema := gonest.NewSchema[AddressEntity](...)`
// -- reused below as the itemRef passed to Object(ref).
func newAddressTestSchema(t *testing.T) *schema.Schema {
	t.Helper()
	zero := &addressEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	return schema.New(typ, uintptr(unsafe.Pointer(zero)))
}

// TestArray_SetsFormatAndReturnsNewArraySchema proves AR-01: Array()
// sets format="array" on the field's own PropertyBuilder and returns a
// brand new *ArraySchema whose embedded PropertyBuilder is the SAME
// pointer as the field (pointer-identity, same precedent as prior branches).
func TestArray_SetsFormatAndReturnsNewArraySchema(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	pb := m.Property(&zero.Tags)

	am := pb.Array()

	if am == nil {
		t.Fatal("Array() returned nil *ArraySchema")
	}
	if pb.FormatValue() != "array" {
		t.Fatalf("FormatValue() = %q, want %q", pb.FormatValue(), "array")
	}
	if am.PropertyBuilder != pb {
		t.Fatal("ArraySchema.PropertyBuilder is not the same pointer as the field's PropertyBuilder")
	}
}

// TestArray_CalledTwice_ProducesIndependentItemState proves the Edge Case /
// Done-when: calling Array() twice on the same PropertyBuilder does not
// panic, and the second *ArraySchema's item state is independent of the
// first (fresh synthetic item builder every call).
func TestArray_CalledTwice_ProducesIndependentItemState(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	pb := m.Property(&zero.Tags)

	am1 := pb.Array()
	am1.Items(func(item *schema.ArraySchema) {
		item.String()
	})

	am2 := pb.Array()

	if am1 == am2 {
		t.Fatal("second Array() call returned the same *ArraySchema as the first")
	}
	if am2.ItemBuilder() == am1.ItemBuilder() {
		t.Fatal("second Array() call reused the first call's item builder")
	}
	if am2.ItemBuilder().FormatValue() != "" {
		t.Fatalf("second ArraySchema's item format = %q, want empty (fresh item)", am2.ItemBuilder().FormatValue())
	}
}

// TestItems_PassesSameArraySchemaAndReturnsIt proves AR-02: Items(fn)
// invokes fn with the SAME *ArraySchema (pointer identity), and Items
// itself returns that same pointer so callers can chain quantity Min/Max
// after the callback.
func TestItems_PassesSameArraySchemaAndReturnsIt(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Tags).Array()

	var received *schema.ArraySchema
	got := am.Items(func(inner *schema.ArraySchema) {
		received = inner
	})

	if received != am {
		t.Fatal("Items(fn) did not pass the same *ArraySchema to the callback")
	}
	if got != am {
		t.Fatal("Items(fn) did not return the same *ArraySchema")
	}
}

// TestItems_StringItem_SetsItemFormatAndMinMax proves AR-03 for the String
// item branch: m.String().Min(1).Max(50) inside the callback configures the
// synthetic item builder, not the field, and Min/Max are retrievable via the
// returned *StringSchema.
func TestItems_StringItem_SetsItemFormatAndMinMax(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Tags).Array()

	var sm *schema.StringSchema
	am.Items(func(inner *schema.ArraySchema) {
		sm = inner.String().Min(1).Max(50)
	})

	if am.ItemBuilder().FormatValue() != "" {
		t.Fatalf("item FormatValue() = %q, want empty (bare string)", am.ItemBuilder().FormatValue())
	}
	minV, ok := sm.MinValue()
	if !ok || minV != 1 {
		t.Fatalf("item MinValue() = (%d, %v), want (1, true)", minV, ok)
	}
	maxV, ok := sm.MaxValue()
	if !ok || maxV != 50 {
		t.Fatalf("item MaxValue() = (%d, %v), want (50, true)", maxV, ok)
	}
}

// TestItems_IntegerItem_SetsItemFormatAndMinMax proves AR-03 for the
// Integer item branch: m.Integer().Min(0).Max(100) inside the callback
// configures the synthetic item builder with format "int64" plus Min/Max.
func TestItems_IntegerItem_SetsItemFormatAndMinMax(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Scores).Array()

	var nm *schema.NumericSchema
	am.Items(func(inner *schema.ArraySchema) {
		nm = inner.Integer().Min(0).Max(100)
	})

	if am.ItemBuilder().FormatValue() != "int64" {
		t.Fatalf("item FormatValue() = %q, want %q", am.ItemBuilder().FormatValue(), "int64")
	}
	minV, ok := nm.MinValue()
	if !ok || minV != 0 {
		t.Fatalf("item MinValue() = (%d, %v), want (0, true)", minV, ok)
	}
	maxV, ok := nm.MaxValue()
	if !ok || maxV != 100 {
		t.Fatalf("item MaxValue() = (%d, %v), want (100, true)", maxV, ok)
	}
}

// TestArraySchema_FieldMethods_NeverMutateItem proves AR-04: Required/
// Nullable/Description/Examples called on the *ArraySchema (inside or
// outside the callback) always mutate the FIELD container, never the item --
// explicit proof the item's corresponding fields stay zero-value.
func TestArraySchema_FieldMethods_NeverMutateItem(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Tags).Array()

	am.Items(func(inner *schema.ArraySchema) {
		inner.String().Min(1).Max(50)
		inner.Required()
		inner.Nullable()
		inner.Description("Tags do usuário")
		inner.Examples("admin", "beta")
	})

	// Field container got mutated.
	if !am.IsRequired() {
		t.Fatal("field IsRequired() = false, want true")
	}
	if !am.IsNullable() {
		t.Fatal("field IsNullable() = false, want true")
	}
	if am.DescriptionText() != "Tags do usuário" {
		t.Fatalf("field DescriptionText() = %q, want %q", am.DescriptionText(), "Tags do usuário")
	}
	if len(am.ExamplesList()) != 2 {
		t.Fatalf("field ExamplesList() = %v, want 2 examples", am.ExamplesList())
	}

	// Item never got mutated by those 4 calls.
	item := am.ItemBuilder()
	if item.IsRequired() {
		t.Fatal("item IsRequired() = true, want false (Required() must never touch the item)")
	}
	if item.IsNullable() {
		t.Fatal("item IsNullable() = true, want false (Nullable() must never touch the item)")
	}
	if item.DescriptionText() != "" {
		t.Fatalf("item DescriptionText() = %q, want empty (Description() must never touch the item)", item.DescriptionText())
	}
	if len(item.ExamplesList()) != 0 {
		t.Fatalf("item ExamplesList() = %v, want empty (Examples() must never touch the item)", item.ExamplesList())
	}
}

// TestArraySchema_Object_SetsItemRefWithPointerIdentity proves AR-06:
// Object(ref) inside the callback stores ref as itemRef, retrievable via
// ItemRef() with pointer identity to the already-registered *Schema.
func TestArraySchema_Object_SetsItemRefWithPointerIdentity(t *testing.T) {
	addressSchema := newAddressTestSchema(t)
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Addresses).Array()

	am.Items(func(inner *schema.ArraySchema) {
		inner.Object(addressSchema)
		inner.Required()
		inner.Description("Endereços do usuário")
	})
	am.Min(1)

	ref, ok := am.ItemRef()
	if !ok {
		t.Fatal("ItemRef() ok = false, want true")
	}
	if ref != addressSchema {
		t.Fatal("ItemRef() did not return the same *Schema pointer passed to Object()")
	}
}

// TestArraySchema_QuantityMinMax_DoesNotCollideWithItemMinMax proves AR-05:
// Min/Max called directly on *ArraySchema (chained after Items(fn)
// returns) stores ARRAY quantity, distinct and non-colliding with the item's
// own Min/Max set inside the callback via the *StringSchema/*NumericSchema
// wrapper.
func TestArraySchema_QuantityMinMax_DoesNotCollideWithItemMinMax(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Tags).Array()

	var sm *schema.StringSchema
	am.Items(func(inner *schema.ArraySchema) {
		sm = inner.String().Min(1).Max(50)
	}).Min(2).Max(10)

	qMin, ok := am.MinValue()
	if !ok || qMin != 2 {
		t.Fatalf("array quantity MinValue() = (%d, %v), want (2, true)", qMin, ok)
	}
	qMax, ok := am.MaxValue()
	if !ok || qMax != 10 {
		t.Fatalf("array quantity MaxValue() = (%d, %v), want (10, true)", qMax, ok)
	}

	// Item's own Min/Max (1..50) must remain independently readable (via the
	// *StringSchema wrapper captured inside the callback) and unaffected
	// by the array's quantity Min/Max (2..10).
	itemMin, ok := sm.MinValue()
	if !ok || itemMin != 1 {
		t.Fatalf("item MinValue() = (%d, %v), want (1, true)", itemMin, ok)
	}
	itemMax, ok := sm.MaxValue()
	if !ok || itemMax != 50 {
		t.Fatalf("item MaxValue() = (%d, %v), want (50, true)", itemMax, ok)
	}
}

// TestArraySchema_MinValue_NeverCalled_ReturnsFalse proves the getter's
// "never called" vs "called with 0" distinction, same precedent as
// StringSchema/NumericSchema's own MinValue/MaxValue.
func TestArraySchema_MinValue_NeverCalled_ReturnsFalse(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Tags).Array()

	if _, ok := am.MinValue(); ok {
		t.Fatal("MinValue() ok = true before Min() was ever called")
	}
	if _, ok := am.MaxValue(); ok {
		t.Fatal("MaxValue() ok = true before Max() was ever called")
	}
}

// TestArraySchema_ItemRef_NeverCalled_ReturnsFalse proves ItemRef()'s
// "never called" distinction, mirroring MinValue/MaxValue's own shape.
func TestArraySchema_ItemRef_NeverCalled_ReturnsFalse(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Tags).Array()

	if _, ok := am.ItemRef(); ok {
		t.Fatal("ItemRef() ok = true before Object() was ever called")
	}
}

// TestArraySchema_ItemsNeverCalled_ItemStaysZeroValue proves the Edge
// Case: Array() alone (no Items(fn)) leaves the item unformatted, no panic.
func TestArraySchema_ItemsNeverCalled_ItemStaysZeroValue(t *testing.T) {
	zero, m := newArrayTestSchema(t)
	am := m.Property(&zero.Tags).Array()

	if am.ItemBuilder().FormatValue() != "" {
		t.Fatalf("item FormatValue() = %q, want empty when Items(fn) was never called", am.ItemBuilder().FormatValue())
	}
	if _, ok := am.ItemRef(); ok {
		t.Fatal("ItemRef() ok = true when Object() was never called")
	}
}

// TestInsightMd_TagsScoresAddresses_Verbatim reproduces the 3 INSIGHT.md
// cases verbatim (Tags []string, Scores []int, Addresses []AddressEntity)
// end to end, asserting the field-level format, item-level format/Min/Max,
// and the container's own Required/Description/Examples/quantity.
func TestInsightMd_TagsScoresAddresses_Verbatim(t *testing.T) {
	addressSchema := newAddressTestSchema(t)
	zero, m := newArrayTestSchema(t)

	// Tags []string
	tagsPB := m.Property(&zero.Tags)
	tagsPB.Array().Items(func(mm *schema.ArraySchema) {
		mm.String().Min(1).Max(50)
		mm.Required()
		mm.Description("Tags do usuário")
		mm.Examples("admin", "beta")
	})
	if tagsPB.FormatValue() != "array" {
		t.Fatalf("Tags field FormatValue() = %q, want %q", tagsPB.FormatValue(), "array")
	}
	if !tagsPB.IsRequired() {
		t.Fatal("Tags field IsRequired() = false, want true")
	}
	if tagsPB.DescriptionText() != "Tags do usuário" {
		t.Fatalf("Tags field DescriptionText() = %q", tagsPB.DescriptionText())
	}

	// Scores []int
	scoresPB := m.Property(&zero.Scores)
	scoresAM := scoresPB.Array()
	scoresAM.Items(func(mm *schema.ArraySchema) {
		mm.Integer().Min(0).Max(100)
		mm.Required()
		mm.Description("Notas do usuário")
		mm.Examples(80, 95)
	})
	if scoresPB.FormatValue() != "array" {
		t.Fatalf("Scores field FormatValue() = %q, want %q", scoresPB.FormatValue(), "array")
	}
	if scoresAM.ItemBuilder().FormatValue() != "int64" {
		t.Fatalf("Scores item FormatValue() = %q, want %q", scoresAM.ItemBuilder().FormatValue(), "int64")
	}

	// Addresses []AddressEntity
	addrPB := m.Property(&zero.Addresses)
	addrAM := addrPB.Array()
	addrAM.Items(func(mm *schema.ArraySchema) {
		mm.Object(addressSchema)
		mm.Required()
		mm.Description("Endereços do usuário")
	}).Min(1)

	if addrPB.FormatValue() != "array" {
		t.Fatalf("Addresses field FormatValue() = %q, want %q", addrPB.FormatValue(), "array")
	}
	ref, ok := addrAM.ItemRef()
	if !ok || ref != addressSchema {
		t.Fatalf("Addresses ItemRef() = (%v, %v), want (addressSchema, true)", ref, ok)
	}
	minV, ok := addrAM.MinValue()
	if !ok || minV != 1 {
		t.Fatalf("Addresses quantity MinValue() = (%d, %v), want (1, true)", minV, ok)
	}
	if !addrPB.IsRequired() {
		t.Fatal("Addresses field IsRequired() = false, want true")
	}
}
