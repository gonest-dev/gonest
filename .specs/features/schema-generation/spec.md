# Schema Generation from Metadata Specification

## Problem Statement

Milestones 4-6 built a way to DECLARE (`NewMetadata[T]`) and VALIDATE (`MustJsonBody`/`MustParams`/`MustQuery`) against `Metadata` -- this feature is the THIRD consumer: generate an OpenAPI 3.1 `paths`/`components.schemas` structure from the same declarations, so a route documented via `Route.RequestBody(m)`/`Route.Response(status, m)`/`Route.PathParams(m)`/`Route.QueryParams(m)` produces real API documentation without a second, separate declaration. See `context.md` for the full decision trail (NestJS `@Api*` decorator → gonest builder-method mapping, override semantics, schema naming, undocumented-route behavior) and the `App.Root()` prerequisite discovered while designing this.

## Goals

- [x] `App` gains a way to reach the fully-assembled module tree post-bootstrap (prerequisite -- `Module.OwnControllers()`/`Controller.OwnRoutes()` already retain everything needed, `App` just doesn't expose an entry point today)
- [x] `Metadata.Title(s string)`/`TitleText() string` -- whole-type level (same tier as `Description`), used as the `components.schemas` key + schema `"title"` field, defaulting to the Go type's own name when unset
- [x] `Controller.Tags(...string)`/`Controller.BearerAuth()` -- inherited by every route in the controller unless overridden
- [x] `Route` gains documentation-builder methods: `Summary(s)`, `Description(s)`, `OperationId(s)`, `Tags(...string)` (overrides controller's), `BearerAuth()` (overrides controller's), `RequestBody(m *Metadata)`, `Response(status int, m ...*Metadata)` (variadic -- zero args documents a bodyless status), `PathParams(m *Metadata)`, `QueryParams(m *Metadata)`, `ExcludeFromDocs()`, `Deprecated()`
- [x] A generator (`internal/openapi`) that walks the full module tree (root + `ImportedModules()`, recursively) and, for every NON-excluded route, produces an OpenAPI path item (method, path, summary, description, operationId, tags, parameters from `PathParams`/`QueryParams`, requestBody from `RequestBody`, responses from `Response` calls, deprecated flag, security requirement from bearer auth) plus, for every distinct `*Metadata` referenced anywhere in the walk (`RequestBody`/`Response`/`PathParams`/`QueryParams`, and recursively through `Array`/`Object` nesting), a named entry in `components.schemas`
- [x] Schema generation for a single `*Metadata`/`*PropertyBuilder` covers every branch this codebase has built: String-family (`type: string`, `format`, `minLength`/`maxLength`/`pattern`), Numeric-family (`type: integer`/`number`, `format`, `minimum`/`maximum`), Boolean (`type: boolean`), DateTime/Date (`type: string`, `format: date-time`/`date`), Array (`type: array`, `items` -- either inline from `ItemBuilder()` or `$ref` from `ItemRef()`, `minItems`/`maxItems`), Object (`type: object`, either `$ref` from `MetadataRef()` or `additionalProperties: true` when `IsAdditionalProperties()`), `Required`/`Nullable` (OpenAPI 3.1 `required` array + `type` array including `"null"`), `Description`/`Examples`
- [x] Nested `Array`/`Object` referencing an already-registered `*Metadata` (via `ItemRef()`/`MetadataRef()`) emits `"$ref": "#/components/schemas/<Name>"` instead of inlining, and that referenced schema is walked/emitted exactly ONCE in `components.schemas` regardless of how many places reference it (dedup by `*Metadata` pointer identity)
- [x] `PropertyBuilder.Custom(fn)` fields: since `fn` is an opaque Go closure with no declarative shape, schema generation CANNOT infer a type/format from it -- documented as a known limitation (Out of Scope), field still appears in the schema (name, required/nullable, description) without a `type`/`format`

## Out of Scope

| Feature                                                                                                           | Reason                                                                                                                                 |
| ----------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `SetupSwagger`, actually SERVING the generated document over HTTP, Swagger UI HTML                                | Separate ROADMAP feature ("Swagger UI Setup"), depends on this one existing first                                                      |
| Inferring a schema shape from a `Custom(fn)` field's closure                                                      | Not mechanically possible -- `fn` is opaque Go code, no declarative metadata to read; documented limitation                            |
| Full OpenAPI 3.1 spec coverage (`servers`, multiple security schemes beyond bearer, `callbacks`, `webhooks`, etc) | Nothing in INSIGHT.md or this feature's discussion asks for these; `OpenAPI` (already shipped) only covers what its own spec asked for |
| Validating the GENERATED document against the OpenAPI 3.1 JSON Schema meta-schema                                 | No requirement asks for this; the generator's own correctness is proven by targeted tests, not external spec validation                |

---

## User Stories

### P0: `App` exposes its module tree (prerequisite, blocking) ⭐ MVP

**User Story**: As the framework itself, a schema generator (or anything else that needs the full app graph post-bootstrap) needs a way to reach the root `*Module` -- today `App` doesn't expose it.

**Acceptance Criteria**:

1. WHEN `NewApp`/`MustNewApp` returns THEN the resulting `*App` SHALL expose an accessor reaching the same root `*Module` `internal/app`'s own bootstrap already assembled internally
2. WHEN that accessor is used to walk `Module.ImportedModules()` (existing) + `Module.OwnControllers()` (existing) + `Controller.OwnRoutes()` (existing) THEN system SHALL reach EVERY controller/route registered anywhere in the app, including nested imports

**Independent Test**: build a small module tree (root importing a sub-module with its own controller/route), confirm the new accessor + existing `Own*()`/`ImportedModules()` methods together reach every route, by name/path, with no gaps.

---

### P1: `Metadata.Title` + `Controller`/`Route` documentation-builder methods

**User Story**: As a gonest user, I want to declare `route.Summary(...)`, `route.RequestBody(m)`, `route.Response(status, m)`, etc. (context.md's Decision 1 mapping) alongside a route, and `Metadata.Title(...)` to control its schema's name, matching the NestJS `@Api*` decorator equivalents.

**Acceptance Criteria**:

1. WHEN `Metadata.Title(s)` is called THEN `TitleText()` SHALL return `s`; when never called, `TitleText()` SHALL return `""` (caller falls back to the Go type name -- generator's job, not `Metadata`'s)
2. WHEN `Controller.Tags(...)`/`BearerAuth()` are called THEN every `Route` under that controller SHALL inherit them UNLESS the route itself calls its own `Tags(...)`/`BearerAuth()`, which REPLACES the controller's value entirely for that route (context.md's Decision 1 override semantics)
3. WHEN `Route.Response(status, m ...*Metadata)` is called with ZERO metadata args THEN system SHALL document `status` with no body; with ONE arg, document `status` with that body schema; calling `Response` MULTIPLE times with different `status` values SHALL accumulate multiple documented responses (not overwrite)
4. WHEN `Route.ExcludeFromDocs()` is called THEN this route SHALL be omitted entirely from generation (P4)

**Independent Test**: build a controller with `Tags`/`BearerAuth` set, one route overriding both, one route inheriting both, one route calling `Response` twice for different statuses; assert every getter reports exactly what was declared, with correct override resolution.

---

### P2: Schema generation walks the app, builds `paths` + `components.schemas`

**User Story**: As a gonest user, once I've declared `RequestBody`/`Response`/`PathParams`/`QueryParams`/etc on my routes, I want a generator to walk my whole app and produce the OpenAPI `paths`/`components.schemas` structures automatically, without a second declaration.

**Acceptance Criteria**:

1. WHEN the generator walks the app THEN EVERY registered route (root module + all imported modules, recursively) NOT marked `ExcludeFromDocs()` SHALL appear in the generated `paths`, keyed by its full path (controller's `PathPrefix()` + route's own `Path()`) and HTTP method
2. WHEN a route has NO documentation calls at all (context.md's Decision 4) THEN it SHALL STILL appear in `paths`, using whatever is inferable from `Route`'s own always-present state (method, path, default status via `Code()`)
3. WHEN a `*Metadata` is referenced (directly or via nested `Array`/`Object`) from MULTIPLE places THEN it SHALL appear in `components.schemas` EXACTLY ONCE (dedup by pointer identity), referenced everywhere else via `"$ref"`
4. WHEN a `*PropertyBuilder` is walked for schema generation THEN system SHALL emit the correct OpenAPI shape for EVERY branch family this codebase supports (String/Numeric/Boolean/DateTime-Date/Array/Object), per this spec's Goals list

**Independent Test**: reproduce INSIGHT.md's `UserEntity`/`AddressEntity` shape (nested `Array`/`Object`, at least one `$ref`-reused nested type) fully documented on 2+ routes; assert the generated `components.schemas` has EXACTLY one entry for the shared nested type, and every route's path item has the correct method/parameters/requestBody/responses shape.

---

## Edge Cases

- WHEN a field has `Custom(fn)` set THEN its schema entry SHALL still include name/`required`/`nullable`/`description` but OMIT `type`/`format`-specific validators (Goals' documented limitation)
- WHEN the SAME `*Metadata` is registered with a Go type name that collides with ANOTHER differently-registered `*Metadata`'s default name (e.g. two different packages both defining a type literally named `Address`) AND neither sets `Title` THEN behavior is UNSPECIFIED (not defended against -- same "trust the caller" stance as the rest of this codebase; if this becomes a real problem, `Title` is already the documented escape hatch)
- WHEN `Route.RequestBody`/`Response`/`PathParams`/`QueryParams` is called MORE THAN ONCE for the same slot (e.g. `RequestBody` twice) THEN last-write-wins (except `Response`, which accumulates per DISTINCT status per P1's AC3 -- calling `Response` twice for the SAME status overwrites that one status's entry)

---

## Requirement Traceability

| Requirement ID | Story                                                                                                                                            | Phase | Status |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ------ |
| SG-00          | P0: App exposes module tree, reachable via existing Own*/ImportedModules                                                                         | Done  | Done   |
| SG-01          | P1: Metadata.Title/TitleText                                                                                                                     | Done  | Done   |
| SG-02          | P1: Controller.Tags/BearerAuth inherited, Route-level overrides replace                                                                          | Done  | Done   |
| SG-03          | P1: Route documentation builder methods (Summary/Description/OperationId/RequestBody/Response/PathParams/QueryParams/ExcludeFromDocs/Deprecated) | Done  | Done   |
| SG-04          | P2: generator walks full module tree, produces paths for every non-excluded route                                                                | Done  | Done   |
| SG-05          | P2: components.schemas deduped by Metadata pointer identity, $ref reuse                                                                          | Done  | Done   |
| SG-06          | P2: schema shape correct for every existing branch family                                                                                        | Done  | Done   |

**ID format:** `SG-[NUMBER]`

**Coverage:** 7 total, 7 mapped.

---

## Success Criteria

- [x] INSIGHT.md's `UserEntity`/`AddressEntity` shape (nested Array/Object, $ref reuse) generates a correct, complete OpenAPI document fragment (paths + components.schemas)
- [x] Every documented decorator-equivalent (Summary/Tags/RequestBody/Response/PathParams/QueryParams/BearerAuth/ExcludeFromDocs/Deprecated) round-trips correctly into the generated output
- [x] Zero regressions in existing test suite
