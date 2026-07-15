# Schema Generation from Metadata Design

**Spec**: `.specs/features/schema-generation/spec.md`
**Context**: `.specs/features/schema-generation/context.md`

## Architecture Overview

```
internal/app/app.go (existing, extended -- P0)
        │
        + func (a *App) Root() *module.Module -- exposes the SAME root
              *module.Module already stored internally (unexported `root`
              field, set during bootstrap) -- pure accessor, zero new state

internal/metadata/metadata.go (existing, extended -- P1)
        │
        + Metadata gains `title string` field (whole-TYPE level, sibling
              of the existing `description` field, NOT a PropertyBuilder
              field) + Title(s)/TitleText() setter/getter pair

internal/controller/controller.go (existing, extended -- P1)
        │
        + Controller gains `tags []string`, `bearerAuth bool` fields +
              Tags(...string)/OwnTags() and BearerAuth()/HasBearerAuth()

internal/route/route.go (existing, extended -- P1, NEW dependency on
        internal/metadata -- confirmed safe, metadata never imports route)
        │
        + Route gains: summary, description, operationId string; tags
              []string (nil = "inherit controller's", set = override);
              bearerAuthSet, bearerAuthValue bool (two bools -- distinguishes
              "never called" from "called, value false" the same way every
              AD-012 getter already does with *int/pointer-based "unset"
              sentinels, except bool needs an explicit "was it set" flag
              since false has no natural nil); requestBody *metadata.Metadata;
              responses map[int][]*metadata.Metadata (int -> status, slice
              len 0 or 1 -- 0 means "documented, no body", matches
              Response(status) with zero variadic args); pathParams,
              queryParams *metadata.Metadata; excluded, deprecated bool
        + One setter + one getter per field, mechanical, same
              setter-returns-self / getter-plain convention as everywhere
              else in this codebase

internal/openapi (existing package from "OpenAPI Document Builder" -- P2,
        BIG extension)
        │
        ├── OpenAPI (existing) gains unexported `paths
        │      map[string]map[string]any` (outer key: full path string,
        │      inner key: lowercase HTTP method) and `schemas
        │      map[string]any` (components.schemas, keyed by schema name)
        │      -- both populated ONLY by Generate, never by hand-written
        │      builder calls (Title/Contact/etc stay exactly as they are)
        │
        ├── func Generate(doc *OpenAPI, root *module.Module) --
        │      NEW entry point, walks root + root.ImportedModules()
        │      (recursively, cycle-safe via a visited-module set --
        │      Modules CAN import each other in diamond shapes) + every
        │      Module's OwnControllers() + every Controller's OwnRoutes(),
        │      building doc.paths/doc.schemas as it goes
        │
        ├── func (doc *OpenAPI) Document() map[string]any --
        │      NEW, assembles the FULL OpenAPI 3.1 JSON-ready structure
        │      (openapi/info/paths/components/security from every field
        │      doc already holds, old AND new) -- THIS is what a future
        │      "Swagger UI Setup" feature will `json.Marshal`
        │
        ├── walkController/walkRoute (unexported) -- build one path item
        │      map[string]any per non-excluded route: method/summary/
        │      description/operationId/tags/deprecated/security (from
        │      resolved bearer-auth, controller-inherited-or-route-
        │      overridden)/parameters (from PathParams+QueryParams)/
        │      requestBody/responses
        │
        └── schemaFor(p *metadata.PropertyBuilder, schemas map[string]any,
               visiting map[*metadata.Metadata]bool) map[string]any --
               THE recursive core, one *PropertyBuilder -> one OpenAPI
               Schema Object (as map[string]any), dispatches on
               p.KindValue() exactly like internal/validate's own
               validateValue does (same source of truth, different
               destination -- schema instead of violation)
```

This feature has 4 layers, roughly independent except the last: (P0) `App.Root()`, a one-method accessor; (P1) new declarative surface on `Metadata`/`Controller`/`Route` (mechanical, additive, zero behavior beyond storing values); (P2) the actual walker + recursive schema builder, the only genuinely complex part. P0/P1 have essentially zero design risk (pure additive getters/setters, same convention as every prior feature this session). P2 is where the real work is.

---

## Components

### `App.Root()` (P0)

- **Location**: `internal/app/app.go`
- **Interface**: `func (a *App) Root() *module.Module { return a.root }`
- **Rationale**: `a.root` already exists (set during bootstrap, confirmed by prior investigation) -- this is the ENTIRE change, a one-line getter.

### `Metadata.Title`/`TitleText` (P1)

- **Location**: `internal/metadata/metadata.go`
- **Interface**: `func (m *Metadata) Title(s string) *Metadata` / `func (m *Metadata) TitleText() string` -- same shape as the existing `Description`/`DescriptionText` pair on `Metadata` (NOT `PropertyBuilder` -- this is whole-type, like `Description`, not per-field)

