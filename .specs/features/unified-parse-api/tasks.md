# Unified Parse API — Tasks

**Spec**: `.specs/features/unified-parse-api/spec.md`
**Status**: Draft

---

## Execution Plan

### Phase 1: Foundation — `Parseable` interface (Sequential)

```
T1
```

### Phase 2: Source implementations (Parallel)

```
     ┌→ T2 [P]
     ├→ T3 [P]
T1 ──┤→ T4 [P]
     ├→ T5 [P]
     └→ T6 [P]
```

### Phase 3: `RestContext` reshape (Sequential)

```
T1 complete → T7        (RawBody + BodySource — depends only on T1, not on sources)
T2..T6 + T7 complete → T8  (wire sources into Context)
```

### Phase 4: Public API entry points (Sequential)

```
T8 → T9
```

### Phase 5: Remove legacy + migrate call sites (Sequential)

```
T9 → T10 → T11 → T12
```

### Phase 6: Final gate

```
T12 → T13
```

---

## Task Breakdown

### T1: Define `Parseable` interface in `internal/execution`

**What**: Create the `Parseable` interface with a single unexported method `parse(dst any, s *Schema) error` in `internal/execution/context.go`. Placing it here avoids the import cycle: `internal/validate` already imports `internal/execution`, so `validate`'s source structs can implement `execution.Parseable` without any new dependency direction. `gonest.go` re-exports it as `type Parseable = execution.Parseable`.
**Where**: `internal/execution/context.go` (new type)
**Depends on**: None
**Reuses**: `*schema.Schema` (imported via `internal/validate` chain — check if `execution` needs to add `schema` import or if the method signature uses `any`)
**Requirement**: PARSE-01..06

> **Import note**: `execution` must NOT import `internal/schema` if that creates a cycle. Use `any` for the schema parameter in the unexported method: `parse(dst any, schema any) error`. The concrete `*schema.Schema` cast happens inside each source's `parse` implementation in `internal/validate`, which already imports `schema`.

**Done when**:
- [ ] `type Parseable interface { parse(dst any, schema any) error }` defined in `internal/execution/context.go`
- [ ] No new import cycles (`go build ./...` passes)

**Tests**: none (interface only, no behavior)
**Gate**: build

**Commit**: `refactor(execution): introduce Parseable interface`

---

### T2: Implement `paramsSource` (params `Parseable`) [P]

**What**: Create `paramsSource` struct implementing `Parseable`, wrapping the current `ParseRestParams` logic. Its `parse` method populates `dst` from path params using the `param:` struct tag.
**Where**: `internal/validate/params.go`
**Depends on**: T1
**Reuses**: Existing `ParseRestParams[T]` / `MustParseRestParams[T]` implementation in same file
**Requirement**: PARSE-01

**Done when**:
- [ ] `paramsSource` struct defined with a `ctx` field (type `*execution.Context`)
- [ ] `parse(dst any, s *Schema) error` delegates to the existing internal params-parse logic (no logic duplication — extract shared helper if needed)
- [ ] Gate: `go build ./...` passes

**Tests**: unit — existing `params_test.go` must still pass (0 regressions)
**Gate**: quick (`go test ./internal/validate/...`)

**Commit**: `refactor(validate): extract paramsSource implementing Parseable`

---

### T3: Implement `querySource` (query `Parseable`) [P]

**What**: Create `querySource` struct implementing `Parseable`, wrapping the current `ParseRestQuery` logic.
**Where**: `internal/validate/query.go`
**Depends on**: T1
**Reuses**: Existing `ParseRestQuery[T]` / `MustParseRestQuery[T]` implementation in same file
**Requirement**: PARSE-02

**Done when**:
- [ ] `querySource` struct defined with a `ctx` field
- [ ] `parse(dst any, s *Schema) error` delegates to existing internal query-parse logic
- [ ] Gate: `go test ./internal/validate/...` passes

**Tests**: unit — existing `query_test.go` must still pass
**Gate**: quick (`go test ./internal/validate/...`)

