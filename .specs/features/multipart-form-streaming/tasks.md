# Multipart Form Streaming Tasks

**Design**: `.specs/features/multipart-form-streaming/design.md`
**Status**: Done (T1-T6, T6 satisfied by T3 -- AD-022 in STATE.md)

---

## Execution Plan

### Phase 1: Foundation (Sequential -- both touch `internal/adapter/fiber/fiber.go`)

```
T1 → T2
```

### Phase 2: Core Implementation (Sequential -- T3 needs T2's FormStream)

```
T2 → T3 → T4
```

### Phase 3: Verification (Sequential -- proves the actual streaming claim)

```
T4 → T5 → T6
```

No `[P]` tasks in this feature -- T1/T2 share one file (`fiber.go`), and
everything after is a straight dependency chain (each layer needs the one
below it to compile). Not worth forcing artificial parallelism onto a
6-task chain.

---

## Task Breakdown

### T1: `HttpAdapter.Init(opts AppOptions)` + `AppOptions.EnableFormStreaming`

**What**: Change `HttpAdapter.Init()` to `Init(opts AppOptions)`; add `EnableFormStreaming bool` to `AppOptions`; update `newAdapter[T,PT]`/`NewApp`'s one call site; update `FiberApp.Init` to build `fiber.New(fiber.Config{StreamRequestBody: opts.EnableFormStreaming, DisablePreParseMultipartForm: opts.EnableFormStreaming})`.
**Where**: `internal/app/app.go` (interface + `newAdapter`/`NewApp` call site), `internal/app/options.go` (new field), `internal/adapter/fiber/fiber.go` (`Init` signature + body)
**Depends on**: None
**Reuses**: existing lazy-init-once guard in `FiberApp.Init`
**Requirement**: MPF-01

**Done when**:

- [ ] `HttpAdapter.Init(opts AppOptions)` compiles, `newAdapter`/`NewApp` thread `opts` through
- [ ] `FiberApp.Init(opts app.AppOptions)` builds `fiber.Config` conditionally, both flags always set together
- [ ] `AppOptions.EnableFormStreaming` field exists, defaults `false`, root `gonest.AppOptions` alias picks it up automatically (type alias, no re-export work needed)
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: baseline unchanged (pure interface/plumbing, no new test needed here -- proven indirectly by T5/T6)

**Tests**: none (plumbing only, no observable behavior yet -- Test Coverage Matrix has no "adapter construction wiring" row; covered end-to-end by T5/T6)
**Gate**: quick (`go test ./... -race`)

**Commit**: `feat(app)!: thread AppOptions into HttpAdapter.Init for form streaming config`

---

### T2: `Responder.BodyStream()` + `Context.FormStream()`

**What**: Add `BodyStream() (io.Reader, string, bool)` to `execution.Responder`; implement it for real in `fiberResponder` (via `c.RequestCtx().RequestBodyStream()` + boundary parsed from `Content-Type`); add `Context.FormStream()` thin delegation; add a stub (`return nil, "", false`) to every existing fake `Responder` implementation.
**Where**: `internal/execution/context.go` (interface + `FormStream`), `internal/adapter/fiber/fiber.go` (real impl), plus a 1-line stub each in: `gonest_test.go`, `internal/execution/context_test.go`, `internal/filter/filter_test.go`, `internal/guard/guard_test.go`, `internal/interceptor/interceptor_test.go`, `internal/middleware/middleware_test.go`, `internal/route/route_test.go`, `internal/validate/params_test.go`, `internal/validate/query_test.go`, `internal/validate/validate_test.go`
**Depends on**: T1 (same file, avoid merge conflicts; also `app.AppOptions` needs to exist for `EnableFormStreaming` to be checkable, though `BodyStream`'s own `ok` logic can just check `req.bodyStream != nil`/boundary presence directly rather than re-reading `AppOptions` -- either way, doing T1 first keeps the file's history linear)
**Reuses**: same one-line-delegation pattern as `Body()`/`Queries()`; same "stub every fake responder" precedent as Terminus's `SendString`
**Requirement**: MPF-02

**Done when**:

