# Enum Branches Specification

## Problem Statement

Building `.examples/full-text-search`'s `search.FieldsSchemaFor[T]` (a generic helper constraining a `Select []string`/`Remove []string` query field to one of T's own JSON field names) exposed a real gap: `StringSchema`/`NumericSchema` have `Min`/`Max`/`Pattern` but no `Enum` -- there is no declarative way to say "this value must be one of a fixed set", forcing a `Custom(fn func(raw any) (any, error))` workaround that (a) re-implements membership checking by hand per call site, (b) never shows up in generated OpenAPI docs as `enum: [...]` (Custom is pure runtime validation, invisible to `internal/openapi`), and (c) loses Go-level type safety at the call site (`Custom`'s `fn` takes/returns `any`).

## Goals

- [ ] `StringSchema` gains `Enum(items ...string) *StringSchema`, chainable like `Min`/`Max`/`Pattern`
- [ ] `NumericSchema` gains `Enum(items ...int64) *NumericSchema`, chainable like `Min`/`Max`
- [ ] `internal/validate` rejects any value not present in a field's own registered Enum list (when set), same "collect every violation" stance the rest of `validatePrimitive` already has
- [ ] `internal/openapi` emits `enum: [...]` on the generated schema for any field where Enum was called, so generated docs/Swagger UI show the real constraint instead of a bare `type: string`/`type: integer`
- [ ] `.examples/full-text-search`'s `search.FieldsSchemaFor` migrates from `Custom(fn)` to the new `Enum(...)`, once it exists

## Out of Scope

| Feature | Reason |
| --- | --- |
| `Enum(items ...any)` on a shared/generic entry point | Rejected by the user during design discussion: an `any`-typed signature can't be inferred at the call site the way `Min(int)`/`Pattern(string)` already aren't -- callers would need explicit type arguments or lose compile-time type-checking on the enum values themselves, breaking this project's own "type-safe premise" (the same reason a fully reflection-derived `Where` was rejected earlier in `.examples/full-text-search`, see that example's `SchemaMap` doc comment) |
| `BooleanSchema.Enum`/`DateSchema.Enum` | Boolean's value domain is already exactly 2 values (no enum ever narrows it further); a date/time Enum has no concrete call site anywhere in this repo or `.examples/*` today -- add if a real need appears, same "no speculative API surface" stance as every prior branch feature |
| Enum validated against a numeric FLOAT type (`float32`/`float64`) | No real call site; `NumericSchema.Enum` ships for the integer family only (`int64`, matching `Integer()`'s own storage type), mirroring `search.MatchNumberIntSchema` vs `MatchNumberFloatSchema`'s already-established kind split in `.examples/full-text-search` |
| Enum validated against `[]string`/array VALUES (as opposed to each array item) | `ArraySchema`'s own item builder already reuses `StringSchema`/`NumericSchema` verbatim (`am.item`) -- `Items(func(m *gonest.ArraySchema) { m.String().Enum(...) })` already works for free once `StringSchema.Enum` exists; no separate `ArraySchema.Enum` needed |

---

## User Stories

### P1: `StringSchema.Enum`/`NumericSchema.Enum` declare and validate a fixed value set ⭐ MVP

**User Story**: As a gonest user, I want `s.Property(&t.Status).String().Enum("active", "inactive")` (or the numeric equivalent) to reject any value outside that set, with the violation collected the same way every other `validatePrimitive` check already is -- no more hand-rolled `Custom(fn)` for "one of a fixed list".

**Why P1**: This is the entire feature -- without runtime validation, an `Enum(...)` call would be purely decorative (OpenAPI-only), which defeats the actual gap found (`search.FieldsSchemaFor`'s `Custom(fn)` workaround exists BECAUSE nothing else rejects an invalid value at request time).

**Acceptance Criteria**:

1. WHEN `.Enum(items ...string)` is called on a `*StringSchema` THEN system SHALL store the list, retrievable via a getter (`EnumValues() ([]string, bool)`, same "never called" vs "called with zero items" distinction `MinValue`/`MaxValue` already use), and return the SAME `*StringSchema` so the chain continues
2. WHEN `.Enum(items ...int64)` is called on a `*NumericSchema` THEN system SHALL store the list the same way, returning `*NumericSchema`
3. WHEN a request value arrives for a field with Enum set AND the value is NOT in the list THEN `internal/validate` SHALL record a violation (e.g. `"must be one of [a b c]"`, matching this repo's existing violation message style) instead of accepting it
4. WHEN a request value arrives for a field with Enum set AND the value IS in the list THEN validation SHALL succeed exactly as it does today for a field with no Enum
5. WHEN a field has NO Enum call THEN validation behavior SHALL be byte-for-byte unchanged from today (Enum is purely additive, opt-in per field)

**Independent Test**: build a minimal schema with one `String().Enum("a","b")` field and one `Integer().Enum(1,2,3)` field; assert requests with an in-list value pass, an out-of-list value fails with a violation naming the field, and a field with no `Enum()` call still accepts any value of the right primitive type (regression check against `string-family-branches`/`numeric-boolean-branches`' own existing test suites).

---

### P2: Enum appears in generated OpenAPI output

**User Story**: As a gonest user relying on `OpenapiGenerate`/Swagger UI, I want a field with `.Enum(...)` to show up in the generated schema as `"enum": [...]`, so the constraint is visible to API consumers/Swagger UI without reading the Go source.

**Why P2**: Not required for the validation gap itself (P1 already closes that), but Enum is meaningless as pure internal validation if the project's own OpenAPI generator -- the whole point of `gonest.Schema` per its own package doc comment -- silently drops it, same reasoning that already motivated `Min`/`Max`/`Pattern` showing up in `internal/openapi`'s existing output.

**Acceptance Criteria**:

1. WHEN `internal/openapi.Generate` walks a `*Schema` whose field has `Enum` set THEN the generated component schema for that field SHALL include an `"enum"` array with the exact values registered
2. WHEN a field has no `Enum` call THEN the generated schema SHALL have no `"enum"` key (not an empty array), matching how `Min`/`Max`/`Pattern` already omit their own keys when unset

**Independent Test**: generate OpenAPI JSON for the same minimal schema from P1's test, assert the JSON's `components.schemas.<Title>.properties.<field>.enum` array matches exactly, and that a field with no Enum has no `enum` key at all.

---

### P3: `.examples/full-text-search` migrates off `Custom(fn)`

**User Story**: As the maintainer dogfooding gonest via `.examples/full-text-search`, I want `search.FieldsSchemaFor[T]` to use the new `Enum(...)` instead of its current `Custom(fn)` workaround, so the example demonstrates the real, intended API instead of the gap that motivated this feature.

**Why P3**: Cosmetic/consistency cleanup once P1 exists -- the example already works correctly today via `Custom(fn)`; this just replaces the workaround with the real mechanism once available, and gets the OpenAPI-visibility bonus from P2 for free.

**Acceptance Criteria**:

1. WHEN `search.FieldsSchemaFor[T]` is rebuilt using `.String().Enum(search.FieldNames[T]()...)` THEN the existing behavior (valid name passes, invalid name fails with a violation) SHALL be unchanged, proven by re-running the exact `curl` verification already done manually for this example (valid `select`/`remove` → 200, typo'd name → 400 listing the real field names)

---

## Edge Cases

- WHEN `.Enum()` is called with ZERO items THEN system SHALL treat this the same as "never called" for validation purposes (no items means nothing to check against -- validating "must be one of []" would reject every value, almost certainly not the caller's intent) but MUST still distinguish "called with zero items" from "never called" at the getter level (same `(value, bool)` pattern `MinValue`/`MaxValue`/`PatternValue` already use), matching this codebase's established "never silently guess intent" stance
- WHEN `.Enum(...)` is called MORE THAN ONCE on the same branch builder THEN system SHALL follow this package's existing "last-call-wins, no panic" precedent (same as every other branch method: `String`/`Email`/`Integer`/etc.)
- WHEN a field has BOTH `Pattern`/`Min`/`Max` AND `Enum` set THEN system SHALL apply Enum ADDITIONALLY (not instead of) the existing checks -- an enum value that also violates Min/Max/Pattern should still collect those violations too, same "collect every violation, never short-circuit on the first" stance `validatePrimitive` already has for its own existing checks
- WHEN `Enum` is combined with `Nullable()` and the request value is explicit JSON `null` THEN system SHALL accept it (Nullable's existing null-handling runs before the enum membership check, unchanged) -- `null` is never itself required to be a member of the Enum list

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| ENUM-01 | P1: StringSchema.Enum stores + chains | Implementing | Verified |
| ENUM-02 | P1: NumericSchema.Enum stores + chains | Implementing | Verified |
| ENUM-03 | P1: validate.go rejects out-of-list values, collects violation | Implementing | Verified |
| ENUM-04 | P1: no-Enum fields unchanged (regression safety) | Implementing | Verified |
| ENUM-05 | P2: openapi.Generate emits "enum" array when set | Implementing | Verified |
| ENUM-06 | P2: no "enum" key when unset | Implementing | Verified |
| ENUM-07 | P3: search.FieldsSchemaFor migrates off Custom(fn) | Implementing | Verified |

**ID format:** `ENUM-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 7 total, 7 mapped and Verified. `go test ./... -race -count=1` green (24 core packages);
`.examples/full-text-search` builds/vets clean and manually curl-verified (valid `fields.select` → 200,
invalid → 400 with `"must be one of [...]"`). See AD-047 in STATE.md.

---

## Success Criteria

- [ ] `.String().Enum(...)`/`.Integer().Enum(...)` compile, chain, validate, and round-trip into generated OpenAPI exactly like `Min`/`Max`/`Pattern` already do
- [ ] Zero regressions in the existing test suite (`go test ./... -race`, all packages green)
- [ ] `.examples/full-text-search` builds, `go vet` clean, and the manual `curl` QUERY /person verification (valid/invalid `fields.select`) still passes after migrating off `Custom(fn)`
