# String-family Branches Specification

## Problem Statement

"Metadata Registration Core" built the foundation (`Metadata`, `Property(&t.X)` returning a `*PropertyBuilder` with only the 4 common constraints: `Required`/`Nullable`/`Description`/`Examples`) but deliberately left EVERY type+format branch method (`.String()`, `.Email()`, etc.) unbuilt, flagging the Go-specific design problem this feature must solve: each branch needs to return a MORE SPECIFIC builder with its own extra validators, while keeping the 4 common constraints chainable, and Go has no method overloading or clean "return-type polymorphism." This feature builds the first branch family — the 10 string-shaped OpenAPI 3.1 type+format combinations INSIGHT.md lists (`String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`), all of which share the exact same extra validators (`Min`/`Max` for length, `Pattern` for an additional regex constraint) — and, in doing so, SETTLES the Go representation pattern every subsequent branch family (Numeric/Boolean, Date/Time) will mechanically repeat.

## Goals

- [ ] `Property(&t.X)`'s returned `*PropertyBuilder` gains 10 new methods (`String()`, `Email()`, `Uuid()`, `Uri()`, `Hostname()`, `Ipv4()`, `Ipv6()`, `Password()`, `Byte()`, `Binary()`), each returning a `*StringMetadata` builder tagged with that branch's own OpenAPI format string (or no format, for the bare `String()` case)
- [ ] `*StringMetadata` has `Min(int)`/`Max(int)`/`Pattern(string)` (length/regex validators) PLUS the 4 common constraints (`Required`/`Nullable`/`Description`/`Examples`) re-declared with `*StringMetadata` as their return type, so the FULL chain from INSIGHT.md's own examples works: `m.Property(&t.Email).Email().Required().Description("Email do usuário").Examples("[EMAIL_ADDRESS]")`, `m.Property(&t.Zip).String().Required().Pattern(...).Description("CEP").Examples("01310-100")`
- [ ] Settle, with a concrete working implementation, the Go pattern for "branch-specific builder embeds the common base, re-declares the 4 shared methods with its OWN return type" — documented clearly enough that the NEXT branch-family feature (Numeric & Boolean) can mechanically repeat it without re-deriving the approach

## Out of Scope

| Feature | Reason |
| --- | --- |
| Numeric (`Integer`/`Int32`/`Float`/`Double`), Boolean, Date/Time branches | ROADMAP.md's own next 2 features, separate scope. This feature proves the PATTERN works via the string family specifically; those features apply the same pattern to their own branch-specific validators (`Min`/`Max` for numbers, no extra validator for Boolean, etc.) |
| Array/Object nested metadata (`Array()`, `Object()`, `Items()`) | Milestone 5, AD-002 already settled that builder's shape (linear, variadic `Items`) -- unrelated to this feature's branch-return-type problem, since Array/Object wrap ANOTHER already-built metadata value rather than needing their own "which format" selection |
| Actually reading/using the registered format+validators for anything (OpenAPI schema output, runtime validation) | Milestones 6-7, same as "Metadata Registration Core"'s own Out of Scope -- this feature only builds the REGISTRATION side |
| Validating `Pattern`'s regex syntax at registration time (e.g. panicking on an invalid regex string) | INSIGHT.md's own examples pass raw pattern strings (e.g. `` `^\d{5}-?\d{3}$` ``) with no indication of eager validation -- out of scope unless a concrete need appears; this feature just stores whatever string is passed |

---

## User Stories

### P1: 10 string-shaped branches, each with Min/Max/Pattern plus the common 4 ⭐ MVP

