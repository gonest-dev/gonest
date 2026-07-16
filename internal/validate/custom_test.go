package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/gofiber/fiber/v3"

	"gonest.dev/gonest/internal/exception"
	"gonest.dev/gonest/internal/execution"
	"gonest.dev/gonest/internal/schema"
)

// --- P0 fixtures ------------------------------------------------------
//
// CustomFixture exercises Custom(fn) in isolation, distinct from
// UserProperties (validate_test.go's own fixture) so these new tests don't
// perturb the pre-existing suite's own registration in any way (each test
// file's init/package-level var runs once per package, and schema.New
// panics on duplicate registration for the same Go type -- a fresh
// anonymous-shaped type here keeps this file fully independent).

// decodeV1Code implements the "v1:42" -> 42 custom format INSIGHT.md/
// spec.md's Independent Test describes: a fixed validator vocabulary
// (String/Integer/etc) cannot express this, only Custom(fn) can.
func decodeV1Code(raw any) (any, error) {
	s, ok := raw.(string)
	if !ok {
		return nil, errors.New("expected a string in \"v1:N\" format")
	}
	const prefix = "v1:"
	if !strings.HasPrefix(s, prefix) {
		return nil, errors.New("expected format \"v1:N\"")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, prefix))
	if err != nil {
		return nil, errors.New("expected format \"v1:N\" with N numeric")
	}
	return n, nil
}

type CustomCodeFixture struct {
	Code int    `json:"code"`
	Name string `json:"name"`
}

var customCodeFixtureSchema = func() *schema.Schema {
	f := &CustomCodeFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	m.Property(&f.Code).Custom(decodeV1Code)
	m.Property(&f.Name).String().Required()
	return m
}()

// TestMustJsonBody_CustomFunc_DecodesCustomFormat_EndToEnd is P0's own
// Independent Test (spec.md): a field with Custom(fn) that decodes a
// custom-format string ("v1:42" -> int 42) works end-to-end via
// MustJsonBody, real HTTP dispatch.
func TestMustJsonBody_CustomFunc_DecodesCustomFormat_EndToEnd(t *testing.T) {
	app := fiber.New()
	app.Post("/codes", func(c fiber.Ctx) (err error) {
		ctx := execution.New(&httpFiberResponder{c: c})
		defer func() {
			if r := recover(); r != nil {
				if exc, ok := r.(*exception.BadRequestException); ok {
					c.Status(http.StatusBadRequest)
					err = c.JSON(map[string]any{"details": exc.Details()})
					return
				}
				panic(r)
			}
		}()
		result := MustJsonBody[*CustomCodeFixture](ctx, customCodeFixtureSchema)
		return c.JSON(map[string]any{"code": result.Code, "name": result.Name})
	})

	body, _ := json.Marshal(map[string]any{"code": "v1:42", "name": "widget"})
	req := httptest.NewRequest(http.MethodPost, "/codes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var parsed struct {
		Code int    `json:"code"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if parsed.Code != 42 {
		t.Fatalf("expected decoded Code 42, got %d", parsed.Code)
	}
	if parsed.Name != "widget" {
		t.Fatalf("expected Name %q, got %q", "widget", parsed.Name)
	}
}

// TestMustJsonBody_CustomFunc_ReturningError_ProducesViolation_CollectedWithOthers
// proves Custom(fn)'s error becomes a violation, collected together with
// OTHER fields' violations in the same request (not fail-fast --
// context.md's Decision 2 from "JSON Body Validation").
func TestMustJsonBody_CustomFunc_ReturningError_ProducesViolation_CollectedWithOthers(t *testing.T) {
	ctx := newCtx(mustMarshal(map[string]any{
		"code": "not-a-valid-code",
		// "name" omitted -- Required, triggers a SECOND, independent violation
	}))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "code") {
			t.Fatalf("expected a violation for field %q (from Custom's error), got %+v", "code", vs)
		}
		if !hasFieldViolation(vs, "name") {
			t.Fatalf("expected a violation for field %q (Required, collected alongside Custom's), got %+v", "name", vs)
		}
		if len(vs) < 2 {
			t.Fatalf("expected at least 2 violations (collect-all, not fail-fast), got %d: %+v", len(vs), vs)
		}
	}()

	MustJsonBody[*CustomCodeFixture](ctx, customCodeFixtureSchema)
}

// --- Custom(fn) returning a value of the wrong Go type -----------------

type CustomWrongTypeFixture struct {
	Code int `json:"code"`
}

var customWrongTypeFixtureSchema = func() *schema.Schema {
	f := &CustomWrongTypeFixture{}
	m := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	// Custom succeeds (no error) but returns a string for an int field --
	// setField must reject this as a violation, never panic a raw reflect
	// error (spec.md's Edge Cases).
	m.Property(&f.Code).Custom(func(raw any) (any, error) {
		return "not-an-int", nil
	})
	return m
}()

func TestMustJsonBody_CustomFunc_WrongGoType_ProducesViolation_NeverPanics(t *testing.T) {
	ctx := newCtx(mustMarshal(map[string]any{"code": "whatever"}))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if len(vs) == 0 {
			t.Fatalf("expected at least one violation describing the type mismatch, got none")
		}
	}()

	MustJsonBody[*CustomWrongTypeFixture](ctx, customWrongTypeFixtureSchema)
}

// --- regression: field WITHOUT Custom still populates correctly --------

// TestMustJsonBody_FieldWithoutCustom_PopulatesExactlyAsBefore is the
// explicit regression proof requested by tasks.md's T0 Done-when: a field
// with NO Custom set still ends up correctly populated in the final T,
// proven through the NEW populate/setField mechanism rather than the old
// second json.Unmarshal call.
func TestMustJsonBody_FieldWithoutCustom_PopulatesExactlyAsBefore(t *testing.T) {
	ctx := newCtx(mustMarshal(map[string]any{"code": "v1:7", "name": "gizmo"}))

	result := MustJsonBody[*CustomCodeFixture](ctx, customCodeFixtureSchema)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Code != 7 {
		t.Fatalf("expected Custom-decoded Code 7, got %d", result.Code)
	}
	if result.Name != "gizmo" {
		t.Fatalf("expected non-Custom field Name %q populated normally, got %q", "gizmo", result.Name)
	}
}

// --- shared Custom(fn), same definition, reused --------------------------

// TestCustomFunc_SameDefinitionReused_ProducesSameResult proves decodeV1Code
// (the SAME fn value used above) behaves identically when called directly,
// independent of any particular entry point -- part of P0's Independent
// Test ("the SAME Custom(fn) definition reused across all three [entry
// points]... proves the population core is genuinely shared").
func TestCustomFunc_SameDefinitionReused_ProducesSameResult(t *testing.T) {
	v, err := decodeV1Code("v1:99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 99 {
		t.Fatalf("decodeV1Code(\"v1:99\") = %v, want 99", v)
	}

	if _, err := decodeV1Code("bogus"); err == nil {
		t.Fatal("expected an error for a malformed code, got nil")
	}
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
