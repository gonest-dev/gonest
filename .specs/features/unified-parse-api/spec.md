# Unified Parse API Specification

## Problem Statement

The current parse API exposes a dedicated function pair per HTTP data source
(`MustParseRestJsonBody`, `MustParseRestParams`, `MustParseRestQuery`,
`MustParseRestFormBody`, plus their non-panicking equivalents). Developers
must memorize which function to call for each source × behavior combination,
and adding header support would require yet another pair. The name proliferation
prevents discovery via autocomplete and makes the API unwelcoming.

## Goals

- [ ] Unify parsing into two symbols: `Parse[T]` and `MustParse[T]`
- [ ] Make the **data source** explicit via argument (`ctx.Params()`, `ctx.Query()`, `ctx.Headers()`, `ctx.Body().Json()`, `ctx.Body().Form(onFile)`)
- [ ] Add support for parsing **HTTP headers** (missing today)
- [ ] Remove legacy functions (`MustParseRestJsonBody`, `ParseRestJsonBody`, etc.) — breaking change accepted
- [ ] Design `Parseable` so that new body formats (e.g. XML) can be added later by following the same pattern without touching `Parse[T]` / `MustParse[T]`

## Out of Scope

| Feature                                 | Reason                                                                                                                                    |
| --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| XML body parsing                        | Not in scope for this feature, but the `Parseable` architecture makes it a drop-in addition (`ctx.Body().Xml()`) — deferred to ROADMAP |
| Gradual deprecation of legacy functions | User opted for immediate removal                                                                                                          |
| Changes to validation semantics         | Error collection behavior (`*BadRequestException`) does not change                                                                        |

---

## Design Decisions (made during discussion)

| #   | Decision                                                                                                                                                                                                                 |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| D1  | Current `ctx.Body()` (returns `[]byte`) is renamed to `ctx.RawBody()`. The new `ctx.Body()` returns a `BodySource` with `.Json()` and `.Form(onFile)` methods.                                                           |
| D2  | The `onFile func(*FormFile) error` callback for form streaming becomes an argument of the `Form()` method: `ctx.Body().Form(onFile)`.                                                                                    |
| D3  | Legacy functions (`MustParseRestJsonBody`, `ParseRestJsonBody`, `MustParseRestParams`, `ParseRestParams`, `MustParseRestQuery`, `ParseRestQuery`, `MustParseRestFormBody`, `ParseRestFormBody`) are removed immediately. |

---

## Architecture Note

Each method on `RestContext` (`ctx.Params()`, `ctx.Query()`, `ctx.Headers()`,
`ctx.Body().Json()`, `ctx.Body().Form(onFile)`) returns a `Parseable` —
an opaque value that **carries its own parse function** (or closure) alongside
any state needed to execute it (the raw bytes, the query map, the header
accessor, the multipart stream + `onFile` callback, etc.).

`gonest.Parse[T]` and `gonest.MustParse[T]` are thin generics that:

1. Declare `var zero T`
2. Call the parse function stored inside the `Parseable`, passing `&zero`
   and the `*Schema`
3. Return `zero` (and the error, for the non-panicking variant)

This means `Parse[T]` / `MustParse[T]` themselves contain **no source-specific
logic** — adding a new source (e.g. XML) only requires implementing a new
`Parseable` constructor (`ctx.Body().Xml()`); the two public entry points
remain untouched.

---

## API Sketch

