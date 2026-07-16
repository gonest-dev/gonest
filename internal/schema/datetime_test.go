package schema_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// dateTimeEntity reproduces INSIGHT.md's UserEntity CreatedAt/UpdatedAt/
// DeletedAt fields (lines ~379-402) plus one bare Date()-branch field, used
// across this file's tests -- mirrors numeric_test.go's numericBoolEntity
// helper struct/newNumericTestSchema pattern for the DateTime/Date
// branches.
type dateTimeEntity struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	BirthDate time.Time
	Id        int64
}

// newDateTimeTestSchema mirrors schema_test.go's own newTestSchema /
// numeric_test.go's newNumericTestSchema helper (same pattern: construct a
// zero value, keep it alive via the returned pointer, build *Schema from
// its address).
func newDateTimeTestSchema(t *testing.T) (*dateTimeEntity, *schema.Schema) {
	t.Helper()
	zero := &dateTimeEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	m := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, m
}

// TestDateTime_ReturnsSamePropertyBuilder proves DateTime() returns the SAME
// *PropertyBuilder value (identity, not a copy or a new wrapper type) and
// sets FormatValue() to "date-time" -- mirrors numeric_test.go's
// TestBoolean_ReturnsSamePropertyBuilder, DateTime/Date being the second
// branch family (after Boolean) with no wrapper type.
func TestDateTime_ReturnsSamePropertyBuilder(t *testing.T) {
	zero, m := newDateTimeTestSchema(t)

	pb := m.Property(&zero.CreatedAt)
	got := pb.DateTime()

	if got != pb {
		t.Fatal("DateTime() did not return the SAME *PropertyBuilder (identity check failed)")
	}
	if got.FormatValue() != "date-time" {
		t.Errorf("FormatValue() = %q, want %q after DateTime()", got.FormatValue(), "date-time")
	}
}

// TestDate_ReturnsSamePropertyBuilder proves Date() returns the SAME
// *PropertyBuilder value and sets FormatValue() to "date".
func TestDate_ReturnsSamePropertyBuilder(t *testing.T) {
	zero, m := newDateTimeTestSchema(t)

	pb := m.Property(&zero.BirthDate)
	got := pb.Date()

	if got != pb {
		t.Fatal("Date() did not return the SAME *PropertyBuilder (identity check failed)")
	}
	if got.FormatValue() != "date" {
		t.Errorf("FormatValue() = %q, want %q after Date()", got.FormatValue(), "date")
	}
}

// TestDateTime_CommonConstraintsWork proves Required/Nullable/Description/
// Examples work normally on DateTime()'s return value -- trivially true
// since it IS *PropertyBuilder, but proven explicitly the same way
// TestBoolean_CommonConstraintsWork does for Boolean().
func TestDateTime_CommonConstraintsWork(t *testing.T) {
	zero, m := newDateTimeTestSchema(t)

	now := time.Now()
	pb := m.Property(&zero.CreatedAt).DateTime().Required().Nullable().Description("Data de criação do usuário").Examples(now)

	if !pb.IsRequired() {
		t.Error("IsRequired() = false, want true")
	}
	if !pb.IsNullable() {
		t.Error("IsNullable() = false, want true")
	}
	if pb.DescriptionText() != "Data de criação do usuário" {
		t.Errorf("DescriptionText() = %q, want %q", pb.DescriptionText(), "Data de criação do usuário")
	}
	examples := pb.ExamplesList()
	if len(examples) != 1 || examples[0] != now {
		t.Errorf("ExamplesList() = %v, want [%v]", examples, now)
	}
}

