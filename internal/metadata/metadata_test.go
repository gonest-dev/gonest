package metadata_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/gonest-dev/gonest/internal/metadata"
)

// userEntity reproduces INSIGHT.md's 7-field UserEntity example verbatim
// (spec.md's Independent Test), used to empirically confirm the
// pointer-offset field identification algorithm works for every field kind
// INSIGHT.md's own example uses: int64, string, bool, time.Time, *time.Time.
type userEntity struct {
	Id        int64
	Name      string
	Email     string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// newTestMetadata simulates what T2's NewMetadata[T] wrapper (root package,
// out of scope for this task) will do: construct a zero value, take its
// address as baseAddr, build a *Metadata for it. zero is returned alongside
// m so the caller can keep it alive/addressable for the whole test -- taking
// &zero's field addresses after this helper returns is safe because Go's
// escape analysis moves zero to the heap the moment its address is taken and
// handed outside this function's frame (via the returned pointer).
func newTestMetadata(t *testing.T) (*userEntity, *metadata.Metadata) {
	t.Helper()
	zero := &userEntity{}
	m := metadata.New(reflect.TypeOf(*zero), uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// TestProperty_IdentifiesEachFieldCorrectly proves the pointer-offset
// algorithm (design.md's "Field identification algorithm") correctly
// identifies EVERY one of userEntity's 7 fields, individually, by name and
// type -- not just "compiles". This is the empirical confirmation
// design.md's Tech Decisions table flags as "not independently re-verified
// this session" -- this test IS that verification.
func TestProperty_IdentifiesEachFieldCorrectly(t *testing.T) {
	zero, m := newTestMetadata(t)

	tests := []struct {
		name     string
		fieldPtr any
		wantName string
		wantType reflect.Type
	}{
		{"Id", &zero.Id, "Id", reflect.TypeOf(int64(0))},
		{"Name", &zero.Name, "Name", reflect.TypeOf("")},
		{"Email", &zero.Email, "Email", reflect.TypeOf("")},
		{"IsActive", &zero.IsActive, "IsActive", reflect.TypeOf(false)},
		{"CreatedAt", &zero.CreatedAt, "CreatedAt", reflect.TypeOf(time.Time{})},
		{"UpdatedAt", &zero.UpdatedAt, "UpdatedAt", reflect.TypeOf(time.Time{})},
		{"DeletedAt", &zero.DeletedAt, "DeletedAt", reflect.TypeOf((*time.Time)(nil))},
	}

	for _, tt := range tests {
		pb := m.Property(tt.fieldPtr)
		if pb == nil {
			t.Fatalf("Property(&zero.%s) returned nil", tt.name)
		}
		field := pb.Field()
		if field.Name != tt.wantName {
			t.Errorf("Property(&zero.%s).Field().Name = %q, want %q (field misidentified)", tt.name, field.Name, tt.wantName)
		}
		if field.Type != tt.wantType {
			t.Errorf("Property(&zero.%s).Field().Type = %v, want %v", tt.name, field.Type, tt.wantType)
		}
	}
}

// TestProperty_DoesNotSwapNeighboringFields proves that registering Name
// does not accidentally alias with Email (its struct neighbor) -- the
// specific failure mode design.md's offset technique could produce if
// offsets were computed or compared incorrectly.
func TestProperty_DoesNotSwapNeighboringFields(t *testing.T) {
	zero, m := newTestMetadata(t)

	namePb := m.Property(&zero.Name)
	emailPb := m.Property(&zero.Email)

	if namePb.Field().Name != "Name" {
		t.Fatalf("Property(&zero.Name).Field().Name = %q, want \"Name\"", namePb.Field().Name)
	}
	if emailPb.Field().Name != "Email" {
		t.Fatalf("Property(&zero.Email).Field().Name = %q, want \"Email\"", emailPb.Field().Name)
	}
	if namePb == emailPb {
		t.Fatal("Property(&zero.Name) and Property(&zero.Email) returned the SAME *PropertyBuilder -- fields were aliased")
	}
}

// TestProperty_ForeignPointerPanics proves Property panics clearly when
// given a pointer that does not belong to the type passed to New (spec.md
// AC3 / MDR-03) -- e.g. a wholly unrelated local variable's address.
func TestProperty_ForeignPointerPanics(t *testing.T) {
	_, m := newTestMetadata(t)

	var unrelated int64 = 42

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for foreign pointer, got none")
		}
	}()
	m.Property(&unrelated)
}

// TestProperty_DuplicateRegistrationPanics proves calling Property twice for
// the SAME field panics with a clear message (design.md's Error Handling
// Strategy: panic chosen over silent merge, since merge semantics are
// ambiguous and INSIGHT.md never demonstrates this case).
func TestProperty_DuplicateRegistrationPanics(t *testing.T) {
	zero, m := newTestMetadata(t)

	m.Property(&zero.Id)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate Property registration, got none")
		}
	}()
	m.Property(&zero.Id)
}