- [ ] `Responder` interface has `BodyStream()`; every existing implementer (11 files) compiles
- [ ] `fiberResponder.BodyStream()` returns a real `io.Reader` + boundary when the request is genuinely multipart AND the server was built with streaming enabled; `(nil, "", false)` otherwise
- [ ] `Context.FormStream()` delegates to `ctx.res.BodyStream()`
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: existing count unchanged (no new test here -- `fiberResponder.BodyStream`'s real behavior is proven by T5/T6's real HTTP dispatch, same precedent as `Body()` itself having no dedicated unit test beyond what consumes it)

**Tests**: none directly (adapter-layer plumbing, same tier as `Body()`/`Queries()` -- proven by consumers)
**Gate**: quick (`go test ./... -race`)

**Commit**: `feat(execution)!: add Responder.BodyStream/Context.FormStream for raw body access`

---

### T3: `internal/validate.FormFile` + `ParseFormBody`/`MustFormBody`

**What**: New file `internal/validate/form.go` -- `FormFile` type (wraps `*multipart.Part`), `ParseFormBody[T any](ctx, m, onFile) (T, error)` (walks `mime/multipart.NewReader(stream, boundary)`, dispatches field-vs-file per part, validates via `form:"..."` tag reusing `tagKeyVisible`/`coerceParamString`/`validateValue`/`populate`), `MustFormBody[T any](ctx, m, onFile) T` (panic wrapper).
**Where**: `internal/validate/form.go` (new)
**Depends on**: T2 (`ctx.FormStream()`)
**Reuses**: `resolveSchema`, `violation`, `tagKeyVisible`, `coerceParamString`, `validateValue`, `populate`, `exception.NewBadRequestException` -- all unchanged, imported from the same package
**Requirement**: MPF-03, MPF-04, MPF-06, MPF-07

**Done when**:

- [ ] `ParseFormBody` returns `(zero, error)` when `ctx.FormStream()` reports `ok=false` (streaming not enabled / not multipart)
- [ ] Form fields (`filename == ""`) collected into presence map, keyed by `form:"..."` tag, validated exactly like `ParseQuery`'s own field loop
- [ ] File parts (`filename != ""`) trigger `onFile` synchronously, BEFORE the next part is read
- [ ] `onFile` returning a non-nil error aborts the walk and surfaces as `*BadRequestException` (field = form field name)
- [ ] `Custom(fn)` on a `form:"..."` field receives the raw string, unchanged from `param`/`query`'s own convention
- [ ] `MustFormBody` panics on any `ParseFormBody` error
- [ ] Unit tests (fake `Responder`, in-memory `multipart.Writer`-built request body, NOT yet proving true streaming -- that's T5/T6's job) cover: happy path, missing required field, `onFile` error, malformed multipart stream
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: baseline + this task's new unit tests, all passing

**Tests**: unit (same tier as `ParseParams`/`ParseQuery`'s own existing unit-test style -- fake `Responder` returning a real `io.Reader` over an in-memory `multipart.Writer`-built body is enough to prove the field/file DISPATCH logic; it does NOT need a real TCP/HTTP round-trip for this task)
**Gate**: quick (`go test ./... -race`)

**Commit**: `feat(validate): add ParseFormBody/MustFormBody for multipart/form-data`

---

### T4: Root `gonest.ParseRestFormBody`/`MustParseRestFormBody`/`FormFile`

**What**: Thin root wrappers matching AD-021's Parse/Must pair shape, plus the `FormFile` type alias.
**Where**: `gonest.go`
**Depends on**: T3
**Reuses**: `validate.ParseFormBody`/`validate.MustFormBody`/`validate.FormFile` -- pure delegation, same shape as every other `ParseRestXxx` pair
**Requirement**: MPF-05

**Done when**:

- [ ] `gonest.ParseRestFormBody[T](ctx *RestContext, m *Schema, onFile func(*FormFile) error) (T, error)` compiles and delegates
- [ ] `gonest.MustParseRestFormBody[T](...)  T` compiles and delegates
- [ ] `gonest.FormFile = validate.FormFile` alias exported
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: baseline unchanged (pure wrapper, no new logic -- proven by T5/T6 calling the ROOT names specifically)

**Tests**: none (pure delegation, same as every other root `ParseRestXxx` wrapper -- none of those have their own dedicated test beyond what calls them at root level in `gonest_test.go`)
**Gate**: quick (`go test ./... -race`)

**Commit**: `feat(gonest): export ParseRestFormBody/MustParseRestFormBody/FormFile at root`

---

### T5: Real HTTP dispatch test proving TRUE streaming (P1's Independent Test) -- DONE

**SPEC_DEVIATION discovered during execution**: the original plan (below, struck through in spirit) assumed `app.Test(req)` could prove streaming. Confirmed via direct source read that it CANNOT: Fiber v3's `App.Test` calls `httputil.DumpRequest(req, true)`, which fully reads `req.Body` into memory BEFORE `ServeConn` ever runs -- a gated/blocking `io.Reader` passed as `req.Body` would just hang `Test()` itself before the fasthttp server ever saw a byte, proving nothing. Switched to a REAL TCP dial (matching `internal/app/app_test.go`'s own `TestNewApp_UserControllerRealHttpClient_EndToEndOverRealPort` precedent: fixed `127.0.0.1:<port>`, `OnListen` synchronization, `t.Cleanup` + `Shutdown()`), since `net/http.Client`/`Transport` genuinely streams an `io.Reader` request body progressively over a real connection.

