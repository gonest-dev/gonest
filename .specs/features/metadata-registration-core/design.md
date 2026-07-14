# Metadata Registration Core Design

**Spec**: `.specs/features/metadata-registration-core/spec.md`

## Architecture Overview

```
internal/metadata (new package, AD-004 pattern -- first package of an
entirely new domain: schema/reflection, not DI graph or HTTP dispatch)
        │
        ├── Metadata          -- top-level, one per NewMetadata[T] call
        │      Description(string) *Metadata
        │      Property(fieldPtr any) *PropertyBuilder
        │
        └── PropertyBuilder   -- one per field registered via Property
               Required() *PropertyBuilder
               Nullable() *PropertyBuilder
               Description(string) *PropertyBuilder
               Examples(...any) *PropertyBuilder
               (getters: IsRequired/IsNullable/DescriptionText/ExamplesList
                -- see Tech Decisions for the setter/getter naming split)

root gonest package (gonest.go, per AD-009): NewMetadata[T] (generic wrapper,
Go can't var-alias a generic func -- same idiom as MustInject/NewApp),
type Metadata = metadata.Metadata, type PropertyBuilder = metadata.PropertyBuilder
```

`NewMetadata[T any](fn func(t *T, m *Metadata)) *Metadata` is generic at the ROOT level (needs `T` to build a zero-value `*T` and know its `reflect.Type`), but the INTERNAL package's own constructor is a plain function taking `reflect.Type` directly (Go's "no type params on methods" rule, L-001 in STATE.md, doesn't block this since `New` here is a free function, not a method — but the internal constructor still can't be `New[T any]` if it needs to be called from a NON-generic context cleanly; see Tech Decisions for the exact split between root's generic wrapper and internal's type-erased core).

---

## Components

### `Metadata` (top-level, one per `NewMetadata[T]` call)

