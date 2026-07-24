# Accessor Sync Specification

## Problem Statement

When implementing PATCH-style handlers or domain entity update methods (e.g. `entity.Update(dto)`), a caller currently has to manually call `e.Name.Sync(&props.Name)` or `props.Name.Sync(&e.Name)` for every individual `gonest.Accessor[T]` field. In structs with many properties, this is repetitive and error-prone.

## Goals

- [x] Add `gonest.SyncAccessorFields(dst any, src any)` (and `internal/accessor.SyncAccessorFields(dst, src)`) to automatically synchronize dirty `Accessor[T]` fields from `src` to `dst`.
- [x] Support matching fields by field name (e.g., `Name`) and by `json` tag name (e.g., `json:"name"`).
- [x] Support `dst` fields being either `gonest.Accessor[T]`, concrete primitive/struct `T`, or pointer `*T`.
- [x] Recursively traverse embedded (anonymous) structs in both `src` and `dst` (e.g., `PersonEntity` embedding `PersonProps`).
- [x] Re-export `gonest.SyncAccessorFields` at the root of `gonest.go`.

## Out of Scope

| Feature | Reason |
| --- | --- |
| Automatic sync for non-Accessor fields in `src` | Accessor sync is specifically designed around dirty-tracking. Non-Accessor fields in `src` do not carry dirty state and should not be blindly copied. |
| Non-struct `src` or `dst` | Reflection-based struct field matching only applies to struct and pointer-to-struct types. Passing primitives or slices panics with a clear message. |

---

## User Stories

### P1: `SyncAccessorFields(dst, src)` syncs dirty Accessor fields ⭐ MVP

**User Story**: As a developer, I want `gonest.SyncAccessorFields(entity, props)` to automatically copy all dirty `Accessor[T]` fields from `props` into `entity` without manually invoking `.Sync()` or `.Set()` per field.

**Acceptance Criteria**:
1. WHEN `SyncAccessorFields(dst, src)` is called AND `src` contains a dirty `Accessor[T]` field THEN the corresponding field in `dst` SHALL be updated.
2. WHEN `src` contains a non-dirty `Accessor[T]` field THEN the corresponding field in `dst` SHALL remain unchanged.
3. WHEN `dst` field is an `Accessor[T]` THEN updating it SHALL also mark `dst`'s field as dirty.
4. WHEN `dst` field is a raw type `T` (e.g., `string`) or pointer `*T` THEN updating it SHALL set the value directly into `dst`.
5. WHEN `src` or `dst` contains embedded (anonymous) structs THEN `SyncAccessorFields` SHALL inspect fields within embedded structs recursively.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| ACC-01 | P1: Sync dirty Accessor -> Accessor in dst | Verified | Verified |
| ACC-02 | P1: Sync dirty Accessor -> raw T or *T in dst | Verified | Verified |
| ACC-03 | P1: Skip non-dirty fields in src | Verified | Verified |
| ACC-04 | P1: Traverse embedded structs in src and dst | Verified | Verified |
| ACC-05 | P1: Re-export gonest.SyncAccessorFields at root | Verified | Verified |

**ID format:** `ACC-[NUMBER]`
