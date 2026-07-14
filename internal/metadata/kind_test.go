package metadata_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/gonest-dev/gonest/internal/metadata"
)

// kindEntity exercises every branch family (String/Numeric/Boolean/
// DateTime/Date/Array/Object) against real fields, standing in for the same
// role arrayEntity/userEntity play in the sibling _test.go files -- this is
// the P0 (json-body-validation feature) read-path proof: build a *Metadata,
// discard every wrapper reference the declaring closures return, then read
// EVERYTHING back through Metadata.OwnProperties() + PropertyBuilder's own
// (P0-new) getters alone.
type kindEntity struct {
	Name      string
	Age       int
	Score     float64
	Active    bool
	CreatedAt time.Time
	Birthday  time.Time
	Tags      []string
	Address   kindAddressEntity
	Meta      map[string]any
}

type kindAddressEntity struct {
	City string
	Zip  string
}

func newKindTestMetadata(t *testing.T) (*kindEntity, *metadata.Metadata) {
	t.Helper()
	zero := &kindEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { metadata.Deregister(typ) })
	m := metadata.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

func newKindAddressTestMetadata(t *testing.T) *metadata.Metadata {
	t.Helper()
	zero := &kindAddressEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { metadata.Deregister(typ) })
	return metadata.New(typ, uintptr(unsafe.Pointer(zero)))
}

// TestPropertyBuilder_ReadsEveryBranchFamilyWithoutWrapperReference is P0's
// own Independent Test (spec.md): builds a *Metadata via every one of the 4
// branch families (String w/ Min/Max/Pattern, Numeric w/ Min/Max, Array w/
// item+quantity+ItemRef, Object w/ ref and AdditionalProperties), discards
// every wrapper reference the declaring closure returned, then reads
// everything back via Metadata.OwnProperties() + PropertyBuilder's own
// getters alone -- proving the relocation (T0) actually works, not just
// compiles.
func TestPropertyBuilder_ReadsEveryBranchFamilyWithoutWrapperReference(t *testing.T) {
	zero, m := newKindTestMetadata(t)
	addressMetadata := newKindAddressTestMetadata(t)

	// Build via all 4 families, deliberately NOT keeping any *StringMetadata/
	// *NumericMetadata/*ArrayMetadata/*ObjectMetadata reference around --
	// each branch call's return value is discarded immediately (chained and
	// dropped), same as spec.md's Independent Test demands.
	m.Property(&zero.Name).String().Min(1).Max(50).Pattern("^[a-z]+$")
	m.Property(&zero.Age).Integer().Min(0).Max(120)
	m.Property(&zero.Tags).Array().Items(func(am *metadata.ArrayMetadata) {
		am.String().Min(1).Max(10)
	}).Min(1).Max(5)
	m.Property(&zero.Address).Object(func(om *metadata.ObjectMetadata) {
		om.Metadata(addressMetadata)
	})
	m.Property(&zero.Meta).Object(func(om *metadata.ObjectMetadata) {
		om.AdditionalProperties()
	})

	// Now read EVERYTHING back through Metadata.OwnProperties() +
	// PropertyBuilder getters alone -- no wrapper reference survives past
	// this point.
	byField := map[string]*metadata.PropertyBuilder{}
	for _, pb := range m.OwnProperties() {
		byField[pb.Field().Name] = pb
	}

	// String family.
	namePB, ok := byField["Name"]
	if !ok {
		t.Fatal("Name field not found in OwnProperties()")
	}
	if min, ok := namePB.MinValue(); !ok || min != 1 {
		t.Fatalf("Name.MinValue() = (%d, %v), want (1, true)", min, ok)
	}
	if max, ok := namePB.MaxValue(); !ok || max != 50 {
		t.Fatalf("Name.MaxValue() = (%d, %v), want (50, true)", max, ok)
	}
	if pattern := namePB.PatternValue(); pattern != "^[a-z]+$" {
		t.Fatalf("Name.PatternValue() = %q, want %q", pattern, "^[a-z]+$")
	}
	if kind := namePB.KindValue(); kind != "string" {
		t.Fatalf("Name.KindValue() = %q, want %q", kind, "string")
	}

	// Numeric family.
	agePB, ok := byField["Age"]
	if !ok {
		t.Fatal("Age field not found in OwnProperties()")
	}
	if min, ok := agePB.MinValue(); !ok || min != 0 {
		t.Fatalf("Age.MinValue() = (%d, %v), want (0, true)", min, ok)
	}
	if max, ok := agePB.MaxValue(); !ok || max != 120 {
		t.Fatalf("Age.MaxValue() = (%d, %v), want (120, true)", max, ok)
	}
	if kind := agePB.KindValue(); kind != "integer" {
		t.Fatalf("Age.KindValue() = %q, want %q", kind, "integer")
	}

	// Array family: field-level quantity Min/Max + item builder.
	tagsPB, ok := byField["Tags"]
	if !ok {
		t.Fatal("Tags field not found in OwnProperties()")
	}
	if min, ok := tagsPB.MinValue(); !ok || min != 1 {
		t.Fatalf("Tags.MinValue() = (%d, %v), want (1, true)", min, ok)
	}
	if max, ok := tagsPB.MaxValue(); !ok || max != 5 {
		t.Fatalf("Tags.MaxValue() = (%d, %v), want (5, true)", max, ok)
	}
	if kind := tagsPB.KindValue(); kind != "array" {
		t.Fatalf("Tags.KindValue() = %q, want %q", kind, "array")
	}
	item := tagsPB.ItemBuilder()
	if item == nil {
		t.Fatal("Tags.ItemBuilder() = nil, want the synthetic item builder")
	}
	if kind := item.KindValue(); kind != "string" {
		t.Fatalf("Tags item KindValue() = %q, want %q", kind, "string")
	}
	if min, ok := item.MinValue(); !ok || min != 1 {
		t.Fatalf("Tags item MinValue() = (%d, %v), want (1, true)", min, ok)
	}
	if max, ok := item.MaxValue(); !ok || max != 10 {
		t.Fatalf("Tags item MaxValue() = (%d, %v), want (10, true)", max, ok)
	}

	// Object family: ref.
	addressPB, ok := byField["Address"]
	if !ok {
		t.Fatal("Address field not found in OwnProperties()")
	}
	if kind := addressPB.KindValue(); kind != "object" {
		t.Fatalf("Address.KindValue() = %q, want %q", kind, "object")
	}
	ref, ok := addressPB.MetadataRef()
	if !ok || ref != addressMetadata {
		t.Fatalf("Address.MetadataRef() = (%v, %v), want (addressMetadata, true)", ref, ok)
	}
	if addressPB.IsAdditionalProperties() {
		t.Fatal("Address.IsAdditionalProperties() = true, want false")
	}

	// Object family: AdditionalProperties.
	metaPB, ok := byField["Meta"]
	if !ok {
		t.Fatal("Meta field not found in OwnProperties()")
	}
	if kind := metaPB.KindValue(); kind != "object" {
		t.Fatalf("Meta.KindValue() = %q, want %q", kind, "object")
	}
	if !metaPB.IsAdditionalProperties() {
		t.Fatal("Meta.IsAdditionalProperties() = false, want true")
	}
	if _, ok := metaPB.MetadataRef(); ok {
		t.Fatal("Meta.MetadataRef() ok = true, want false (never called)")
	}
}

