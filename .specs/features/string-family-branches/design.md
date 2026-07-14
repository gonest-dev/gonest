# String-family Branches Design

**Spec**: `.specs/features/string-family-branches/spec.md`

## Architecture Overview

```
internal/metadata (existing package from "Metadata Registration Core",
extended -- NOT a new package, string-family builders need access to
PropertyBuilder's own unexported fields, same "intimately related types
share a package" precedent as internal/exception's builtin.go)
        │
        ├── PropertyBuilder (existing, EXTENDED)
        │      + format string field (NEW -- persists which branch was
        │        chosen, stored on the SHARED builder, not the disposable
        │        branch wrapper -- see Tech Decisions for why this matters)
        │      + FormatValue() string (NEW getter)
        │      + 10 NEW branch methods: String()/Email()/Uuid()/Uri()/
        │        Hostname()/Ipv4()/Ipv6()/Password()/Byte()/Binary(),
        │        each sets p.format then returns &StringMetadata{p}
        │
        └── StringMetadata (NEW, internal/metadata/string.go)
               embeds *PropertyBuilder (pointer -- same shared object,
               not a copy)
               Min(int)/Max(int)/Pattern(string) -- string-specific
               Required()/Nullable()/Description()/Examples() -- REDECLARED
               with *StringMetadata return type (delegates to the embedded
               PropertyBuilder's own method, then returns self)
```

The critical design insight (see Tech Decisions): the "which branch was chosen" fact (`format`) is stored on `PropertyBuilder` itself -- the object ALREADY sitting in `Metadata.properties[offset]` since `Property(&t.X)` ran -- not on the disposable `*StringMetadata` wrapper each branch call constructs fresh. This is what makes it safe for a dev to call `.String()`, get back a `*StringMetadata`, and have that fact persist even though nothing about `Metadata`'s own map changes -- the wrapper is just a typed VIEW onto the same underlying object.

---

## Components

### `PropertyBuilder` (existing, extended)

- **Purpose**: gains the ability to record which type+format branch a dev selected, and the 10 branch-selection methods themselves.
- **Location**: `internal/metadata/metadata.go` (existing file from "Metadata Registration Core", extended -- NOT a new file, since `format` is a new field on the EXISTING struct)
- **Interfaces** (additions only):
  - New unexported field: `format string`
  - `func (p *PropertyBuilder) FormatValue() string` -- getter, `""` if no branch was ever selected
  - `func (p *PropertyBuilder) String() *StringMetadata` -- sets `p.format = ""` (bare string, no format), returns `&StringMetadata{PropertyBuilder: p}`
  - `func (p *PropertyBuilder) Email() *StringMetadata` -- `p.format = "email"`
  - `func (p *PropertyBuilder) Uuid() *StringMetadata` -- `p.format = "uuid"`
  - `func (p *PropertyBuilder) Uri() *StringMetadata` -- `p.format = "uri"`
  - `func (p *PropertyBuilder) Hostname() *StringMetadata` -- `p.format = "hostname"`
  - `func (p *PropertyBuilder) Ipv4() *StringMetadata` -- `p.format = "ipv4"`
  - `func (p *PropertyBuilder) Ipv6() *StringMetadata` -- `p.format = "ipv6"`
  - `func (p *PropertyBuilder) Password() *StringMetadata` -- `p.format = "password"`
  - `func (p *PropertyBuilder) Byte() *StringMetadata` -- `p.format = "byte"`
  - `func (p *PropertyBuilder) Binary() *StringMetadata` -- `p.format = "binary"`
  - Format string values match OpenAPI 3.1's own format vocabulary exactly (lowercase, matching the spec's own `format: email`/`format: uuid`/etc conventions)
- **Dependencies**: none new
- **Reuses**: the existing `PropertyBuilder` struct from "Metadata Registration Core" -- this feature is purely additive to it

### `StringMetadata` (new type, same package)

