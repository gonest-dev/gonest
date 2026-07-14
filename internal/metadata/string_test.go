package metadata_test

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/gonest-dev/gonest/internal/metadata"
)

// stringBranchEntity exercises all 10 string-family branches on distinct
// fields, plus INSIGHT.md's own two verbatim chain examples (Email/Zip),
// reused across this file's tests (spec.md's Independent Test: "construct a
// minimal struct exercising all 10 to prove none of the 10 was missed or
// behaves differently from the others").
type stringBranchEntity struct {
	Str      string
	Email    string
	Uuid     string
	Uri      string
	Hostname string
	Ipv4     string
	Ipv6     string
	Password string
	Byte     string
	Binary   string
	Zip      string
}

// newStringTestMetadata mirrors metadata_test.go's own newTestMetadata
// helper (same pattern: construct a zero value, keep it alive via the
// returned pointer, build *Metadata from its address).
func newStringTestMetadata(t *testing.T) (*stringBranchEntity, *metadata.Metadata) {
	t.Helper()
	zero := &stringBranchEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { metadata.Deregister(typ) })
	m := metadata.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// TestPropertyBuilder_FormatValueDefaultsEmpty proves a fresh PropertyBuilder
// (before any branch method is called) reports "" from FormatValue, not some
// hidden default.
func TestPropertyBuilder_FormatValueDefaultsEmpty(t *testing.T) {
	zero, m := newStringTestMetadata(t)

	pb := m.Property(&zero.Str)

	if pb.FormatValue() != "" {
		t.Errorf("FormatValue() = %q, want \"\" before any branch method was called", pb.FormatValue())
	}
}

// TestStringFamilyBranches_SetsCorrectFormat proves EACH of the 10 branch
// methods returns a *StringMetadata carrying its own correct OpenAPI format
// string (spec.md AC1 / STR-01) -- individually verified, not sampled.
func TestStringFamilyBranches_SetsCorrectFormat(t *testing.T) {
	tests := []struct {
		name       string
		fieldPtr   func(*stringBranchEntity) *string
		call       func(*metadata.PropertyBuilder) *metadata.StringMetadata
		wantFormat string
	}{
		{"String", func(e *stringBranchEntity) *string { return &e.Str }, (*metadata.PropertyBuilder).String, ""},
		{"Email", func(e *stringBranchEntity) *string { return &e.Email }, (*metadata.PropertyBuilder).Email, "email"},
		{"Uuid", func(e *stringBranchEntity) *string { return &e.Uuid }, (*metadata.PropertyBuilder).Uuid, "uuid"},
		{"Uri", func(e *stringBranchEntity) *string { return &e.Uri }, (*metadata.PropertyBuilder).Uri, "uri"},
		{"Hostname", func(e *stringBranchEntity) *string { return &e.Hostname }, (*metadata.PropertyBuilder).Hostname, "hostname"},
		{"Ipv4", func(e *stringBranchEntity) *string { return &e.Ipv4 }, (*metadata.PropertyBuilder).Ipv4, "ipv4"},
		{"Ipv6", func(e *stringBranchEntity) *string { return &e.Ipv6 }, (*metadata.PropertyBuilder).Ipv6, "ipv6"},
		{"Password", func(e *stringBranchEntity) *string { return &e.Password }, (*metadata.PropertyBuilder).Password, "password"},
		{"Byte", func(e *stringBranchEntity) *string { return &e.Byte }, (*metadata.PropertyBuilder).Byte, "byte"},
		{"Binary", func(e *stringBranchEntity) *string { return &e.Binary }, (*metadata.PropertyBuilder).Binary, "binary"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zero, m := newStringTestMetadata(t)
			pb := m.Property(tt.fieldPtr(zero))

			sm := tt.call(pb)
			if sm == nil {
				t.Fatalf("%s() returned nil *StringMetadata", tt.name)
			}
			if sm.FormatValue() != tt.wantFormat {
				t.Errorf("%s().FormatValue() = %q, want %q", tt.name, sm.FormatValue(), tt.wantFormat)
			}
			// also visible via the original *PropertyBuilder, proving format
			// is stored on the SHARED object, not the disposable wrapper.
			if pb.FormatValue() != tt.wantFormat {
				t.Errorf("after %s(), original PropertyBuilder.FormatValue() = %q, want %q", tt.name, pb.FormatValue(), tt.wantFormat)
			}
		})
	}
}