// TestDateTime_InsightCreatedAtUpdatedAtDeletedAtChains reproduces
// INSIGHT.md's UserEntity CreatedAt/UpdatedAt/DeletedAt chains verbatim
// (lines ~399-401):
//
//	m.Property(&t.CreatedAt).DateTime().Required().Description("Data de criação do usuário").Examples(time.Now())
//	m.Property(&t.UpdatedAt).DateTime().Required().Description("Data de atualização do usuário").Examples(time.Now())
//	m.Property(&t.DeletedAt).DateTime().Nullable().Description("Data de exclusão do usuário").Examples(nil, time.Now())
//
// mirrors numeric_test.go's TestNumericSchema_InsightIsActiveChain.
func TestDateTime_InsightCreatedAtUpdatedAtDeletedAtChains(t *testing.T) {
	zero, m := newDateTimeTestSchema(t)

	now := time.Now()

	createdAt := m.Property(&zero.CreatedAt).DateTime().Required().Description("Data de criação do usuário").Examples(now)
	updatedAt := m.Property(&zero.UpdatedAt).DateTime().Required().Description("Data de atualização do usuário").Examples(now)
	deletedAt := m.Property(&zero.DeletedAt).DateTime().Nullable().Description("Data de exclusão do usuário").Examples(nil, now)

	if createdAt.FormatValue() != "date-time" {
		t.Errorf("CreatedAt: FormatValue() = %q, want %q", createdAt.FormatValue(), "date-time")
	}
	if !createdAt.IsRequired() {
		t.Error("CreatedAt: IsRequired() = false, want true")
	}
	if createdAt.DescriptionText() != "Data de criação do usuário" {
		t.Errorf("CreatedAt: DescriptionText() = %q, want %q", createdAt.DescriptionText(), "Data de criação do usuário")
	}
	if examples := createdAt.ExamplesList(); len(examples) != 1 || examples[0] != now {
		t.Errorf("CreatedAt: ExamplesList() = %v, want [%v]", examples, now)
	}

	if updatedAt.FormatValue() != "date-time" {
		t.Errorf("UpdatedAt: FormatValue() = %q, want %q", updatedAt.FormatValue(), "date-time")
	}
	if !updatedAt.IsRequired() {
		t.Error("UpdatedAt: IsRequired() = false, want true")
	}
	if updatedAt.DescriptionText() != "Data de atualização do usuário" {
		t.Errorf("UpdatedAt: DescriptionText() = %q, want %q", updatedAt.DescriptionText(), "Data de atualização do usuário")
	}
	if examples := updatedAt.ExamplesList(); len(examples) != 1 || examples[0] != now {
		t.Errorf("UpdatedAt: ExamplesList() = %v, want [%v]", examples, now)
	}

	if deletedAt.FormatValue() != "date-time" {
		t.Errorf("DeletedAt: FormatValue() = %q, want %q", deletedAt.FormatValue(), "date-time")
	}
	if !deletedAt.IsNullable() {
		t.Error("DeletedAt: IsNullable() = false, want true")
	}
	if deletedAt.IsRequired() {
		t.Error("DeletedAt: IsRequired() = true, want false")
	}
	if deletedAt.DescriptionText() != "Data de exclusão do usuário" {
		t.Errorf("DeletedAt: DescriptionText() = %q, want %q", deletedAt.DescriptionText(), "Data de exclusão do usuário")
	}
	examples := deletedAt.ExamplesList()
	if len(examples) != 2 || examples[0] != nil || examples[1] != now {
		t.Errorf("DeletedAt: ExamplesList() = %v, want [nil %v]", examples, now)
	}
}

// TestDateThenDateTime_NoPanicLastWins proves calling Date() then DateTime()
// on the SAME *PropertyBuilder does not panic, and FormatValue reflects the
// LAST call -- mirrors numeric_test.go's TestBooleanThenInteger_NoPanicLastWins.
func TestDateThenDateTime_NoPanicLastWins(t *testing.T) {
	zero, m := newDateTimeTestSchema(t)

	pb := m.Property(&zero.CreatedAt)

	pb.Date()
	if pb.FormatValue() != "date" {
		t.Fatalf("after Date(), FormatValue() = %q, want %q", pb.FormatValue(), "date")
	}

	got := pb.DateTime()
	if got != pb {
		t.Fatal("DateTime() after Date() did not return the same *PropertyBuilder")
	}
	if pb.FormatValue() != "date-time" {
		t.Errorf("after Date().DateTime(), FormatValue() = %q, want %q (last-write-wins)", pb.FormatValue(), "date-time")
	}
}

// TestIntegerThenDateTime_NoPanicLastWins proves calling a cross-branch-
// family sequence (Integer() then DateTime()) on the SAME *PropertyBuilder
// does not panic, and FormatValue reflects the LAST call -- spec.md's Edge
// Cases: "no cross-branch-family special-casing" (same claim numeric_test.go's
// TestIntegerThenBoolean_NoPanicLastWins already proves for Integer/Boolean).
func TestIntegerThenDateTime_NoPanicLastWins(t *testing.T) {
	zero, m := newDateTimeTestSchema(t)

	pb := m.Property(&zero.Id)

	pb.Integer()
	if pb.FormatValue() != "int64" {
		t.Fatalf("after Integer(), FormatValue() = %q, want %q", pb.FormatValue(), "int64")
	}

	got := pb.DateTime()
	if got != pb {
		t.Fatal("DateTime() after Integer() did not return the same *PropertyBuilder")
	}
	if pb.FormatValue() != "date-time" {
		t.Errorf("after Integer().DateTime(), FormatValue() = %q, want %q (last-write-wins)", pb.FormatValue(), "date-time")
	}
}
