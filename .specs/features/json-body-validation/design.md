# JSON Body Validation Design

**Spec**: `.specs/features/json-body-validation/spec.md`
**Context**: `.specs/features/json-body-validation/context.md`

## Architecture Overview

```
internal/metadata (existing package, P0+P1 changes)
        │
        ├── PropertyBuilder (existing, EXTENDED with 8 new fields + getters)
        │      NEW fields (all unexported, all reused across branch families
        │      since only ONE branch family is ever active per instance --
        │      see Tech Decisions):
        │        kind                 string        -- NEW: OpenAPI "type"
        │                                               dimension, orthogonal
        │                                               to "format" (fixes
        │                                               the pre-existing
        │                                               String-vs-Boolean
        │                                               format=="" collision)
        │        min, max             *int          -- relocated from
        │                                               StringMetadata/
        │                                               NumericMetadata/
        │                                               ArrayMetadata
        │        pattern              string        -- relocated from
        │                                               StringMetadata
        │        item                 *PropertyBuilder -- relocated from
        │                                               ArrayMetadata
        │        itemRef              *Metadata     -- relocated from
        │                                               ArrayMetadata
        │        ref                  *Metadata     -- relocated from
        │                                               ObjectMetadata
        │        additionalProperties bool          -- relocated from
        │                                               ObjectMetadata
        │      NEW exported getters (mirror each wrapper's own getter name,
        │      so StringMetadata.MinValue()/etc keep compiling via Go's
        │      automatic promotion once the wrapper's OWN duplicate field is
        │      deleted -- zero method-body changes needed on any wrapper):
        │        MinValue() (int,bool) / MaxValue() (int,bool) /
        │        PatternValue() string / ItemBuilder() *PropertyBuilder /
        │        ItemRef() (*Metadata,bool) / MetadataRef() (*Metadata,bool) /
        │        IsAdditionalProperties() bool / KindValue() string
        │
        ├── EVERY branch method (String/Email/.../Binary, Integer/Int32/
        │      Float/Double, Boolean, DateTime/Date, Array, Object -- in
        │      metadata.go -- AND ArrayMetadata's 18 mirrored item-branch
        │      methods in array.go) gains ONE new line setting p.kind (or
        │      am.item.kind), alongside the format line it already sets
        │
        ├── registry.go (NEW FILE) -- process-wide map[reflect.Type]*Metadata
        │      Register(t, m) -- called from New() itself, panics on duplicate
        │      Lookup(t) (*Metadata, bool)
        │
        └── StringMetadata/NumericMetadata/ArrayMetadata/ObjectMetadata
               (existing files, MINIMAL edit): DELETE each wrapper's own
               now-duplicate field declarations (min/max/pattern on String;
               min/max on Numeric; item/itemRef/min/max on Array; ref/
               additionalProperties on Object) -- every method body that
               referenced those fields (e.g. `s.min = &n`) keeps compiling
               UNCHANGED, now resolving via Go's promoted-field access to the
               embedded *PropertyBuilder's own new fields (same package, so
               unexported promoted field access works)

internal/execution (existing package, P2 change)
        │
        ├── Responder interface -- gains Body() []byte
        └── Context -- gains Body() []byte, delegates to res.Body()

internal/adapter/fiber (existing package, P2 change)
        └── fiberResponder -- Body() []byte { return r.c.Body() } (no
              defensive copy needed -- see Tech Decisions)

internal/validate (NEW package -- mirrors internal/route's own role: reads
        execution.Context + internal/metadata + internal/exception, none of
        which should import each other back)
        │
        ├── MustJsonBody[T any](ctx *execution.Context) T -- real impl behind
        │      gonest.MustJsonBody[T] (T is a pointer type at the call site,
        │      e.g. MustJsonBody[*UserProperties])
        │        1. reflect.TypeOf(zero T).Elem() -- the pointed-to struct
        │           type, looked up via metadata.Lookup; panics "no metadata
        │           registered" if not found (spec.md's Edge Cases)
        │        2. json.Unmarshal body into `any` (map[string]any at the
        │           top level, for a struct-shaped body) -- ground truth for
        │           BOTH key-presence (Required checks) AND JSON value TYPE
        │           checking (context.md's Decision 1) -- parse failure here
        │           panics BadRequestException immediately (can't validate
        │           per-field if the JSON itself doesn't parse)
        │        3. validateStruct(presenceMap, m, "") walks m.OwnProperties(),
        │           recursing into validateValue per field -- COLLECTS every
        │           violation (context.md's Decision 2), does not stop early
        │        4. if violations exist: panic exception.NewBadRequestException(violations)
        │        5. otherwise: json.Unmarshal body AGAIN, this time into a
        │           fresh *StructType (reflect.New(structType).Interface()),
        │           and return it as T
        │
        └── validateValue/validateStruct/validateArray/validateObject --
               the recursive core (context.md's Decision 3), dispatches on
               PropertyBuilder.KindValue() ("string"/"integer"/"number"/
               "boolean"/"array"/"object"), never on FormatValue() alone
               (KindValue is what P0's fix makes reliable)
```

