# Array Builder Design

**Spec**: `.specs/features/array-builder/spec.md`

## Architecture Overview

```
internal/metadata (existing package, extended again)
        │
        ├── PropertyBuilder (existing, EXTENDED)
        │      + Array() -- NEW, sets p.format = "array", returns
        │        &ArrayMetadata{PropertyBuilder: p, item: &PropertyBuilder{}}
        │        (item is a SYNTHETIC PropertyBuilder -- zero reflect.StructField,
        │        never registered in Metadata.properties, exists only to REUSE
        │        StringMetadata/NumericMetadata's branch methods for the item's
        │        own Min/Max without writing new item-specific validator code)
        │
        └── ArrayMetadata (NEW, internal/metadata/array.go)
               embeds *PropertyBuilder (the FIELD container, e.g. `Tags`)
               + item *PropertyBuilder (synthetic, the array's element)
               + itemRef *Metadata (set instead of item's own format when the
                 element is a nested struct -- Items' Object(ref) case)
               + min, max *int (ARRAY quantity, separate from item's own
                 Min/Max which lives on the *StringMetadata/*NumericMetadata
                 wrapper returned by ArrayMetadata.String()/.Integer()/etc)

               Items(fn func(m *ArrayMetadata)) *ArrayMetadata
                 -- calls fn(am), returns am (same pointer -- lets the caller
                 chain .Min()/.Max() AFTER the callback for quantity)

               String()/Email()/Uuid()/.../Integer()/.../Boolean()/DateTime()/
               Date() -- ONE method per existing PropertyBuilder branch,
               EXCEPT each operates on am.item (not am.PropertyBuilder) and
               returns the SAME wrapper type the original branch already
               returns (*StringMetadata, *NumericMetadata, or bare
               *PropertyBuilder for Boolean/DateTime/Date) -- item gets
               Min/Max/Pattern FOR FREE, zero new validator code

               Object(ref *Metadata) *ArrayMetadata
                 -- sets am.itemRef = ref (equivalent $ref), returns am

               Min(v int)/Max(v int) *ArrayMetadata -- ARRAY quantity (own
               fields, distinct from item's)

               Required()/Nullable()/Description()/Examples() -- REDECLARED,
               same embed+redeclare pattern as StringMetadata/NumericMetadata,
               but delegate to am.PropertyBuilder (the FIELD, never am.item)
```

This feature introduces the first DUAL-STATE branch: `ArrayMetadata` holds two separate `*PropertyBuilder`-shaped things (`PropertyBuilder` for the field, `item` for the element) and routes every method to the correct one by DEFINITION, not by runtime inspection -- `Required`/`Nullable`/`Description`/`Examples`/`Min`/`Max` (own) always touch the field/quantity side; `String`/`Integer`/etc always touch `item`. The callback shape (`Items(fn func(m *ArrayMetadata))`, `m` being the SAME `*ArrayMetadata`) is what makes this unambiguous at the call site: everything inside the callback is a method on one object, so which state a method mutates is fixed by which method you called, not by chain position (see spec.md's Problem Statement -- this is why the user changed INSIGHT.md's `Items()` to callback-shaped instead of returning a freestanding item wrapper to chain off of).

---

## Components

### `PropertyBuilder` (existing, extended)

- **Purpose**: gains `Array()`, the entry point into the dual-state branch.
- **Location**: `internal/metadata/metadata.go` (existing, extended)
- **Interfaces** (addition only):
  - `func (p *PropertyBuilder) Array() *ArrayMetadata` -- `p.format = "array"`, returns `&ArrayMetadata{PropertyBuilder: p, item: &PropertyBuilder{}}` (fresh synthetic item builder every call -- calling `Array()` twice on the same `p` discards whatever item state the first `*ArrayMetadata` had, same last-write-wins precedent as every branch method, just at struct-allocation granularity instead of a single field)
- **Dependencies**: none new
- **Reuses**: `format` field (existing)

### `ArrayMetadata` (new type, same package)

