# Middleware Tasks

**Design**: `.specs/features/middleware/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

```
T1 (internal/middleware: Next/Middleware/New/Handler)
        │
        ├──> T2 [P] (Controller.Use real type + OwnMiddleware)
        └──> T3 [P] (Module.Use new method + OwnMiddleware)
                │
                ▼
        T4 (Stage 2.5 composition in internal/app)
                │
                ▼
        T5 (root re-exports)
```

**Nota de paralelismo (L-003):** T2 (`internal/controller`) e T3 (`internal/module`) tocam pacotes DIFERENTES, sem tipo cruzado entre si — ambos só dependem de `internal/middleware` (T1), nenhum depende do outro. Seguro rodar em paralelo.

---

## Task Breakdown

### T1: `internal/middleware` — `Next`/`Middleware`/`New`/`Handler` ✅ DONE (evaluator: PASS-WITH-NOTE, commit `804f440` — nota: self-report do dev alegou 5 testes, real são 4, satisfaz o mínimo de 4+ mesmo assim)

**What**: new package. `type Next func(ctx *httpctx.Context)`, `type Middleware struct { handler func(ctx *httpctx.Context, next Next) }`, `func New(fn func(*Middleware)) *Middleware` (runs `fn` IMMEDIATELY, not deferred — see design.md's Tech Decisions), `func (m *Middleware) Handler(h func(ctx *httpctx.Context, next Next))`, `func (m *Middleware) HandlerFunc() func(ctx *httpctx.Context, next Next)`.
**Where**: `internal/middleware/middleware.go`, `internal/middleware/middleware_test.go`
**Depends on**: None
**Reuses**: `httpctx.Context` (T2 of "Controller & Route Registration")
**Requirement**: MW-01

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `New(fn)` runs `fn` immediately (test proves it, e.g. a side-effect inside `fn` is observable right after `New` returns, before any `Declare`-like call — there is none to call)
- [x] `Handler(h)` stores `h`, `HandlerFunc()` returns exactly that stored function (identity-callable — call it in a test, confirm `ctx`/`next` both reach the handler body correctly)
- [x] `HandlerFunc()` returns `nil` if `Handler` was never called (mirrors `Pipe.HandlerFunc()`'s zero-value contract — confirm via test)
- [x] A `Next` value can be constructed directly from a plain `func(ctx *httpctx.Context)` (proves the type-identity claim in design.md — a route Handler's own shape is directly assignable to `Next` with zero adapter code)
- [x] Gate check passes
- [x] Test count: 4+ (immediate execution, Handler/HandlerFunc round-trip, nil zero-value, Next type-identity)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(middleware): add Next/Middleware core types`

---

### T2: `Controller.Use` real type + `OwnMiddleware` [P] ✅ DONE (evaluator: PASS, commit `39b969e`)

**What**: `internal/controller/controller.go`'s existing `Use(items ...Middleware)` stub (T6, placeholder `Middleware struct{}`) changes signature to `Use(items ...*middleware.Middleware)` (real type from T1), storing them for real (already stores into `c.middleware []Middleware` today — change that field's type to `[]*middleware.Middleware`). Add `OwnMiddleware() []*middleware.Middleware` accessor (defensive copy, mirrors `OwnRoutes`). Do NOT touch `Guards`/`Interceptors`/`Filters` — those keep the existing placeholder `Middleware struct{}` stub type unchanged (still out of scope, separate future features).
**Where**: `internal/controller/controller.go` (existing, extend), `internal/controller/controller_test.go` (existing — update any test currently passing the OLD placeholder type to `Use`, add new tests)
**Depends on**: T1
**Reuses**: `middleware.Middleware` from T1, the existing `OwnRoutes`/`Own*` defensive-copy pattern already in this file
**Requirement**: MW-01, MW-04, MW-05 (controller-level parts)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Controller.Use(m1, m2, ...)` stores real `*middleware.Middleware` values (not the placeholder stub) in registration order
- [x] `OwnMiddleware()` returns a defensive copy — test proves mutating the returned slice does not affect internal state (same proof style as `TestOwnRoutes_ReturnsCopyNotInternalSlice`)
- [x] Any PRE-EXISTING test in `controller_test.go` that called `Use` with the OLD placeholder type is updated to use real `*middleware.Middleware` values and still passes (check `TestPipelineStubs_DoNotAffectObservableState` from T6 specifically — it likely calls `Use` with the old stub, needs updating)
- [x] Gate check passes
- [x] Test count: 3+ (Use stores in order, OwnMiddleware defensive copy, pre-existing test migrated and still green)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): Controller.Use stores real Middleware, add OwnMiddleware accessor`

