package validate

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"gonest.dev/gonest/internal/schema"
)

// --- fixtures ---------------------------------------------------------

// DurationBodyFixture exercises Duration() through the JSON body source,
// with Min/Max bounds so out-of-range values are exercised too.
type DurationBodyFixture struct {
	RelayInterval time.Duration `json:"relayInterval"`
}

var durationBodySchema = func() *schema.Schema {
	f := &DurationBodyFixture{}
	s := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	s.Property(&f.RelayInterval).Duration().Required().Min(1 * time.Second).Max(1 * time.Hour)
	return s
}()

// DurationParamFixture exercises Duration() through the params source (raw
// path param strings, coerceParamString's "duration" pass-through case).
type DurationParamFixture struct {
	Timeout time.Duration `param:"timeout"`
}

var durationParamSchema = func() *schema.Schema {
	f := &DurationParamFixture{}
	s := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	s.Property(&f.Timeout).Duration().Required()
	return s
}()

// DurationEnvFixture exercises Duration() through ParseEnvInto, including a
// Default(time.Duration) value used when the env var is absent (DUR-Edge:
// Default bypasses string parsing entirely).
type DurationEnvFixture struct {
	RelayInterval time.Duration `env:"ENV_DUR_RELAY_INTERVAL"`
}

var durationEnvSchema = func() *schema.Schema {
	f := &DurationEnvFixture{}
	s := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	s.Property(&f.RelayInterval).Duration().Default(5 * time.Second)
	return s
}()

// DurationEnumFixture exercises Duration().Enum(...) through the JSON body
// source.
type DurationEnumFixture struct {
	Interval time.Duration `json:"interval"`
}

var durationEnumSchema = func() *schema.Schema {
	f := &DurationEnumFixture{}
	s := schema.New(reflect.TypeOf(*f), uintptr(unsafe.Pointer(f)))
	s.Property(&f.Interval).Duration().Required().Enum(5*time.Second, 10*time.Second)
	return s
}()

// --- JSON body (DUR-03/04/05) ------------------------------------------

func TestDuration_JSONBody_HappyPath_Populates(t *testing.T) {
	ctx := newCtx([]byte(`{"relayInterval":"5s"}`))

	result := mustParseJSON[DurationBodyFixture](ctx, durationBodySchema)

	if result.RelayInterval != 5*time.Second {
		t.Fatalf("expected RelayInterval %v, got %v", 5*time.Second, result.RelayInterval)
	}
}

// TestDuration_JSONBody_MalformedString_ProducesFieldViolation proves a
// malformed duration string surfaces as a per-field violation at the
// VALIDATE stage (DUR-04) -- not a populate-stage crash/generic error.
func TestDuration_JSONBody_MalformedString_ProducesFieldViolation(t *testing.T) {
	ctx := newCtx([]byte(`{"relayInterval":"not-a-duration"}`))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "relayInterval") {
			t.Fatalf("expected a violation for field %q, got %+v", "relayInterval", vs)
		}
	}()

	mustParseJSON[DurationBodyFixture](ctx, durationBodySchema)
}

// TestDuration_JSONBody_BelowMin_ProducesFieldViolation proves Min compares
// the PARSED duration value, not the raw string's length (DUR-05):
// "500ms" is 4 characters (shorter strings would pass a length check) but
// its VALUE is below Min(1s).
func TestDuration_JSONBody_BelowMin_ProducesFieldViolation(t *testing.T) {
	ctx := newCtx([]byte(`{"relayInterval":"500ms"}`))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "relayInterval") {
			t.Fatalf("expected a violation for field %q, got %+v", "relayInterval", vs)
		}
	}()

	mustParseJSON[DurationBodyFixture](ctx, durationBodySchema)
}

func TestDuration_JSONBody_AboveMax_ProducesFieldViolation(t *testing.T) {
	ctx := newCtx([]byte(`{"relayInterval":"2h"}`))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "relayInterval") {
			t.Fatalf("expected a violation for field %q, got %+v", "relayInterval", vs)
		}
	}()

	mustParseJSON[DurationBodyFixture](ctx, durationBodySchema)
}

func TestDuration_JSONBody_EnumViolation(t *testing.T) {
	ctx := newCtx([]byte(`{"interval":"1m"}`)) // not in {5s, 10s}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		exc := expectBadRequest(t, r)
		vs := violationsOf(t, exc)
		if !hasFieldViolation(vs, "interval") {
			t.Fatalf("expected a violation for field %q, got %+v", "interval", vs)
		}
	}()

	mustParseJSON[DurationEnumFixture](ctx, durationEnumSchema)
}

func TestDuration_JSONBody_EnumAllowedValue_Populates(t *testing.T) {
	ctx := newCtx([]byte(`{"interval":"10s"}`))

	result := mustParseJSON[DurationEnumFixture](ctx, durationEnumSchema)

	if result.Interval != 10*time.Second {
		t.Fatalf("expected Interval %v, got %v", 10*time.Second, result.Interval)
	}
}

// --- params (DUR-03, coerceParamString's "duration" pass-through) -------

func TestDuration_Params_HappyPath_Populates(t *testing.T) {
	ctx := newParamCtx("/svc/:timeout", map[string]string{"timeout": "1h30m"})

	result := mustParseParams[DurationParamFixture](ctx, durationParamSchema)

	if result.Timeout != 90*time.Minute {
		t.Fatalf("expected Timeout %v, got %v", 90*time.Minute, result.Timeout)
	}
}

// --- env (DUR-03 + Default bypasses parsing) -----------------------------

func TestDuration_Env_Present_ParsesStringValue(t *testing.T) {
	t.Setenv("ENV_DUR_RELAY_INTERVAL", "30s")

	var dst DurationEnvFixture
	if err := ParseEnvInto(&dst, durationEnvSchema); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dst.RelayInterval != 30*time.Second {
		t.Fatalf("expected RelayInterval %v, got %v", 30*time.Second, dst.RelayInterval)
	}
}

func TestDuration_Env_Absent_UsesTypedDefault(t *testing.T) {
	var dst DurationEnvFixture
	if err := ParseEnvInto(&dst, durationEnvSchema); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dst.RelayInterval != 5*time.Second {
		t.Fatalf("expected Default value %v, got %v", 5*time.Second, dst.RelayInterval)
	}
}
