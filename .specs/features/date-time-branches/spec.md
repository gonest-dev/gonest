# Date/Time Branches Specification

## Problem Statement

"Numeric & Boolean Branches" proved a branch can skip its own wrapper type entirely when it has no format-specific extra validators (`Boolean()`). INSIGHT.md's branch list shows `DateTime()` (`format: date-time`) and `Date()` (`format: date`) with NO extra validators documented (unlike String's `Min/Max/Pattern` or Numeric's `Min/Max`) -- same simplification applies to both.

## Goals

- [ ] `Property(&t.X)` gains `DateTime()` -- sets `format = "date-time"`, returns bare `*PropertyBuilder` (no wrapper)
- [ ] `Property(&t.X)` gains `Date()` -- sets `format = "date"`, returns bare `*PropertyBuilder` (no wrapper)
- [ ] Reproduce INSIGHT.md's `UserEntity.CreatedAt`/`UpdatedAt`/`DeletedAt` chains verbatim (`DateTime().Required()...`, `DateTime().Nullable()...`)

## Out of Scope

| Feature | Reason |
| --- | --- |
| Array/Object nested metadata | Milestone 5, AD-002 already settled |
| Reading/using format+validators for OpenAPI/validation | Milestones 6-7 |
| Any Min/Max/Pattern-equivalent for date/time | INSIGHT.md lists none for this family |

---

## User Stories

### P1: DateTime and Date, both wrapper-less ⭐ MVP

**User Story**: As a gonest user, I want `m.Property(&t.CreatedAt).DateTime().Required().Description(...).Examples(time.Now())` (INSIGHT.md's own example) and an equivalent `Date()` call to compile and store correctly.

**Acceptance Criteria**:

1. WHEN `DateTime()` is called on a `*PropertyBuilder` THEN system SHALL set `format = "date-time"` and return the SAME `*PropertyBuilder` (identity, no new type)
2. WHEN `Date()` is called on a `*PropertyBuilder` THEN system SHALL set `format = "date"` and return the SAME `*PropertyBuilder`
3. WHEN `Required`/`Nullable`/`Description`/`Examples` are called after either THEN system SHALL behave exactly as `PropertyBuilder`'s own methods (trivially true, same as `Boolean()`'s precedent)

**Independent Test**: reproduce INSIGHT.md's `CreatedAt`/`UpdatedAt` (`DateTime().Required()`) and `DeletedAt` (`DateTime().Nullable()`, with `Examples(nil, time.Now())`) chains; add one `Date()` case not shown in INSIGHT.md to prove it's not missed; assert `FormatValue()` per field and pointer identity (`got == p`) for both branches.

---

## Edge Cases

- WHEN `DateTime()`/`Date()` called after another branch (or vice versa) on the same `*PropertyBuilder` THEN last-write-wins on `format`, no panic -- same precedent as every prior branch feature

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| DT-01 | P1: DateTime() sets format=date-time, returns bare *PropertyBuilder | T1 | Done |
| DT-02 | P1: Date() sets format=date, returns bare *PropertyBuilder | T1 | Done |

**ID format:** `DT-[NUMBER]`

**Coverage:** 2 total, 2 mapped.

---

## Success Criteria

- [x] Both branches work end-to-end, individually verified
- [x] INSIGHT.md's `CreatedAt`/`UpdatedAt`/`DeletedAt` chains compile and store correctly
- [x] Zero regressions in existing test suite (`go test ./...` all green, commit `558e587`)
