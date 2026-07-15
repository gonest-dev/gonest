package schema_test

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/gonest-dev/gonest/internal/schema"
)

// numericBoolEntity exercises all 4 numeric-family branches plus Boolean on
// distinct fields, plus INSIGHT.md's own two verbatim chain examples
// (Id/IsActive), reused across this file's tests (spec.md's Independent
// Test: reproduce INSIGHT.md's UserEntity.Id/IsActive chains verbatim, plus
// at least one call each to Int32/Float/Double).
type numericBoolEntity struct {
	Id       int64
	Int32Val int32
	FloatVal float32
	DblVal   float64
	IsActive bool
}

// newNumericTestSchema mirrors schema_test.go's own newTestSchema /
// string_test.go's newStringTestSchema helper (same pattern: construct a
// zero value, keep it alive via the returned pointer, build *Schema from
// its address).
func newNumericTestSchema(t *testing.T) (*numericBoolEntity, *schema.Schema) {
	t.Helper()
	zero := &numericBoolEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// TestNumericFamilyBranches_SetsCorrectFormat proves EACH of the 4 numeric
// branch methods returns a *NumericSchema carrying its own correct OpenAPI
// format string (spec.md AC1 / NUM-01) -- individually verified, not
// sampled.
func TestNumericFamilyBranches_SetsCorrectFormat(t *testing.T) {
	tests := []struct {
		name       string
		call       func(*schema.PropertyBuilder) *schema.NumericSchema
		wantFormat string
	}{
		{"Integer", (*schema.PropertyBuilder).Integer, "int64"},
		{"Int32", (*schema.PropertyBuilder).Int32, "int32"},
		{"Float", (*schema.PropertyBuilder).Float, "float"},
		{"Double", (*schema.PropertyBuilder).Double, "double"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zero, m := newNumericTestSchema(t)
			pb := m.Property(&zero.Id)

			nm := tt.call(pb)
			if nm == nil {
				t.Fatalf("%s() returned nil *NumericSchema", tt.name)
			}
			if nm.FormatValue() != tt.wantFormat {
				t.Errorf("%s().FormatValue() = %q, want %q", tt.name, nm.FormatValue(), tt.wantFormat)
			}
			// also visible via the original *PropertyBuilder, proving format
			// is stored on the SHARED object, not the disposable wrapper.
			if pb.FormatValue() != tt.wantFormat {
				t.Errorf("after %s(), original PropertyBuilder.FormatValue() = %q, want %q", tt.name, pb.FormatValue(), tt.wantFormat)
			}
		})
	}
}

// TestNumericSchema_MinMaxChainAndStoreCorrectly proves Min/Max each store
// their value and each returns the SAME *NumericSchema, exercised as a
// genuine single-expression chain (spec.md AC2 / NUM-02).
func TestNumericSchema_MinMaxChainAndStoreCorrectly(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	nm := m.Property(&zero.Id).Integer()
	got := nm.Min(0).Max(100)

	if got != nm {
		t.Fatal("Min/Max chain did not return the same *NumericSchema")
	}

	minVal, minOk := nm.MinValue()
	if !minOk || minVal != 0 {
		t.Errorf("MinValue() = (%d, %v), want (0, true)", minVal, minOk)
	}
	maxVal, maxOk := nm.MaxValue()
	if !maxOk || maxVal != 100 {
		t.Errorf("MaxValue() = (%d, %v), want (100, true)", maxVal, maxOk)
	}
}

// TestNumericSchema_MinMaxDefaultUnset proves MinValue/MaxValue report
// (0, false) when never called, distinguishing "never set" from "set to 0".
func TestNumericSchema_MinMaxDefaultUnset(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	nm := m.Property(&zero.Id).Integer()

	if v, ok := nm.MinValue(); ok || v != 0 {
		t.Errorf("MinValue() = (%d, %v), want (0, false) before Min() was ever called", v, ok)
	}
	if v, ok := nm.MaxValue(); ok || v != 0 {
		t.Errorf("MaxValue() = (%d, %v), want (0, false) before Max() was ever called", v, ok)
	}
}