**Commit**: `refactor(validate): extract querySource implementing Parseable`

---

### T4: Implement `jsonBodySource` (JSON body `Parseable`) [P]

**What**: Create `jsonBodySource` struct implementing `Parseable`, wrapping the current `ParseRestJsonBody` logic.
**Where**: `internal/validate/validate.go`
**Depends on**: T1
**Reuses**: Existing `ParseRestJsonBody[T]` / `MustParseRestJsonBody[T]` implementation in same file
**Requirement**: PARSE-03

**Done when**:
- [ ] `jsonBodySource` struct defined with a `ctx` field
- [ ] `parse(dst any, s *Schema) error` delegates to existing internal json-body-parse logic
- [ ] Gate: `go test ./internal/validate/...` passes

**Tests**: unit — existing `validate_test.go` must still pass
**Gate**: quick (`go test ./internal/validate/...`)

**Commit**: `refactor(validate): extract jsonBodySource implementing Parseable`

---

### T5: Implement `formBodySource` (form `Parseable`) [P]

**What**: Create `formBodySource` struct implementing `Parseable`, wrapping the current `ParseRestFormBody` logic. The struct holds the `onFile` callback alongside the `ctx`.
**Where**: `internal/validate/form.go`
**Depends on**: T1
**Reuses**: Existing `ParseRestFormBody[T]` / `MustParseRestFormBody[T]` implementation in same file
**Requirement**: PARSE-04

**Done when**:
- [ ] `formBodySource` struct defined with `ctx` and `onFile func(*FormFile) error` fields
- [ ] `parse(dst any, s *Schema) error` delegates to existing internal form-parse logic
- [ ] `onFile == nil` → file parts are silently skipped (no panic)
- [ ] Gate: `go test ./internal/validate/...` passes

**Tests**: unit — existing `form_test.go` must still pass
**Gate**: quick (`go test ./internal/validate/...`)

**Commit**: `refactor(validate): extract formBodySource implementing Parseable`

---

### T6: Implement `headersSource` (headers `Parseable`) [P]

**What**: Create `headersSource` struct implementing `Parseable`. Its `parse` method reads struct fields via `header:` struct tag using `ctx.Header(name)`. Missing required fields are collected into `*BadRequestException`.
**Where**: `internal/validate/validate.go` (or new `headers.go`)
**Depends on**: T1
**Reuses**: Field-iteration pattern from `params.go` / `query.go`; `ctx.Header(name)` from `internal/execution`
**Requirement**: PARSE-05, PARSE-08

**Done when**:
- [ ] `headersSource` struct defined with a `ctx` field
- [ ] `parse` reads each field by `header:"name"` tag via `ctx.Header(name)` (case-insensitive, delegated to Fiber)
- [ ] Missing required field → error collected in `*BadRequestException`
- [ ] Unit tests written in `validate_test.go` (or new `headers_test.go`) covering: field present, field missing + required, field missing + optional
- [ ] Gate: `go test ./internal/validate/...` passes

**Tests**: unit
**Gate**: quick (`go test ./internal/validate/...`)

**Commit**: `feat(validate): implement headersSource for header parsing`

---

### T7: Rename `ctx.Body()` → `ctx.RawBody()` and introduce `BodySource` (opaque closure carrier)

**What**: In `internal/execution/context.go` and its `Responder` interface: rename `Body() []byte` to `RawBody() []byte`. Add a `BodySource` struct in `internal/execution` that carries two **unexported function fields** (`jsonFn func() Parseable` and `formFn func(func(*FormFile) error) Parseable`) — same opaque-carrier pattern as `Context.WithRoute(any)` (STATE.md L-004). Its `.Json()` and `.Form(onFile)` methods call those closures. Add `ctx.Body() BodySource` returning a zero `BodySource`; the closures are wired in T8 by `internal/route`.

