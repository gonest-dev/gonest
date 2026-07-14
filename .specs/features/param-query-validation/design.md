# Param, Query & Custom Validation Design

**Spec**: `.specs/features/param-query-validation/spec.md`
**Context**: `.specs/features/param-query-validation/context.md`

## Architecture Overview

```
internal/metadata/metadata.go (PropertyBuilder, extended -- P0)
        │
        + custom func(raw any) (any, error)   -- NEW field
        + func (p *PropertyBuilder) Custom(fn func(raw any) (any, error)) *PropertyBuilder
        + func (p *PropertyBuilder) CustomFunc() (func(raw any) (any, error), bool)

internal/validate (existing package, restructured -- P0/P2/P3)
        │
        ├── validateStruct/validateValue/validateArray/validateObject/
        │      validatePrimitive (EXISTING, from "JSON Body Validation" --
        │      logic UNCHANGED except validateValue gains a Custom(fn)
        │      short-circuit at its very top: if p.CustomFunc() is set, call
        │      it instead of dispatching on KindValue, record its error as a
        │      violation if any, STOP -- never falls through to kind/Min/Max
        │      checks for that field)
        │
        ├── populate(dest reflect.Value, presence map[string]any, m *metadata.Metadata, tag string)
        │      NEW, shared by all 3 public entry points -- walks
        │      m.OwnProperties(), for each field:
        │        - resolves key via tagKey(p.Field(), tag) (tag = "json"/
        │          "param"/"query" depending on caller)
        │        - raw, ok := presence[key]; skip if !ok (already recorded as
        │          violation during the validate pass, nothing to populate)
        │        - if p.CustomFunc() set: call it AGAIN (same fn, same raw --
        │          deliberately re-run rather than caching the first pass's
        │          result, see Tech Decisions) -- ok is already guaranteed
        │          true here since a failing Custom already short-circuited
        │          validateValue earlier and this field wouldn't reach
        │          populate with a violation pending (MustJsonBody/
        │          MustParams/MustQuery all check violations BEFORE calling
        │          populate)
        │        - else: reflect.Value.Set the ALREADY-VALIDATED raw value
        │          (coerced to the field's own Go type -- see setField below)
        │
        ├── setField(fieldVal reflect.Value, raw any) error -- converts raw
        │      (string/float64/bool/map[string]any/[]any, or Custom's own
        │      returned any) into fieldVal's Go type and Sets it; returns an
        │      error (never panics) if incompatible -- P0's Edge Case
        │
        ├── coerceParamString(raw string, kind string) (any, error) -- NEW,
        │      P2/P3 only: converts a raw path/query STRING into the same
        │      any-shape validateValue already expects (string stays string;
        │      "integer"/"number" parse via strconv; "boolean" via
        │      strconv.ParseBool) -- lets Params/Query REUSE validateValue/
        │      validatePrimitive UNCHANGED once the string is coerced, same
        │      as JSON's own float64/bool/string shape. Runs AFTER checking
        │      for Custom(fn) (Custom gets the RAW STRING, never a coerced
        │      value -- spec.md's P2.3/P3.3)
        │
        ├── func MustJsonBody[T any](ctx *execution.Context) T -- REFACTORED:
        │      validation pass UNCHANGED (still 2-pass presence+type via
        │      json.Unmarshal into `any`), but the FINAL "build T" step
        │      changes from a single json.Unmarshal(body, result) call to
        │      populate(reflect.ValueOf(result).Elem(), presenceMap, m, "json")
        │
        ├── func MustParams[T any](ctx *execution.Context) T -- NEW: resolves
        │      T's Metadata via registry (same as MustJsonBody), builds a
        │      presence map by walking m.OwnProperties(), resolving each
        │      field's "param" tag key, checking route.HasParam(key) (via
        │      ctx.Route() type-asserted to *route.Route -- same pattern
        │      root param.go already used) and reading ctx.Param(key);
        │      coerces via coerceParamString, validates via validateValue
        │      (reused), collects violations, panics BadRequestException or
        │      calls populate(..., "param") and returns
        │
        └── func MustQuery[T any](ctx *execution.Context) T -- NEW: same
               shape as MustParams, but presence/raw values come from
               ctx.Queries() (NEW Context method, see below) instead of
               Route.HasParam/ctx.Param

internal/execution/context.go (Responder/Context, extended -- P3 infra)
        │
        ├── Responder gains Queries() map[string]string
        └── Context gains Queries() map[string]string (one-line delegation,
              same pattern as Body())

internal/adapter/fiber (extended -- P3 infra)
        └── fiberResponder.Queries() map[string]string { return r.c.Queries() }

REMOVED (P1):
        internal/pipe/                          -- entire package deleted
        internal/route/param.go                  -- entire file deleted (old
                                                     MustParam[T]/defaultCoerce/
                                                     callPipeHandler, fully
                                                     superseded)
        internal/route/route.go's paramPipes/Param/PipeFor
        gonest.go's Pipe/NewPipe/MustParam[T](ctx,name) exports
```

