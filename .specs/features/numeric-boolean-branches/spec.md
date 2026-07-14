# Numeric & Boolean Branches Specification

## Problem Statement

"String-family Branches" settled the Go pattern for a type+format branch that NEEDS its own extra validators beyond the 4 common ones (`Required`/`Nullable`/`Description`/`Examples`): embed `*PropertyBuilder`, store the chosen format on the SHARED object, redeclare the 4 common methods manually so the chain stays fluent. This feature applies that exact pattern to the 4 numeric branches (`Integer`/`Int32`/`Float`/`Double`, all sharing `Min`/`Max`) AND introduces a NEW, simpler case this codebase hasn't needed yet: `Boolean()`, which per INSIGHT.md's own comment ("Boolean() -> sem format") has NO format string and NO extra validators at all -- meaning it can return the bare `*PropertyBuilder` directly, no wrapper type needed.

## Goals

- [ ] `Property(&t.X)`'s returned `*PropertyBuilder` gains 4 new numeric branch methods (`Integer()`, `Int32()`, `Float()`, `Double()`), each returning a `*NumericMetadata` builder tagged with that branch's own OpenAPI format string, sharing `Min(int)`/`Max(int)` validators plus the 4 common constraints re-declared (same pattern as `StringMetadata`)
- [ ] `Property(&t.X)` gains a `Boolean()` method returning the BARE `*PropertyBuilder` itself (not a new wrapper type) -- since Boolean has no format and no extra validators per INSIGHT.md, there's nothing a wrapper type would add
- [ ] Confirm the "no format, no extra validators -> just return `*PropertyBuilder`" simplification is correct by testing that `Boolean()`'s return value still fully supports `Required`/`Nullable`/`Description`/`Examples` (trivially true since it IS `*PropertyBuilder`, but worth proving explicitly as the FIRST branch that doesn't need its own type)

## Out of Scope

| Feature | Reason |
| --- | --- |
| Date/Time branches (`DateTime`/`Date`) | ROADMAP.md's own next feature, separate scope |
| Array/Object nested metadata | Milestone 5, AD-002 already settled |
| Reading/using the registered format+validators for anything (OpenAPI/validation) | Milestones 6-7, same as every prior Metadata Builder feature's Out of Scope |
| Validating `Min > Max` at registration time | Same "trust the caller" stance already established in "Metadata Registration Core"/"String-family Branches" |
| A distinct `IntegerMetadata`/`FloatMetadata`/etc. per numeric branch | INSIGHT.md's own comment groups `Integer()`/`Int32()`/`Float()`/`Double()` under one shared `Min/Max` validator set -- same "one type serves the whole family" reasoning "String-family Branches" already used for its 10 branches, not 10 separate types |

---

## User Stories

### P1: 4 numeric branches sharing Min/Max, plus a format-less Boolean ⭐ MVP

**User Story**: As a gonest user, I want `m.Property(&t.Id).Integer().Required().Description(...).Examples(...)` (INSIGHT.md's own example, `UserEntity.Id`) and `m.Property(&t.IsActive).Boolean().Required().Description(...).Examples(...)` (INSIGHT.md's `UserEntity.IsActive`) to compile and correctly store every constraint, for all 4 numeric branches plus Boolean.

**Why P1**: This is the entire feature -- these 5 branches (4 numeric + Boolean) are what ROADMAP.md's "Numeric & Boolean Branches" names, and Boolean specifically tests whether the "no wrapper needed" simplification (a case String-family never needed) actually works cleanly.

**Acceptance Criteria**:

1. WHEN any of `Integer()`/`Int32()`/`Float()`/`Double()` is called on a `*PropertyBuilder` THEN system SHALL return a `*NumericMetadata` carrying that branch's own format string (`"int64"` for `Integer()` per INSIGHT.md's own comment "format: int64 default", `"int32"` for `Int32()`, `"float"` for `Float()`, `"double"` for `Double()`)
2. WHEN `.Min(int)`/`.Max(int)` are called on a `*NumericMetadata` THEN system SHALL store each value, EACH call returning the SAME `*NumericMetadata` so the chain continues, matching `StringMetadata`'s exact established pattern
3. WHEN `.Required()`/`.Nullable()`/`.Description(string)`/`.Examples(...any)` are called on a `*NumericMetadata` THEN system SHALL behave identically in effect to the bare `*PropertyBuilder` case (same underlying storage) while STILL returning `*NumericMetadata` (chain-preserving, same redeclaration pattern as `StringMetadata`)
4. WHEN `Boolean()` is called on a `*PropertyBuilder` THEN system SHALL return the SAME `*PropertyBuilder` value (not a new type) with its `format` field set to `""` (or left unset -- Design phase decides whether Boolean sets an explicit empty format or simply never touches it, since it has none to record) -- `Required`/`Nullable`/`Description`/`Examples` all still work because they're just `*PropertyBuilder`'s own methods

**Independent Test**: reproduce INSIGHT.md's `UserEntity.Id` (`Integer()`) and `UserEntity.IsActive` (`Boolean()`) chains verbatim, plus at least one call each to `Int32()`/`Float()`/`Double()` (not shown in INSIGHT.md's examples) to prove none of the 4 numeric branches was missed, asserting every stored value (format, min, max, required, nullable, description, examples) per field.

---

## Edge Cases

- WHEN a numeric branch method is called TWICE on the same `*PropertyBuilder` (e.g. `Integer()` then `Float()`) THEN system SHALL behave the same as `StringMetadata`'s own established precedent -- last-write-wins on `format`, no panic
- WHEN `Boolean()` is called AFTER a numeric/string branch already ran on the same `*PropertyBuilder` (or vice versa) THEN system SHALL simply overwrite `format` (to `""` for Boolean, since it has none) -- same last-write-wins stance, no cross-branch-family special-casing
- WHEN `.Min()`/`.Max()` are called with `min > max` THEN not validated, same stance as "String-family Branches"

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| NUM-01 | P1: 4 numeric branches return *NumericMetadata with correct format | Design | Pending |
| NUM-02 | P1: Min/Max store correctly, chainable | Design | Pending |
| NUM-03 | P1: common 4 constraints work through NumericMetadata, still return *NumericMetadata | Design | Pending |
| NUM-04 | P1: Boolean() returns bare *PropertyBuilder, no wrapper needed | Design | Pending |

**ID format:** `NUM-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 4 total, 0 mapped to tasks yet, 4 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] All 4 numeric branches work end-to-end, each individually verified
- [ ] `Boolean()`'s "no wrapper needed" simplification works cleanly, proven not just assumed
- [ ] The INSIGHT.md `UserEntity.Id`/`IsActive` chains compile and store correctly
- [ ] Zero regressions in the existing test suite (~18 packages before this feature starts)
