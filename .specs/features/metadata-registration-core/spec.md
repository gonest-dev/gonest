# Metadata Registration Core Specification

## Problem Statement

Milestone 4 (Metadata Builder — Primitivos) needs a way for a dev to declare, PER STRUCT FIELD, a set of OpenAPI 3.1-shaped constraints (`Required`/`Nullable`/`Description`/`Examples` plus, in later features, type+format-specific validators like `String().Min().Max()`) WITHOUT struct tags — using a fluent builder keyed by the field's own POINTER (`m.Property(&t.Id)`), per INSIGHT.md's own example. This is a fundamentally different domain from everything built in Milestones 1-3 (DI graph, HTTP dispatch) — it's schema/reflection-shaped, and the same declaration is meant to eventually feed BOTH OpenAPI document generation (Milestone 7) and runtime request validation (Milestone 6). This feature builds ONLY the foundation: `NewMetadata[T]`, `Property(&t.X)`, and the base set of constraints common to EVERY future type+format branch. The branches themselves (`String()`, `Integer()`, `Boolean()`, etc. — each of which changes what methods are available afterward, per INSIGHT.md's own type+format-flattened design) are explicitly a SEPARATE, later feature ("String-family Branches", etc.) — see Out of Scope.

## Goals

- [ ] `gonest.NewMetadata[T](fn func(t *T, m *Metadata))` lets a dev declare metadata for type `T`, matching INSIGHT.md's exact call shape (`gonest.NewMetadata[UserEntity](func (t *UserEntity, m *gonest.Metadata) {...})`)
- [ ] `m.Property(&t.X)` identifies WHICH field of `T` is being annotated purely by the field's own pointer address — no struct tags, no field-name strings — matching INSIGHT.md's exact call shape (`m.Property(&t.Id)`)
- [ ] The base constraint set common to every future branch (`Required()`, `Nullable()`, `Description(string)`, `Examples(...any)`) is chainable off `Property(&t.X)`'s return value, matching INSIGHT.md's fluent chain style (`m.Property(&t.Id).Required().Description("...").Examples(...)` — minus the type+format branch call itself, e.g. `.Integer()`, which is out of scope here)
- [ ] `m.Description(string)` sets a description for the WHOLE metadata declaration (the struct itself, not a field) — matches INSIGHT.md's `m.Description("Entidade de usuário")` at the top of the builder fn

## Out of Scope

| Feature | Reason |
| --- | --- |
| Any type+format branch method (`String()`, `Integer()`, `Boolean()`, `DateTime()`, etc.) | ROADMAP.md explicitly splits these into their own later features ("String-family Branches", "Numeric & Boolean Branches", "Date/Time Branches") — this feature only builds what's common to ALL of them, not any specific one. Consequence: `Property(&t.X)`'s return value in THIS feature has no branch methods yet — a dev cannot fully replicate INSIGHT.md's example end-to-end until the branch features land, only the foundation this feature builds |
| `Array()`/`Object()`/`Items()` (nested/collection metadata) | Milestone 5, explicitly later per ROADMAP.md. AD-002 (STATE.md) already settled the builder-shape decision for THOSE methods specifically (linear builder, variadic `Items`) — not relevant to this feature's narrower scope |
| Actually READING/using the registered metadata for anything (OpenAPI generation, runtime validation) | Milestones 6-7, explicitly later. This feature only builds the REGISTRATION side — a `*Metadata` value that HOLDS what was declared, inspectable via accessors for a later feature to consume, but does nothing with it yet (no schema output, no validation logic) |
| Struct-tag-based declaration (an alternative to the pointer-identified `Property(&t.X)` approach) | INSIGHT.md's own example is explicit about avoiding struct tags for this ("cada método é uma combinação type+format... sem tipo pai") — the pointer-identification approach is the ground truth to build, not a design choice left open |

---

## User Stories

### P1: Declare metadata for a struct type, identify fields by pointer ⭐ MVP

**User Story**: As a gonest user, I want `gonest.NewMetadata[UserEntity](func(t *UserEntity, m *gonest.Metadata) {...})` with `m.Property(&t.Id)` inside it so that I can declare per-field constraints without struct tags, using Go's own type system (the field's pointer) to identify which field I mean — exactly matching INSIGHT.md's own example.