This feature is TWO layers: a blocking prerequisite fix (P0: give every branch's declared constraint a permanent home on the shared `PropertyBuilder`, plus close the String-vs-Boolean type-ambiguity gap with a new `kind` field) that touches 4 already-shipped features' internals with ZERO public API change, and the actual new capability (P1-P5: a registry so `MustJsonBody[T]` can find metadata without an explicit argument, body access on `Context`, and a recursive validator that walks `Metadata.OwnProperties()` collecting every violation).

---

## Components

### `PropertyBuilder` (existing, extended -- P0)

- **Purpose**: becomes the SOLE permanent home for every constraint any branch declares, closing the "wrapper discarded, data lost" gap discovered while designing this feature (see spec.md's P0 story).
- **Location**: `internal/metadata/metadata.go` (existing, extended)
- **New fields** (all unexported): `kind string`, `min, max *int`, `pattern string`, `item *PropertyBuilder`, `itemRef *Metadata`, `ref *Metadata`, `additionalProperties bool`
- **New exported getters**: `MinValue() (int, bool)`, `MaxValue() (int, bool)`, `PatternValue() string`, `ItemBuilder() *PropertyBuilder`, `ItemRef() (*Metadata, bool)`, `MetadataRef() (*Metadata, bool)`, `IsAdditionalProperties() bool`, `KindValue() string` -- identical shape/nil-handling to each wrapper's own existing getter (e.g. `MinValue`'s `(0, false)` when never set)
- **Every existing branch method gains ONE line** setting `kind` alongside the `format` line it already sets:
  - `String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary` → `p.kind = "string"`
  - `Integer`/`Int32` → `p.kind = "integer"`; `Float`/`Double` → `p.kind = "number"`
  - `Boolean` → `p.kind = "boolean"` (THIS is what fixes the pre-existing collision -- `Boolean` and `String` both set `format = ""`, but now have DIFFERENT `kind`)
  - `DateTime`/`Date` → `p.kind = "string"` (OpenAPI 3.1 puts `date-time`/`date` under `type: string`, matching `format`)
  - `Array` → `p.kind = "array"`; `Object` → `p.kind = "object"`
- **Dependencies**: none new
- **Reuses**: nothing removed, purely additive except for the wrapper-side field deletions described below

### `StringMetadata`/`NumericMetadata`/`ArrayMetadata`/`ObjectMetadata` (existing, P0 minimal edit)

- **Purpose**: DELETE each wrapper's own now-duplicate field declarations. Every method body stays textually IDENTICAL (`s.min = &n`, `am.itemRef = ref`, `om.additionalProperties = true`, etc.) -- Go's promoted-field access through the embedded `*PropertyBuilder` resolves these automatically once the wrapper's own conflicting field is gone, since `StringMetadata`/etc and `PropertyBuilder` are all in the SAME package (unexported promoted field access works across an embedding within one package).
- **Location**: `internal/metadata/string.go`, `numeric.go`, `array.go`, `object.go` (existing, each loses a handful of field-declaration lines only)
- **Also for `ArrayMetadata`**: `PropertyBuilder.Array()` (in `metadata.go`) changes from `return &ArrayMetadata{PropertyBuilder: p, item: &PropertyBuilder{}}` to first doing `p.item = &PropertyBuilder{}` on `p` itself, THEN `return &ArrayMetadata{PropertyBuilder: p}` -- the synthetic item builder now lives on `p` (reachable later via `PropertyBuilder.ItemBuilder()`), not on the ephemeral `ArrayMetadata` wrapper
- **Verification**: every existing test in `string_test.go`/`numeric_test.go`/`array_test.go`/`object_test.go` (and `gonest_test.go`'s root-alias reproductions) MUST pass unchanged -- they only exercise each wrapper's own public methods, never inspect field storage location directly

### `internal/metadata/registry.go` (NEW file, same package -- P1)

- **Purpose**: process-wide lookup from a Go struct type to its registered `*Metadata`, so `MustJsonBody[T]` can find metadata without an explicit argument (INSIGHT.md's call shape has none).
- **Interfaces**:
  - `func Register(t reflect.Type, m *Metadata)` -- panics if `t` already registered (`"gonest: NewMetadata already registered for type %s"`, mirrors `Property`'s own double-registration panic message style)
  - `func Lookup(t reflect.Type) (*Metadata, bool)`
  - Both guarded by a `sync.RWMutex` (process-wide state, potentially touched from concurrent `init()` functions across packages during startup -- same caution class as L-006's global-state lesson, though this registry is INTENTIONALLY long-lived for the process, unlike `internal/resolve`'s per-bootstrap `pendingEdges` which needed a `Reset()`)
- **Called from**: `metadata.New(structType, baseAddr)` itself (the function `NewMetadata[T]`'s root wrapper already calls) -- registration becomes automatic and impossible to forget, not something the root wrapper has to remember to call separately

### `execution.Responder` / `execution.Context` (existing, extended -- P2)

- **Purpose**: expose the raw request body, currently inaccessible from any `Context` method.
- **Location**: `internal/execution/context.go` (existing, extended)
- **Interfaces**: `Responder` gains `Body() []byte`; `Context` gains `func (ctx *Context) Body() []byte { return ctx.res.Body() }` (same one-line delegation pattern every other `Context` method already uses)

### `internal/adapter/fiber` (existing, extended -- P2)

- **Purpose**: real implementation of the new `Body()` method.
- **Interfaces**: `func (r *fiberResponder) Body() []byte { return r.c.Body() }` -- see Tech Decisions for why no defensive copy is needed here (unlike L-009's `GetParam` fix)

### `internal/validate` (NEW package -- P3-P5)

- **Purpose**: the real implementation behind `gonest.MustJsonBody[T]` -- reads `*execution.Context`, `internal/metadata`, panics `*exception.BadRequestException` on failure. New package (not folded into `internal/metadata`, which stays introspection-only per its own package doc, and not into `internal/route`, which is param-coercion-specific) -- mirrors how `internal/route` itself is a thin cross-cutting layer over `internal/execution` + `internal/pipe`.
- **Location**: `internal/validate/validate.go` (new package)
- **Interfaces**:
  - `func MustJsonBody[T any](ctx *execution.Context) T`
  - Unexported recursive core: `validateStruct(presence map[string]any, m *metadata.Metadata, pathPrefix string) []violation`, `validateValue(raw any, p *metadata.PropertyBuilder, path string) []violation`, `validateArray(raw any, p *metadata.PropertyBuilder, path string) []violation`, `validateObject(raw any, p *metadata.PropertyBuilder, path string) []violation`, `validatePrimitive(raw any, p *metadata.PropertyBuilder, path string) []violation`
  - `type violation struct { Field, Message string }` -- exported as `Violation` if `BadRequestException`'s `details any` should carry a typed slice, or kept as `[]map[string]string`-equivalent; see Tech Decisions for the exact shape chosen
- **Dependencies**: `internal/metadata`, `internal/execution`, `internal/exception`, `encoding/json`, `reflect`
- **Reuses**: `Metadata.OwnProperties()`, every `PropertyBuilder` getter (old and P0-new), `exception.NewBadRequestException`

---

## Data Models

```go
// internal/metadata/metadata.go, PropertyBuilder EXTENDED:
type PropertyBuilder struct {
    field       reflect.StructField
    required    bool
    nullable    bool
    description string
    examples    []any
    format      string

    // P0 additions -- permanent home for every branch's own constraints,
    // reused across families since only ONE family is ever active per
    // PropertyBuilder instance (format/kind determine which):
    kind                 string        // OpenAPI "type": string/integer/
                                        // number/boolean/array/object
    min, max             *int          // String length / Numeric value /
                                        // Array quantity (mutually exclusive
                                        // per-instance)
    pattern              string        // String only
    item                 *PropertyBuilder // Array's synthetic item builder
    itemRef              *Metadata     // Array's Object(ref)-as-item
    ref                  *Metadata     // Object's own Metadata(ref)
    additionalProperties bool          // Object's own AdditionalProperties()
}
```

```go
// internal/validate/validate.go, NEW:
type violation struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

**Relationships**: `PropertyBuilder.item` is itself a full `*PropertyBuilder` (same type, synthetic instance, zero `reflect.StructField`) -- self-referential by type, not a special case, matches how `ArrayMetadata` already worked before P0, just relocated. `internal/validate` never touches `StringMetadata`/`NumericMetadata`/`ArrayMetadata`/`ObjectMetadata` at all -- every constraint it needs is reachable through `*PropertyBuilder`'s own (P0-extended) surface alone, which is the entire point of P0.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| Body is not valid JSON at all (either pass) | Panic `*BadRequestException` immediately with a single violation (`Field: ""`, parse error message) -- can't collect per-field violations if the JSON itself doesn't parse | Spec.md's P4.1 |
| Body IS valid JSON but not an object at the top level (e.g. a bare `42` or `[1,2,3]`) | `presence.(map[string]any)` type-assertion fails, `presenceMap` is `nil` -- every `Required` field then reports "required" (absent from a nil map), non-required fields are silently skipped (nothing to validate) | Degrades gracefully to "every required field missing," a reasonable and honest description of "you posted the wrong shape entirely" |
| `T` never registered via `NewMetadata[T]` | Panic immediately with "no metadata registered for type X" -- BEFORE touching the body at all | Spec.md's Edge Cases -- never a `nil`-pointer crash |
| A field's JSON value has the WRONG kind entirely (e.g. `true` for a `kind=="string"` field) | Recorded as a violation (`"expected string, got boolean"`-shaped message), does NOT attempt further format-specific checks (`Min`/`Max`/`Pattern`) on a value of the wrong Go-level shape | Avoids nonsensical follow-on errors (e.g. calling `len()` semantics on a bool) |
| `NewMetadata[T]` called twice for the same `T` | Panic (P0/P1 registry) | Spec.md's P1, AC2 |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| P0's field relocation reuses ONE `min`/`max`/`pattern`/etc field set on `PropertyBuilder`, not per-branch-family duplicates (`stringMin`/`numMin`/`arrMin`) | Single shared field set | Only ONE branch family is ever active on a given `PropertyBuilder` INSTANCE at a time (its `format`/`kind` determines which) -- `Tags`'s own field-level `PropertyBuilder` (format `"array"`) and `Tags`'s item `PropertyBuilder` (format `"string"`, a SEPARATE instance, `p.item`) never collide even though both may have non-nil `min`/`max`, because they are different objects. Reusing field names costs nothing and avoids 3x the field count for no behavioral gain. |
| Add a NEW `kind` field, don't just infer type-family from `format` | Explicit `kind` string, set by every branch method | `format == ""` is ALREADY ambiguous between `String()` and `Boolean()` (both set it to empty) -- discovered while designing this feature's validator, which genuinely needs to know "is this JSON value supposed to be a string or a boolean" to validate correctly. This is a real, pre-existing gap in Milestones 4-5's design (OpenAPI's own spec treats `type` and `format` as separate dimensions; this codebase only ever tracked `format`). Fixing it requires touching every branch method with one extra line -- mechanical, low-risk, but necessary; deferring it would make `MustJsonBody` unable to correctly validate `UserEntity`'s own `IsActive bool` field, one of INSIGHT.md's own examples. |
| `internal/adapter/fiber`'s `Body()` does NOT defensively copy the byte slice (unlike L-009's `GetParam` fix, which DOES clone) | Return `c.Body()` unchanged | L-009's bug was about a value RETAINED past the request (stored in a struct field, read later). `MustJsonBody` calls `json.Unmarshal` on the body SYNCHRONOUSLY, within the same request/handler execution `Body()` is called in -- `encoding/json` copies string/byte data into the destination values during decode, it does not retain the input slice itself. As long as no future caller stores the raw `[]byte` from `Context.Body()` past the synchronous validation call, there is no reuse-corruption risk; this is documented explicitly on `Body()`'s own doc comment as a constraint on FUTURE callers, not fixed defensively here (matches this codebase's "trust the caller, document the constraint" stance elsewhere, e.g. `Min > Max` not being validated) |
| Two full `json.Unmarshal` passes (once into `any`/`map[string]any`, once into `T`) rather than one pass with manual field-by-field decoding | Two passes | Context.md's Decision 1 already settled this at the requirements level -- this is just the concrete mechanism. `encoding/json`'s own partial-decode-on-type-mismatch behavior (Go's std lib continues decoding other fields after hitting one `UnmarshalTypeError`, but only surfaces the LAST error, not a full per-field list) is unsuitable for "collect every violation" (context.md's Decision 2) -- decoding into `any` first gives genuinely generic access to every JSON value's own natural Go type (string/float64/bool/nil/map[string]any/[]any) for validation, independent of `T`'s own Go field types, which is what makes wrong-kind detection (e.g. JSON string posted for a `kind=="boolean"` field) possible in the first place. The second pass (into `T`) only ever runs AFTER validation already passed, so its own error path is a fallback, not the primary correctness mechanism. |
| `MustJsonBody[T]`'s registry lookup key is the DEREFERENCED struct type (`reflect.TypeOf(zero T).Elem()`, since `T` itself is a pointer, e.g. `*UserProperties`) | `.Elem()` on the pointer type | Matches `NewMetadata[T]`'s OWN registration key (`T` there is the bare struct, e.g. `UserProperties`, per `gonest.go`'s existing `NewMetadata[T any](fn func(t *T, m *Metadata)) *Metadata` signature) -- `MustJsonBody[*UserProperties]` and `NewMetadata[UserProperties]` must resolve to the SAME registry key for the lookup to ever succeed, and INSIGHT.md's own call shapes (`NewMetadata[UserEntity]` vs `MustJsonBody[*UserProperties]`) confirm this pointer-vs-value asymmetry is intentional, not a bug to paper over |
| Registry panics on duplicate `NewMetadata[T]` for the same `T`, rather than last-wins | Panic | Matches `Property`'s own existing double-registration panic precedent (same file, same package) -- a type should have exactly one canonical metadata declaration; silently allowing a second to overwrite the first would let a typo'd duplicate call silently corrupt validation behavior instead of failing loudly at startup |

---

## Open Questions pra Tasks

- None left unresolved -- context.md settled the 3 user-facing ambiguities, and this design session settled the 2 additional implementation-blocking gaps discovered along the way (wrapper storage relocation, `kind` field). Task breakdown should isolate P0 (storage relocation, HIGH regression risk against 4 already-shipped features) as its own task with its own dedicated evaluator pass BEFORE any of P1-P5 build on top of it, per this codebase's own AD-001 workflow.
