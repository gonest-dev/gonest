# Numeric & Boolean Branches Design

**Spec**: `.specs/features/numeric-boolean-branches/spec.md`

## Architecture Overview

```
internal/metadata (existing package, extended again)
        │
        ├── PropertyBuilder (existing, EXTENDED)
        │      + 4 NEW numeric branch methods: Integer()/Int32()/Float()/
        │        Double(), each sets p.format then returns &NumericMetadata{p}
        │        (MECHANICAL REPEAT of "String-family Branches"'s exact pattern)
        │      + Boolean() -- NEW, returns p ITSELF (*PropertyBuilder), no
        │        wrapper -- the simpler case String-family never needed
        │
        └── NumericMetadata (NEW, internal/metadata/numeric.go)
               embeds *PropertyBuilder (pointer, same shared object)
               Min(int)/Max(int) -- numeric-specific (no Pattern, unlike
               StringMetadata -- INSIGHT.md's own comment only lists Min/Max
               for the numeric family, no regex-equivalent)
               Required()/Nullable()/Description()/Examples() -- REDECLARED,
               IDENTICAL shape to StringMetadata's own redeclarations
```

This feature does TWO things: (1) mechanically repeats "String-family Branches"'s exact embed-and-redeclare pattern for the 4 numeric branches (nothing new to decide here, just apply the settled pattern), and (2) introduces the FIRST branch that needs NO wrapper at all (`Boolean()`), which is architecturally simpler, not more complex -- worth documenting explicitly since it's the first time this codebase proves "not every branch needs its own type."

---

## Components

### `PropertyBuilder` (existing, extended)

- **Purpose**: gains the 4 numeric branch-selection methods plus `Boolean()`.
- **Location**: `internal/metadata/metadata.go` (existing, extended -- same file "String-family Branches" already extended once)
- **Interfaces** (additions only):
  - `func (p *PropertyBuilder) Integer() *NumericMetadata` -- `p.format = "int64"` (INSIGHT.md's own comment: "format: int64 default")
  - `func (p *PropertyBuilder) Int32() *NumericMetadata` -- `p.format = "int32"`
  - `func (p *PropertyBuilder) Float() *NumericMetadata` -- `p.format = "float"`
  - `func (p *PropertyBuilder) Double() *NumericMetadata` -- `p.format = "double"`
  - `func (p *PropertyBuilder) Boolean() *PropertyBuilder` -- sets `p.format = ""` (Boolean has no format, per INSIGHT.md's "sem format" comment -- explicitly setting it to empty rather than leaving whatever was there before matters if a dev calls `.Integer().Boolean()` by mistake, so `Boolean()` should still "claim" the format slot as empty, same last-write-wins precedent as every other branch), returns `p` itself, no wrapper constructed
- **Dependencies**: none new
- **Reuses**: the existing `format` field (added by "String-family Branches")

### `NumericMetadata` (new type, same package)

- **Purpose**: the branch-specific builder for all 4 numeric formats -- ONE type, mirroring `StringMetadata`'s own "one type per shared-validator-set family" reasoning.
- **Location**: `internal/metadata/numeric.go` (new file, same package)
- **Interfaces**:
  - `type NumericMetadata struct { *PropertyBuilder; min, max *int }` (no `pattern` field -- numeric family has no regex-equivalent validator per INSIGHT.md)
  - `func (n *NumericMetadata) Min(v int) *NumericMetadata`
  - `func (n *NumericMetadata) Max(v int) *NumericMetadata`
  - `func (n *NumericMetadata) MinValue() (int, bool)` / `func (n *NumericMetadata) MaxValue() (int, bool)`
  - `func (n *NumericMetadata) Required() *NumericMetadata` / `Nullable()` / `Description(string)` / `Examples(...any)` -- IDENTICAL redeclaration shape to `StringMetadata`'s own 4 methods, just a different receiver type
- **Dependencies**: none new
- **Reuses**: `PropertyBuilder`'s own `Required`/`Nullable`/`Description`/`Examples` (called internally)

---

## Data Models

```go
// internal/metadata/numeric.go, NEW:
type NumericMetadata struct {
    *PropertyBuilder
    min, max *int
}
```

**`Boolean()`'s simplification** (the one genuinely new architectural point this feature makes):

```go
func (p *PropertyBuilder) Boolean() *PropertyBuilder {
    p.format = ""
    return p
}
```

No wrapper struct, no redeclared methods -- `Required()`/`Nullable()`/`Description()`/`Examples()` are ALREADY `*PropertyBuilder`'s own methods (defined in "Metadata Registration Core"), so `m.Property(&t.IsActive).Boolean().Required().Description(...)` compiles and works with ZERO new method declarations beyond `Boolean()` itself. This is the DIRECT payoff of "Metadata Registration Core"'s original design: the base `PropertyBuilder` type was always meant to be usable as-is for a branch with no extra validators, not just as something every branch wraps.

**Relationships**: identical to `StringMetadata`'s established relationship with `PropertyBuilder` -- `NumericMetadata.PropertyBuilder` is a pointer to the SAME shared object already in `Metadata.properties[offset]`.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| Numeric branch called twice, or numeric-then-Boolean (or vice versa) | `format` simply overwritten, last call wins, no panic | Matches spec.md's Edge Cases, identical precedent to "String-family Branches" |
| `Min`/`Max` with `min > max` | Not validated | Same "trust the caller" stance as every prior metadata feature |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `NumericMetadata` has NO `Pattern` method (unlike `StringMetadata`) | Only `Min`/`Max` | INSIGHT.md's own comment block lists `Min/Max` for the numeric family with no regex-equivalent -- no reason to add a validator INSIGHT.md's example set never calls for; if a real need appears later, it can be added then, not speculatively now |
| `Boolean()` returns `*PropertyBuilder` directly, no `BooleanMetadata` wrapper type | Simplest possible branch -- literally return `p` | INSIGHT.md's own comment ("Boolean() -> sem format") states explicitly there's no format and (by omission, unlike every other branch's comment line) no extra validator either -- inventing an empty wrapper type (`type BooleanMetadata struct { *PropertyBuilder }` with zero extra fields/methods) would add a type with no reason to exist beyond "consistency for its own sake" -- this codebase's own convention (avoid unnecessary abstraction, `CLAUDE.md`-equivalent guidance already followed throughout this session) favors the simpler `*PropertyBuilder`-direct-return over a pointless wrapper |
| `Boolean()` explicitly sets `p.format = ""` rather than leaving whatever value was already there | Explicit empty-string assignment | Matches every other branch method's own behavior (each one "claims" the format slot, last-write-wins) -- if `Boolean()` did NOT touch `format`, calling `.Integer().Boolean()` would leave a STALE `"int64"` format on a field the dev clearly meant to be a plain boolean, which would be a real (if minor) surprise for a future OpenAPI-generation consumer reading `FormatValue()` |

---

## Open Questions pra Tasks

- None -- this feature is a mechanical repeat of "String-family Branches"'s already-settled pattern for the 4 numeric branches, plus one new, SIMPLER case (`Boolean()`) with a clear, low-risk design decision (return the bare `*PropertyBuilder`, no new type).