// TestStringMetadata_MinMaxPatternChainAndStoreCorrectly proves Min/Max/
// Pattern each store their value and each returns the SAME *StringMetadata,
// exercised as a genuine single-expression chain (spec.md AC2 / STR-02).
func TestStringMetadata_MinMaxPatternChainAndStoreCorrectly(t *testing.T) {
	zero, m := newStringTestMetadata(t)

	sm := m.Property(&zero.Zip).String()
	got := sm.Min(1).Max(50).Pattern(`^[a-z]+$`)

	if got != sm {
		t.Fatal("Min/Max/Pattern chain did not return the same *StringMetadata")
	}

	minVal, minOk := sm.MinValue()
	if !minOk || minVal != 1 {
		t.Errorf("MinValue() = (%d, %v), want (1, true)", minVal, minOk)
	}
	maxVal, maxOk := sm.MaxValue()
	if !maxOk || maxVal != 50 {
		t.Errorf("MaxValue() = (%d, %v), want (50, true)", maxVal, maxOk)
	}
	if sm.PatternValue() != `^[a-z]+$` {
		t.Errorf("PatternValue() = %q, want %q", sm.PatternValue(), `^[a-z]+$`)
	}
}

// TestStringMetadata_MinMaxDefaultUnset proves MinValue/MaxValue report
// (0, false) when never called, distinguishing "never set" from "set to 0"
// (design.md's Interfaces: "bool reports whether it was ever set").
func TestStringMetadata_MinMaxDefaultUnset(t *testing.T) {
	zero, m := newStringTestMetadata(t)

	sm := m.Property(&zero.Str).String()

	if v, ok := sm.MinValue(); ok || v != 0 {
		t.Errorf("MinValue() = (%d, %v), want (0, false) before Min() was ever called", v, ok)
	}
	if v, ok := sm.MaxValue(); ok || v != 0 {
		t.Errorf("MaxValue() = (%d, %v), want (0, false) before Max() was ever called", v, ok)
	}
	if sm.PatternValue() != "" {
		t.Errorf("PatternValue() = %q, want \"\" before Pattern() was ever called", sm.PatternValue())
	}
}

// TestStringMetadata_CommonConstraintsMutateSharedBuilderAndStayChainable is
// the MOST CRITICAL test (per the task's own instructions): proves Required/
// Nullable/Description/Examples called ON *StringMetadata (a) mutate the
// SAME shared PropertyBuilder (visible via ITS OWN getters, inherited via
// embedding) and (b) still return *StringMetadata, not *PropertyBuilder --
// proven by chaining .Required().Min(5) in a SINGLE expression. If Required()
// returned *PropertyBuilder, this line would fail to COMPILE (no Min method
// on PropertyBuilder), so a successful compile+pass IS the proof (spec.md
// AC3 / STR-03).
func TestStringMetadata_CommonConstraintsMutateSharedBuilderAndStayChainable(t *testing.T) {
	zero, m := newStringTestMetadata(t)

	sm := m.Property(&zero.Zip).String()

	// The critical line: .Required() must return *StringMetadata so .Min()
	// can chain immediately after in the SAME expression.
	got := sm.Required().Min(5).Nullable().Description("CEP").Examples("01310-100")

	if got != sm {
		t.Fatal("Required().Min(5)... chain did not return the same *StringMetadata")
	}

	// Base-4 constraints are visible via PropertyBuilder's OWN getters
	// (inherited through embedding), proving shared-storage, not a copy.
	if !sm.IsRequired() {
		t.Error("IsRequired() = false, want true after StringMetadata.Required()")
	}
	if !sm.IsNullable() {
		t.Error("IsNullable() = false, want true after StringMetadata.Nullable()")
	}
	if sm.DescriptionText() != "CEP" {
		t.Errorf("DescriptionText() = %q, want %q", sm.DescriptionText(), "CEP")
	}
	examples := sm.ExamplesList()
	if len(examples) != 1 || examples[0] != "01310-100" {
		t.Errorf("ExamplesList() = %v, want [01310-100]", examples)
	}

	// Min(5) itself, chained after Required(), must have stored correctly.
	minVal, minOk := sm.MinValue()
	if !minOk || minVal != 5 {
		t.Errorf("MinValue() = (%d, %v), want (5, true)", minVal, minOk)
	}
}