> **Why closure fields instead of direct construction**: `execution` cannot import `internal/validate` (validate already imports execution — cycle). `BodySource` must therefore never reference `jsonBodySource`/`formBodySource` directly. The closures are set by the one package that already bridges both: `internal/route` (which imports `execution` and will gain an import of `validate` in T8).

**Where**: `internal/execution/context.go`
**Depends on**: T1 (needs `Parseable` type already defined in this file)
**Reuses**: `Context.WithRoute(any)` opaque-carrier pattern
**Requirement**: PARSE-07

**Done when**:
- [ ] `Responder.Body() []byte` renamed to `Responder.RawBody() []byte`
- [ ] `Context.Body() []byte` renamed to `Context.RawBody() []byte`
- [ ] `BodySource` struct defined in `execution` with unexported `jsonFn func() Parseable` and `formFn func(func(*FormFile) error) Parseable`
- [ ] `BodySource.Json() Parseable` and `BodySource.Form(onFile func(*FormFile) error) Parseable` implemented by calling the respective closure fields
- [ ] `Context` gains a `bodySource BodySource` field; `ctx.Body() BodySource` returns it
- [ ] `Context.WithBodySource(bs BodySource) *Context` added for route-level wiring (same pattern as `WithRoute`)
- [ ] All internal call sites of old `ctx.Body() []byte` updated to `ctx.RawBody()` (search: `adapter/fiber`, test fakes, `validate/form.go`, `validate/validate.go`)
- [ ] Gate: `go build ./...` passes

**Tests**: unit — `context_test.go` updated: rename `Body`→`RawBody` in fake; smoke test `ctx.Body()` returns a `BodySource` (closures can be nil at this stage — wired in T8)
**Gate**: quick (`go test ./internal/execution/...`)

**Commit**: `refactor(execution): rename Body→RawBody, introduce BodySource opaque carrier`

---

### T8: Wire sources into `RestContext` via `internal/adapter/fiber` and expose `ctx.Params()`, `ctx.Query()`, `ctx.Headers()`


**What**: Two sub-deliverables in one atomic file scope:
1. Add `paramsSource Parseable`, `querySource Parseable`, `headersSource Parseable` unexported fields to `Context`; add `ctx.Params()`, `ctx.Query()`, `ctx.Headers()` returning them; add `Context.WithSources(params, query, headers Parseable, body BodySource) *Context` (same carrier pattern as `WithRoute`).
2. In `internal/adapter/fiber` (confirmed cycle-safe wiring point): add a single import of `internal/validate` and call `ctx.WithSources(...)` right after `ctx.WithRoute(r)` in the per-request dispatch path.

> **Import graph (confirmed via `go list -deps`):**
> - `validate → execution, route` — validate already imports both
> - `route → execution` — does NOT import validate
> - `adapter/fiber → execution, route` — does NOT import validate today ✅ **safe to add**
> - Wiring in `route` would be `validate → route → validate` — confirmed cycle, ruled out.

**Where**: `internal/execution/context.go` (fields + `WithSources`) + `internal/adapter/fiber/fiber.go` (wiring call)
**Depends on**: T2, T3, T6, T7
**Reuses**: `paramsSource`, `querySource`, `headersSource` from T2/T3/T6; `WithRoute` pattern from `execution/context.go`
**Requirement**: PARSE-01, PARSE-02, PARSE-05, PARSE-07, PARSE-08

**Done when**:
- [ ] `ctx.Params() Parseable`, `ctx.Query() Parseable`, `ctx.Headers() Parseable` exist on `*Context`
- [ ] `ctx.Body().Json()` and `ctx.Body().Form(onFile)` return non-nil `Parseable` values during a real request
- [ ] `ctx.WithSources(...)` called in `internal/adapter/fiber` per-request dispatch, right after `WithRoute`
- [ ] Gate: `go build ./...` passes

**Tests**: integration — route dispatch test calling `ctx.Params()`, `ctx.Query()`, `ctx.Headers()`, `ctx.Body().Json()` inside a handler and verifying non-nil `Parseable` returned
**Gate**: quick (`go test ./internal/...`)