**User Story**: As a gonest user, I want `m.Property(&t.Email).Email().Required().Description(...).Examples(...)` and `m.Property(&t.Zip).String().Required().Pattern(...).Description(...).Examples(...)` (INSIGHT.md's own exact examples) to compile and correctly store every constraint, for ALL 10 string-family branches, not just `String()` and `Email()`.

**Why P1**: This is the entire feature -- the 10 branches ARE the string family ROADMAP.md names, and getting the underlying Go pattern right here is what makes every later branch-family feature (Numeric/Boolean, Date/Time) mechanical rather than requiring its own design exploration.

**Acceptance Criteria**:

1. WHEN any of the 10 branch methods (`String()`, `Email()`, `Uuid()`, `Uri()`, `Hostname()`, `Ipv4()`, `Ipv6()`, `Password()`, `Byte()`, `Binary()`) is called on a `*PropertyBuilder` THEN system SHALL return a `*StringMetadata` value carrying that branch's own OpenAPI format string (empty for bare `String()`, `"email"` for `Email()`, `"uuid"` for `Uuid()`, etc. -- matching OpenAPI 3.1's own format vocabulary for each)
2. WHEN `.Min(int)`/`.Max(int)`/`.Pattern(string)` are called on a `*StringMetadata` THEN system SHALL store each value, retrievable via getters, EACH call returning the SAME `*StringMetadata` so the chain continues
3. WHEN `.Required()`/`.Nullable()`/`.Description(string)`/`.Examples(...any)` are called on a `*StringMetadata` (after a branch method already ran) THEN system SHALL behave identically in effect to calling them on a bare `*PropertyBuilder` (same underlying storage, same getters work) while STILL returning `*StringMetadata` (not `*PropertyBuilder`), so `.Min()`/`.Max()`/`.Pattern()` remain chainable afterward too, in ANY order INSIGHT.md's own linear-builder philosophy (AD-002) allows
4. WHEN a fully-chained `*StringMetadata` value is inspected (via `PropertyBuilder`'s own `Field()`/getters, reachable since `*StringMetadata` embeds `*PropertyBuilder`) THEN it SHALL be indistinguishable, for the base-4-constraint data, from what "Metadata Registration Core" already proved for the bare `*PropertyBuilder` case -- this feature adds a NEW layer, it does not change how the base data is stored or retrieved

**Independent Test**: reproduce INSIGHT.md's own `UserEntity`/`AddressEntity` field declarations that use string-family branches (`Name.String()`, `Email.Email()`, `Zip.String().Pattern(...)`) plus at least one call to each of the OTHER 7 branches not shown in INSIGHT.md's examples (`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary` -- construct a minimal struct exercising all 10 to prove none of the 10 was missed or behaves differently from the others), assert every stored value (format, min, max, pattern, required, nullable, description, examples) is correct per field.

---

## Edge Cases

- WHEN `.Min()`/`.Max()` are called with `min > max` (a dev mistake) THEN system SHALL NOT validate this at registration time (out of scope, matches "Metadata Registration Core"'s own "trust the caller, no eager business-rule validation" stance -- a later Milestone 6/7 consumer, if it ever cares, can check when it actually generates a schema/validates a request)
- WHEN a branch method is called TWICE on the SAME `*PropertyBuilder` (e.g. `p.String(); p.Email()`) THEN system SHALL treat this the same way "Metadata Registration Core" already decided for double-`Property`-registration is NOT directly analogous here (branch selection isn't field registration) -- this feature's Design phase decides the exact behavior (most likely: each call is independent, doesn't panic, since nothing stops a dev from discarding the first branch's return value and using the second; only the LAST branch actually assigned to a variable/chained further matters) -- document clearly, don't leave ambiguous
- WHEN `Examples(...any)` is called on a `*StringMetadata` with non-string example values (e.g. an `int`) THEN system SHALL NOT type-check examples against the branch's own expected value type at registration time -- same "trust the caller" stance, out of scope (a later validation-consuming feature might care, this one doesn't)

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| STR-01 | P1: all 10 branch methods return *StringMetadata with correct format | Design | Pending |
| STR-02 | P1: Min/Max/Pattern store correctly, chainable | Design | Pending |
| STR-03 | P1: common 4 constraints work identically through StringMetadata, still return *StringMetadata | Design | Pending |
| STR-04 | P1: StringMetadata's base data indistinguishable from bare PropertyBuilder's | Design | Pending |

**ID format:** `STR-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 4 total, 0 mapped to tasks yet, 4 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] All 10 string-family branches work end-to-end, each individually verified (not just 2-3 representative ones)
- [ ] The full INSIGHT.md chain shape (`.Email().Required().Description(...).Examples(...)` and `.String().Required().Pattern(...).Description(...).Examples(...)`) compiles and stores correctly
- [ ] The embed-and-redeclare Go pattern this feature settles is documented clearly enough (design.md) that "Numeric & Boolean Branches" (the next feature) can apply it mechanically
- [ ] Zero regressions in the existing test suite (~18 packages before this feature starts)