- **Purpose**: the branch-specific builder for all 10 string-family formats -- ONE type serves all 10, since they share identical extra validators (`Min`/`Max`/`Pattern`), differing only in the `format` string already persisted on the embedded `PropertyBuilder`.
- **Location**: `internal/metadata/string.go` (new file, SAME package as `Metadata`/`PropertyBuilder` -- needs to embed `*PropertyBuilder` and call its own methods, no need for cross-package accessors)
- **Interfaces**:
  - `type StringMetadata struct { *PropertyBuilder; min, max *int; pattern string }` (embedded field unexported name is `PropertyBuilder`, `min`/`max`/`pattern` unexported)
  - `func (s *StringMetadata) Min(n int) *StringMetadata`
  - `func (s *StringMetadata) Max(n int) *StringMetadata`
  - `func (s *StringMetadata) Pattern(p string) *StringMetadata`
  - `func (s *StringMetadata) MinValue() (int, bool)` / `MaxValue() (int, bool)` -- getters, `bool` reports whether it was ever set (distinguishes "never called" from "called with 0")
  - `func (s *StringMetadata) PatternValue() string`
  - `func (s *StringMetadata) Required() *StringMetadata` -- delegates to `s.PropertyBuilder.Required()` (the embedded pointer's own method, mutating the SHARED object), then `return s`
  - `func (s *StringMetadata) Nullable() *StringMetadata` -- same pattern
  - `func (s *StringMetadata) Description(d string) *StringMetadata` -- same pattern
  - `func (s *StringMetadata) Examples(examples ...any) *StringMetadata` -- same pattern
- **Dependencies**: none new
- **Reuses**: `PropertyBuilder`'s own `Required`/`Nullable`/`Description`/`Examples` methods (called internally, not re-implemented)

---

## Data Models

```go
// internal/metadata/metadata.go, PropertyBuilder EXTENDED with:
type PropertyBuilder struct {
    field       reflect.StructField
    required    bool
    nullable    bool
    description string
    examples    []any
    format      string // NEW this feature
}

// internal/metadata/string.go, NEW:
type StringMetadata struct {
    *PropertyBuilder
    min, max *int
    pattern  string
}
```

**Method-delegation pattern** (the mechanical shape "Numeric & Boolean Branches" will repeat):

```go
func (p *PropertyBuilder) Email() *StringMetadata {
    p.format = "email"
    return &StringMetadata{PropertyBuilder: p}
}

func (s *StringMetadata) Min(n int) *StringMetadata {
    s.min = &n
    return s
}

func (s *StringMetadata) Required() *StringMetadata {
    s.PropertyBuilder.Required() // mutates the SHARED object
    return s                      // returns self, not the embedded PropertyBuilder,
                                    // so .Min()/.Max()/.Pattern() stay chainable after
}
```

**Relationships**: `StringMetadata.PropertyBuilder` is a POINTER to the exact same object `Metadata.properties[offset]` already holds (set during `Property(&t.X)`, before any branch method ran). Calling `.String()`/`.Email()`/etc TWICE on the same `*PropertyBuilder` (spec.md's Edge Cases) simply overwrites `p.format` each time -- last call wins, no panic, deterministic. Each call also constructs a FRESH `*StringMetadata` wrapper, but since both wrappers point at the SAME underlying `PropertyBuilder`, any `Required()`/etc calls made through EITHER wrapper mutate the one shared object -- there's no risk of "the wrong wrapper's data getting lost" the way there would be if `StringMetadata` held its OWN copy of `required`/`nullable`/etc.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| A branch method (`.String()`/`.Email()`/etc.) called twice on the same field | `p.format` simply gets overwritten by the second call -- no panic, no error, deterministic last-write-wins | Matches spec.md's Edge Cases -- branch selection isn't field registration (unlike `Property` itself, which DOES panic on double-registration), no reason to be as strict here since nothing is lost or corrupted, just superseded |
| `Min`/`Max` called with `min > max` | Not validated at registration time | Matches spec.md's Edge Cases -- same "trust the caller" stance as the rest of this metadata system |
| `Pattern` called with syntactically invalid regex | Not validated at registration time (stored as a plain string) | Matches spec.md's Edge Cases -- regex compilation/validation is a future consumer's concern (Milestone 6, runtime validation), not this registration-only feature's |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `format` is stored on the SHARED `PropertyBuilder` (persisted in `Metadata.properties[offset]`), not on the disposable `StringMetadata` wrapper each branch call constructs | `p.format = "email"` runs INSIDE the branch method, mutating the object already in the map, BEFORE the wrapper is even constructed and returned | If `format` lived only on `StringMetadata`, a discarded wrapper (e.g. a dev calling `.String()` without chaining anything further, by mistake, then calling `.Email()` on the SAME `PropertyBuilder` reference and using THAT chain) would have no way to communicate which branch "won" back to the object a future consumer (Milestone 7's OpenAPI generator) actually has access to via `Metadata.OwnProperties()` (which returns `[]*PropertyBuilder`, not `[]*StringMetadata`) -- storing `format` on the shared object is the ONLY way the choice survives past the branch call itself |
| ONE `StringMetadata` type serves all 10 branches (not 10 separate types) | The 10 branches genuinely share identical extra validators (`Min`/`Max`/`Pattern`) -- INSIGHT.md's own comment block lists `Min/Max(len), Pattern` once for the whole string family, not per-branch | No reason to duplicate `Min`/`Max`/`Pattern` 10 times when the underlying storage/validators are identical -- only the `format` VALUE differs, and that's already handled by the branch METHOD setting `p.format` before construction, not by the TYPE itself needing to vary |
| `StringMetadata` lives in `internal/metadata` (same package as `PropertyBuilder`), not a new `internal/metadata/string` sub-package | Same package, new file | `StringMetadata` embeds `*PropertyBuilder` and needs to call ITS methods (`Required()`/`Nullable()`/etc, already exported so this isn't strictly required for access, but the branch-construction methods like `String()`/`Email()` need to set the UNEXPORTED `p.format` field directly) -- keeping this in the same package avoids needing to export `format`/add an unnecessary setter just to cross a package boundary that buys nothing here (unlike Provider/Module/Controller, which are genuinely independent DI-graph concepts warranting AD-004's package-per-type isolation, `PropertyBuilder`/`StringMetadata` are two intimately coupled views of the SAME underlying field registration) |
| The 4 common methods (`Required`/`Nullable`/`Description`/`Examples`) are MECHANICALLY RE-DECLARED on `StringMetadata` (each one-liner delegating to the embedded `PropertyBuilder`'s own method, then returning `s`), not inherited automatically | Deliberate duplication, not automatic | Go's embedding WOULD promote `PropertyBuilder`'s methods onto `StringMetadata` automatically for FREE if left alone -- but a promoted `Required()` would return `*PropertyBuilder` (the embedded type's own return type), breaking the chain (`.Required().Min(5)` would fail to compile, since `*PropertyBuilder` has no `Min` method). Re-declaring each method with `*StringMetadata` as ITS OWN return type is the ONLY way to keep chaining fluent across the base-vs-branch-specific boundary in Go -- this is the exact "mechanical duplication, but simple and magic-free" tradeoff flagged as the likely approach in "Metadata Registration Core"'s own design.md, now confirmed as the concrete implementation. "Numeric & Boolean Branches" (next feature) will repeat this exact 4-method redeclaration pattern for its own branch type(s). |

---

## Open Questions pra Tasks

- None -- every genuine design question (where `format` lives, one type vs ten, package placement, the redeclaration pattern) is resolved above with clear rationale. Implementation-detail choices (exact panic/error message wording, if any is needed) follow this codebase's existing "gonest: <clear description>" convention, no separate decision round needed.