**Commit**: `feat(execution,fiber): wire Parseable sources into Context at dispatch time`

---

### T9: Add `gonest.Parse[T]` and `gonest.MustParse[T]` to `gonest.go`

**What**: Expose the two public generic entry points in `gonest.go`. Each creates `var zero T`, calls `src.parse(&zero, m)` (via `resolveSchema` check), and returns.
**Where**: `gonest.go` — new section `// Validation (Unified Parse API)`
**Depends on**: T8
**Reuses**: Existing `resolveSchema` pattern from `internal/validate`; `Schema` / `Parseable` types
**Requirement**: PARSE-01..06, PARSE-06

**Done when**:
- [ ] `func Parse[T any](src Parseable, s *Schema) (T, error)` exported and documented
- [ ] `func MustParse[T any](src Parseable, s *Schema) T` exported and documented
- [ ] Schema mismatch → panic with clear message (via `resolveSchema`)
- [ ] Gate: `go build ./...` passes

**Tests**: unit — integration tests in `gonest_test.go` exercising each source via `Parse` and `MustParse`
**Gate**: quick (`go test . -run TestParse`)

**Commit**: `feat(gonest): expose Parse[T] and MustParse[T] unified entry points`

---

### T10: Remove legacy functions from `internal/validate`

**What**: Delete the 8 legacy exported functions: `ParseRestJsonBody`, `MustParseRestJsonBody`, `ParseRestParams`, `MustParseRestParams`, `ParseRestQuery`, `MustParseRestQuery`, `ParseRestFormBody`, `MustParseRestFormBody` from their respective files.
**Where**: `internal/validate/validate.go`, `params.go`, `query.go`, `form.go`
**Depends on**: T9 (public entry points exist before removing internals)
**Requirement**: PARSE-09

**Done when**:
- [ ] All 8 legacy function bodies removed
- [ ] Gate: `go build ./...` passes (no remaining callers in internal packages)

**Tests**: none — this is deletion; gate proves no remaining references
**Gate**: build (`go build ./...`)

**Commit**: `refactor(validate): remove legacy ParseRest* functions`

---

### T11: Remove legacy functions from `gonest.go`

**What**: Delete the 8 legacy public wrappers from `gonest.go`: `ParseRestJsonBody`, `MustParseRestJsonBody`, `ParseRestParams`, `MustParseRestParams`, `ParseRestQuery`, `MustParseRestQuery`, `ParseRestFormBody`, `MustParseRestFormBody`. Remove the now-empty `// Validation (JSON Body Validation feature)` section header.
**Where**: `gonest.go`
**Depends on**: T10
**Requirement**: PARSE-09

**Done when**:
- [ ] All 8 wrappers removed from `gonest.go`
- [ ] Gate: `go build ./...` passes

**Tests**: none — deletion
**Gate**: build (`go build ./...`)

**Commit**: `refactor(gonest): remove legacy ParseRest* public wrappers`

---

### T12: Migrate all internal call sites to new API

**What**: Update every remaining reference to the legacy API in tests and examples.
**Where**:
- `gonest_test.go` (references to `MustParseRestJsonBody`, `ParseRestJsonBody`, etc.)
- `internal/validate/*_test.go` (any remaining direct legacy calls)
- `internal/app/*_test.go`
- `.examples/blog-api/module/*/controller.go`
- `.examples/simple-todo/controller.go`

**Depends on**: T11
**Requirement**: PARSE-09

**Done when**:
- [ ] `grep -r "ParseRestJsonBody\|ParseRestParams\|ParseRestQuery\|ParseRestFormBody\|MustParseRestJsonBody\|MustParseRestParams\|MustParseRestQuery\|MustParseRestFormBody" .` returns empty (except `.specs/` and STATE.md)
- [ ] `go test ./...` passes (23 packages, same count as baseline)

**Tests**: unit + integration (all existing tests)
**Gate**: quick (`go test ./...`)