// TestStringMetadata_InsightEmailChain verifies INSIGHT.md's Email chain
// end-to-end, all values checked.
func TestStringMetadata_InsightEmailChain(t *testing.T) {
	zero, m := newStringTestMetadata(t)

	sm := m.Property(&zero.Email).Email().Required().Description("Email do usuário").Examples("[EMAIL_ADDRESS]")

	if sm.FormatValue() != "email" {
		t.Errorf("FormatValue() = %q, want %q", sm.FormatValue(), "email")
	}
	if !sm.IsRequired() {
		t.Error("IsRequired() = false, want true")
	}
	if sm.DescriptionText() != "Email do usuário" {
		t.Errorf("DescriptionText() = %q, want %q", sm.DescriptionText(), "Email do usuário")
	}
	examples := sm.ExamplesList()
	if len(examples) != 1 || examples[0] != "[EMAIL_ADDRESS]" {
		t.Errorf("ExamplesList() = %v, want [[EMAIL_ADDRESS]]", examples)
	}
}

// TestStringMetadata_InsightZipChain verifies INSIGHT.md's Zip chain
// end-to-end, all values checked.
func TestStringMetadata_InsightZipChain(t *testing.T) {
	zero, m := newStringTestMetadata(t)

	sm := m.Property(&zero.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`).Description("CEP").Examples("01310-100")

	if sm.FormatValue() != "" {
		t.Errorf("FormatValue() = %q, want \"\" for bare String()", sm.FormatValue())
	}
	if !sm.IsRequired() {
		t.Error("IsRequired() = false, want true")
	}
	if sm.PatternValue() != `^\d{5}-?\d{3}$` {
		t.Errorf("PatternValue() = %q, want %q", sm.PatternValue(), `^\d{5}-?\d{3}$`)
	}
	if sm.DescriptionText() != "CEP" {
		t.Errorf("DescriptionText() = %q, want %q", sm.DescriptionText(), "CEP")
	}
	examples := sm.ExamplesList()
	if len(examples) != 1 || examples[0] != "01310-100" {
		t.Errorf("ExamplesList() = %v, want [01310-100]", examples)
	}
}

// TestStringFamilyBranches_CalledTwiceLastWins proves calling two branch
// methods in sequence on the SAME *PropertyBuilder does not panic, and
// FormatValue reflects the LAST call (design.md's Error Handling Strategy:
// "last-write-wins, no panic, deterministic").
func TestStringFamilyBranches_CalledTwiceLastWins(t *testing.T) {
	zero, m := newStringTestMetadata(t)

	pb := m.Property(&zero.Str)

	firstSm := pb.String()
	if firstSm.FormatValue() != "" {
		t.Fatalf("after first branch call String(), FormatValue() = %q, want \"\"", firstSm.FormatValue())
	}

	secondSm := pb.Email()
	if secondSm.FormatValue() != "email" {
		t.Fatalf("after second branch call Email(), FormatValue() = %q, want %q", secondSm.FormatValue(), "email")
	}

	// The FIRST wrapper's FormatValue must ALSO reflect the second call,
	// since both wrappers share the same underlying PropertyBuilder.
	if firstSm.FormatValue() != "email" {
		t.Errorf("first wrapper's FormatValue() = %q after second branch call, want %q (shared storage)", firstSm.FormatValue(), "email")
	}
	if pb.FormatValue() != "email" {
		t.Errorf("original PropertyBuilder.FormatValue() = %q, want %q", pb.FormatValue(), "email")
	}
}