**Why P1**: This is the entire feature — without a working `NewMetadata`/`Property` foundation, no later branch feature (String, Integer, etc.) has anything to attach its own methods to.

**Acceptance Criteria**:

1. WHEN `gonest.NewMetadata[T](fn)` is called THEN system SHALL construct a zero-value (or equivalently addressable) `T`, pass a pointer to it plus a fresh `*Metadata` builder into `fn`, and return the resulting `*Metadata` once `fn` has run
2. WHEN `fn` calls `m.Property(&t.X)` for a field `X` of the `t` passed into `fn` THEN system SHALL correctly identify WHICH field of `T` was referenced (by field pointer address, using the offset between `&t.X` and `t` itself to locate the matching field in `T`'s own `reflect.Type`) — this identification must be correct for AT LEAST the primitive kinds INSIGHT.md's own example uses (int64, string, bool, time.Time, *time.Time)
3. WHEN `m.Property(&t.X)` is called with a pointer that does NOT belong to `t` (a bug in the dev's own declaration, e.g. a local variable's address instead of a field of `t`) THEN system SHALL panic with a clear message at declaration time, rather than silently registering nonsense or corrupting other fields' metadata
4. WHEN `Property(&t.X)`'s returned builder has `.Required()`/`.Nullable()`/`.Description(string)`/`.Examples(...any)` called on it (any subset, any order, matching INSIGHT.md's fluent chain style) THEN each call SHALL be reflected in that field's own stored metadata, retrievable via accessors, and SHALL return the SAME builder value so the chain can continue
5. WHEN `m.Description(string)` (on the top-level `*Metadata`, not a field builder) is called THEN it SHALL set the whole-type description, distinct from any individual field's own `Description(...)` call

**Independent Test**: reproduce INSIGHT.md's `UserEntity` example verbatim (all 7 fields: `Id`/`Name`/`Email`/`IsActive`/`CreatedAt`/`UpdatedAt`/`DeletedAt`, minus the branch-specific calls like `.Integer()`/`.String()` which don't exist yet — see Out of Scope), assert each field's `Required`/`Nullable`/`Description`/`Examples` were stored correctly and are attributable to the RIGHT field (not accidentally swapped/aliased with a neighboring field).

---

## Edge Cases

- WHEN `Property(&t.X)` is called TWICE for the SAME field `X` (a dev mistake, or a deliberate "add more constraints later" pattern) THEN system SHALL either merge onto the same stored entry or panic with a clear "field already registered" message — pick ONE deterministic behavior and document it clearly (this feature's Design phase decides which, since INSIGHT.md's own example never does this and doesn't show which is intended)
- WHEN `NewMetadata[T]` is called for a `T` that is not a struct (e.g. a primitive, a map, a slice) THEN system SHALL panic with a clear message at call time — `Property(&t.X)` fundamentally requires `T` to have addressable fields
- WHEN two SEPARATE `NewMetadata[SameType]` calls exist for the same Go type `T` in the same program (a dev accidentally declares metadata for `UserEntity` twice) THEN this feature makes NO promise about deduplication/conflict detection — each `NewMetadata` call produces its own independent `*Metadata` value, out of scope to reconcile (a later feature, if a real need shows up, could add a registry keyed by type)

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| MDR-01 | P1: NewMetadata[T] constructs and runs fn correctly | Design | Pending |
| MDR-02 | P1: Property(&t.X) correctly identifies the field | Design | Pending |
| MDR-03 | P1: Property with a foreign pointer panics clearly | Design | Pending |
| MDR-04 | P1: Required/Nullable/Description/Examples chain correctly, same return type | Design | Pending |
| MDR-05 | P1: top-level Metadata.Description distinct from field-level | Design | Pending |

**ID format:** `MDR-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 5 total, 0 mapped to tasks yet, 5 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] `Property(&t.X)` correctly identifies the right field for every field in a multi-field struct (INSIGHT.md's 7-field `UserEntity` example, verified field-by-field, not just "compiles")
- [ ] A foreign/invalid pointer passed to `Property` fails loudly and clearly, not silently
- [ ] Zero regressions in the existing test suite (Milestones 1-3 complete, ~17 packages before this feature starts)