**Commit**: `refactor: migrate all call sites from legacy Parse* to MustParse/Parse`

---

### T13: Final gate + update INSIGHT-PARSE.md

**What**: Run full suite, verify zero legacy symbols, and confirm the `INSIGHT-PARSE.md` sketch compiles as-is. Update STATE.md with the new AD entry.
**Where**: root, `INSIGHT-PARSE.md`, `.specs/project/STATE.md`
**Depends on**: T12

**Done when**:
- [ ] `go test ./...` passes — 23 packages, no new failures
- [ ] `go build ./...` passes
- [ ] `INSIGHT-PARSE.md` code compiles (add a `_test.go` compile-only test if needed)
- [ ] STATE.md has new AD entry documenting the Parseable/unified parse decision
- [ ] Traceability table in spec.md updated — all PARSE-0x → Verified

**Tests**: integration (full suite)
**Gate**: full (`go test ./...`)

**Commit**: `chore: finalize unified-parse-api feature — update STATE, verify gate`

---

## Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Parseable interface | 1 type definition | ✅ |
| T2: paramsSource | 1 struct + 1 method | ✅ |
| T3: querySource | 1 struct + 1 method | ✅ |
| T4: jsonBodySource | 1 struct + 1 method | ✅ |
| T5: formBodySource | 1 struct + 1 method | ✅ |
| T6: headersSource | 1 struct + 1 method + tests | ✅ |
| T7: RawBody + BodySource | 1 file reshape (closure carrier) | ✅ |
| T8: 3 ctx methods + dispatch wiring | execution fields + 1 dispatch callsite | ✅ |
| T9: Parse[T] + MustParse[T] | 2 functions in 1 file | ✅ |
| T10: Remove validate legacy | deletion in 4 files | ✅ |
| T11: Remove gonest.go legacy | deletion in 1 file | ✅ |
| T12: Migrate call sites | mechanical find+replace | ✅ |
| T13: Final gate + STATE | verification + 1 doc update | ✅ |

## Diagram-Definition Cross-Check

| Task | Depends On (body) | Diagram Shows | Status |
| ---- | ----------------- | ------------- | ------ |
| T1 | None | Start | ✅ |
| T2 | T1 | T1→T2 | ✅ |
| T3 | T1 | T1→T3 | ✅ |
| T4 | T1 | T1→T4 | ✅ |
| T5 | T1 | T1→T5 | ✅ |
| T6 | T1 | T1→T6 | ✅ |
| T7 | T1 only | T1→T7 (parallel with T2..T6) | ✅ |
| T8 | T2, T3, T6, T7 | T2..T6+T7→T8 | ✅ |
| T9 | T8 | T8→T9 | ✅ |
| T10 | T9 | T9→T10 | ✅ |
| T11 | T10 | T10→T11 | ✅ |
| T12 | T11 | T11→T12 | ✅ |
| T13 | T12 | T12→T13 | ✅ |

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| ---- | ---------- | --------------- | --------- | ------ |
| T1 | Interface (no behavior) | none | none | ✅ |
| T2 | `internal/validate` (params) | unit | unit | ✅ |
| T3 | `internal/validate` (query) | unit | unit | ✅ |
| T4 | `internal/validate` (json body) | unit | unit | ✅ |
| T5 | `internal/validate` (form) | unit | unit | ✅ |
| T6 | `internal/validate` (headers — new) | unit | unit | ✅ |
| T7 | `internal/execution` (Context + BodySource) | unit | unit | ✅ |
| T8 | `internal/execution` (Context) + dispatch wiring | integration | integration | ✅ |
| T9 | Public API (`gonest.go`) | unit | unit | ✅ |
| T10 | Deletion in validate | none (deletion) | none | ✅ |
| T11 | Deletion in gonest.go | none (deletion) | none | ✅ |
| T12 | Test + example migration | unit+integration | unit+integration | ✅ |
| T13 | Full suite gate | integration | integration | ✅ |
