# Object Builder Design

**Spec**: `.specs/features/object-builder/spec.md`

## Architecture Overview

```
internal/metadata (existing package, extended again)
        │
        ├── PropertyBuilder (existing, EXTENDED)
        │      + Object(fn func(om *ObjectMetadata)) -- NEW, sets
        │        p.format = "object", builds &ObjectMetadata{PropertyBuilder: p}
        │        (SAME p, no synthetic item -- unlike Array(), Object has no
        │        item/element concept, the field itself IS the object), runs
        │        fn(om), returns om
        │
        └── ObjectMetadata (NEW, internal/metadata/object.go)
               embeds *PropertyBuilder (the SAME shared object as the field --
               single-state, no dual-builder complexity ArrayMetadata needed)
               + ref *Metadata (set by Metadata(ref), equivalent $ref)
               + additionalProperties bool (set by AdditionalProperties())

               Metadata(ref *Metadata) *ObjectMetadata -- om.ref = ref, return om
               AdditionalProperties() *ObjectMetadata -- om.additionalProperties = true, return om
               MetadataRef() (*Metadata, bool) -- getter
               IsAdditionalProperties() bool -- getter

               Required()/Nullable()/Description()/Examples() -- REDECLARED,
               delegate DIRECTLY to om.PropertyBuilder (the field) -- no
               routing ambiguity to resolve (unlike ArrayMetadata's field-vs-
               item split), since ObjectMetadata has exactly ONE underlying
               PropertyBuilder, same one whether called inside or outside
               the Object(fn) callback
```

`ObjectMetadata` is architecturally SIMPLER than `ArrayMetadata` (AD-011's dual-state design) -- it needs the callback shape (`Object(fn func(om *ObjectMetadata))`) only for API-shape consistency with `Array()`/`Items()` (per the user's own INSIGHT.md), not because it resolves any real ambiguity: there is only ONE `*PropertyBuilder` in play here, so `om.Required()` called inside the callback and `Object(fn).Nullable()` called after it returns mutate the exact same object either way. This is worth documenting explicitly since a future reader might expect `ObjectMetadata` to need a synthetic secondary builder the way `ArrayMetadata` does -- it does not.

---

## Components

### `PropertyBuilder` (existing, extended)

- **Purpose**: gains `Object()`, the entry point into the (simple, single-state) object branch.
- **Location**: `internal/metadata/metadata.go` (existing, extended)
- **Interfaces** (addition only):
  - `func (p *PropertyBuilder) Object(fn func(om *ObjectMetadata)) *ObjectMetadata` -- `p.format = "object"`, `om := &ObjectMetadata{PropertyBuilder: p}`, `fn(om)`, `return om`
- **Dependencies**: `ObjectMetadata` (new, same package)
- **Reuses**: `format` field (existing)

### `ObjectMetadata` (new type, same package)

- **Purpose**: the field-level builder for nested-object schemas -- either a reused `*Metadata` reference (`$ref`) or an open/free-form schema flag.
- **Location**: `internal/metadata/object.go` (new file, same package)
- **Interfaces**:
  - `type ObjectMetadata struct { *PropertyBuilder; ref *Metadata; additionalProperties bool }`
  - `func (om *ObjectMetadata) Metadata(ref *Metadata) *ObjectMetadata` -- `om.ref = ref`, `return om`
  - `func (om *ObjectMetadata) MetadataRef() (*Metadata, bool)` -- `return om.ref, om.ref != nil`
  - `func (om *ObjectMetadata) AdditionalProperties() *ObjectMetadata` -- `om.additionalProperties = true`, `return om`
  - `func (om *ObjectMetadata) IsAdditionalProperties() bool` -- `return om.additionalProperties`
  - `func (om *ObjectMetadata) Required() *ObjectMetadata` / `Nullable()` / `Description(string)` / `Examples(...any)` -- REDECLARED, delegate to `om.PropertyBuilder`'s own methods, same shape as `StringMetadata`/`NumericMetadata`/`ArrayMetadata`'s own redeclarations