// TestNumericSchema_CommonConstraintsMutateSharedBuilderAndStayChainable is
// the MOST CRITICAL test (per the task's own instructions): proves Required/
// Nullable/Description/Examples called ON *NumericSchema (a) mutate the
// SAME shared PropertyBuilder (visible via ITS OWN getters, inherited via
// embedding) and (b) still return *NumericSchema, not *PropertyBuilder --
// proven by chaining .Required().Min(5) in a SINGLE expression. If Required()
// returned *PropertyBuilder, this line would fail to COMPILE (no Min method
// on PropertyBuilder), so a successful compile+pass IS the proof (spec.md
// AC3 / NUM-03).
func TestNumericSchema_CommonConstraintsMutateSharedBuilderAndStayChainable(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	nm := m.Property(&zero.Id).Integer()

	// The critical line: .Required() must return *NumericSchema so .Min()
	// can chain immediately after in the SAME expression.
	got := nm.Required().Min(5).Nullable().Description("ID do usuário").Examples(int64(1))

	if got != nm {
		t.Fatal("Required().Min(5)... chain did not return the same *NumericSchema")
	}

	// Base-4 constraints are visible via PropertyBuilder's OWN getters
	// (inherited through embedding), proving shared-storage, not a copy.
	if !nm.IsRequired() {
		t.Error("IsRequired() = false, want true after NumericSchema.Required()")
	}
	if !nm.IsNullable() {
		t.Error("IsNullable() = false, want true after NumericSchema.Nullable()")
	}
	if nm.DescriptionText() != "ID do usuário" {
		t.Errorf("DescriptionText() = %q, want %q", nm.DescriptionText(), "ID do usuário")
	}
	examples := nm.ExamplesList()
	if len(examples) != 1 || examples[0] != int64(1) {
		t.Errorf("ExamplesList() = %v, want [1]", examples)
	}

	// Min(5) itself, chained after Required(), must have stored correctly.
	minVal, minOk := nm.MinValue()
	if !minOk || minVal != 5 {
		t.Errorf("MinValue() = (%d, %v), want (5, true)", minVal, minOk)
	}
}

// TestNumericSchema_InsightIdChain verifies INSIGHT.md's Id chain
// end-to-end, all values checked.
func TestNumericSchema_InsightIdChain(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	nm := m.Property(&zero.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))

	if nm.FormatValue() != "int64" {
		t.Errorf("FormatValue() = %q, want %q", nm.FormatValue(), "int64")
	}
	if !nm.IsRequired() {
		t.Error("IsRequired() = false, want true")
	}
	if nm.DescriptionText() != "ID do usuário" {
		t.Errorf("DescriptionText() = %q, want %q", nm.DescriptionText(), "ID do usuário")
	}
	examples := nm.ExamplesList()
	if len(examples) != 1 || examples[0] != int64(1) {
		t.Errorf("ExamplesList() = %v, want [1]", examples)
	}
}

// TestBoolean_ReturnsSamePropertyBuilder proves Boolean() returns the SAME
// *PropertyBuilder value (identity, not a copy or a new wrapper type) --
// spec.md AC4 / NUM-04, the feature's genuinely new architectural claim.
func TestBoolean_ReturnsSamePropertyBuilder(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	pb := m.Property(&zero.IsActive)
	got := pb.Boolean()

	if got != pb {
		t.Fatal("Boolean() did not return the SAME *PropertyBuilder (identity check failed)")
	}
	if got.FormatValue() != "" {
		t.Errorf("FormatValue() = %q, want \"\" after Boolean()", got.FormatValue())
	}
}

// TestBoolean_CommonConstraintsWork proves Required/Nullable/Description/
// Examples work normally on Boolean()'s return value -- trivially true since
// it IS *PropertyBuilder, but spec.md's Independent Test demands this be
// proven explicitly, not just assumed, as the FIRST branch with no wrapper.
func TestBoolean_CommonConstraintsWork(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	pb := m.Property(&zero.IsActive).Boolean().Required().Nullable().Description("Status do usuário").Examples(true)

	if !pb.IsRequired() {
		t.Error("IsRequired() = false, want true")
	}
	if !pb.IsNullable() {
		t.Error("IsNullable() = false, want true")
	}
	if pb.DescriptionText() != "Status do usuário" {
		t.Errorf("DescriptionText() = %q, want %q", pb.DescriptionText(), "Status do usuário")
	}
	examples := pb.ExamplesList()
	if len(examples) != 1 || examples[0] != true {
		t.Errorf("ExamplesList() = %v, want [true]", examples)
	}
}

