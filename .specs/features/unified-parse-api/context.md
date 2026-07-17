# Unified Parse API Context

**Gathered:** 2026-07-17
**Spec:** `.specs/features/unified-parse-api/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Replace the family of source-specific parse functions (`MustParseRestJsonBody`,
`ParseRestJsonBody`, `MustParseRestParams`, `ParseRestParams`, `MustParseRestQuery`,
`ParseRestQuery`, `MustParseRestFormBody`, `ParseRestFormBody`) with two unified generic
entry points — `gonest.Parse[T]` and `gonest.MustParse[T]` — that accept an opaque
`Parseable` value carrying its own parse logic. The source is made explicit at the
call site via context methods: `ctx.Params()`, `ctx.Query()`, `ctx.Headers()`,
`ctx.Body().Json()`, `ctx.Body().Form(onFile)`.

---

## Implementation Decisions

### Unified entry points

- The two public symbols are `gonest.Parse[T](src Parseable, schema *Schema) (T, error)`
  and `gonest.MustParse[T](src Parseable, schema *Schema) T` — not `ParseRest*` variants.
- `Parse[T]` / `MustParse[T]` contain **no source-specific logic**; they declare `var zero T`,
  call `src.parse(&zero, schema)`, and return. Adding a new source never touches these two functions.

### `Parseable` interface

- Named `Parseable` (not `ParseSource`, not `Source`) — user's explicit choice.
- Single unexported method: `parse(dst any, m *Schema) error`.
- Unexported method → only types defined inside `internal/validate` (or packages that embed them)
  can satisfy it. Devusers never implement `Parseable` directly; they only receive values of it
  from `ctx` methods.

### `ctx.Body()` reshape

- Current `ctx.Body() []byte` is renamed `ctx.RawBody() []byte` — user's explicit choice.
- New `ctx.Body()` returns a `BodySource` intermediate type.
- `BodySource.Json()` returns a `Parseable` for JSON body parsing.
- `BodySource.Form(onFile func(*FormFile) error)` returns a `Parseable` for multipart form parsing.
- The `onFile` callback is an argument of `Form()`, not of `Parse[T]` / `MustParse[T]`.
- `onFile == nil` → file parts are silently skipped (no panic).

### New `ctx.Headers()` source

- `ctx.Headers()` returns a `Parseable` reading fields via the `header:"name"` struct tag
  using `ctx.Header(name)` (case-insensitive, Fiber-delegated).
- Missing required header field → collected into `*BadRequestException` (same convention as
  params/query/json).
- This is a **net-new capability** — no equivalent exists today.

### New `ctx.Params()` and `ctx.Query()` on `RestContext`

- `ctx.Params()` and `ctx.Query()` are added as convenience methods on `*execution.Context`,
  returning their respective `Parseable` values.
- They do NOT replace the existing `ctx.Param(name string)` and `ctx.Queries()` low-level
  accessors — those remain untouched.

### Breaking change: immediate removal of legacy functions

- All 8 legacy public wrappers removed from `gonest.go` and all corresponding exported
  functions removed from `internal/validate` simultaneously — no deprecation period.
- All internal call sites (tests, `.examples/`) are migrated in the same PR.

### XML body parsing

- Out of scope for this feature but explicitly acknowledged as a **drop-in addition** once
  `BodySource` exists: `BodySource.Xml()` would return a new `Parseable` without touching
  `Parse[T]` / `MustParse[T]`. Deferred to ROADMAP.

---

## Specific References

- `INSIGHT-PARSE.md` at the repo root — the code sketch that is the target end-state.
  The code in that file must compile unchanged after this feature lands.
- AD-019 in STATE.md — established passing `*Schema` as an explicit argument (no global
  registry lookup). The new API inherits this constraint: `Parse[T]` still requires the
  `*Schema` value in hand.
- AD-004 in STATE.md — explains why public generic functions must be real wrappers (Go cannot
  re-export generic functions via `var`). `Parse[T]` / `MustParse[T]` in `gonest.go` are
  real wrapper functions calling the internal implementation.

---

## Deferred Ideas

- `ctx.Body().Xml()` — XML body parsing following the same `Parseable` pattern.
- Making `Parseable` partially or fully exported if external authors ever need to implement
  custom sources (e.g., protobuf body, gRPC metadata). Deferred until there is demand.