// TestKindValue_BooleanAndString_AreDifferent proves the core bug this task
// fixes: Boolean() and String() both leave FormatValue() == "" (a
// pre-existing collision, design.md's Tech Decisions), but KindValue() now
// distinguishes them.
func TestKindValue_BooleanAndString_AreDifferent(t *testing.T) {
	zero, m := newKindTestMetadata(t)

	strPB := m.Property(&zero.Name).String().PropertyBuilder
	boolPB := m.Property(&zero.Active).Boolean()

	if strPB.FormatValue() != boolPB.FormatValue() {
		t.Fatalf("expected FormatValue() collision (both empty), got %q vs %q", strPB.FormatValue(), boolPB.FormatValue())
	}
	if strPB.KindValue() == boolPB.KindValue() {
		t.Fatalf("KindValue() collided: String() = %q, Boolean() = %q, want different", strPB.KindValue(), boolPB.KindValue())
	}
	if strPB.KindValue() != "string" {
		t.Fatalf("String().KindValue() = %q, want %q", strPB.KindValue(), "string")
	}
	if boolPB.KindValue() != "boolean" {
		t.Fatalf("Boolean().KindValue() = %q, want %q", boolPB.KindValue(), "boolean")
	}
}

// TestKindValue_EveryBranch proves KindValue() reports the correct OpenAPI
// "type" for every one of the 18 primitive branches, plus Array and Object
// -- 20 cases total, mirroring design.md's Components table exactly.
func TestKindValue_EveryBranch(t *testing.T) {
	addressMetadata := newKindAddressTestMetadata(t)

	tests := []struct {
		name string
		want string
	}{
		{"String", "string"},
		{"Email", "string"},
		{"Uuid", "string"},
		{"Uri", "string"},
		{"Hostname", "string"},
		{"Ipv4", "string"},
		{"Ipv6", "string"},
		{"Password", "string"},
		{"Byte", "string"},
		{"Binary", "string"},
		{"Integer", "integer"},
		{"Int32", "integer"},
		{"Float", "number"},
		{"Double", "number"},
		{"Boolean", "boolean"},
		{"DateTime", "string"},
		{"Date", "string"},
		{"Array", "array"},
		{"Object", "object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each case uses its own throwaway *Metadata + field pointer so
			// Property's own double-registration panic never fires across
			// subtests -- mirrors newKindTestMetadata's own baseAddr
			// alignment requirement (Property requires fieldPtr to belong
			// to m's own struct), so build a fresh single-field struct per
			// case instead of reusing kindEntity's own fields.
			zero := &struct{ V string }{}
			typ := reflect.TypeOf(*zero)
			t.Cleanup(func() { metadata.Deregister(typ) })
			fm := metadata.New(typ, uintptr(unsafe.Pointer(zero)))
			var pb *metadata.PropertyBuilder
			switch tt.name {
			case "String":
				pb = fm.Property(&zero.V).String().PropertyBuilder
			case "Email":
				pb = fm.Property(&zero.V).Email().PropertyBuilder
			case "Uuid":
				pb = fm.Property(&zero.V).Uuid().PropertyBuilder
			case "Uri":
				pb = fm.Property(&zero.V).Uri().PropertyBuilder
			case "Hostname":
				pb = fm.Property(&zero.V).Hostname().PropertyBuilder
			case "Ipv4":
				pb = fm.Property(&zero.V).Ipv4().PropertyBuilder
			case "Ipv6":
				pb = fm.Property(&zero.V).Ipv6().PropertyBuilder
			case "Password":
				pb = fm.Property(&zero.V).Password().PropertyBuilder
			case "Byte":
				pb = fm.Property(&zero.V).Byte().PropertyBuilder
			case "Binary":
				pb = fm.Property(&zero.V).Binary().PropertyBuilder
			case "Integer":
				pb = fm.Property(&zero.V).Integer().PropertyBuilder
			case "Int32":
				pb = fm.Property(&zero.V).Int32().PropertyBuilder
			case "Float":
				pb = fm.Property(&zero.V).Float().PropertyBuilder
			case "Double":
				pb = fm.Property(&zero.V).Double().PropertyBuilder
			case "Boolean":
				pb = fm.Property(&zero.V).Boolean()
			case "DateTime":
				pb = fm.Property(&zero.V).DateTime()
			case "Date":
				pb = fm.Property(&zero.V).Date()
			case "Array":
				pb = fm.Property(&zero.V).Array().PropertyBuilder
			case "Object":
				pb = fm.Property(&zero.V).Object(func(om *metadata.ObjectMetadata) { om.Metadata(addressMetadata) }).PropertyBuilder
			}
			if got := pb.KindValue(); got != tt.want {
				t.Fatalf("%s: KindValue() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestKindValue_ArrayItemBranches_MirrorPropertyBuilder proves KindValue()
// is correct via ArrayMetadata's own 17 mirrored item-branch methods
// (String/.../Boolean/DateTime/Date on the item), reachable through
// ArrayMetadata.ItemBuilder().KindValue() -- the array.go half of T0's
// "kind" line additions (design.md's Components table).
func TestKindValue_ArrayItemBranches_MirrorPropertyBuilder(t *testing.T) {
	tests := []struct {
		name string
		set  func(am *metadata.ArrayMetadata)
		want string
	}{
		{"String", func(am *metadata.ArrayMetadata) { am.String() }, "string"},
		{"Email", func(am *metadata.ArrayMetadata) { am.Email() }, "string"},
		{"Uuid", func(am *metadata.ArrayMetadata) { am.Uuid() }, "string"},
		{"Uri", func(am *metadata.ArrayMetadata) { am.Uri() }, "string"},
		{"Hostname", func(am *metadata.ArrayMetadata) { am.Hostname() }, "string"},
		{"Ipv4", func(am *metadata.ArrayMetadata) { am.Ipv4() }, "string"},
		{"Ipv6", func(am *metadata.ArrayMetadata) { am.Ipv6() }, "string"},
		{"Password", func(am *metadata.ArrayMetadata) { am.Password() }, "string"},
		{"Byte", func(am *metadata.ArrayMetadata) { am.Byte() }, "string"},
		{"Binary", func(am *metadata.ArrayMetadata) { am.Binary() }, "string"},
		{"Integer", func(am *metadata.ArrayMetadata) { am.Integer() }, "integer"},
		{"Int32", func(am *metadata.ArrayMetadata) { am.Int32() }, "integer"},
		{"Float", func(am *metadata.ArrayMetadata) { am.Float() }, "number"},
		{"Double", func(am *metadata.ArrayMetadata) { am.Double() }, "number"},
		{"Boolean", func(am *metadata.ArrayMetadata) { am.Boolean() }, "boolean"},
		{"DateTime", func(am *metadata.ArrayMetadata) { am.DateTime() }, "string"},
		{"Date", func(am *metadata.ArrayMetadata) { am.Date() }, "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zero := &struct{ V []string }{}
			typ := reflect.TypeOf(*zero)
			t.Cleanup(func() { metadata.Deregister(typ) })
			fm := metadata.New(typ, uintptr(unsafe.Pointer(zero)))
			am := fm.Property(&zero.V).Array()
			tt.set(am)
			if got := am.ItemBuilder().KindValue(); got != tt.want {
				t.Fatalf("%s item: KindValue() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