### `Controller.Tags`/`BearerAuth` (P1)

- **Location**: `internal/controller/controller.go`
- **Interface**: `func (c *Controller) Tags(tags ...string) *Controller` / `func (c *Controller) OwnTags() []string` (defensive copy, same pattern as `OwnRoutes`/`OwnMiddleware`); `func (c *Controller) BearerAuth() *Controller` / `func (c *Controller) HasBearerAuth() bool`

### `Route`'s documentation builder methods (P1)

- **Location**: `internal/route/route.go`
- **Interfaces** (all mechanical setter-returns-`*Route` + plain getter, matching `HttpCode`/`Code`'s existing shape):
  - `Summary(s string) *Route` / `SummaryText() string`
  - `Description(s string) *Route` / `DescriptionText() string`
  - `OperationId(s string) *Route` / `OperationIdText() string`
  - `Tags(tags ...string) *Route` / `OwnTags() ([]string, bool)` -- bool distinguishes "never called" (inherit controller's) from "called with the SAME tags a controller already has" (still an explicit override, just happens to match)
  - `BearerAuth() *Route` / `HasBearerAuth() (bool, bool)` -- first bool is the value, second is "was it ever called" (same never-called-vs-explicit distinction as `Tags`)
  - `RequestBody(m *metadata.Metadata) *Route` / `RequestBodyMetadata() (*metadata.Metadata, bool)`
  - `Response(status int, m ...*metadata.Metadata) *Route` / `Responses() map[int]*metadata.Metadata` (internally stores `map[int]*metadata.Metadata`, nil value = documented-no-body -- design.md's earlier sketch said `map[int][]*metadata.Metadata` but a plain `map[int]*metadata.Metadata` with a nil-valued entry for "no body" is simpler and equally expressive, since spec.md's AC3 only requires distinguishing "no body" from "has body," not multiple bodies per status)
  - `PathParams(m *metadata.Metadata) *Route` / `PathParamsMetadata() (*metadata.Metadata, bool)`
  - `QueryParams(m *metadata.Metadata) *Route` / `QueryParamsMetadata() (*metadata.Metadata, bool)`
  - `ExcludeFromDocs() *Route` / `IsExcludedFromDocs() bool`
  - `Deprecated() *Route` / `IsDeprecated() bool`

### `internal/openapi.Generate` + the recursive schema core (P2)

- **Location**: `internal/openapi/generate.go` (new file, same package as `OpenAPI`)
- **Interfaces**:
  - `func Generate(doc *OpenAPI, root *module.Module)`
  - unexported `walkModule(m *module.Module, visitedModules map[*module.Module]bool, doc *OpenAPI)`, `walkController(c *controller.Controller, doc *OpenAPI)`, `walkRoute(prefix string, c *controller.Controller, r *route.Route, doc *OpenAPI)`
  - unexported `schemaFor(p *metadata.PropertyBuilder, doc *OpenAPI, visiting map[*metadata.Metadata]bool) map[string]any` -- the recursive core
  - unexported `registerSchema(m *metadata.Metadata, doc *OpenAPI, visiting map[*metadata.Metadata]bool) string` -- ensures `m` has EXACTLY ONE entry in `doc.schemas` (dedup via a NEW `doc.schemaNames map[*metadata.Metadata]string` cache, pointer-keyed), returns the schema's name (for building a `$ref` string) -- called by `schemaFor` whenever it hits `ItemRef()`/`MetadataRef()`, and by `walkRoute` for `RequestBody`/`Response`/`PathParams`/`QueryParams`
  - `func (doc *OpenAPI) Document() map[string]any`

---

## Data Models

```go
// internal/openapi/openapi.go, OpenAPI EXTENDED:
type OpenAPI struct {
    // ...existing fields (specVersion, title, description, version,
    // contactName/Url/Email, licenseName/Url, bearerAuth) unchanged
    paths       map[string]map[string]any // "path" -> "method" -> path item
    schemas     map[string]any            // "SchemaName" -> Schema Object
    schemaNames map[*metadata.Metadata]string // dedup cache, pointer-keyed
}
```

**Schema Object shape** (produced by `schemaFor`, OpenAPI 3.1 / JSON Schema 2020-12 vocabulary):

```go
// String-family example:
map[string]any{
    "type":        "string",           // or []string{"string","null"} if Nullable
    "format":      "email",            // omitted if FormatValue() == ""
    "minLength":   1,                  // omitted if MinValue() not set
    "maxLength":   50,
    "pattern":     `^\d{5}-?\d{3}$`,   // omitted if PatternValue() == ""
    "description": "...",              // omitted if DescriptionText() == ""
    "examples":    []any{"a","b"},     // omitted if ExamplesList() empty
}

// Array example:
map[string]any{
    "type":     "array",
    "items":    map[string]any{ /* schemaFor(p.ItemBuilder(), ...) OR $ref */ },
    "minItems": 1, // omitted if not set
    "maxItems": 10,
}

// Object-with-ref example:
map[string]any{ "$ref": "#/components/schemas/AddressEntity" }

// Object-with-AdditionalProperties example:
map[string]any{ "type": "object", "additionalProperties": true }
```

**Relationships**: `doc.schemaNames` is the SOLE dedup mechanism (P2's spec.md AC3) -- `registerSchema` checks it FIRST before ever walking a `*Metadata`'s own properties, so a diamond-shaped reference graph (two different routes both referencing the same `addressMetadata`) never double-walks or double-emits.

---

## Error Handling Strategy

| Scenario                                                                                     | Treatment                                                                                                                                         | Impact                                                                                                                                                                            |
| -------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Custom(fn)`-only field (no `KindValue()`-derivable type)                                    | `schemaFor` emits ONLY `description`/`examples` (if set) -- no `type`/`format`/validators -- documented limitation, not an error                  | spec.md's Edge Cases                                                                                                                                                              |
| Two different `*Metadata` values whose Go type shares the same name AND neither sets `Title` | Both get registered under the literal SAME key in `doc.schemas` (last-registered wins the map slot) -- NOT defended against                       | spec.md's Edge Cases, "trust the caller" stance                                                                                                                                   |
| Circular module imports (`A` imports `B`, `B` imports `A`)                                   | `walkModule`'s `visitedModules` set prevents infinite recursion -- each `*Module` walked AT MOST once regardless of how many times it's reachable | Not explicitly asked for by any spec.md story, but a correctness necessity given `ImportedModules()` existing without any acyclic guarantee documented elsewhere in this codebase |

---

## Tech Decisions (only non-obvious ones)

| Decision                                                                                                                     | Choice                                              | Rationale                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| ---------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Schema Objects represented as `map[string]any`, not a typed Go struct hierarchy mirroring the full OpenAPI 3.1 spec          | `map[string]any` throughout                         | A fully-typed OpenAPI struct library (Info/PathItem/Operation/Parameter/RequestBody/Response/Schema, each with dozens of optional fields) is a MUCH larger surface than anything this feature's spec.md actually asks for -- `map[string]any` is directly `json.Marshal`-able (future "Swagger UI Setup" feature's only real requirement), self-documenting via its own key strings, and avoids inventing struct fields for OpenAPI spec areas (`servers`, `callbacks`, etc) explicitly marked Out of Scope. If a future need for stricter typing appears, it can be layered on top without touching this feature's own logic. |
| `doc.schemaNames map[*metadata.Metadata]string` is a NEW field on `OpenAPI`, not a field added to `Metadata` itself          | Dedup cache lives on the DOCUMENT, not the Metadata | A single `*Metadata` could, in principle, be walked by TWO DIFFERENT `Generate` calls against two different `*OpenAPI`s (e.g. two separate API versions documented from overlapping metadata) -- caching the assigned schema NAME on `Metadata` itself would leak state across unrelated document generations. Keeping the cache document-scoped avoids that entirely, at the cost of one extra map per `Generate` call (negligible).                                                                                                                                                                                          |
| `Route.Responses()` returns `map[int]*metadata.Metadata` (nil value = no-body), not `map[int][]*metadata.Metadata`           | Single value per status, nil-able                   | spec.md's AC3 only requires distinguishing "no body" from "has body" per status, never asks for MULTIPLE alternative body schemas per status (that would be OpenAPI's `oneOf`/content-negotiation territory, well beyond anything requested) -- simpler shape, easier to walk, no behavior lost against the actual requirement                                                                                                                                                                                                                                                                                                 |
| `Tags`/`BearerAuth` override resolution happens INSIDE `walkRoute` (P2), not stored pre-resolved on `Route` itself during P1 | Resolution deferred to generation time              | `Route` doesn't know about its owning `Controller` at declaration time (no back-reference) -- `Controller.OwnRoutes()` is how the relationship is discovered, only during the P2 walk, which is naturally the first point BOTH the controller's and the route's own state are simultaneously in scope to resolve "did the route override, or should it inherit"                                                                                                                                                                                                                                                                |

---

## Open Questions pra Tasks

- None left unresolved -- context.md's 4 user-confirmed decisions plus this design's own resolution of the deferred override-resolution question and the dedup-cache placement question cover everything spec.md's stories need. Task breakdown should keep P0/P1 (mechanical, low-risk, many small additive methods across 4 packages) SEPARATE from P2 (the actual walker/schema-builder, the only place with real logic to get wrong) so P2 gets its own focused evaluator pass without P0/P1's sheer method-count diluting review attention.
