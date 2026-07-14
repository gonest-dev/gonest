# JSON Body Validation Specification

## Problem Statement

Milestones 4-5 built a way to DECLARE constraints (`NewMetadata[T]`, `Property(&t.X).String().Min(1).Max(50)...`, `Array()`/`Object()`) but nothing yet READS them. `MustJsonBody[T](ctx)` (INSIGHT.md's own call shape: `gonest.MustJsonBody[*UserProperties](ctx)`) is the first consumer: parse the request body as JSON into `T`, validate every field against `T`'s registered `*Metadata`, panic a structured `BadRequestException` (per-field violation list) if anything fails, otherwise return the populated `*T`.

Three design decisions were made explicit with the user before this spec (see `context.md`): (1) Required-field presence uses a 2-pass unmarshal (map for key-presence, struct for typed value) to distinguish `{}` from `{"name":""}`; (2) validation COLLECTS every violation across the whole body rather than failing on the first; (3) `Array`/`Object` fields validate RECURSIVELY (array items each checked, object refs walked into) from this first version, not deferred.

A prerequisite gap this feature must also close: there is currently NO global registry mapping a Go type to its `*Metadata` -- `NewMetadata[T]` today just returns a `*Metadata`, the caller decides what to do with it (usually assign to a package var). `MustJsonBody[T]` has no `*Metadata` argument in INSIGHT.md's call shape, so it must be able to FIND `T`'s metadata on its own -- this requires `NewMetadata[T]` to self-register into a process-wide registry keyed by `reflect.Type`.

## Goals

- [ ] `NewMetadata[T]` self-registers the built `*Metadata` into a new global registry (keyed by `T`'s `reflect.Type`), panicking if `T` was already registered (one metadata declaration per type -- same panic-on-conflict precedent as `Property`'s own double-registration check)
- [ ] `execution.Context` (and its `Responder` interface, and the Fiber-backed implementation) gains a way to read the RAW request body bytes -- infra gap, nothing reads the body today
- [ ] `MustJsonBody[T any](ctx *Context) T` (T is a pointer type at the call site, e.g. `MustJsonBody[*UserProperties]`) -- looks up `T`'s (dereferenced) registered `*Metadata`, panics if none registered; unmarshals the body twice (map for presence, `T` for typed value); walks every registered property RECURSIVELY (primitives directly, `Array` per-item, `Object` via its `ref`) collecting every violation; if any violations exist, panics `*BadRequestException` with `details` = ordered list of `{field, message}`; otherwise returns the populated `*T`
- [ ] Reproduce a `UserProperties`-shaped body validation end-to-end (real HTTP dispatch via `app.Test`, not manual construction -- same precedent as L-012 in STATE.md: wiring bugs only surface through a real request)

## Out of Scope

| Feature | Reason |
| --- | --- |
| `MustParam[T]` gaining metadata-based coercion/validation | Separate feature ("Param/Query Validation", ROADMAP.md's Milestone 6, 2nd feature) |
| Validating `AdditionalProperties()`-flagged open sub-schemas structurally | No fixed shape exists to check against by definition -- only presence/type-as-object (if declared) is checked, not per-key rules |
| Custom/pluggable validators beyond what Milestones 4-5 already declared (`Min`/`Max`/`Pattern`/`Required`/`Nullable`) | Nothing in INSIGHT.md or ROADMAP.md asks for extensibility here yet |
| Partial/PATCH-shaped validation (all fields optional regardless of `Required()`) | No example or requirement mentions this; `MustJsonBody[T]` validates against the metadata exactly as declared |

---

## User Stories

### P0: Relocate branch-wrapper storage onto the shared `PropertyBuilder` (prerequisite, blocking) ⭐ MVP

**User Story**: As the framework itself, a validator needs to read back EVERY constraint a dev declared (`Min`/`Max`/`Pattern` for String/Numeric; `item`/`itemRef`/quantity `Min`/`Max` for Array; `ref`/`additionalProperties` for Object) from a `*Metadata` built earlier -- but today none of that is possible. `StringMetadata`/`NumericMetadata`/`ArrayMetadata`/`ObjectMetadata` each store their OWN extra state on the wrapper struct itself (not the shared `*PropertyBuilder`), and the wrapper is discarded the instant the declaring closure returns (nothing keeps a reference) -- confirmed empirically by the "Array Builder" feature's own dev sub-agent (SPEC_DEVIATION note, T1: "reconstructing a fresh wrapper afterward will NOT recover [Min/Max]"), which was accepted at the time as "not blocking, no Done-when item requires it" -- Milestone 6 is exactly the point where it becomes blocking.

**Fix**: relocate `min`/`max`/`pattern` (String), `min`/`max` (Numeric), `item`/`itemRef`/`min`/`max` (Array), `ref`/`additionalProperties` (Object) from each wrapper struct's OWN fields onto NEW fields directly on the shared `PropertyBuilder` (the one object `Metadata.properties[offset]` actually retains). Every wrapper's public methods (`Min`/`Max`/`Pattern`/`MinValue`/`MaxValue`/`PatternValue`/`Items`/`Object`/`Metadata`/`AdditionalProperties`/`ItemBuilder`/`ItemRef`/`MinValue`/`MaxValue` etc) keep their EXACT signatures and behavior -- only WHERE the value is written changes (`s.PropertyBuilder.stringMin = &n` instead of `s.min = &n`). This is pure internal storage relocation: zero breaking change to any of the 4 already-shipped features' public API, and every existing test for those 4 features (String-family, Numeric & Boolean, Array Builder, Object Builder) continues to pass unchanged, since none of them assert WHERE state lives, only what the wrapper's own methods return.

**Acceptance Criteria**:

1. WHEN `.String().Min(1).Max(50)` (or any String-family branch) is called AND the resulting `*Metadata` is inspected later (via a NEW accessor on `PropertyBuilder`, not through the discarded `*StringMetadata`) THEN system SHALL report the same `Min`/`Max`/`Pattern` values that were set, without needing the original `*StringMetadata` reference
2. WHEN the same is done for `Integer()`/`Int32()`/`Float()`/`Double()` (Numeric family) THEN system SHALL report `Min`/`Max` the same way
3. WHEN `Array()`/`Items(fn)` declares an item format + quantity `Min`/`Max` THEN system SHALL report all of it (item format/validators, quantity `Min`/`Max`, `ItemRef` if `Object(ref)` was used as the item) from `PropertyBuilder` alone, without the original `*ArrayMetadata` reference
4. WHEN `Object(fn)` declares a `Metadata(ref)` or `AdditionalProperties()` THEN system SHALL report both from `PropertyBuilder` alone
5. WHEN every existing test suite for "String-family Branches", "Numeric & Boolean Branches", "Array Builder", "Object Builder" is re-run AFTER this relocation THEN system SHALL show ZERO regressions (same precedent as every prior refactor in this codebase, e.g. AD-010's rename)

**Independent Test**: for each of the 4 existing branch families, build a `*Metadata` via `NewMetadata[T]`, discard every wrapper reference the declaring closure returned (don't keep `*StringMetadata`/etc around), then read back every constraint through `Metadata.OwnProperties()` + the new `PropertyBuilder`-level accessors alone; assert values match what was declared. Also: full existing test suite (`go test ./... -race`) green.

---

### P1: Global metadata registry (prerequisite) ⭐ MVP

**User Story**: As the framework itself, `NewMetadata[T]` needs to remember what `T` maps to so `MustJsonBody[T]` can find it without an explicit argument.

**Acceptance Criteria**:

1. WHEN `NewMetadata[T]` is called for a `T` never registered before THEN system SHALL store the resulting `*Metadata` in a process-wide registry keyed by `T`'s `reflect.Type`
2. WHEN `NewMetadata[T]` is called AGAIN for the same `T` THEN system SHALL panic (one declaration per type, same precedent as `Property`'s double-registration panic)
3. WHEN a lookup function is called for a `T` never registered THEN system SHALL report "not found" (used by `MustJsonBody` to panic a clear error, not a `nil`-pointer crash)

**Independent Test**: register `NewMetadata[UserProperties]`, look it up successfully; register `NewMetadata[UserProperties]` a second time, confirm panic; look up an unregistered type, confirm "not found" signal.

---

### P2: Request body access on Context (prerequisite)

**User Story**: As the framework itself, `MustJsonBody` needs the raw request body bytes, which nothing in `execution.Context`/`Responder` exposes today.

**Acceptance Criteria**:

1. WHEN a route Handler (or anything holding `*Context`) calls the new body-access method THEN system SHALL return the raw request body as `[]byte`, sourced from the real Fiber request in production and from a fake `Responder` in tests (same `Responder`-interface seam every other `Context` method already uses)

**Independent Test**: fake `Responder` returns fixed bytes; `Context`'s new method returns them unchanged. Separately, a real HTTP dispatch (`app.Test`) with a JSON body confirms the Fiber-backed `Responder` returns the actual posted bytes.

---

### P3: `MustJsonBody[T]` -- happy path

**User Story**: As a gonest user, I want `properties := gonest.MustJsonBody[*UserProperties](ctx)` (INSIGHT.md's own call shape) to return a populated `*UserProperties` when the posted JSON satisfies every constraint `NewMetadata[UserProperties]` declared.

**Acceptance Criteria**:

1. WHEN the request body is valid JSON AND every registered property's constraints are satisfied THEN system SHALL return a populated `*T` with every field set from the JSON, no panic
2. WHEN a property has NO extra format-specific validators (e.g. `Boolean()`) THEN system SHALL still enforce `Required`/`Nullable` (the 4 common constraints apply regardless of branch)

**Independent Test**: real HTTP dispatch (`app.Test`) posting a fully-valid JSON body against a `UserProperties`-shaped metadata (mixing at least: a `Required` `String`, a `Required` `Integer` with `Min`/`Max`, a `Nullable` `DateTime`); assert 200-equivalent success and the returned value's fields match the posted JSON.

---

### P4: `MustJsonBody[T]` -- validation failures, collected

**User Story**: As a gonest user, when I POST an invalid body, I want ONE response listing EVERY violation (missing required field, value out of `Min`/`Max` range, pattern mismatch, wrong JSON type for the declared format), not just the first one found.

**Acceptance Criteria**:

1. WHEN the body is syntactically invalid JSON THEN system SHALL panic `*BadRequestException` with a details entry describing the parse failure (not a raw `encoding/json` panic/crash)
2. WHEN a `Required` field's JSON key is absent (per the 2-pass presence check, context.md's Decision 1) THEN system SHALL record a violation for that field, even if every OTHER field is valid
3. WHEN a field's value violates its own format-specific validator (`Min`/`Max`/`Pattern` for String; `Min`/`Max` for Numeric; wrong JSON type entirely, e.g. a JSON string where an `Integer` was declared) THEN system SHALL record a violation for that field
4. WHEN MULTIPLE fields violate simultaneously THEN system SHALL collect ALL violations (context.md's Decision 2) into `BadRequestException`'s `details`, not stop at the first
5. WHEN `Nullable()` was set for a field AND its JSON value is `null` THEN system SHALL accept `null` without recording a violation, even if the field would otherwise be `Required`

**Independent Test**: real HTTP dispatch posting a body with AT LEAST 2 simultaneous violations (one missing-required, one out-of-range); assert `BadRequestException`'s status (400) and that `details` contains BOTH violations, not just one.

---

### P5: Recursive validation -- Array and Object fields

**User Story**: As a gonest user, an `Array()`-typed field (e.g. `Tags []string` with `Items(fn){ m.String().Min(1).Max(50) }`) validates EVERY item against the item's own constraints, and an `Object()`-typed field (e.g. `Address AddressEntity` via `Metadata(addressMetadata)`) recurses into the referenced `*Metadata`'s own property checks (context.md's Decision 3).

**Acceptance Criteria**:

1. WHEN an `Array`-typed field's JSON value has an item that violates the item's own format validator THEN system SHALL record a violation identifying WHICH item (index) and what failed
2. WHEN an `Array`-typed field's own quantity `Min`/`Max` (`ArrayMetadata.MinValue`/`MaxValue`) is violated (too few/many items) THEN system SHALL record a violation for the field itself (not a specific item)
3. WHEN an `Object`-typed field's `MetadataRef()` is set AND the nested JSON object violates one of the referenced `*Metadata`'s own property constraints THEN system SHALL record a violation identifying the NESTED field's path (e.g. `"address.zip"`, not just `"address"`)
4. WHEN an `Object`-typed field has `IsAdditionalProperties() == true` (open schema) THEN system SHALL NOT attempt structural validation of its sub-keys (context.md's Decision 3 / spec's Out of Scope)

**Independent Test**: real HTTP dispatch reproducing INSIGHT.md's `UserEntity` shape (`Tags []string`, `Addresses []AddressEntity` via `Object(ref)`-typed item, `Address AddressEntity` direct) with BOTH a bad array item and a bad nested object field in the same request; assert both violations appear in `details` with distinguishable field paths.

---

## Edge Cases

- WHEN `MustJsonBody[T]` is called for a `T` that was NEVER registered via `NewMetadata[T]` THEN system SHALL panic with a clear "no metadata registered for type X" message -- never a `nil`-pointer crash
- WHEN the body is EMPTY (zero bytes, e.g. no `Content-Type: application/json` or empty POST) THEN system SHALL treat it as a JSON parse failure (Acceptance Criteria P4.1), same as malformed JSON -- not a special-cased "empty is fine" path
- WHEN a field is `Nullable()` AND its JSON value is genuinely ABSENT (key missing entirely, not `null`) THEN system SHALL treat this the SAME as `Required` would (absence is absence, `Nullable` only means "null is an acceptable VALUE", not "presence is optional") -- unless the field is ALSO not `Required()`, in which case absence is fine (only `Required` fields need presence; `Nullable` alone says nothing about presence)

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| JV-00 | P0: relocate String/Numeric/Array/Object wrapper storage onto shared PropertyBuilder, zero regressions in existing 4 features | T0 | Done |
| JV-01 | P1: NewMetadata[T] self-registers into global registry, panics on duplicate T | T1 | Done |
| JV-02 | P1: lookup function reports not-found for unregistered T | T1 | Done |
| JV-03 | P2: Context/Responder gains raw body access | T2 | Done |
| JV-04 | P3: MustJsonBody[T] happy path returns populated *T | T3/T4 | Done |
| JV-05 | P4: malformed JSON / missing-required / format violations panic BadRequestException with per-field details | T3 | Done |
| JV-06 | P4: multiple violations collected together, not fail-fast | T3 | Done |
| JV-07 | P4: Nullable + null value accepted even if Required | T3 | Done |
| JV-08 | P5: Array items validated recursively (format + quantity) | T3/T4 | Done |
| JV-09 | P5: Object ref validated recursively with nested field path | T3/T4 | Done |

**ID format:** `JV-[NUMBER]`

**Coverage:** 10 total, 10 mapped.

---

## Success Criteria

- [x] `UserEntity`-shaped body (INSIGHT.md, including nested `Tags`/`Addresses`/`Address`) validates correctly end-to-end via real HTTP dispatch, both happy path and multi-violation path
- [x] Every violation from a single bad request is visible in ONE `BadRequestException.details`, with distinguishable field paths (including nested/array paths)
- [x] Zero regressions in existing test suite (`go test ./... -race` verde em toda etapa, commits `d012c7e`→`a9bbda9`)
