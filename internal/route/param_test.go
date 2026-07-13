package route

import "testing"

func TestDefaultCoerce_String(t *testing.T) {
	got, err := defaultCoerce[string]("hello")
	if err != nil {
		t.Fatalf("defaultCoerce[string](%q) error = %v, want nil", "hello", err)
	}
	if got != "hello" {
		t.Fatalf("defaultCoerce[string](%q) = %q, want %q", "hello", got, "hello")
	}
}

func TestDefaultCoerce_Int(t *testing.T) {
	got, err := defaultCoerce[int]("42")
	if err != nil {
		t.Fatalf("defaultCoerce[int](%q) error = %v, want nil", "42", err)
	}
	if got != 42 {
		t.Fatalf("defaultCoerce[int](%q) = %d, want %d", "42", got, 42)
	}
}

func TestDefaultCoerce_Int64(t *testing.T) {
	got, err := defaultCoerce[int64]("9223372036854775807")
	if err != nil {
		t.Fatalf("defaultCoerce[int64](...) error = %v, want nil", err)
	}
	if got != 9223372036854775807 {
		t.Fatalf("defaultCoerce[int64](...) = %d, want %d", got, int64(9223372036854775807))
	}
}

func TestDefaultCoerce_Bool(t *testing.T) {
	got, err := defaultCoerce[bool]("true")
	if err != nil {
		t.Fatalf("defaultCoerce[bool](%q) error = %v, want nil", "true", err)
	}
	if got != true {
		t.Fatalf("defaultCoerce[bool](%q) = %v, want %v", "true", got, true)
	}
}

func TestDefaultCoerce_Float64(t *testing.T) {
	got, err := defaultCoerce[float64]("3.14")
	if err != nil {
		t.Fatalf("defaultCoerce[float64](%q) error = %v, want nil", "3.14", err)
	}
	if got != 3.14 {
		t.Fatalf("defaultCoerce[float64](%q) = %v, want %v", "3.14", got, 3.14)
	}
}

func TestDefaultCoerce_InvalidConversion(t *testing.T) {
	_, err := defaultCoerce[int]("not-a-number")
	if err == nil {
		t.Fatalf("defaultCoerce[int](%q) error = nil, want non-nil", "not-a-number")
	}
}

// Context.Param (internal/httpctx) returns "" for a param name that doesn't
// exist on the current route (Fiber's c.Params semantics: unmatched key
// yields the zero value, not an error). defaultCoerce itself has no
// knowledge of "this route" — it only sees the raw string handed to it.
// For string T, an empty raw value converts successfully to "" since empty
// string IS a valid string value; there is no way for defaultCoerce alone to
// distinguish "absent param" from "present but empty" for T=string. This is
// exactly why the future public MustParam[T] (T5) must perform its own
// existence check via ctx.Param before calling defaultCoerce (or use a
// non-string T, where an empty raw string reliably fails coercion, as this
// test demonstrates for int).
func TestDefaultCoerce_EmptyRawSignalsAbsenceForNonStringTypes(t *testing.T) {
	_, err := defaultCoerce[int]("")
	if err == nil {
		t.Fatalf("defaultCoerce[int](%q) error = nil, want non-nil (empty raw, as ctx.Param returns for an absent param, must not silently coerce to a valid int)", "")
	}
}