This feature has THREE layers, in dependency order: (1) `Custom(fn)` + the shared `populate` core, which REQUIRES refactoring `MustJsonBody`'s final build step (P0, touches already-shipped code, highest risk); (2) deleting `Pipe` and the old singular `MustParam` (P1, pure removal, low risk once P0 makes them truly redundant); (3) the two new whole-object entry points `MustParams`/`MustQuery` (P2/P3, additive, reuse P0's validation/population core almost entirely, the only new logic is `coerceParamString` and the two infra additions for reading path/query raw values).

---

## Components

### `PropertyBuilder` (existing, extended -- P0)

- **Purpose**: gains the `Custom(fn)` escape hatch, the direct replacement for `Pipe`'s original intent (context.md's Decision 4).
- **Location**: `internal/metadata/metadata.go`
- **Interfaces**:
  - `func (p *PropertyBuilder) Custom(fn func(raw any) (any, error)) *PropertyBuilder` -- stores `fn`, returns `p` (bare, no wrapper -- same precedent as `Boolean()`/`DateTime()`, `Custom` has no format-specific extra validators of its own to chain)
  - `func (p *PropertyBuilder) CustomFunc() (func(raw any) (any, error), bool)` -- getter, `(nil, false)` if never set (same nil-handling shape as every other AD-012 getter)
- **Dependencies**: none new
- **Reuses**: nothing removed -- purely additive

### `internal/validate` (existing package, restructured)

- **Purpose**: unify how `MustJsonBody`/`MustParams`/`MustQuery` validate AND populate `T`, so `Custom(fn)` behaves identically regardless of source (body/path/query).
- **Location**: `internal/validate/validate.go` (extended), possibly split into `populate.go`/`params.go`/`query.go` for readability (dev's call, document if split)
- **Key restructure**: `validateValue`'s FIRST check becomes `Custom(fn)`, before the existing `raw == nil`/`KindValue()` dispatch:
  ```go
  func validateValue(raw any, p *metadata.PropertyBuilder, path string) []violation {
      if fn, ok := p.CustomFunc(); ok {
          if _, err := fn(raw); err != nil {
              return []violation{{Field: path, Message: err.Error()}}
          }
          return nil // Custom succeeded, no further checks
      }
      // ...existing null/kind dispatch, unchanged
  }
  ```
- **New `populate` function**: see Architecture Overview above for full shape. Called ONLY after `validateValue`'s pass already confirmed zero violations (mirrors `MustJsonBody`'s existing two-step "validate everything, THEN build" order).
- **Dependencies**: `internal/route` (NEW -- `MustParams` needs `route.Route.HasParam`, mirroring what root `param.go` used to do before removal)
- **Reuses**: `Metadata.OwnProperties()`, every `PropertyBuilder` getter (AD-012's + this feature's new `CustomFunc`)

### `execution.Responder`/`Context` (existing, extended -- P3 infra)