- **Purpose**: holds the whole-type description plus every field registered via `Property`.
- **Location**: `internal/metadata/metadata.go` (new file, new package)
- **Interfaces**:
  - `type Metadata struct { structType reflect.Type; baseAddr uintptr; description string; properties map[uintptr]*PropertyBuilder }` (all unexported)
  - `func New(structType reflect.Type, baseAddr uintptr) *Metadata` -- internal, type-erased constructor (see Tech Decisions for why `reflect.Type`/`uintptr` here, not a generic `T`)
  - `func (m *Metadata) Description(s string) *Metadata` -- sets the whole-type description, returns `m` for chaining
  - `func (m *Metadata) DescriptionText() string` -- getter (see Tech Decisions for setter/getter naming split)
  - `func (m *Metadata) Property(fieldPtr any) *PropertyBuilder` -- identifies which field of the type `m` was built for is being referenced by `fieldPtr`'s address, panics clearly if `fieldPtr` doesn't belong to that type or was already registered (spec.md's Edge Cases: panic on double-registration, not silent merge)
  - `func (m *Metadata) OwnProperties() []*PropertyBuilder` -- defensive-copy accessor for a later feature (branches, then OpenAPI/validation consumers) to walk every registered field
- **Dependencies**: `reflect`, `unsafe` (for `uintptr(unsafe.Pointer(...))` address arithmetic -- see Tech Decisions)
- **Reuses**: nothing -- first type in a new domain

### `PropertyBuilder` (one per field registered via `Property`)

- **Purpose**: holds one field's own constraints (`Required`/`Nullable`/`Description`/`Examples` in THIS feature; future branch features add type+format-specific methods on top).
- **Location**: `internal/metadata/metadata.go` (same file -- small, tightly coupled to `Metadata`, no reason to split yet)
- **Interfaces**:
  - `type PropertyBuilder struct { field reflect.StructField; required, nullable bool; description string; examples []any }` (all unexported)
  - `func (p *PropertyBuilder) Required() *PropertyBuilder`
  - `func (p *PropertyBuilder) Nullable() *PropertyBuilder`
  - `func (p *PropertyBuilder) Description(s string) *PropertyBuilder`
  - `func (p *PropertyBuilder) Examples(examples ...any) *PropertyBuilder`
  - Getters: `IsRequired() bool`, `IsNullable() bool`, `DescriptionText() string`, `ExamplesList() []any` (defensive copy for the slice)
  - `func (p *PropertyBuilder) Field() reflect.StructField` -- exposes which struct field this builder is for (needed by later branch/OpenAPI/validation features to know the field's own Go type/json tag/name)
- **Dependencies**: `reflect`
- **Reuses**: nothing

---

## Data Models

```go
// internal/metadata
type Metadata struct {
    structType reflect.Type
    baseAddr   uintptr
    description string
    properties map[uintptr]*PropertyBuilder // keyed by field offset from baseAddr
}

type PropertyBuilder struct {
    field       reflect.StructField
    required    bool
    nullable    bool
    description string
    examples    []any
}
```

**Field identification algorithm** (the core, non-obvious mechanism this feature depends on):

```go
// root gonest.go's generic wrapper:
func NewMetadata[T any](fn func(t *T, m *Metadata)) *Metadata {
    var zero T
    m := metadata.New(reflect.TypeOf(zero), uintptr(unsafe.Pointer(&zero)))
    fn(&zero, m)
    return m
}

// internal/metadata's Property:
func (m *Metadata) Property(fieldPtr any) *PropertyBuilder {
    fieldAddr := reflect.ValueOf(fieldPtr).Pointer()
    offset := fieldAddr - m.baseAddr
    if _, exists := m.properties[offset]; exists {
        panic("gonest: field already registered via Property")
    }
    field, ok := findFieldByOffset(m.structType, uintptr(offset))
    if !ok {
        panic("gonest: Property(...) pointer does not belong to the type passed to NewMetadata")
    }
    pb := &PropertyBuilder{field: field}
    m.properties[offset] = pb
    return pb
}

func findFieldByOffset(t reflect.Type, offset uintptr) (reflect.StructField, bool) {
    for _, f := range reflect.VisibleFields(t) {
        if f.Offset == offset {
            return f, true
        }
    }
    return reflect.StructField{}, false
}
```

**Relationships**: `zero` (the `T` value `NewMetadata` constructs) lives as a local variable inside the ROOT wrapper's stack frame for the ENTIRE duration of `fn`'s synchronous call -- `fn` runs INSIDE `NewMetadata`, never after it returns, so `&zero`'s address stays valid and stable throughout every `Property(&t.X)` call `fn` makes. This is the same "escape analysis keeps it alive as long as something holds a live reference during the call" guarantee any local-variable-address-taking Go code relies on -- not fragile, but worth stating explicitly since the whole mechanism depends on it.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| `Property(&t.X)` called with a pointer NOT belonging to the `T` passed to `NewMetadata` (e.g. a local variable, a different struct's field) | `findFieldByOffset` finds no match at that offset, panics with a clear message identifying the problem | spec.md AC3 -- fails loudly at declaration time, never silently corrupts another field's metadata |
| `Property(&t.X)` called TWICE for the same field | Panics with a clear "already registered" message | spec.md's Edge Cases -- deterministic, chosen over silent merge because merge semantics (does the second call's `Required()` OVERRIDE or ADD to the first's?) are genuinely ambiguous and INSIGHT.md never shows this case; panic is the safe default, matches this codebase's "fail fast over guess" convention |
| `NewMetadata[T]` called for a non-struct `T` | `reflect.TypeOf(zero).Kind() != reflect.Struct` check, panic with clear message | spec.md's Edge Cases -- `Property` fundamentally requires addressable struct fields |
| Two separate `NewMetadata[SameType]` calls for the same Go type | No detection, no promise -- each is independent | spec.md's Edge Cases, explicitly out of scope for this feature |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Field identification via pointer-address-offset comparison (`unsafe.Pointer` arithmetic + `reflect.VisibleFields`), not struct tags or a field-name string | `Property(&t.X)`'s argument IS the field's own address; offset from the struct's own base address identifies which `reflect.StructField` it is | This is the ONLY way to satisfy INSIGHT.md's exact `m.Property(&t.Id)` call shape (no tag string, no field-name string) -- it's a known, established technique in the Go ecosystem for struct-field-identifying builders (the same class of technique ORM/struct-mapping libraries use for "which field is this" without reflection-by-name or tags). **Not independently re-verified via external docs this session** (no third-party library is being used, this is a from-scratch implementation of a known PATTERN, not a library API) -- T1's own test suite is what proves it actually works for this codebase's specific field kinds (int64, string, bool, time.Time, *time.Time per spec.md's Independent Test), flagged here per the Knowledge Verification Chain's Step 5 ("flag uncertain, don't fabricate") since it's implemented from general Go knowledge, not confirmed against a specific authoritative source this session |
| Root's `NewMetadata[T any]` is generic (needs `T` to build the zero-value + know its type), but `internal/metadata.New` takes `reflect.Type`/`uintptr` directly, not a generic `T` | Type erasure at the internal-package boundary | Go doesn't allow type parameters on METHODS (L-001, STATE.md) -- `Metadata`'s own methods (`Property`, `Description`, etc.) can never be generic over `T`. The generic-ness only needs to exist at the ONE call site that constructs the zero value and needs `T`'s concrete type (`NewMetadata[T]` itself) -- everything downstream (`Metadata`/`PropertyBuilder`'s own methods) operates on `reflect.Type`/`uintptr`/`any`, no generics needed or possible. This mirrors how `internal/pipe`/`internal/exception` already avoid generic methods by working through `any`+reflect. |
| Setter/getter method-name split (`Description(s string) *Metadata` vs `DescriptionText() string`, `Required() *PropertyBuilder` vs `IsRequired() bool`) | Distinct names, not overloaded | Go has no method overloading -- `Description()` (getter, no args) and `Description(s string)` (setter, one arg, different return type) cannot coexist under the same name. Mirrors this codebase's existing precedent (`route.HttpCode(status)` setter vs `route.Code()` getter -- different names for the same underlying concept, already established in `internal/route/route.go`) rather than inventing a new naming convention |
| `Property` returns the SAME `*PropertyBuilder` type for every field regardless of kind (no branch-specific type yet) | Deliberately narrow for THIS feature | The problem of "each branch (`String()`, `Integer()`, etc.) needs to return a MORE SPECIFIC builder type with its own extra methods, while still keeping `Required`/`Nullable`/`Description`/`Examples` chainable" is real and non-trivial in Go (no method overloading, no "return-type polymorphism" without either code duplication per branch or a generic self-referencing pattern) -- but it is NOT this feature's problem to solve. This feature's `PropertyBuilder` is the COMMON BASE the next feature ("String-family Branches") will build branch-specific types on top of (most likely via embedding `*PropertyBuilder` inside each branch-specific type, each branch type re-declaring `Required`/`Nullable`/`Description`/`Examples` with its OWN return type -- mechanical duplication, but simple and magic-free; that decision belongs to the NEXT feature's own design.md, not this one) |

---

## Open Questions pra Tasks

- The double-`Property`-registration panic message wording and the non-struct-`T` panic message wording are implementation details, not architectural decisions -- follow this codebase's existing "gonest: <clear description>" panic message convention (see `internal/pipe`, `internal/route`) at implementation time, no separate decision round needed.
- T1's own test suite is what EMPIRICALLY confirms the pointer-offset field-identification technique works correctly for this codebase's needs (per the Tech Decisions table's "not independently re-verified" flag) -- if T1's developer finds the technique doesn't work as expected for some field kind, that's a real finding to report clearly, not something to silently work around.
