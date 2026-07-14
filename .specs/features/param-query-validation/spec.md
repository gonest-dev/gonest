# Param, Query & Custom Validation Specification

## Problem Statement

ROADMAP.md's Milestone 6 second feature was vaguely described ("`MustParam[T]` integra `Pipe` + coerção via metadata"). Working through concrete metacode in INSIGHT.md with the user (see `context.md` for the full decision trail) settled a much larger, precise scope: (1) path params and query params both become WHOLE-OBJECT validated (`MustParams[T]`/`MustQuery[T]`, mirroring `MustJsonBody[T]`), replacing the existing single-param `MustParam[T](ctx, name)`; (2) `Pipe` (a shipped Milestone 3 feature) is removed entirely, its original "custom per-param transform" intent absorbed into a new `PropertyBuilder.Custom(fn)` escape hatch living inside `Metadata` declarations; (3) because `Custom(fn)`'s transformed value must reach the final populated struct, `MustJsonBody` (already shipped this session, commits `25ab1e3`/`a9bbda9`) gets its "build `T`" step refactored from a single `json.Unmarshal` call to a reflect-based, field-by-field population shared with the two new entry points.

## Goals

- [ ] `PropertyBuilder.Custom(fn func(raw any) (any, error)) *PropertyBuilder` -- when set, the validator calls `fn(raw)` instead of built-in kind/format/Min/Max/Pattern checks for that field; `fn`'s error becomes a violation, `fn`'s returned value is what populates `T`
- [ ] `internal/validate` gains a shared reflect-based population core used by ALL THREE public entry points (`MustJsonBody`, `MustParams`, `MustQuery`) -- one field-by-field walk, not three separate mechanisms
- [ ] `MustJsonBody[T]` refactored to use the shared population core (same validation/violation-collection behavior as before, only the final "build `T`" step changes) -- zero regression in its own existing test suite's OBSERVABLE behavior (some internal test helpers may need updating if they inspected internals, but every assertion on `MustJsonBody`'s own public behavior must keep passing)
- [ ] `MustParams[T any](ctx *execution.Context) T` -- validates path params against `NewMetadata[T]`, same violation-collection/BadRequestException shape as `MustJsonBody`
- [ ] `MustQuery[T any](ctx *execution.Context) T` -- validates query string params against `NewMetadata[T]`, same shape
- [ ] `internal/pipe` package, `gonest.Pipe`/`gonest.NewPipe`, `Route.Param`/`Route.PipeFor`, and the singular `gonest.MustParam[T](ctx, name)` are ALL removed
- [ ] `internal/execution.Responder`/`Context` gain query-string access (new infra gap, same class as T2's `Body()` addition in "JSON Body Validation")
- [ ] INSIGHT.md rewritten: every `MustParam[int64](ctx, "user_id")` call site becomes `MustParams[T](ctx)`; the Middleware/Pipe example section drops `ParseIntPipe`/`route.Param` entirely (replaced with a `Custom(fn)` example demonstrating genuinely custom logic a fixed validator vocabulary can't express, e.g. decoding a non-trivial encoded ID)

## Out of Scope

| Feature | Reason |
| --- | --- |
| Repeated query keys as arrays (`?tags=a&tags=b` → `[]string`) | No example asks for this; query fields in this feature are flat/scalar (string/int/bool/etc), same primitive branches as body fields minus Array/Object nesting |
| `Custom(fn)` reading/interacting with OpenAPI generation (Milestone 7) | Future work, not touched here |
| Validating Array/Object-typed fields within `MustParams`/`MustQuery` | Path/query params are fundamentally flat key-value pairs (unlike a JSON body) -- Array/Object branches stay meaningful only for `MustJsonBody`; `MustParams`/`MustQuery` support the SAME primitive branches (String family, Numeric family, Boolean, DateTime/Date) plus `Custom`, not Array/Object |

---

## User Stories

### P0: `Custom(fn)` + unified reflect-based population (prerequisite, blocking) ⭐ MVP

**User Story**: As the framework itself, once `Custom(fn)` exists, its transformed value must be the one that ends up in the final populated struct returned by `MustJsonBody`/`MustParams`/`MustQuery` -- not silently ignored by a separate `json.Unmarshal` call that never consults it.

**Acceptance Criteria**:

1. WHEN `Custom(fn)` is set on a `*PropertyBuilder` THEN the validator SHALL call `fn(raw)` for that field's value INSTEAD OF its built-in kind/format/Min/Max/Pattern checks (`raw` is the same generically-decoded value the existing validator already produces -- `string`/`float64`/`bool`/`nil`/`map[string]any`/`[]any` for JSON body; a raw path/query string for `MustParams`/`MustQuery`, see P2/P3)
2. WHEN `fn` returns a non-nil `error` THEN system SHALL record a violation for that field (message from the error), same collection behavior as every other violation type (context.md's Decision 2 from "JSON Body Validation" -- collect all, don't stop early)
3. WHEN `fn` returns `(value, nil)` THEN system SHALL use `value` as the field's final value when building `T` (via `reflect.Value.Set`, with a type-assertion/conversion check -- if `value`'s type is incompatible with the destination field's Go type, treat as a violation, don't panic a reflect error)
4. WHEN a field has NO `Custom(fn)` set THEN system SHALL populate it exactly as before (JSON-decoded natural value for body fields; parsed/coerced value for param/query fields, see P2/P3)
5. WHEN `MustJsonBody[T]`'s full existing test suite (T0-T4 of "JSON Body Validation") is re-run after this refactor THEN system SHALL show ZERO regressions in observable behavior (same precedent as every prior refactor -- AD-010's rename, AD-012's storage relocation)

**Independent Test**: a field with `Custom(fn)` that decodes a custom-format string (e.g. `"v1:42"` → `42`) into an `int`, verified end-to-end via `MustJsonBody` (real HTTP dispatch, body-sourced) AND via `MustParams`/`MustQuery` (once P2/P3 exist) with the SAME `Custom(fn)` definition reused across all three -- proves the population core is genuinely shared, not three parallel implementations that happen to look similar.

---

### P1: Remove Pipe

**User Story**: As the framework itself, once `Custom(fn)` covers Pipe's original "custom transform" intent, the separate `Pipe` object type, its registration mechanism, and the singular `MustParam[T](ctx, name)` that consulted it become dead weight.

**Acceptance Criteria**:

1. WHEN this feature ships THEN `internal/pipe` package SHALL be deleted, `gonest.Pipe`/`gonest.NewPipe` root exports SHALL be removed, `Route.Param`/`Route.PipeFor`/`Route.paramPipes` SHALL be removed
2. WHEN this feature ships THEN `gonest.MustParam[T](ctx, name)` (singular) SHALL be removed -- every existing call site (root package tests, INSIGHT.md) migrates to `MustParams[T](ctx)`
3. WHEN `go build ./...`/`go vet ./...` run after removal THEN system SHALL show NO dangling references to any removed symbol

**Independent Test**: `grep -r "internal/pipe\|gonest.Pipe\|gonest.NewPipe\|MustParam\[" --include=*.go .` (excluding `MustParams`/`MustQuery`, which contain `MustParam` as a prefix) returns zero matches outside historical `.specs/` documentation.

---

### P2: `MustParams[T]` -- path params, whole-object

**User Story**: As a gonest user, I want `params := gonest.MustParams[*UserIdParams](ctx)` (INSIGHT.md's own settled call shape) to validate every path param declared in `T`'s `NewMetadata[T]` against the route's actual `:name` segments, returning a populated `*T`.

**Acceptance Criteria**:

1. WHEN every field of `T` (identified by a `param:"name"` struct tag, falling back to the field's own Go name if absent -- same tag-resolution pattern `MustJsonBody` already uses for `json:"..."`) has a corresponding `:name` segment on the CURRENT route (checked via `Route.HasParam`, already existing) with a value that satisfies its declared constraints THEN system SHALL return a populated `*T`, no panic
2. WHEN a field's route segment is ABSENT (`Route.HasParam` is false for that name) OR its raw string value fails to coerce/validate against the declared branch (`Integer()`+`Min`/`Max`, `String()`+`Pattern`, etc.) THEN system SHALL record a violation (collected together with any other field's violation, same collect-all behavior as `MustJsonBody`)
3. WHEN `Custom(fn)` is set on a param field THEN system SHALL call `fn(raw)` with `raw` being the RAW STRING value (not a JSON-decoded `any` -- path segments are always strings at the wire level), same error/value handling as P0

**Independent Test**: real HTTP dispatch to a route with 2+ path params (e.g. `/user/:user_id/order/:order_id`), one valid one invalid, confirm `MustParams` panics `BadRequestException` with exactly the invalid one's violation; separately confirm a fully-valid request returns a correctly populated `*T`.

---

### P3: `MustQuery[T]` -- query string params, whole-object

**User Story**: As a gonest user, I want `query := gonest.MustQuery[*ListUsersQuery](ctx)` (INSIGHT.md's own settled call shape) to validate every query param declared in `T`'s metadata (via a `query:"name"` tag) against the request's actual query string, returning a populated `*T`.

**Acceptance Criteria**:

1. WHEN every field of `T` has a corresponding query key present with a value satisfying its declared constraints THEN system SHALL return a populated `*T`, no panic
2. WHEN a `Required` field's query key is ABSENT, or a present value fails validation THEN system SHALL record a violation, collected with any others
3. WHEN `Custom(fn)` is set on a query field THEN system SHALL call `fn(raw)` with the raw query string value, same handling as P0/P2

**Independent Test**: real HTTP dispatch with a query string missing a `Required` param AND another present-but-out-of-range param simultaneously; confirm both violations appear together. Separately confirm happy path.

---

## Edge Cases

- WHEN a `T` passed to `MustParams`/`MustQuery` was never registered via `NewMetadata[T]` THEN system SHALL panic a clear "no metadata registered" message, same precedent as `MustJsonBody`'s own Edge Case (JSON Body Validation spec.md)
- WHEN `Custom(fn)`'s returned value's Go type does NOT match the destination struct field's own type THEN system SHALL record a violation describing the mismatch, NEVER panic a raw `reflect` error (spec.md's own "never crash, always structured error" stance, consistent throughout this codebase)
- WHEN a field has BOTH `Custom(fn)` AND a format branch (`String()`/`Integer()`/etc) set (e.g. `.Integer().Custom(fn)`) THEN `Custom(fn)` SHALL take priority -- last-call-wins is the existing precedent for `format`/`kind` overwrites throughout Milestones 4-5, `Custom` follows the same "whichever was configured last governs" rule structurally (it's just one more thing `PropertyBuilder` tracks)

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| PQ-00 | P0: Custom(fn) + unified reflect population, MustJsonBody refactored, zero regressions | T0 | Done |
| PQ-01 | P1: internal/pipe, Route.Param/PipeFor, gonest.Pipe/NewPipe, MustParam[T](ctx,name) all removed | T3 | Done |
| PQ-02 | P2: MustParams[T] validates path params, collect-all violations | T1 | Done |
| PQ-03 | P2: MustParams[T] supports Custom(fn) with raw string value | T1 | Done |
| PQ-04 | P3: MustQuery[T] validates query params, collect-all violations | T2 | Done |
| PQ-05 | P3: MustQuery[T] supports Custom(fn) with raw string value | T2 | Done |

**ID format:** `PQ-[NUMBER]`

**Coverage:** 6 total, 6 mapped.

---

## Success Criteria

- [x] `Custom(fn)` works identically across `MustJsonBody`, `MustParams`, `MustQuery` -- population core genuinely unified
- [x] Pipe fully removed, zero dangling references, `go build ./...` clean
- [x] INSIGHT.md's every route example uses `MustParams`/`MustQuery` where applicable, no `MustParam[T](ctx,name)` singular remains
- [x] Zero regressions in existing test suite, including "JSON Body Validation"'s own suite re-verified after the population-core refactor (commits `8d1aa85`→`ea9f72a`)
