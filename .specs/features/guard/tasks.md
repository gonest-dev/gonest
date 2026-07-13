# Guard Tasks

**Design**: `.specs/features/guard/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ COMPLETE (T1-T4, todos evaluator PASS)

---

## Execution Plan

```
T1 (internal/guard: Guard/New/Handler) → T2 (Controller.Guards real type + OwnGuards) → T3 (Stage 2.5: gatedHandler) → T4 (root re-exports)
```

Fully sequential — unlike "Middleware" (which had `Controller.Use`/`Module.Use` in independently-touchable packages, safely `[P]`), this feature has no `Module.Guards` counterpart (out of scope, see spec.md), so there's no second package to parallelize against T2. Every task after T1 builds directly on the previous one's output in a chain of dependent, same-area changes.

---

## Task Breakdown

### T1: `internal/guard` — `Guard`/`New`/`Handler` ✅ DONE (evaluator: PASS, commit `4e8d03f`)

**What**: new package. `type Guard struct { handler func(ctx *httpctx.Context) bool }` (unexported field), `func New(fn func(*Guard)) *Guard` (runs `fn` IMMEDIATELY, not deferred — mirrors `internal/middleware.New`'s precedent, see design.md's Tech Decisions), `func (g *Guard) Handler(h func(ctx *httpctx.Context) bool)`, `func (g *Guard) HandlerFunc() func(ctx *httpctx.Context) bool` (`nil` if `Handler` never called).
**Where**: `internal/guard/guard.go`, `internal/guard/guard_test.go`
**Depends on**: None
**Reuses**: `httpctx.Context`, the exact "immediate execution" pattern from `internal/middleware/middleware.go` (T1 of "Middleware")
**Requirement**: GRD-01

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `New(fn)` runs `fn` immediately (test proves it, e.g. observable side-effect right after `New` returns)
- [x] `Handler(h)` stores `h`, `HandlerFunc()` returns exactly that stored function — call it in a test with a fake `ctx`, confirm the returned `bool` is genuinely the handler's own decision (test both `true` and `false` paths)
- [x] `HandlerFunc()` returns `nil` if `Handler` was never called
- [x] Gate check passes
- [x] Test count: 4+ (immediate execution, Handler/HandlerFunc round-trip returning true, round-trip returning false, nil zero-value)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(guard): add Guard core type`

---

### T2: `Controller.Guards` real type + `OwnGuards` ✅ DONE (evaluator: PASS, commit `4f29ed8`)

**What**: `internal/controller/controller.go`'s existing `Guards(items ...Middleware)` stub changes to `Guards(items ...*guard.Guard)` (real type from T1), storing into a field whose type changes from `[]Middleware` to `[]*guard.Guard`. Add `OwnGuards() []*guard.Guard` accessor (defensive copy, mirrors `OwnMiddleware`/`OwnRoutes`). Do NOT touch `Interceptors`/`Filters` — those keep the placeholder `Middleware struct{}` stub unchanged (still out of scope).
**Where**: `internal/controller/controller.go` (existing, extend), `internal/controller/controller_test.go` (existing — migrate any pre-existing test that called `Guards` with the OLD placeholder type, e.g. check `TestPipelineStubs_DoNotAffectObservableState` — it likely still calls `Guards(Middleware{})`, needs updating to `guard.New(nil)`)
**Depends on**: T1
**Reuses**: `guard.Guard` from T1, the exact `Use`/`OwnMiddleware` pattern this file already grew in "Middleware" (T2 of that feature)
**Requirement**: GRD-01, GRD-05 (storage/ordering part)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Controller.Guards(g1, g2, ...)` stores real `*guard.Guard` values in registration order
- [x] `OwnGuards()` returns a defensive copy — test proves mutating the returned slice does not affect internal state (same proof style as `TestOwnMiddleware_ReturnsCopyNotInternalSlice`)
- [x] `TestPipelineStubs_DoNotAffectObservableState` (or equivalent pre-existing test using the old `Guards` stub) migrated to real `*guard.Guard` values, still passes, and its assertions correctly reflect that `Interceptors`/`Filters` are STILL no-op while `Guards` (like `Use` before it) now genuinely stores something
- [x] Gate check passes
- [x] Test count: 3+ (Guards stores in order, OwnGuards defensive copy, pre-existing test migrated and still green)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): Controller.Guards stores real Guard, add OwnGuards accessor`

---

### T3: Stage 2.5 `gatedHandler` in `internal/app` ✅ DONE (evaluator: PASS, commit `2f7fbc4`)

