package schema_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// durationEntity mirrors dateTimeEntity/numericBoolEntity's own helper
// struct pattern (one field per branch exercised across this file's tests).
type durationEntity struct {
	RelayInterval time.Duration
	Timeout       time.Duration
}

func newDurationTestSchema(t *testing.T) (*durationEntity, *schema.Schema) {
	t.Helper()
	zero := &durationEntity{}
	typ := reflect.TypeOf(*zero)
	t.Cleanup(func() { schema.Deregister(typ) })
	s := schema.New(typ, uintptr(unsafe.Pointer(zero)))
	return zero, s
}

// TestDuration_SetsCorrectFormatAndKind proves Duration() returns a
// *DurationSchema carrying format/kind "duration" (DUR-01).
func TestDuration_SetsCorrectFormatAndKind(t *testing.T) {
	zero, s := newDurationTestSchema(t)

	ds := s.Property(&zero.RelayInterval).Duration()
	if ds == nil {
		t.Fatal("Duration() returned nil *DurationSchema")
	}
	if ds.FormatValue() != "duration" {
		t.Errorf("FormatValue() = %q, want %q", ds.FormatValue(), "duration")
	}
	if ds.KindValue() != "duration" {
		t.Errorf("KindValue() = %q, want %q", ds.KindValue(), "duration")
	}
}

// TestDurationSchema_MinMaxChainAndRoundTrip proves Min/Max store and read
// back the exact time.Duration value, each call returning the SAME
// *DurationSchema (DUR-02).
func TestDurationSchema_MinMaxChainAndRoundTrip(t *testing.T) {
	zero, s := newDurationTestSchema(t)

	ds := s.Property(&zero.RelayInterval).Duration()
	got := ds.Min(1 * time.Second).Max(1 * time.Hour)

	if got != ds {
		t.Fatal("Min/Max chain did not return the same *DurationSchema")
	}

	minVal, minOk := ds.MinValue()
	if !minOk || minVal != 1*time.Second {
		t.Errorf("MinValue() = (%v, %v), want (%v, true)", minVal, minOk, time.Second)
	}
	maxVal, maxOk := ds.MaxValue()
	if !maxOk || maxVal != 1*time.Hour {
		t.Errorf("MaxValue() = (%v, %v), want (%v, true)", maxVal, maxOk, time.Hour)
	}
}

// TestDurationSchema_MinMaxDefaultUnset proves MinValue/MaxValue report
// (0, false) when never called.
func TestDurationSchema_MinMaxDefaultUnset(t *testing.T) {
	zero, s := newDurationTestSchema(t)

	ds := s.Property(&zero.RelayInterval).Duration()

	if v, ok := ds.MinValue(); ok || v != 0 {
		t.Errorf("MinValue() = (%v, %v), want (0, false) before Min() was ever called", v, ok)
	}
	if v, ok := ds.MaxValue(); ok || v != 0 {
		t.Errorf("MaxValue() = (%v, %v), want (0, false) before Max() was ever called", v, ok)
	}
}

// TestDurationSchema_EnumChainAndRoundTrip proves Enum stores and reads back
// the exact time.Duration list, returning the SAME *DurationSchema.
func TestDurationSchema_EnumChainAndRoundTrip(t *testing.T) {
	zero, s := newDurationTestSchema(t)

	ds := s.Property(&zero.RelayInterval).Duration()
	got := ds.Required().Enum(5*time.Second, 10*time.Second, 1*time.Minute)

	if got != ds {
		t.Fatal("Required().Enum(...) chain did not return the same *DurationSchema")
	}

	values, ok := ds.EnumValues()
	want := []time.Duration{5 * time.Second, 10 * time.Second, 1 * time.Minute}
	if !ok || !reflect.DeepEqual(values, want) {
		t.Errorf("EnumValues() = (%v, %v), want (%v, true)", values, ok, want)
	}
}

// TestDurationSchema_EnumDefaultUnset proves EnumValues reports (nil, false)
// when Enum was never called.
func TestDurationSchema_EnumDefaultUnset(t *testing.T) {
	zero, s := newDurationTestSchema(t)

	ds := s.Property(&zero.RelayInterval).Duration()

	if values, ok := ds.EnumValues(); ok || values != nil {
		t.Errorf("EnumValues() = (%v, %v), want (nil, false) before Enum() was ever called", values, ok)
	}
}

// TestDurationSchema_CommonConstraintsMutateSharedBuilderAndStayChainable
// proves Required/Nullable/Description/Examples called on *DurationSchema
// mutate the SAME shared PropertyBuilder and still return *DurationSchema --
// proven by chaining .Required().Min(...) in a single expression (mirrors
// numeric_test.go's own most-critical test).
func TestDurationSchema_CommonConstraintsMutateSharedBuilderAndStayChainable(t *testing.T) {
	zero, s := newDurationTestSchema(t)

	ds := s.Property(&zero.RelayInterval).Duration()

	got := ds.Required().Min(5 * time.Second).Nullable().Description("Relay interval").Examples(5 * time.Second)

	if got != ds {
		t.Fatal("Required().Min(...)... chain did not return the same *DurationSchema")
	}
	if !ds.IsRequired() {
		t.Error("IsRequired() = false, want true after DurationSchema.Required()")
	}
	if !ds.IsNullable() {
		t.Error("IsNullable() = false, want true after DurationSchema.Nullable()")
	}
	if ds.DescriptionText() != "Relay interval" {
		t.Errorf("DescriptionText() = %q, want %q", ds.DescriptionText(), "Relay interval")
	}
	examples := ds.ExamplesList()
	if len(examples) != 1 || examples[0] != 5*time.Second {
		t.Errorf("ExamplesList() = %v, want [%v]", examples, 5*time.Second)
	}

	minVal, minOk := ds.MinValue()
	if !minOk || minVal != 5*time.Second {
		t.Errorf("MinValue() = (%v, %v), want (%v, true)", minVal, minOk, 5*time.Second)
	}
}

// TestDurationThenInteger_NoPanicLastWins proves calling Duration() then a
// cross-branch-family method (Integer()) on the SAME *PropertyBuilder does
// not panic, and FormatValue/KindValue reflect the LAST call.
func TestDurationThenInteger_NoPanicLastWins(t *testing.T) {
	zero, s := newDurationTestSchema(t)

	pb := s.Property(&zero.RelayInterval)

	pb.Duration()
	if pb.FormatValue() != "duration" {
		t.Fatalf("after Duration(), FormatValue() = %q, want %q", pb.FormatValue(), "duration")
	}

	nm := pb.Integer()
	if nm.FormatValue() != "int64" {
		t.Errorf("after Duration().Integer(), FormatValue() = %q, want %q (last-write-wins)", nm.FormatValue(), "int64")
	}
	if pb.KindValue() != "integer" {
		t.Errorf("after Duration().Integer(), KindValue() = %q, want %q (last-write-wins)", pb.KindValue(), "integer")
	}
}