// TestNumericSchema_InsightIsActiveChain verifies INSIGHT.md's IsActive
// chain end-to-end, all values checked, using Boolean() directly (mirrors
// TestNumericSchema_InsightIdChain's Integer() equivalent).
func TestNumericSchema_InsightIsActiveChain(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	pb := m.Property(&zero.IsActive).Boolean().Required().Description("Status do usuário").Examples(true)

	if pb.FormatValue() != "" {
		t.Errorf("FormatValue() = %q, want \"\"", pb.FormatValue())
	}
	if !pb.IsRequired() {
		t.Error("IsRequired() = false, want true")
	}
	if pb.DescriptionText() != "Status do usuário" {
		t.Errorf("DescriptionText() = %q, want %q", pb.DescriptionText(), "Status do usuário")
	}
	examples := pb.ExamplesList()
	if len(examples) != 1 || examples[0] != true {
		t.Errorf("ExamplesList() = %v, want [true]", examples)
	}
}

// TestNumericFamilyBranches_CalledTwiceLastWins proves calling two branch
// methods in sequence on the SAME *PropertyBuilder does not panic, and
// FormatValue reflects the LAST call (design.md's Error Handling Strategy:
// "last-write-wins, no panic").
func TestNumericFamilyBranches_CalledTwiceLastWins(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	pb := m.Property(&zero.Id)

	firstNm := pb.Integer()
	if firstNm.FormatValue() != "int64" {
		t.Fatalf("after first branch call Integer(), FormatValue() = %q, want %q", firstNm.FormatValue(), "int64")
	}

	secondNm := pb.Float()
	if secondNm.FormatValue() != "float" {
		t.Fatalf("after second branch call Float(), FormatValue() = %q, want %q", secondNm.FormatValue(), "float")
	}

	// The FIRST wrapper's FormatValue must ALSO reflect the second call,
	// since both wrappers share the same underlying PropertyBuilder.
	if firstNm.FormatValue() != "float" {
		t.Errorf("first wrapper's FormatValue() = %q after second branch call, want %q (shared storage)", firstNm.FormatValue(), "float")
	}
	if pb.FormatValue() != "float" {
		t.Errorf("original PropertyBuilder.FormatValue() = %q, want %q", pb.FormatValue(), "float")
	}
}

// TestIntegerThenBoolean_NoPanicLastWins proves calling Integer() then
// Boolean() (or vice versa) on the SAME *PropertyBuilder does not panic, and
// FormatValue reflects the LAST call -- spec.md's Edge Cases: "no
// cross-branch-family special-casing".
func TestIntegerThenBoolean_NoPanicLastWins(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	pb := m.Property(&zero.Id)

	pb.Integer()
	if pb.FormatValue() != "int64" {
		t.Fatalf("after Integer(), FormatValue() = %q, want %q", pb.FormatValue(), "int64")
	}

	got := pb.Boolean()
	if got != pb {
		t.Fatal("Boolean() after Integer() did not return the same *PropertyBuilder")
	}
	if pb.FormatValue() != "" {
		t.Errorf("after Integer().Boolean(), FormatValue() = %q, want \"\" (last-write-wins)", pb.FormatValue())
	}
}

// TestBooleanThenInteger_NoPanicLastWins is the reverse of
// TestIntegerThenBoolean_NoPanicLastWins: Boolean() first, then Integer(),
// proving FormatValue ends up "int64", no panic either direction.
func TestBooleanThenInteger_NoPanicLastWins(t *testing.T) {
	zero, m := newNumericTestSchema(t)

	pb := m.Property(&zero.Id)

	pb.Boolean()
	if pb.FormatValue() != "" {
		t.Fatalf("after Boolean(), FormatValue() = %q, want \"\"", pb.FormatValue())
	}

	nm := pb.Integer()
	if nm.FormatValue() != "int64" {
		t.Errorf("after Boolean().Integer(), FormatValue() = %q, want %q (last-write-wins)", nm.FormatValue(), "int64")
	}
}