- **Dependencies**: none new
- **Reuses**: `PropertyBuilder`'s own `Required`/`Nullable`/`Description`/`Examples` (called internally, same object as the field)

---

## Data Models

```go
// internal/metadata/object.go, NEW:
type ObjectMetadata struct {
    *PropertyBuilder                // the FIELD itself (e.g. Address AddressEntity) -- format="object"
    ref                  *Metadata  // set by Metadata(ref) -- equivalent $ref
    additionalProperties bool       // set by AdditionalProperties() -- open/free-form schema
}
```

**Relationships**: `ObjectMetadata.PropertyBuilder` is the SAME shared object already in `Metadata.properties[offset]` -- identical relationship to `StringMetadata`/`NumericMetadata`/`ArrayMetadata.PropertyBuilder` (the field side). Unlike `ArrayMetadata`, there is no second synthetic `*PropertyBuilder` -- `ObjectMetadata` has exactly one state to route to.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| Both `Metadata(ref)` and `AdditionalProperties()` called in the same callback | Both stored independently, no panic | Spec.md's Edge Cases -- no example needs mutual exclusion, future OpenAPI consumer decides priority |
| `Object(fn)` called twice on the same `*PropertyBuilder` | Second call builds a NEW `*ObjectMetadata` wrapping the SAME `p` -- `ref`/`additionalProperties` from the first call are simply not carried into the second (no shared mutable state beyond `p` itself, which only holds `format`/`required`/etc, not `ref`) | Same last-write-wins precedent as every branch; simpler than `ArrayMetadata`'s case since there's no orphaned synthetic item to worry about |
| `fn` is `nil` | Not defended against -- calling a nil func panics with Go's own runtime error | Spec.md's Edge Cases -- "trust the caller" stance, consistent with every prior metadata feature |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `ObjectMetadata` has NO synthetic secondary `*PropertyBuilder` (unlike `ArrayMetadata.item`) | Single embedded `*PropertyBuilder`, same object as the field | `Array()` needed a synthetic item because the ELEMENT's type/format (e.g. "each string 1-50 chars") is genuinely distinct from the FIELD's own type/format ("array", required, description). `Object()` has no such split -- the field itself IS the object, there's no separate "element" to describe. `Object()`'s callback shape exists purely to mirror `Items(fn)`'s API surface (per INSIGHT.md), not because it resolves a real dual-scope ambiguity the way AD-011 did for Array. |
| `Required`/`Nullable`/`Description`/`Examples` behave IDENTICALLY whether called inside or outside the `Object(fn)` callback | No special-casing needed | Direct consequence of the single-state design above -- `om` passed into `fn` and `om` returned by `Object(fn)` are the literal same pointer, so there is no "inside vs outside" distinction to make, unlike `ArrayMetadata` where the callback's SCOPE (item vs field) mattered. This is the one design point worth calling out explicitly, since a reader familiar with `ArrayMetadata`'s dual-state routing might expect similar complexity here and find none. |
| `AdditionalProperties()` is a bare flag (`bool`), not a nested schema builder | `additionalProperties bool` only -- no support yet for typing the open schema's own values | INSIGHT.md's only free-form example (`om.AdditionalProperties()`, zero args) never demonstrates typing the open schema further -- adding that capability speculatively would be scope creep beyond what any example calls for; can be added later if a real need appears (same "don't invent unused API surface" stance as `numeric-boolean-branches`'s decision to skip a `Pattern`-equivalent for numerics) |

---

## Open Questions pra Tasks

- None -- `ObjectMetadata` is a mechanical, SIMPLER repeat of the embed+redeclare pattern already established (`StringMetadata`/`NumericMetadata`/`ArrayMetadata`), with zero dual-state routing to design (unlike `ArrayMetadata`, which needed AD-011's callback-scoped item/field split).