```go
package ex

import "gonest.dev/gonest"

type Headers struct { XApiKey string `header:"x-api-key"` }
var HeadersSchema = gonest.Schema[Headers](func(t *Headers, s *gonest.Schema) {
  s.Property(&t.XApiKey).Description("X API Key")
})

type Params struct { UserID string `param:"user_id"` }
var ParamsSchema = gonest.Schema[Params](func(t *Params, s *gonest.Schema) {
  s.Property(&t.UserID).Description("User ID")
})

type Query struct { Limit string `query:"limit"` }
var QuerySchema = gonest.Schema[Query](func(t *Query, s *gonest.Schema) {
  s.Property(&t.Limit).Description("Limit")
})

type BodyJson struct { Name string `json:"name"` }
var BodyJsonSchema = gonest.Schema[BodyJson](func(t *BodyJson, s *gonest.Schema) {
  s.Property(&t.Name).Description("Name")
})

type BodyForm struct { Name string `form:"name"` }
var BodyFormSchema = gonest.Schema[BodyForm](func(t *BodyForm, s *gonest.Schema) {
  s.Property(&t.Name).Description("Name")
})

var Controller = gonest.NewController(func(controller *gonest.Controller) {
  controller.Path("/ex")
  controller.RoutePatch("/", func(ctx *gonest.RestContext) {
    // non-panicking variants: Parse[T] returns (T, error)
    headers  := gonest.MustParse[Headers](ctx.Headers(), HeadersSchema)
    params   := gonest.MustParse[Params](ctx.Params(), ParamsSchema)
    query    := gonest.MustParse[Query](ctx.Query(), QuerySchema)
    bodyJson := gonest.MustParse[BodyJson](ctx.Body().Json(), BodyJsonSchema)
    bodyForm := gonest.MustParse[BodyForm](ctx.Body().Form(nil), BodyFormSchema)
    _ = headers; _ = params; _ = query; _ = bodyJson; _ = bodyForm
  })
})
```

---

## User Stories

### P1: Unified parse API ⭐ MVP

**User Story**: As a developer using the gonest library, I want to parse and validate HTTP data using `gonest.Parse[T]` / `gonest.MustParse[T]` with the source as an explicit argument, so that I can discover what is available via autocomplete without memorizing source-specific function names.

**Why P1**: This is the core of the feature — everything else depends on it.

**Acceptance Criteria**:

1. WHEN the developer calls `gonest.MustParse[T](ctx.Params(), schema)` THEN system SHALL parse path params and return `T`, panicking on validation error
2. WHEN the developer calls `gonest.Parse[T](ctx.Params(), schema)` THEN system SHALL parse path params and return `(T, error)` where error is `*BadRequestException` on validation failure
3. WHEN the developer calls `gonest.MustParse[T](ctx.Query(), schema)` THEN system SHALL parse the query string and return `T`
4. WHEN the developer calls `gonest.Parse[T](ctx.Query(), schema)` THEN system SHALL parse the query string and return `(T, error)`
5. WHEN the developer calls `gonest.MustParse[T](ctx.Body().Json(), schema)` THEN system SHALL parse the JSON body and return `T`
6. WHEN the developer calls `gonest.Parse[T](ctx.Body().Json(), schema)` THEN system SHALL parse the JSON body and return `(T, error)`
7. WHEN the developer calls `gonest.MustParse[T](ctx.Body().Form(onFile), schema)` THEN system SHALL parse the multipart form body (invoking `onFile` for each file part) and return `T`
8. WHEN the developer calls `gonest.Parse[T](ctx.Body().Form(onFile), schema)` THEN system SHALL parse the multipart form body and return `(T, error)`
9. WHEN the developer calls `gonest.MustParse[T](ctx.Headers(), schema)` THEN system SHALL parse HTTP headers and return `T` (new source, absent today)
10. WHEN the developer calls `gonest.Parse[T](ctx.Headers(), schema)` THEN system SHALL parse HTTP headers and return `(T, error)`
11. WHEN the `*Schema` passed was built for a different type than `T` THEN system SHALL panic with a clear message (same behavior as current `resolveSchema`)

**Independent Test**: The code from `INSIGHT-PARSE.md` compiles and `go test ./...` passes with no error.

---

### P1: Rename `ctx.Body()` → `ctx.RawBody()` and introduce new `ctx.Body()` ⭐ MVP

**User Story**: As a developer, I want `ctx.Body()` to return an intermediate `BodySource` type with `.Json()` and `.Form(onFile)` methods, so that I can discover supported body formats directly via autocomplete.

**Why P1**: Blocking for the unified API — `ctx.Body().Json()` and `ctx.Body().Form(onFile)` cannot exist while `Body()` returns `[]byte`.

**Acceptance Criteria**:

1. WHEN the developer calls `ctx.Body()` THEN system SHALL return a `BodySource` (opaque type) with `.Json()` and `.Form(onFile func(*FormFile) error)` methods
2. WHEN the developer calls `ctx.RawBody()` THEN system SHALL return `[]byte` with the same behavior as the current `ctx.Body()`
3. WHEN the developer calls `ctx.Body().Json()` THEN system SHALL return a `Parseable` encoding the source as "JSON body"
4. WHEN the developer calls `ctx.Body().Form(onFile)` THEN system SHALL return a `Parseable` encoding the source as "multipart form" + the `onFile` callback
5. WHEN `AppOptions.EnableFormStreaming` is not enabled OR the `Content-Type` is not `multipart/form-data` THEN `ctx.Body().Form(onFile)` SHALL behave like the current `ParseRestFormBody` (panic with plain string — coding/config error, not a validation error)

