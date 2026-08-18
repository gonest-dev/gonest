# Duration Branch Specification

## Problem Statement

`Date()`/`DateTime()` proved the "no wrapper, bare `*PropertyBuilder`" pattern for a
string-format branch with no extra validators. A `time.Duration`-typed field
(e.g. `RelayInterval time.Duration`, Go-string form `"5s"`/`"1h30m"`, NOT ISO-8601)
needs the OPPOSITE: a branch WITH numeric-style validators (`Min`/`Max`/`Enum`
comparing the parsed duration VALUE, not the string's length) even though its
external JSON/param/env representation is a string. `kind="string"` cannot be
reused as-is: `validatePrimitive`'s `"string"` case treats `Min`/`Max` as string
LENGTH, which is the wrong semantic for a duration bound. A new `kind="duration"`
is required, with its own `validatePrimitive` case and its own `setField`
string->`time.Duration` conversion (the field's Go type is `int64`-kind, not the
struct-kind `time.Time` that lets `Date`/`DateTime` ride the existing
json-round-trip fallback for free).

## Goals

- [ ] `PropertyBuilder.Duration()` returns a new `*DurationSchema` (embeds
      `*PropertyBuilder`, same redeclare-chain-methods pattern as `NumericSchema`)
      with `format="duration"`, `kind="duration"`
- [ ] `DurationSchema.Min(time.Duration)`/`Max(time.Duration)`/`Enum(...time.Duration)`
      store nanoseconds via the SAME `p.min`/`p.max`/`p.enumInt` fields
      `NumericSchema` already uses -- typed accessors convert back to
      `time.Duration` on read
- [ ] `Required`/`Nullable`/`Description`/`Examples` redeclared to keep the chain
      typed as `*DurationSchema` (same reason `NumericSchema` redeclares them);
      `Default`/`Custom`/`Sanitize` stay promoted from the embedded
      `*PropertyBuilder` (chain ends there, same as `.Integer().Default(5)` today)
- [ ] Every `Parseable` source (JSON body, params, query, headers, form, env)
      accepts a Go-duration-format STRING (`"5s"`) for a `time.Duration` field and
      populates it correctly
- [ ] `validatePrimitive` gains a `"duration"` case: parses the string via
      `time.ParseDuration`, a parse failure becomes a proper per-field
      `violation` (an IMPROVEMENT over `Date`/`DateTime`, whose malformed input
      only surfaces as a generic populate-stage error) -- then compares the
      PARSED value against `Min`/`Max`/`Enum`
- [ ] `setField` gains a `time.Duration`-typed-field case: string raw ->
      `time.ParseDuration` -> `SetInt`; non-string raw (JSON number, or an
      already-`time.Duration`-typed `Default`/`Custom` return value) keeps
      working via the existing `AssignableTo`/numeric branches, unchanged
- [ ] `coerceParamString` gains a `"duration"` case (pass raw string through
      unchanged, same as `"string"` -- the real parse happens in
      `validatePrimitive`/`setField`)
- [ ] `internal/graphql/scalar.go` maps format `"duration"` -> GraphQL custom
      scalar `Duration`, same table `"date-time"`/`"date"` already use
- [ ] `gonest.go` re-exports `type DurationSchema = schema.DurationSchema`

## Out of Scope

| Feature | Reason |
| --- | --- |
| ISO-8601 duration strings (`"P3DT12H"`) | Target Go type is `time.Duration`; the codebase is Go-idiomatic throughout (dotenv, scheduler already use Go's own duration string format) |
| Pattern()/regex validator on DurationSchema | No concrete use case; Min/Max/Enum cover the numeric-comparison need this feature exists for |
| Runtime enforcement of Min>Max | Same "trust the caller" stance every other branch already takes |

---

## User Stories

### P1: `Duration()` branch with Min/Max/Enum, working across every Parseable source ⭐ MVP

**User Story**: As a gonest user, I want
`s.Property(&t.RelayInterval).Duration().Min(1*time.Second).Max(1*time.Hour).Default(5*time.Second)`
to validate and populate a `time.Duration` field from a human-readable string
(`"5s"`, `"1h30m"`) arriving via JSON body, path/query params, headers, a form
field, or an env var.

**Acceptance Criteria**:

1. WHEN `Duration()` is called on a `*PropertyBuilder` THEN system SHALL return a
   `*DurationSchema` with `FormatValue()=="duration"`, `KindValue()=="duration"`
2. WHEN `.Min(time.Duration)`/`.Max(time.Duration)`/`.Enum(...time.Duration)` are
   called THEN system SHALL store and read back the exact `time.Duration` value
   (nanosecond-precision round trip), each call returning the SAME
   `*DurationSchema` so the chain continues
3. WHEN a valid duration string (e.g. `"5s"`) is submitted for a `Duration()`
   field via any of the 6 `Parseable` sources THEN system SHALL populate the
   target `time.Duration` field with the correctly parsed value
4. WHEN an invalid duration string (e.g. `"not-a-duration"`) is submitted THEN
   system SHALL report a per-field `violation` at the VALIDATE stage (not a
   later populate-stage crash/generic error)
5. WHEN the parsed duration violates `Min`/`Max`/`Enum` THEN system SHALL report
   a per-field `violation` comparing the PARSED duration value, not the raw
   string's length

**Independent Test**: mirror `datetime_test.go`/`numeric_test.go`'s own patterns
-- one test file in `internal/schema` for the builder (identity, chaining,
Min/Max/Enum round-trip, last-call-wins vs other branches), one set of cases in
`internal/validate` exercising all 6 sources end-to-end (happy path, malformed
string, out-of-range).

---

## Edge Cases

- WHEN `Duration()` is called AFTER another branch method already ran on the
  same `*PropertyBuilder` (or vice versa) THEN system SHALL simply overwrite
  `format`/`kind` -- same last-write-wins, no cross-branch-family
  special-casing every other branch already establishes
- WHEN `Default(time.Duration(...))` is used and the source key is absent THEN
  system SHALL use that Go value directly, bypassing string parsing entirely
  (same precedent `env.go`'s own doc comment already documents for every other
  branch's `Default`)

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| DUR-01 | Duration() returns *DurationSchema with correct format/kind | Design | Verified |
| DUR-02 | Min/Max/Enum store and round-trip time.Duration correctly | Design | Verified |
| DUR-03 | All 6 Parseable sources populate a time.Duration field from a duration string | Design | Verified |
| DUR-04 | Malformed duration string surfaces as a per-field violation at validate stage | Design | Verified |
| DUR-05 | Min/Max/Enum violations compare the parsed duration value | Design | Verified |

**ID format:** `DUR-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 5 total, 5 mapped, 0 unmapped

---

## Success Criteria

- [ ] `Duration()` compiles and chains exactly like `Integer()`/`Boolean()` today
- [ ] All 6 `Parseable` sources correctly populate a `time.Duration` field
- [ ] Malformed duration input is a validate-stage violation, not a populate crash
- [ ] GraphQL SDL generation declares `scalar Duration` when used
- [ ] Zero regressions in the existing test suite