**What (as built)**: `gonest_test.go`'s `TestParseRestFormBody_RealHTTPDispatch_StreamsFileWithoutFullBuffering` -- an `app.MustListen` on a real port, POSTing a multipart body via `net/http.Client` whose file part is split across an `io.Pipe`: the first half (20000 bytes) is written immediately, the second half is GATED behind a channel the test controls. `onFile` closes a `onFileReached` channel THE INSTANT it's invoked, before it ever reads. The test asserts `onFileReached` fires BEFORE the gate is released -- proof that gonest never buffered the whole body first, since doing so would require the gated second half to have already arrived. Releasing the gate afterward lets the request complete normally, additionally proving the full round-trip (form field + file content, byte-for-byte) still works end to end.
**Where**: `gonest_test.go` (root-level end-to-end test, matching e.g. `TestHealthController_RootAlias_InsightExample_*`'s own precedent for whole-feature root tests)
**Depends on**: T4
**Reuses**: the real-TCP-dial + `OnListen`/`Shutdown` synchronization pattern from `internal/app/app_test.go`'s `TestNewApp_UserControllerRealHttpClient_EndToEndOverRealPort`
**Requirement**: MPF-01, MPF-02, MPF-03, MPF-04, MPF-05 (end-to-end proof of all of P1)

**Done when**:

- [x] Test builds a real multipart body via `mime/multipart.Writer` (1 field + 1 file part)
- [x] Test builds a real app/route with `EnableFormStreaming: true`, calls `MustParseRestFormBody` inside the handler
- [x] Assertion: returned `T`'s field matches the posted value
- [x] Assertion: the `onFile`-copied buffer matches the posted file's exact bytes
- [x] Assertion (the actual streaming proof): `onFileReached` fires before the gated second half of the file is ever released
- [x] Gate check passes: `go test ./... -race`
- [x] Test count: baseline + 1 new test, passing (`TestParseRestFormBody_RealHTTPDispatch_StreamsFileWithoutFullBuffering`, 0.10s)

**Tests**: integration (real HTTP dispatch over a real TCP port, matches the Test Coverage Matrix's own "Bind/Listen real" row)
**Gate**: full (`go test ./... -race`)

**Commit**: `test(gonest): prove ParseRestFormBody streams files without full buffering (T5)`

---

### T6: P2/P3 acceptance-criteria test cases -- SATISFIED BY T3, no additional task needed

**Decision made during execution**: `internal/validate/form_test.go` (written as part of T3, BEFORE T5 existed) already has
`TestParseFormBody_OnFileError_AbortsWithBadRequest` (P2: `onFile`'s error surfaces as `*BadRequestException` with the field name and message reachable in `Details()`) and
`TestParseFormBody_CustomFunc_ReceivesRawString_NotCoerced` (P3: `Custom(fn)` on a `form:"..."` field receives the raw string) --
both exercise `ParseFormBody` directly (not through a real HTTP round-trip), which is enough to prove the LOGIC (same precedent as `ParseParams`/`ParseQuery`'s own unit-level `Custom(fn)`/error-path tests, which also aren't duplicated at the real-HTTP tier). Re-proving the identical logic a SECOND time through a real TCP dial (T5's much heavier harness) would just be redundant test weight for zero new coverage -- T5 already proves the real-dispatch PATH works at all (P1); P2/P3 are about the VALIDATION LOGIC, already proven in T3. No new task executed.
**Requirement**: MPF-06, MPF-07 -- satisfied by `internal/validate/form_test.go`'s `TestParseFormBody_OnFileError_AbortsWithBadRequest`/`TestParseFormBody_CustomFunc_ReceivesRawString_NotCoerced` (T3).
**Gate**: full (`go test ./... -race`)

**Commit**: `test(validate): cover ParseRestFormBody's onFile-error and Custom(fn) paths`

---

## Parallel Execution Map

```
T1 ──→ T2 ──→ T3 ──→ T4 ──→ T5 ──→ T6
```

Straight sequential chain -- see Execution Plan above for why no `[P]`
flags apply (shared files in Phase 1, strict compile-order dependency
everywhere else).

---

## Task Granularity Check

| Task                                          | Scope                                  | Status      |
| ------------------------------------------------ | ----------------------------------------- | -------------- |
| T1: HttpAdapter.Init + AppOptions field         | 1 interface signature + 1 struct field  | ✅ Granular |
| T2: Responder.BodyStream + Context.FormStream   | 1 interface method + 1 real impl + stubs | ✅ Granular (stubs are 1-liners, same file/commit as precedent) |
| T3: FormFile + ParseFormBody/MustFormBody        | 1 new file, 1 concept (multipart walk)  | ✅ Granular |
| T4: Root Parse/MustParse/FormFile exports        | 1 file, pure delegation                 | ✅ Granular |
| T5: Real streaming-proof test                    | 1 test file, 1 concern (prove no buffering) | ✅ Granular |
| T6: P2/P3 test cases                             | Same file as T5, 2 more cases            | ✅ Granular (no new production code) |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows                  | Status  |
| ---- | ------------------------ | --------------------------------- | ------- |
| T1   | None                    | (start of chain)                  | ✅ Match |
| T2   | T1                      | T1 → T2                           | ✅ Match |
| T3   | T2                      | T2 → T3                           | ✅ Match |
| T4   | T3                      | T3 → T4                           | ✅ Match |
| T5   | T4                      | T4 → T5                           | ✅ Match |
| T6   | T5                      | T5 → T6                           | ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified                          | Matrix Requires                                                | Task Says | Status |
| ---- | ------------------------------------------------------ | ------------------------------------------------------------------ | --------- | ------ |
| T1   | Adapter construction wiring (`HttpAdapter`/`AppOptions`) | Not in matrix (new layer, no observable behavior alone)         | none      | ✅ OK (proven end-to-end by T5) |
| T2   | `Responder`/`Context` (raw stream access)             | "Route/Pipe/Context isolados" row says unit, BUT `BodyStream`'s real behavior needs a real fasthttp stream, same exception `Body()` itself already has (no dedicated unit test, proven by consumers) | none      | ✅ OK (consistent with existing `Body()`/`Queries()` precedent) |
| T3   | `internal/validate` (new parsing/validation logic)    | "Route/Pipe/Context isolados" row → unit                          | unit      | ✅ OK |
| T4   | Root `gonest.go` wrappers                             | Not in matrix (pure delegation, same as every other root wrapper) | none      | ✅ OK (consistent with existing `ParseRestJsonBody` etc, which also have no dedicated wrapper-level test) |
| T5   | Real dispatch via Fiber (`internal/adapter/fiber`)    | "Dispatch de rota via Fiber real" row → integration               | integration | ✅ OK |
| T6   | Same layer as T5                                      | integration                                                        | integration | ✅ OK |

---

## Tools

No MCP/skill needed beyond what's already used this session (direct source reading for `fasthttp`/`fiber` already done in design.md's research pass -- no further Context7/web lookups anticipated unless something in `mime/multipart`'s stdlib behavior needs checking during T3).