---

### T3: `Module.Use` (new) + `OwnMiddleware` [P] ✅ DONE (evaluator: PASS, commit `bb4558f`)

**What**: `internal/module/module.go` gains a NEW method (did not exist before this feature): `func (m *Module) Use(items ...*middleware.Middleware)`, storing into a new `middleware []*middleware.Middleware` field. Add `OwnMiddleware() []*middleware.Middleware` accessor (defensive copy, mirrors `OwnProviders`/`OwnControllers`). Per design.md: ANY `*Module` can call `Use` (no type-level restriction to "root only" — Go can't express that), but this task only adds the storage/accessor; whether it's actually CONSULTED only for the root module is T4's concern, not this task's.
**Where**: `internal/module/module.go` (existing, extend), `internal/module/module_test.go` (existing — add tests)
**Depends on**: T1
**Reuses**: `middleware.Middleware` from T1, the exact `Own*` defensive-copy pattern already used for `OwnProviders`/`OwnControllers` in this file
**Requirement**: MW-06 (storage part — actual "root-only" behavior is T4)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Module.Use(m1, m2, ...)` stores real `*middleware.Middleware` values in registration order (new method, no pre-existing behavior to preserve)
- [x] `OwnMiddleware()` returns a defensive copy — test proves mutating the returned slice does not affect internal state (same proof style as `TestModule_OwnProviders_ReturnsCopyNotInternalSlice`)
- [x] Gate check passes
- [x] Test count: 2+ (Use stores in order, OwnMiddleware defensive copy)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(module): add Module.Use and OwnMiddleware accessor`

---

### T4: Stage 2.5 composition in `internal/app` ✅ DONE (evaluator: PASS, commit `d349fa1` — SPEC_DEVIATION confirmado genuíno: `Header()`/`SetHeader()` são stores diferentes, request vs response, ver relato do dev)

**What**: `internal/app/app.go`'s `registerRoutes` (Stage 2.5, T8) currently registers each route's bare `HandlerFunc()` with the adapter. Extend it to compose a middleware chain per route: `root.OwnMiddleware()` (global, ALWAYS consulted regardless of which module a controller belongs to — this is where "root module only" actually takes effect, per design.md) prepended to that route's OWN controller's `OwnMiddleware()` (via the existing `routableController` local interface, extended to require `OwnMiddleware() []*middleware.Middleware` — already satisfied by `*controller.Controller` post-T2), composed outward so global middleware ends up OUTERMOST (runs first). Register the COMPOSED `func(ctx *httpctx.Context)` with `adapter.RegisterRoute`, not the bare route Handler.
**Where**: `internal/app/app.go` (existing `registerRoutes`, extended), `internal/app/app_test.go` (existing — add tests)
**Depends on**: T2, T3
**Reuses**: `root` (the SAME `*module.Module` parameter `registerRoutes`/`NewApp` already receive, no new parameter), `routableController` (existing local interface, extended), `middleware.Next`'s type-identity with a route Handler's shape (T1)
**Requirement**: MW-02, MW-03, MW-04, MW-05, MW-06, MW-07, MW-08

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] A route with a SINGLE controller-level middleware runs it before the route Handler, via a real `app.Test` dispatch (not a unit-level check of the composition function alone)
- [x] A middleware calling `next(ctx)` continues the chain; a middleware NOT calling `next(ctx)` short-circuits (route Handler never runs) — both proven via real dispatch
- [x] Multiple controller-level middleware run in registration order (test with 2+, each appending a distinguishable marker — implemented via real `net/http.Response` header inspection instead of the originally-suggested read-then-append technique, see SPEC_DEVIATION note above)
- [x] A middleware mutating `ctx` before calling `next` is visible to a LATER middleware and the route Handler (same `*httpctx.Context` instance, not a copy) — proven via `ctx.WithRoute`/`ctx.Route()` repurposed as an any-carrier (isolated from any test using `MustParam`'s Pipe-via-Route lookup, confirmed by evaluator)
- [x] Root-module `Use()` middleware runs for EVERY route in the app, including a controller that itself has ZERO `Use()` calls — proven via 2 controllers, only one with its own middleware, both hit by requests, both show the global marker
- [x] Global middleware runs BEFORE controller-level middleware — proven via explicit ordered-slice assertion
- [x] An app with ZERO `Use()` calls anywhere behaves EXACTLY as it did before this feature (zero regression) — T9's `UserController` end-to-end test confirmed unmodified and still passing
- [x] A panicking middleware is caught by the SAME existing recover wrapper (`internal/fiberapp`) and produces the correct Exception/generic-500 response, exactly like a panicking route Handler already does — proven via real dispatch, panicking with a built-in `exception.Exception` from inside a middleware
- [x] Gate check passes
- [x] Test count: 8+ (single mw runs before handler, next continues, missing next short-circuits, order among 2+ controller mw, ctx mutation visible downstream, global-applies-to-controller-without-own-mw, global-before-local ordering, zero-regression for no-Use apps, middleware panic uses existing recovery)

**Tests**: integration (real Fiber dispatch via `app.Test`, per TESTING.md's established pattern for anything touching the actual request/response cycle)
**Gate**: full

**Commit**: `feat(app): compose Middleware chain (global + controller) in Stage 2.5 route registration`

---

### T5: Root re-exports

**What**: root `gonest` package gets `Middleware`, `Next` (type aliases) and `NewMiddleware` (`var NewMiddleware = middleware.New` — plain alias, `New` is not generic, no wrapper needed, same idiom as `NewHttpException`/the 5 built-in exception constructors from the previous feature).
**Where**: new file at repo root, e.g. `middleware.go`, root-level test file
**Depends on**: T1, T2, T3, T4
**Reuses**: exact `type X = pkg.X` / `var Y = pkg.Y` idiom already used at root (see `exception.go` at repo root for the most recent precedent of this exact pattern for a non-generic constructor)
**Requirement**: MW-01 through MW-08 (surface-level completion)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `gonest.NewMiddleware(fn)`, `gonest.Middleware`, `gonest.Next` all resolve and work at root
- [ ] INSIGHT.md's `RequestIdMiddleware` example (UUID generation, `ctx.SetHeader`, `next(ctx)`) reproduced verbatim via root aliases, attached via `controller.Use(RequestIdMiddleware)` through the root `Controller`/`Module` aliases, dispatched via a real `app.Test` request, confirms the header lands correctly — this is the feature's own Independent Test from spec.md, run one more time through the ROOT package specifically (not `internal/*` directly) to prove the public API surface is real, not just the internal implementation
- [ ] Gate check passes
- [ ] Test count: 2+ (root-level smoke test for `NewMiddleware`/`Middleware`/`Next` resolving, the INSIGHT.md `RequestIdMiddleware` reproduction end-to-end through root aliases)

**Tests**: unit (the INSIGHT.md reproduction is itself an integration-style dispatch, but lives in root-package test file per this codebase's established "root smoke test" convention — see `exception_test.go`/`app_test.go` at repo root for precedent)

**Gate check**: quick (T4 already proved the underlying composition logic thoroughly via `internal/app`'s integration tests — T5 just proves the public alias surface reaches the same behavior, doesn't need to re-prove every edge case)

**Commit**: `feat(middleware): re-export Middleware/Next/NewMiddleware at root`

---

## Parallel Execution Map

```
T1 (solo)
  │
  ├──> T2 [P] (internal/controller)
  └──> T3 [P] (internal/module)
          │
          ▼
        T4 (internal/app, depends on both T2 and T3)
          │
          ▼
        T5 (root)
```

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: internal/middleware core types | 1 arquivo novo, pacote novo pequeno e coeso | ✅ Granular |
| T2: Controller.Use real type | 1 arquivo existente, mudança de assinatura + 1 accessor novo | ✅ Granular |
| T3: Module.Use novo | 1 arquivo existente, 1 método novo + 1 accessor novo | ✅ Granular |
| T4: Stage 2.5 composição | 1 arquivo existente, 1 responsabilidade nova coesa (mas com bastante superfície de teste, é o coração da feature) | ✅ Granular — mais denso que as outras tasks mas ainda 1 arquivo, 1 função estendida |
| T5: Root re-exports | 1 arquivo novo, mecânico | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| --- | --- | --- | --- |
| T1 | None | None | ✅ Match |
| T2 | T1 | T1 → T2 [P] | ✅ Match |
| T3 | T1 | T1 → T3 [P] | ✅ Match |
| T4 | T2, T3 | T2,T3 → T4 | ✅ Match |
| T5 | T1, T2, T3, T4 | T4 → T5 | ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, sem HTTP real | unit | unit | ✅ OK |
| T2 | Builder isolado (Controller), sem dispatch real | unit | unit | ✅ OK |
| T3 | Builder isolado (Module), sem dispatch real | unit | unit | ✅ OK |
| T4 | Dispatch de rota via Fiber real (composição de handler) | integration | integration | ✅ OK |
| T5 | Re-export + 1 reprodução end-to-end via root | unit (com 1 caso integration-style embutido) | unit | ✅ OK — T4 já cobre a fundo, T5 só prova a superfície pública |

Nenhuma violação.