**What**: `internal/app/app.go`'s `registerRoutes`/`composeHandler` (extended by "Middleware", T4 of that feature) currently composes the middleware chain around `route.HandlerFunc()` directly. Insert a new innermost layer, `gatedHandler`, that evaluates the route's controller's guards (in registration order, short-circuiting at the first `false` via `panic(exception.NewForbiddenException(nil))`) before calling `route.HandlerFunc()` — then feed `gatedHandler` (not `route.HandlerFunc()`) into the EXISTING, UNCHANGED middleware-composition loop. Extend the local `routableController` interface to also require `OwnGuards() []*guard.Guard` (already satisfied by `*controller.Controller` post-T2).
**Where**: `internal/app/app.go` (existing, extended — new import `internal/guard`, new import `internal/exception`), `internal/app/app_test.go` (existing — add tests)
**Depends on**: T2
**Reuses**: `guard.Guard`/`HandlerFunc` (T1), `Controller.OwnGuards()` (T2), `exception.NewForbiddenException` (already complete, "HttpException Core"), the EXISTING middleware-composition loop from "Middleware" (unmodified logic, only its input changes)
**Requirement**: GRD-02, GRD-03, GRD-04, GRD-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when** (all require REAL `app.Test` dispatch):
- [x] A single guard returning `true` lets the route Handler run
- [x] A single guard returning `false` produces `403 Forbidden` (`{"name":"ForbiddenException","message":"","details":null}`), route Handler does NOT run
- [x] A guard panicking with a custom `exception.Exception` (e.g. `exception.NewUnauthorizedException(nil)`) produces THAT exception's status/body, route Handler does NOT run
- [x] Multiple guards (2+) evaluate in registration order; if the FIRST returns `false`, the SECOND never runs (test proves this via an observable side-effect in the second guard that must NOT have happened — e.g. a flag/counter checked to be untouched)
- [x] A controller with BOTH `Use()` (middleware) and `Guards()` registered runs middleware BEFORE guards BEFORE the Handler — proven via explicit ordered-sequence assertion (reuse the order-recorder technique from "Middleware"'s own T4 tests)
- [x] A controller with zero `Guards()` calls behaves EXACTLY as it did before this feature (zero regression) — confirm an EXISTING pre-feature test (e.g. "Middleware"'s own T4 tests, or T9's `UserController` end-to-end example) still passes UNMODIFIED
- [x] A guard panicking with a NON-Exception value still produces the same generic 500 as any other panic (non-regression of the existing recovery behavior, not new to this feature but worth one explicit proof)
- [x] Gate check passes
- [x] Test count: 8+ (true proceeds, false→403, custom exception panic, short-circuit on 2+ guards, middleware-then-guard-then-handler ordering, zero-regression for no-Guards controllers, non-Exception guard panic still generic 500)

**Tests**: integration (real Fiber dispatch via `app.Test`)
**Gate**: full

**Commit**: `feat(app): add Guard evaluation (gatedHandler) to Stage 2.5, wraps route Handler before Middleware composition`

---

### T4: Root re-exports ✅ DONE (evaluator: PASS, commit `97ba40c` — nota menor: teste raiz confere status code mas não conteúdo do body pro nome da exception; comportamento já coberto a fundo em T3, não bloqueante)

**What**: root `gonest` package gets `Guard` (type alias) and `NewGuard` (`var NewGuard = guard.New` — plain alias, `New` is not generic, same idiom as `NewMiddleware`/`NewHttpException`).
**Where**: new file at repo root, `guard.go`, root-level test file
**Depends on**: T1, T2, T3
**Reuses**: exact `type X = pkg.X` / `var Y = pkg.Y` idiom already used at root (see `middleware.go` at repo root, most recent precedent)
**Requirement**: GRD-01 through GRD-05 (surface-level completion)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `gonest.NewGuard(fn)`, `gonest.Guard` resolve and work at root
- [x] INSIGHT.md's `AuthGuard` example (adapted per spec.md's Out of Scope: no `MustInject`, some other way to reach an `AuthService`-equivalent check — e.g. closing over an already-constructed value) reproduced through root aliases, attached via `controller.Guards(...)` through root `Controller`/`Module`/`NewApp` aliases, dispatched via real `app.Test`: missing/invalid check → exception response, valid check → route Handler runs
- [x] Gate check passes
- [x] Test count: 2+ (root-level smoke test for `NewGuard`/`Guard` resolving, the adapted `AuthGuard` reproduction end-to-end through root aliases)

**Tests**: unit (integration-style dispatch, root-package convention — see `middleware_test.go` at repo root for precedent)
**Gate**: quick

**Commit**: `feat(guard): re-export Guard/NewGuard at root`

---

## Parallel Execution Map

```
Fully sequential: T1 → T2 → T3 → T4
```

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: internal/guard core type | 1 arquivo novo, pacote novo pequeno e coeso | ✅ Granular |
| T2: Controller.Guards real type | 1 arquivo existente, mudança de assinatura + 1 accessor novo (mecânico, mesmo padrão de T2 da feature Middleware) | ✅ Granular |
| T3: Stage 2.5 gatedHandler | 1 arquivo existente, 1 responsabilidade nova coesa (mas denso em testes, é o coração da feature) | ✅ Granular |
| T4: Root re-exports | 1 arquivo novo, mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, sem HTTP real | unit | unit | ✅ OK |
| T2 | Builder isolado (Controller), sem dispatch real | unit | unit | ✅ OK |
| T3 | Dispatch de rota via Fiber real (composição de handler + recovery) | integration | integration | ✅ OK |
| T4 | Re-export + reprodução end-to-end via root | unit (com 1 caso integration-style embutido) | unit | ✅ OK — T3 já cobre a fundo |

Nenhuma violação.