// TestPropertyBuilder_ChainStoresAllFourConstraints proves Required/
// Nullable/Description/Examples all store correctly, each returns the SAME
// *PropertyBuilder (chain continues), and the getters reflect exactly what
// was set -- a real chained call, not four isolated calls.
func TestPropertyBuilder_ChainStoresAllFourConstraints(t *testing.T) {
	zero, m := newTestMetadata(t)

	pb := m.Property(&zero.Name)
	got := pb.Required().Nullable().Description("the user's name").Examples("Alice", "Bob")

	if got != pb {
		t.Fatal("chained calls did not return the same *PropertyBuilder")
	}
	if !pb.IsRequired() {
		t.Error("IsRequired() = false, want true after Required()")
	}
	if !pb.IsNullable() {
		t.Error("IsNullable() = false, want true after Nullable()")
	}
	if pb.DescriptionText() != "the user's name" {
		t.Errorf("DescriptionText() = %q, want %q", pb.DescriptionText(), "the user's name")
	}
	examples := pb.ExamplesList()
	if len(examples) != 2 || examples[0] != "Alice" || examples[1] != "Bob" {
		t.Errorf("ExamplesList() = %v, want [Alice Bob]", examples)
	}
}

// TestPropertyBuilder_DefaultsAreZeroValue proves a fresh PropertyBuilder
// (before any setter call) reports false/empty/nil, not some hidden truthy
// default.
func TestPropertyBuilder_DefaultsAreZeroValue(t *testing.T) {
	zero, m := newTestMetadata(t)

	pb := m.Property(&zero.Email)

	if pb.IsRequired() {
		t.Error("IsRequired() = true before Required() was ever called")
	}
	if pb.IsNullable() {
		t.Error("IsNullable() = true before Nullable() was ever called")
	}
	if pb.DescriptionText() != "" {
		t.Errorf("DescriptionText() = %q, want \"\" before Description() was ever called", pb.DescriptionText())
	}
	if len(pb.ExamplesList()) != 0 {
		t.Errorf("ExamplesList() = %v, want empty before Examples() was ever called", pb.ExamplesList())
	}
}

// TestPropertyBuilder_ExamplesListReturnsDefensiveCopy proves mutating the
// slice returned by ExamplesList does not affect the builder's internal
// state (same mutate-then-recheck pattern as Controller.OwnMiddleware's own
// test).
func TestPropertyBuilder_ExamplesListReturnsDefensiveCopy(t *testing.T) {
	zero, m := newTestMetadata(t)

	pb := m.Property(&zero.Id).Examples(1, 2, 3)

	got := pb.ExamplesList()
	got[0] = 999

	got2 := pb.ExamplesList()
	if got2[0] != 1 {
		t.Fatal("ExamplesList() leaked mutable internal slice: mutation of returned slice affected subsequent call")
	}
}

// TestMetadata_DescriptionIsDistinctFromFieldDescription proves
// Metadata.Description (whole-type) is stored/retrieved independently from
// any individual PropertyBuilder's own Description (spec.md AC5 / MDR-05).
func TestMetadata_DescriptionIsDistinctFromFieldDescription(t *testing.T) {
	zero, m := newTestMetadata(t)

	got := m.Description("Entidade de usuário")
	if got != m {
		t.Fatal("Metadata.Description did not return the same *Metadata for chaining")
	}

	pb := m.Property(&zero.Name)
	pb.Description("the user's name")

	if m.DescriptionText() != "Entidade de usuário" {
		t.Errorf("Metadata.DescriptionText() = %q, want %q", m.DescriptionText(), "Entidade de usuário")
	}
	if pb.DescriptionText() != "the user's name" {
		t.Errorf("PropertyBuilder.DescriptionText() = %q, want %q", pb.DescriptionText(), "the user's name")
	}

	// Setting the field's description must not retroactively affect the
	// type-level description already set above.
	if m.DescriptionText() == pb.DescriptionText() {
		t.Fatal("Metadata-level and field-level Description got conflated")
	}
}

// TestMetadata_DescriptionDefaultsEmpty proves DescriptionText returns ""
// before Description is ever called on a fresh Metadata.
func TestMetadata_DescriptionDefaultsEmpty(t *testing.T) {
	_, m := newTestMetadata(t)

	if m.DescriptionText() != "" {
		t.Errorf("DescriptionText() = %q, want \"\" before Description() was ever called", m.DescriptionText())
	}
}

// TestOwnProperties_ReturnsAllRegisteredFields proves OwnProperties returns
// every PropertyBuilder registered via Property so far.
func TestOwnProperties_ReturnsAllRegisteredFields(t *testing.T) {
	zero, m := newTestMetadata(t)

	m.Property(&zero.Id)
	m.Property(&zero.Name)
	m.Property(&zero.Email)

	got := m.OwnProperties()
	if len(got) != 3 {
		t.Fatalf("OwnProperties() returned %d items, want 3", len(got))
	}
}

// TestOwnProperties_ReturnsCopyNotInternalSlice proves OwnProperties is a
// defensive-copy accessor (same pattern as Controller.OwnMiddleware/
// Module.OwnProviders): mutating the returned slice must not affect
// subsequent calls' results.
func TestOwnProperties_ReturnsCopyNotInternalSlice(t *testing.T) {
	zero, m := newTestMetadata(t)

	idPb := m.Property(&zero.Id)
	m.Property(&zero.Name)

	got := m.OwnProperties()
	got[0] = nil

	got2 := m.OwnProperties()
	for _, pb := range got2 {
		if pb == nil {
			t.Fatal("OwnProperties() leaked mutable internal slice: mutation of returned slice affected subsequent call")
		}
	}
	_ = idPb
}

// TestNew_NonStructTypePanics proves New panics clearly when structType is
// not a struct Kind (spec.md's Edge Cases / Error Handling Strategy table) --
// e.g. reflect.TypeOf(42), a primitive int.
func TestNew_NonStructTypePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for non-struct structType, got none")
		}
	}()
	metadata.New(reflect.TypeOf(42), 0)
}