- **Purpose**: the dual-state builder for arrays -- field-level constraints (embedded `*PropertyBuilder`) plus item-level constraints (`item *PropertyBuilder`, reusing every existing branch method) plus array-quantity `Min`/`Max` (own fields).
- **Location**: `internal/metadata/array.go` (new file, same package)
- **Interfaces**:
  - `type ArrayMetadata struct { *PropertyBuilder; item *PropertyBuilder; itemRef *Metadata; min, max *int }`
  - `func (am *ArrayMetadata) Items(fn func(m *ArrayMetadata)) *ArrayMetadata` -- `fn(am)`, `return am`
  - `func (am *ArrayMetadata) String() *StringMetadata` -- `am.item.format = ""`, `return &StringMetadata{PropertyBuilder: am.item}` (mechanical repeat of `PropertyBuilder.String()`'s own body, just against `am.item` instead of `p`)
  - Same mechanical repeat for `Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary` (→ `*StringMetadata`), `Integer`/`Int32`/`Float`/`Double` (→ `*NumericMetadata`), `Boolean`/`DateTime`/`Date` (→ bare `am.item`, a `*PropertyBuilder`) -- ALL 18 existing branch methods, each a 2-line repeat against `am.item`
  - `func (am *ArrayMetadata) Object(ref *Metadata) *ArrayMetadata` -- `am.itemRef = ref`, `return am`
  - `func (am *ArrayMetadata) Min(v int) *ArrayMetadata` / `func (am *ArrayMetadata) Max(v int) *ArrayMetadata` -- ARRAY quantity, own `min`/`max *int` fields (NOT the item's -- item's own `Min`/`Max` lives on the `*StringMetadata`/`*NumericMetadata` returned by the item branch methods above)
  - `func (am *ArrayMetadata) MinValue() (int, bool)` / `MaxValue() (int, bool)` -- quantity getters, same shape as `NumericMetadata`'s own
  - `func (am *ArrayMetadata) ItemBuilder() *PropertyBuilder` -- raw accessor to `am.item`, needed by a future consumer (Milestone 7's OpenAPI generator) to read the item's own `FormatValue()`/`Field()`-shaped state; `Field()` on a synthetic item returns a zero `reflect.StructField` (documented, not fixed here -- out of scope per spec.md)
  - `func (am *ArrayMetadata) ItemRef() (*Metadata, bool)` -- returns `am.itemRef` and whether `Object(ref)` was ever called
  - `func (am *ArrayMetadata) Required() *ArrayMetadata` / `Nullable()` / `Description(string)` / `Examples(...any)` -- REDECLARED, delegate to `am.PropertyBuilder`'s own methods (the FIELD, e.g. `Tags`), same shape as `StringMetadata`/`NumericMetadata`'s own redeclarations
- **Dependencies**: `StringMetadata`, `NumericMetadata` (both existing, reused as-is for item wrapping)
- **Reuses**: every existing branch method's OWN logic, applied to `am.item` instead of `p` -- zero new validator logic, only routing

---

## Data Models

```go
// internal/metadata/array.go, NEW:
type ArrayMetadata struct {
    *PropertyBuilder        // the FIELD (e.g. Tags []string) -- format="array"
    item    *PropertyBuilder // synthetic, the array's ELEMENT -- never registered
    itemRef *Metadata        // set instead of item's format when element is a nested struct
    min, max *int            // ARRAY quantity (item's own Min/Max lives elsewhere, see above)
}
```

**Relationships**: `ArrayMetadata.PropertyBuilder` is the SAME shared object already in `Metadata.properties[offset]` (identical relationship to `StringMetadata`/`NumericMetadata`). `ArrayMetadata.item` is a BRAND NEW `*PropertyBuilder`, never stored in any `Metadata.properties` map -- it exists purely as a receiver for the reused branch methods (`String()`/`Integer()`/etc), giving the item `Min`/`Max`/`Pattern` for free through `StringMetadata`/`NumericMetadata`'s own existing logic.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| `Array()` called twice on the same `*PropertyBuilder` | Second call allocates a NEW `*ArrayMetadata` (new `item`) -- first `*ArrayMetadata`'s item state is simply orphaned/discarded if the dev doesn't keep the first reference | Matches spec.md's Edge Cases: last-write-wins on `format`, same precedent as every branch; item state discarding is a natural consequence of allocating fresh, not a new failure mode to handle specially |
| Item branch method (`m.String()` etc) called more than once inside the same `Items(fn)` callback | `am.item.format` simply overwritten, last call wins, no panic | Same precedent as top-level `PropertyBuilder` branch methods |
| `Items(fn)` never called (only `Array()`) | Item stays zero-value (`format == ""`, no `itemRef`) | Spec.md's Edge Cases -- no panic, future OpenAPI consumer decides how to treat |
| `Min`/`Max` with `min > max` (either quantity or item-level) | Not validated | Same "trust the caller" stance as every prior metadata feature |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Item is a SYNTHETIC `*PropertyBuilder` (`&PropertyBuilder{}`, zero `reflect.StructField`), not a new lightweight type | Reuse `PropertyBuilder` itself as the item's storage | The ENTIRE point of this design is to get item `Min`/`Max`/`Pattern` for free by reusing `StringMetadata`/`NumericMetadata` unchanged -- those types are hard-coded to embed `*PropertyBuilder`, so the item MUST be a real `*PropertyBuilder` for `am.item.format = ""; return &StringMetadata{PropertyBuilder: am.item}` to type-check and behave identically to the top-level branch methods. A lighter "ItemBuilder" interface would require either duplicating `StringMetadata`/`NumericMetadata` against it or making those types generic over the receiver -- both strictly more code for zero behavioral gain. |
| `ArrayMetadata` has ITS OWN `min, max *int` fields (quantity), SEPARATE from the item's `Min`/`Max` (which lives on the wrapper returned by `am.String()` etc) | Two independent Min/Max storages | This is the crux of spec.md's Problem Statement: `Array().Items(func(m){ m.String().Min(1).Max(50) ... }).Min(1)` needs BOTH item-length bounds (1..50 chars) AND array-quantity bounds (at least 1 address) to coexist without collision. Because `m.String().Min(...)` returns `*StringMetadata` (bound to `am.item`) while `am.Min(...)` (called directly on `*ArrayMetadata`, e.g. chained after `Items(fn)` returns) is a DIFFERENT method with a DIFFERENT receiver, Go's own method dispatch keeps them from ever colliding -- no runtime disambiguation needed, it falls out of the type system for free. |
| `Required`/`Nullable`/`Description`/`Examples` on `ArrayMetadata` ALWAYS delegate to the field (`am.PropertyBuilder`), never `am.item` | Item never exposes these 4 methods at all (no `Required`/`Nullable`/`Description`/`Examples` methods on item's synthetic builder are ever called through `ArrayMetadata`'s own surface) | Spec.md's Acceptance Criteria (AR-04), confirmed by the user's own INSIGHT.md example: `m.Required()`/`m.Description(...)` inside the `Items(fn)` callback are called on `m` (the `*ArrayMetadata` itself) as SEPARATE statements, never chained off `m.String().Min(1).Max(50)` -- there is no ambiguity to resolve at the type level because the field-level 4 methods are ONLY ever reachable through `*ArrayMetadata` itself, never through the item wrapper types (`*StringMetadata`'s own `Required()` exists structurally since it's reused as-is, but nothing in `ArrayMetadata`'s documented usage ever calls it -- if a dev calls `am.String().Required()` by mistake, it silently sets the SYNTHETIC item's own orphaned `required` field, which nothing reads; harmless but worth noting as a known sharp edge, not fixed here since INSIGHT.md's own examples never do this) |
| `Items(fn)` takes `func(m *ArrayMetadata)`, not `func(m *ArrayMetadata) *ArrayMetadata` or similar | Plain callback, `Items` itself returns `am` after invoking `fn` | Matches the exact shape the user wrote into INSIGHT.md (`Items(func(m *gonest.ArrayMetadata) {...}).Min(1)`) -- `Items` is what carries the return value forward, not the callback |
| `Object(ref *Metadata) *ArrayMetadata` returns `am` (not `void`, not `*NumericMetadata`-shaped item wrapper) | Consistent return-`am`-for-chaining, matches `Items(fn).Min(1)` pattern already established | INSIGHT.md's `Addresses` example calls `m.Object(addressMetadata)` as its own statement (not chained further at the item level -- an object-typed item has no `Min`/`Max`/`Pattern` of its own), so returning `am` costs nothing and stays consistent with every other item-branch method's return-something-chainable habit, even though nothing in the current examples chains off it |

---

## Open Questions pra Tasks

- None -- semantics resolved via user's own INSIGHT.md edit (callback-shaped `Items`) plus this design's dual-`*PropertyBuilder` routing. Implementation is mechanical: 18 near-identical 2-line item-branch methods (copy `PropertyBuilder`'s own body, redirect receiver to `am.item`), plus the field-level redeclare (mechanical repeat of `StringMetadata`/`NumericMetadata`'s own pattern).
