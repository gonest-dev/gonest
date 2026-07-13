# App Bootstrap & Listen Tasks

**Design**: `.specs/features/app-bootstrap-listen/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

### Sequential (all touch `internal/app/app.go` and/or its direct dependents — no real parallelism per L-003)

```
T1 (AppOptions/LogLevel/OnListen types) → T2 (NewApp/MustNewApp + opts) → T3 (HttpAdapter.Listen + FiberApp) → T4 (App.MustListen) → T5 (root re-exports) → T6 (real end-to-end test)
```

---

## Task Breakdown

### T1: `AppOptions`/`LogLevel`/`OnListen` types ✅ DONE (evaluator: PASS, commit `691c653`)

**What**: `internal/app/options.go` (new file) — `AppOptions{BufferLogs bool; LogLevels []LogLevel}`, `LogLevel int` enum (`LogLevelError`/`LogLevelWarn`/`LogLevelLog`/`LogLevelDebug`/`LogLevelVerbose`) with `String()` (same debug-friendly pattern as `HttpMethod.String()`), `OnListen func()`.
**Where**: `internal/app/options.go`, `internal/app/options_test.go`
**Depends on**: None
**Reuses**: `HttpMethod`/`HttpMethod.String()` pattern from `internal/route/method.go` (T1 of prior feature)
**Requirement**: BOOT-06, BOOT-07, BOOT-08

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `AppOptions{}` zero value compiles and is a valid struct
- [x] All 5 `LogLevel` values + `String()` return distinct, readable strings
- [x] `OnListen` is a defined `func()` type (not a bare `func()` inline everywhere), nil-able
- [x] Gate check passes
- [x] Test count: 6+ (1 per LogLevel String() value + zero-value AppOptions + OnListen nil-safety)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(app): add AppOptions/LogLevel/OnListen types`

---

### T2: `NewApp`/`MustNewApp` accept `AppOptions` (breaking signature change) ✅ DONE (evaluator: PASS, commit `e080552`)

**What**: `NewApp[T, PT](root *module.Module, opts AppOptions) (*App, error)` and `MustNewApp[T, PT](root *module.Module, opts AppOptions) *App` — `opts` becomes a required 2nd positional param (ground truth: INSIGHT.md's call sites always pass it, even as `AppOptions{}`). `App` struct gains an `opts AppOptions` field, stored, not otherwise read yet. **Migration**: every existing T8/T9 call site in `internal/app/app_test.go` (and root `app.go`'s wrapper, done in T5) that calls `NewApp[...](root)`/`MustNewApp[...](root)` needs `AppOptions{}` added as the 2nd arg — mechanical, do not skip any.
**Where**: `internal/app/app.go` (existing `App` struct + `NewApp`/`MustNewApp`, extended), `internal/app/app_test.go` (existing call sites updated)
**Depends on**: T1 (needs `AppOptions` type)
**Reuses**: existing Stage 1-3 + 2.5 orchestration, unchanged internally
**Requirement**: BOOT-01, BOOT-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `NewApp[T, PT](root, AppOptions{})` bootstraps identically to how `NewApp[T, PT](root)` did before this task (same DI graph resolved, same routes registered) — zero behavior regression
- [x] `NewApp[T, PT](root, AppOptions{BufferLogs: true, LogLevels: [...]})` also bootstraps successfully, `opts` field on `*App` reflects what was passed (add an unexported-but-testable path, or a same-package test, to confirm storage — no public getter required yet since nothing public reads it)
- [x] Every pre-existing test in `internal/app/app_test.go` that calls `NewApp`/`MustNewApp` is updated to pass `AppOptions{}` (or a non-zero value where the test is specifically about opts) and still passes
- [x] Gate check passes
- [x] Test count: 12+ (all pre-existing `internal/app` tests still passing post-migration, plus 2+ new: bootstrap with zero-value opts, bootstrap with non-zero opts)

**Tests**: unit
**Gate**: full (touches every existing `internal/app` test via the signature migration)

**Commit**: `feat(app): NewApp/MustNewApp accept AppOptions (breaking)`

---

### T3: `HttpAdapter.Listen` gains `onListen` hook + Fiber wiring ✅ DONE (evaluator: PASS, commit `997e238`)

