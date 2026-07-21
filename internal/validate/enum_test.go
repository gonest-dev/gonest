package validate

import (
	"encoding/json"
	"reflect"
	"testing"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// --- T2 fixtures --------------------------------------------------------
//
// EnumFixture exercises Enum() in isolation, distinct from UserProperties
// (validate_test.go's own fixture) so these new tests don't perturb the
// pre-existing suite's own registration in any way (each test file's
// package-level var runs once per package, and schema.New panics on
// duplicate registration for the same Go type -- a fresh struct type here
// keeps this file fully independent). Color combines Enum with Min/Max/
// Pattern on the SAME field, so a single value can violate more than one
// check at once (Done-when's "collect all, not just the first" item); Level
// is a bare numeric Enum with no other constraint.

type EnumFixture struct {
	Color string `json:"color"`
	Level int64  `json:"level"`
	Plain string `json:"plain"`
}

var enumFixtureSchema = func() *schema.Schema {
	f := &EnumFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Color).String().Enum("red", "green", "blue").Min(4).Pattern(`^[a-z]+$`)
	m.Property(&f.Level).Integer().Enum(1, 2, 3)
	// Plain: no Enum() call at all -- proves an un-enum'd field of the same
	// primitive kind is completely unaffected by this feature.
	m.Property(&f.Plain).String().Required()
	return m
}()

func enumFixtureBody(color string, level int64, plain string) []byte {
	b, _ := json.Marshal(map[string]any{"color": color, "level": level, "plain": plain})
	return b
}

// EnumFixtureNullable: a Nullable String with Enum() set, to prove explicit
// JSON null is still accepted BEFORE validatePrimitive's kind-specific
// (including Enum) checks ever run -- validateValue's null handling is
// unchanged by this task.
type EnumFixtureNullable struct {
	Color *string `json:"color"`
}

var enumFixtureNullableSchema = func() *schema.Schema {
	f := &EnumFixtureNullable{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Color).String().Enum("red", "green", "blue").Nullable()
	return m
}()

// --- string Enum ----------------------------------------------------------

func TestMustJsonBody_StringEnum_AllowedValue_Passes(t *testing.T) {
	ctx := newCtx(enumFixtureBody("green", 2, "ok"))

	result := mustParseJSON[EnumFixture](ctx, enumFixtureSchema)

	if result.Color != "green" {
		t.Fatalf("expected Color %q, got %q", "green", result.Color)
	}
}

func TestMustJsonBody_StringEnum_DisallowedValue_RecordsOneViolation(t *testing.T) {
	// "purple" is long enough (Min(4)) and matches Pattern (`^[a-z]+$`), so
	// Enum is the ONLY check that fires -- proves exactly one violation, not
	// a false-positive pile-up from the other checks on this same field.
	ctx := newCtx(enumFixtureBody("purple", 2, "ok"))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "color") {
			t.Fatalf("expected a violation for field %q, got %+v", "color", vs)
		}
		count := 0
		for _, v := range vs {
			if v.Field == "color" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected exactly 1 violation for field %q, got %d: %+v", "color", count, vs)
		}
	}()

	mustParseJSON[EnumFixture](ctx, enumFixtureSchema)
}

// --- integer Enum -----------------------------------------------------------

func TestMustJsonBody_IntegerEnum_AllowedValue_Passes(t *testing.T) {
	ctx := newCtx(enumFixtureBody("green", 2, "ok"))

	result := mustParseJSON[EnumFixture](ctx, enumFixtureSchema)

	if result.Level != 2 {
		t.Fatalf("expected Level 2, got %d", result.Level)
	}
}

func TestMustJsonBody_IntegerEnum_DisallowedValue_RecordsOneViolation(t *testing.T) {
	ctx := newCtx(enumFixtureBody("green", 5, "ok"))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "level") {
			t.Fatalf("expected a violation for field %q, got %+v", "level", vs)
		}
		count := 0
		for _, v := range vs {
			if v.Field == "level" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected exactly 1 violation for field %q, got %d: %+v", "level", count, vs)
		}
	}()

	mustParseJSON[EnumFixture](ctx, enumFixtureSchema)
}

// --- regression: no Enum() call ---------------------------------------------

func TestMustJsonBody_NoEnumCall_AnyValueOfRightTypeStillPasses(t *testing.T) {
	// Plain has no Enum() call at all -- any string value of the right
	// primitive type must still pass exactly as before this task existed.
	ctx := newCtx(enumFixtureBody("green", 2, "anything at all"))

	result := mustParseJSON[EnumFixture](ctx, enumFixtureSchema)

	if result.Plain != "anything at all" {
		t.Fatalf("expected Plain %q, got %q", "anything at all", result.Plain)
	}
}

// --- combined: Enum + Min/Max/Pattern all violated at once -------------------

func TestMustJsonBody_EnumAndPatternAndMin_AllViolationsCollected(t *testing.T) {
	// "AB" is: too short for Min(4), fails Pattern (`^[a-z]+$`, uppercase),
	// and not in the Enum allow-list -- all three must be reported, proving
	// Enum's check doesn't short-circuit (or get short-circuited by) the
	// pre-existing Min/Max/Pattern checks on the same field.
	ctx := newCtx(enumFixtureBody("AB", 2, "ok"))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)

		var colorViolations []violation
		for _, v := range vs {
			if v.Field == "color" {
				colorViolations = append(colorViolations, v)
			}
		}
		if len(colorViolations) != 3 {
			t.Fatalf("expected 3 violations for field %q (Min+Pattern+Enum), got %d: %+v", "color", len(colorViolations), colorViolations)
		}
	}()

	mustParseJSON[EnumFixture](ctx, enumFixtureSchema)
}

// --- Nullable + explicit null on an Enum'd field ----------------------------

func TestMustJsonBody_NullableEnumField_ExplicitNull_Accepted(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"color": nil})
	ctx := newCtx(body)

	result := mustParseJSON[EnumFixtureNullable](ctx, enumFixtureNullableSchema)

	if result.Color != nil {
		t.Fatalf("expected Color to remain nil, got %v", *result.Color)
	}
}