**Independent Test**: All internal `ctx.Body()` call sites migrate to `ctx.RawBody()` without test failures.

---

### P1: Introduce `ctx.Headers()` as a `Parseable` ⭐ MVP

**User Story**: As a developer, I want to parse and validate HTTP headers the same way as params/query/body, including support for the `header:"name"` struct tag.

**Why P1**: The absence of header parsing is a gap in the current API that the new pattern fills naturally.

**Acceptance Criteria**:

1. WHEN the developer calls `ctx.Headers()` THEN system SHALL return a `Parseable` encoding the source as "HTTP headers"
2. WHEN `MustParse[T](ctx.Headers(), schema)` is called THEN system SHALL read each field of `T` by the `header:"name"` struct tag value via `ctx.Header(name)`
3. WHEN a field marked as required in the schema is absent from the request headers THEN system SHALL include the violation in the returned `*BadRequestException`

**Independent Test**: A controller test using `ctx.Headers()` with a struct `{ XApiKey string \`header:"x-api-key"\` }` validates correctly.

---

### P1: Remove legacy functions ⭐ MVP

**User Story**: As a library maintainer, I want to remove `MustParseRestJsonBody`, `ParseRestJsonBody`, `MustParseRestParams`, `ParseRestParams`, `MustParseRestQuery`, `ParseRestQuery`, `MustParseRestFormBody`, and `ParseRestFormBody` from `gonest.go` and `internal/validate`, so that the public API does not accumulate obsolete symbols.

**Why P1**: Explicit, agreed-upon breaking change — keeping the legacy alongside the new API creates confusion.

**Acceptance Criteria**:

1. WHEN `go build ./...` runs after removal THEN system SHALL compile without error (all internal call sites migrated)
2. WHEN `go test ./... -race` runs THEN system SHALL pass with no failure
3. WHEN an external developer tries to call `gonest.MustParseRestJsonBody` THEN system SHALL fail at compile time (symbol does not exist)

**Independent Test**: `go test ./... -race` is green; `grep -r "MustParseRestJsonBody\|ParseRestJsonBody\|..." .` returns empty (except historical references in STATE.md/spec.md).

---

## Edge Cases

- WHEN `ctx.Body().Form(nil)` is called with `onFile == nil` THEN system SHALL treat each file part as ignored silently (or panic with a clear message — to be decided in design)
- WHEN the schema passed to `Parse[T](ctx.Headers(), schema)` has no `header:` struct tags THEN system SHALL return a zero `T` with no error (same behavior as params/query with untagged fields)
- WHEN `ctx.Headers()` is used and the header name has different casing THEN system SHALL normalize the lookup (consistent with current `ctx.Header(name)` behavior)

---

## Requirement Traceability

| Requirement ID | Story                                   | Phase  | Status  |
| -------------- | --------------------------------------- | ------ | ------- |
| PARSE-01       | P1: Unified API — Params                | Execute | Verified |
| PARSE-02       | P1: Unified API — Query                 | Execute | Verified |
| PARSE-03       | P1: Unified API — Body JSON             | Execute | Verified |
| PARSE-04       | P1: Unified API — Body Form             | Execute | Verified |
| PARSE-05       | P1: Unified API — Headers               | Execute | Verified |
| PARSE-06       | P1: Unified API — schema mismatch panic | Execute | Verified |
| PARSE-07       | P1: RawBody rename + BodySource         | Execute | Verified |
| PARSE-08       | P1: ctx.Headers() as Parseable        | Execute | Verified |
| PARSE-09       | P1: Remove legacy functions             | Execute | Verified |

---

## Success Criteria

- [ ] `go test ./... -race` passes after full migration
- [ ] Code from `INSIGHT-PARSE.md` compiles without modification
- [ ] Zero occurrences of legacy symbols outside STATE.md/spec.md
- [ ] `ctx.Headers()`, `ctx.Params()`, `ctx.Query()`, `ctx.Body().Json()`, `ctx.Body().Form(onFile)` exist and work correctly