**What**: `HttpAdapter.Listen(addr string, onListen func()) error` (was `Listen(addr string) error` in T8 — never called by any shipped code path yet, safe to change directly per design.md's Tech Decisions). `*fiberapp.FiberApp.Listen` implements it: registers `onListen` via Fiber v3's `f.app.Hooks().OnListen(...)` (only if `onListen != nil`) BEFORE calling the blocking `f.app.Listen(addr)`. **Before implementing**: re-verify Fiber v3's exact `Hooks().OnListen` callback signature (Knowledge Verification Chain — check vendored source under the Go module cache, or Context7 `gofiber/fiber` v3 docs; design.md flags this as unverified this session).
**Where**: `internal/app/app.go` (`HttpAdapter` interface signature), `internal/fiberapp/fiberapp.go` (`Listen` implementation), `internal/fiberapp/fiberapp_test.go`
**Depends on**: T1 (uses the same `func()` shape `OnListen` wraps, though `HttpAdapter.Listen` itself takes a plain `func()`, not `app.OnListen` — keeps `internal/fiberapp` decoupled from `internal/app`'s `OnListen` type, see design.md's layering)
**Reuses**: T7's existing `FiberApp`/`fiberResponder` machinery, untouched
**Requirement**: BOOT-02, BOOT-03, BOOT-04

**Tools**:
- MCP: Context7 (`gofiber/fiber` v3, `Hooks().OnListen` signature) if local vendored source is inconclusive
- Skill: NONE

**Done when**:
- [x] `HttpAdapter` interface's `Listen` signature updated repo-wide (compiles)
- [x] `FiberApp.Listen(addr, onListen)` with non-nil `onListen` fires the callback exactly once, before the call blocks for good — test proves ordering (e.g. callback appends to a slice/sets a flag BEFORE a concurrent goroutine can observe the block having "returned", using a channel/waitgroup to prove happens-before, not a timing-based sleep)
- [x] `FiberApp.Listen(addr, nil)` does not panic, blocks normally, no hook registered
- [x] Gate check passes
- [x] Test count: 3+ (callback fires once, ordering proven, nil-onListen safe)

**Tests**: integration (real Fiber `Listen`/bind — needs a real ephemeral port, e.g. `:0` or a freed test port; do NOT leave a listener running past the test, close/shutdown cleanly)
**Gate**: full

**Commit**: `feat(http): extend HttpAdapter.Listen with onListen hook, wire Fiber Hooks().OnListen`

---

### T4: `App.MustListen` ✅ DONE (evaluator: PASS, commit `28778a9`)

**What**: `func (a *App) MustListen(addr string, onListen OnListen)` — calls `a.adapter.Listen(addr, onListenFunc)` where `onListenFunc` is `nil` if `onListen == nil`, else the wrapped `func()`. Panics (clear message, includes `addr`) if `Listen` returns an error.
**Where**: `internal/app/app.go` (extended), `internal/app/app_test.go`
**Depends on**: T2 (needs `App.opts` field / extended `NewApp` in place), T3 (needs `HttpAdapter.Listen`'s new signature)
**Reuses**: `HttpAdapter.Listen` from T3, `OnListen` type from T1
**Requirement**: BOOT-02, BOOT-03, BOOT-04, BOOT-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `app.MustListen(addr, gonest.OnListen(fn))` calls `fn` exactly once and blocks (test runs it in a goroutine, waits on a channel the callback closes, then confirms a real request against `addr` succeeds)
- [x] `app.MustListen(addr, nil)` blocks without panicking or calling anything
- [x] `app.MustListen` with an adapter that returns an error from `Listen` (fake/spy `HttpAdapter` in test, not necessarily real Fiber) panics with a message containing the addr and the underlying error
- [x] Gate check passes
- [x] Test count: 3+ (fires callback + blocks, nil-safe, panics on Listen error)

**Tests**: unit (fake `HttpAdapter` spy for the panic/ordering proof) + integration (1 test using the real `fiberapp.FiberApp` to prove the whole chain, may overlap with T6 — keep this one minimal, T6 owns the full `net/http`-client-dial proof)
**Gate**: full

**Commit**: `feat(app): add MustListen, wires OnListen through HttpAdapter.Listen`

---

### T5: Root re-exports (`AppOptions`, `LogLevel`, `OnListen`, `NewApp`/`MustNewApp`/`MustListen`)

**What**: update root `app.go` (existing `NewApp`/`MustNewApp` wrappers — now need the `opts AppOptions` param threaded through) and add `AppOptions`/`LogLevel`/`OnListen` type aliases (new file, e.g. root `options.go`, mirroring `internal/app/options.go`). `App.MustListen` is already promoted automatically since root `App = app.App` is a true type alias (confirmed pattern from T6's evaluator finding on `Controller`) — verify this holds, don't assume.
**Where**: `app.go` (root, existing), `options.go` (root, new)
**Depends on**: T1, T2, T3, T4 (needs everything it re-exports to exist)
**Reuses**: exact idiom already used for `MustInject`/`MustParam`/`NewApp`/`MustNewApp` at root (see `param.go`, existing `app.go`)
**Requirement**: BOOT-01 through BOOT-08 (surface-level completion)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `gonest.AppOptions`, `gonest.LogLevel` + its 5 constants, `gonest.OnListen` all resolve at root package
- [ ] `gonest.NewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})` compiles and works (exact INSIGHT.md call shape)
- [ ] `gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})` compiles and works
- [ ] `app.MustListen(addr, gonest.OnListen(fn))` and `app.MustListen(addr, nil)` both compile and work through the root alias, no extra wrapper needed for `MustListen` itself (confirm type-alias promotion holds; if it doesn't, add the minimal wrapper needed and note why in the report)
- [ ] Gate check passes
- [ ] Test count: 2+ (root-level smoke test compiling/using the exact INSIGHT.md call shapes for both `NewApp` and `MustListen`)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(app): re-export AppOptions/LogLevel/OnListen/MustListen at root`

---

### T6: Real end-to-end — actual `net/http` client against a real bound port

**What**: extend `internal/app/app_test.go` (or a new `app_e2e_test.go` in the same package) with a test that: bootstraps a real app (reuse T9's `UserController`/`AppModule` example), starts `MustListen` in a goroutine against an ephemeral port (`:0` — capture the actual bound port via the adapter or `OnListen`'s callback timing, since `:0` means "OS picks a free port"), confirms `OnListen`'s callback fires (proving BOOT-03 end-to-end, not just at the Fiber-adapter-unit level from T3), then dispatches a REAL `net/http.Client` request (not `app.Test`) against the bound address and confirms a correct response. Shuts down cleanly at test end (no leaked listener/goroutine across test runs — check how to stop a Fiber v3 app cleanly, likely `app.Shutdown()` or similar on the underlying `*fiber.App`, reachable via `FiberApp`'s existing `FiberApp()` accessor from T7).
**Where**: `internal/app/app_test.go` (or new file in same package)
**Depends on**: T5 (needs the full public surface wired)
**Reuses**: T9's `UserController`/`UserService`/`AppModule` example, T7's `FiberApp()` accessor for cleanup
**Requirement**: Success Criteria of spec.md

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Real `net/http.Client` request against the actually-bound port gets a correct status + body (reuse one of T9's 5 routes, e.g. `GET /user/:user_id` or List)
- [ ] `OnListen` callback observably fires before the test proceeds to dial (proven via channel/waitgroup synchronization, not a sleep)
- [ ] No leaked listener/goroutine after the test completes (clean shutdown, verified by the test not hanging and, ideally, a second bind to the same ephemeral-then-freed port succeeding in a follow-up assertion or by explicit `t.Cleanup`)
- [ ] Gate check passes
- [ ] Test count: 1+ (this is inherently a single cohesive end-to-end scenario; may include 2-3 sub-assertions in one test function, consistent with how T9 structured its 5-route test)

**Tests**: integration
**Gate**: full

**Commit**: `test(app): add real net/http end-to-end test for MustListen/OnListen`

---

## Parallel Execution Map

```
Fully sequential: T1 → T2 → T3 → T4 → T5 → T6
```

**Nota de paralelismo (L-003):** todas as tasks tocam `internal/app/app.go` (diretamente ou via dependência de tipo recém-criado nela) ou seus dependentes imediatos (`internal/fiberapp` em T3, root `app.go` em T5) — sem paralelismo real disponível nessa feature, diferente da anterior onde T3/T4 tinham pacotes seguramente isolados.

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: AppOptions/LogLevel/OnListen | 1 arquivo novo, 3 tipos pequenos coesos | ✅ Granular |
| T2: NewApp/MustNewApp + opts | 1 arquivo existente + migração mecânica de call sites | ✅ Granular |
| T3: HttpAdapter.Listen + Fiber | 2 arquivos (interface + impl), 1 responsabilidade | ✅ Granular |
| T4: App.MustListen | 1 arquivo existente, 1 método novo | ✅ Granular |
| T5: Root re-exports | 2 arquivos (1 existente, 1 novo), mecânico | ✅ Granular |
| T6: Real e2e test | 1 arquivo de teste | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| --- | --- | --- | --- |
| T1 | None | None | ✅ Match |
| T2 | T1 | T1 → T2 | ✅ Match |
| T3 | T1 | Sequential chain includes T2 → T3 (file-contention reason, not a data dependency on T2's opts) | ✅ Match (see note below) |
| T4 | T2, T3 | T3 → T4 | ✅ Match |
| T5 | T1, T2, T3, T4 | T4 → T5 | ✅ Match |
| T6 | T5 | T5 → T6 | ✅ Match |

**Note on T3:** T3's task body lists only T1 as a data dependency (it doesn't need `AppOptions`/`NewApp`'s new signature to do its own work), but the execution plan still sequences it after T2 because both edit `internal/app/app.go`'s `HttpAdapter` interface declaration region — running them concurrently risks the same same-file collision class documented in L-003/L-007. Sequencing here is a coordination choice, not a true data dependency; noted explicitly so a future re-read doesn't mistake it for one.

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Config types, no HTTP | unit | unit | ✅ OK |
| T2 | `NewApp` orchestration (DI graph, no real HTTP dispatch) | unit | unit | ✅ OK |
| T3 | Dispatch of rota via Fiber real (bind/listen) | integration | integration | ✅ OK |
| T4 | Mix: panic/ordering logic (unit-testable via fake adapter) + 1 real-adapter proof | unit + integration | unit + integration | ✅ OK |
| T5 | Re-export surface, no new logic | unit | unit | ✅ OK |
| T6 | Real HTTP dispatch via bound port | integration | integration | ✅ OK |

Nenhuma violação. **Nota:** `.specs/codebase/TESTING.md`'s Test Coverage Matrix will need a new row for "bind/Listen real (`internal/fiberapp`, `HttpAdapter.Listen`)" once T3 lands — flagged for the T3 developer/evaluator to update TESTING.md as part of that task's own housekeeping (small addition, not worth a separate task).