- **Purpose**: expose query string params, currently inaccessible (same class of gap `Body()` closed for the body in "JSON Body Validation"'s T2).
- **Interfaces**: `Responder` gains `Queries() map[string]string`; `Context` gains one-line delegation `Queries() map[string]string { return ctx.res.Queries() }`

### Removed: `internal/pipe`, `Route.Param`/`PipeFor`, `internal/route/param.go`, root `Pipe`/`NewPipe`/`MustParam[T]`

- **Purpose**: dead weight once `Custom(fn)` + `MustParams` cover Pipe's entire original use case (context.md's Decision 3).
- **Verification**: `go build ./...` clean after removal, `grep` sweep confirms zero dangling references (spec.md's Independent Test for P1).

---

## Data Models

```go
// internal/metadata/metadata.go, PropertyBuilder EXTENDED (on top of AD-012's fields):
type PropertyBuilder struct {
    // ...existing fields (field, required, nullable, description, examples,
    // format, kind, min, max, pattern, item, itemRef, ref, additionalProperties)
    custom func(raw any) (any, error) // NEW
}
```

```go
// internal/validate/validate.go additions:
func populate(dest reflect.Value, presence map[string]any, m *metadata.Metadata, tag string) error
func setField(fieldVal reflect.Value, raw any) error
func coerceParamString(raw string, kind string) (any, error)
func tagKey(field reflect.StructField, tag string) string // generalizes the existing json-tag-reading helper to take a tag name param
```

**Relationships**: `populate` is the SINGLE place `T`'s fields ever get written across all 3 entry points -- `MustJsonBody` no longer has its own separate "build" logic, `MustParams`/`MustQuery` never had one to begin with. `Custom(fn)` is invoked from TWO different places for the SAME field during one call: once inside `validateValue` (to determine pass/fail), once inside `populate` (to get the value to write) -- see Tech Decisions for why this isn't cached/deduplicated.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| `Custom(fn)` returns an error | Violation recorded, collected with all others (same as every other violation type) | spec.md P0.2 |
| `Custom(fn)`'s returned value's Go type is incompatible with the destination field | `setField` returns an error, `populate` surfaces it as a violation -- NEVER a raw `reflect.Value.Set` panic (which panics on type mismatch by default) | spec.md's Edge Cases |
| Path param string fails to coerce to the declared kind (e.g. `"abc"` for an `Integer()` field) | `coerceParamString` returns an error, recorded as violation, same collect-all behavior | spec.md P2.2 |
| `T` (for `MustParams`/`MustQuery`) never registered via `NewMetadata[T]` | Panic clear message, BEFORE reading any param/query value | spec.md's Edge Cases, same precedent as `MustJsonBody` |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `Custom(fn)` is called TWICE per field (once in `validateValue`, once in `populate`), not cached | Accept the double-call | `fn` is a plain `func(raw any) (any, error)` closure -- caching its result would require threading a `map[path]any` result-cache through both passes, adding real complexity for a cost that's negligible in practice (`fn` runs once per field per request, not in a hot loop) -- and caching risks a SUBTLE bug if `fn` isn't perfectly idempotent (e.g. logs or touches global state) where the two passes would need to agree the cache is still valid. Simpler to just call it twice; document the "must be idempotent, called up to 2x" contract on `Custom`'s own doc comment instead of building infrastructure to avoid it. |
| `populate`'s signature takes `tag string` (a bare string like `"json"`/`"param"`/`"query"`), not a typed enum | String constant, not a new type | Matches this codebase's existing "don't invent abstraction beyond what's needed" stance -- 3 known values, no external extensibility requirement, a `type SourceTag string` with 3 constants would add a name to remember for zero behavioral gain over 3 string literals already spelled out at each of the 3 call sites |
| `MustParams`/`MustQuery` reuse `validateValue`/`validatePrimitive` UNCHANGED by first coercing the raw string into the SAME `any` shape (`string`/`float64`/`bool`) JSON decoding already produces | Coerce-then-reuse, not a parallel validation path | Avoids duplicating Min/Max/Pattern logic a second time for params/query -- `coerceParamString` is the ONLY new validation-adjacent code these two entry points need; everything downstream (kind dispatch, Min/Max/Pattern checks, violation collection) is the exact code "JSON Body Validation" already built and evaluator-approved |
| `Custom(fn)` on `MustParams`/`MustQuery` receives the RAW STRING (never `coerceParamString`'s output) | Raw string always | Spec.md's P2.3/P3.3 -- `Custom`'s whole purpose is handling values the built-in coercion CAN'T express (e.g. a non-numeric encoded ID); running `coerceParamString` first (which would likely fail and produce a spurious violation before `Custom` even gets a chance) would defeat the purpose. `Custom` is checked FIRST, before any coercion attempt, for both JSON body (already `any`-shaped, no coercion needed there) and params/query (raw string, no coercion attempted if `Custom` is present) |
| Old `internal/route/param.go` deleted entirely, not merged into `internal/validate` | Delete, don't merge/rename | Its `defaultCoerce` logic (string→int/int64/bool/float64) is conceptually replaced by `coerceParamString` (same idea, new home, slightly different shape since it needs to produce the `any`-shape `validateValue` expects rather than a generic `T`) -- keeping the OLD file around under a new name would leave dead code (the old `MustParam[T]`/`callPipeHandler` functions) that nothing calls once P1 removes their only caller (root `gonest.MustParam`) |

---

## Open Questions pra Tasks

- None left unresolved -- context.md's 5 decisions (captured across 3 `AskUserQuestion` rounds with the user) settled every ambiguity this design needed. Task breakdown should isolate P0 (touches already-shipped `MustJsonBody` code) with its own dedicated evaluator pass, mirroring how "JSON Body Validation"'s own T0 (storage relocation) was isolated -- P0 here is the SAME risk class (refactoring already-evaluator-approved code without changing its observable behavior).
