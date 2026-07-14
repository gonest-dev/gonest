# Filter Tasks

**Design**: `.specs/features/filter/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

```
T1 (internal/filter: Filter/New/Catch/HandlerFor)
        │
        ├──> T2 [P] (Controller.Filters real type + OwnFilters, delete placeholder Middleware struct)
        └──> T3 [P] (Module.Filters new method + OwnFilters)
                │
                ▼
        T4 (Stage 2.5: filteredHandler, outermost layer)
                │
                ▼
        T5 (root re-exports)
```

**Nota de paralelismo (L-003):** T2 (`internal/controller`) e T3 (`internal/module`) tocam pacotes DIFERENTES, sem tipo cruzado entre si — ambos só dependem de `internal/filter` (T1), nenhum depende do outro. Seguro rodar em paralelo (mesmo padrão já usado em T2/T3 de "Middleware").

---

## Task Breakdown

### T1: `internal/filter` — `Filter`/`New`/`Catch`/`HandlerFor` ✅ DONE (evaluator: PASS, commit `f0fe0af`)

**What**: new package. `type Filter struct { catches map[reflect.Type]reflect.Value }` (unexported field). `func New(fn func(*Filter)) *Filter` — runs `fn` IMMEDIATELY (AD-008, mirrors `middleware.New`/`guard.New`/`interceptor.New`). `func (f *Filter) Catch(exemplar any, handler any)` — `exemplar` identifies the exception type to match via `reflect.TypeOf(exemplar)`; `handler` must be `func(ctx *execution.Context, exc T)` where `T` is exactly that type — reflect-validate at registration time (mirror `internal/pipe/pipe.go`'s `isValidHandlerSignature` style), panic with a clear message if the signature doesn't match. `func (f *Filter) HandlerFor(excType reflect.Type) (reflect.Value, bool)` — exact map lookup.
**Where**: `internal/filter/filter.go`, `internal/filter/filter_test.go`
**Depends on**: None
**Reuses**: `execution.Context`, `internal/pipe/pipe.go`'s reflect-validation pattern (closest existing precedent)
**Requirement**: FLT-01, FLT-03, FLT-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `New(fn)` runs `fn` immediately (test proves it, mirrors T1 pattern from Middleware/Guard/Interceptor)
- [x] `Catch(exemplar, handler)` with a valid `func(ctx *execution.Context, exc T)` signature (T matching `reflect.TypeOf(exemplar)`) registers successfully, `HandlerFor(reflect.TypeOf(exemplar))` returns the stored handler, `ok=true`
- [x] `HandlerFor` with a type that was never `Catch`-registered returns `ok=false`
- [x] `Catch` with a handler whose signature does NOT match (wrong param count, wrong ctx type, wrong exception type, not a func at all) panics with a clear message at registration time — mirror `internal/pipe`'s test coverage shape (wrong param count, wrong types, wrong return, non-func)
- [x] A single `Filter` can `Catch` MULTIPLE distinct exemplar types, each retrievable independently via `HandlerFor`
- [x] The retrieved handler is genuinely callable via reflect (`handlerVal.Call([]reflect.Value{...})`) and the call reaches the original handler body with both `ctx` and the typed exception value intact
- [x] Gate check passes
- [x] Test count: 8+ (immediate execution, Catch+HandlerFor round-trip, HandlerFor miss, 4+ invalid-signature panic cases mirroring Pipe's coverage, multiple distinct Catch types, real reflect.Call round-trip proof)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(filter): add Filter core type with reflect-validated Catch`

---

### T2: `Controller.Filters` real type + `OwnFilters`, delete placeholder [P] ✅ DONE (evaluator: PASS, commit `dd5ad1b` — `TestPipelineStubs_DoNotAffectObservableState` deletado, sem lacuna real de cobertura)

**What**: `internal/controller/controller.go`'s `Filters(items ...Middleware)` (the LAST remaining stub using the T6 placeholder) changes to `Filters(items ...*filter.Filter)` (real type from T1). Field type changes accordingly. Add `OwnFilters() []*filter.Filter` accessor (defensive copy, mirror `OwnGuards`/`OwnInterceptors`/`OwnMiddleware`). Since `Use`/`Guards`/`Interceptors`/`Filters` ALL now use real types, the placeholder `type Middleware struct{}` declared in this file since T6 has ZERO remaining consumers — DELETE it (dead code, per design.md's Tech Decisions, not kept as a shim).
**Where**: `internal/controller/controller.go` (existing, extend + delete dead type), `internal/controller/controller_test.go` (existing — migrate `TestPipelineStubs_DoNotAffectObservableState`'s LAST remaining `Middleware{}` usages for `Filters`, confirm the test still compiles/passes with the placeholder type gone entirely)
**Depends on**: T1
**Reuses**: `filter.Filter` from T1, the exact `Guards`/`OwnGuards` pattern this file grew 3 times already
**Requirement**: FLT-01, FLT-05 (storage/ordering part)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Controller.Filters(f1, f2, ...)` stores real `*filter.Filter` values in registration order
- [x] `OwnFilters()` returns a defensive copy — test proves mutating the returned slice does not affect internal state (mirror `TestOwnInterceptors_ReturnsCopyNotInternalSlice`)
- [x] The placeholder `type Middleware struct{}` is COMPLETELY REMOVED from `controller.go` — confirm via grep/compile that nothing references it anymore anywhere in the file or its tests
- [x] `TestPipelineStubs_DoNotAffectObservableState` deleted (no unique multi-method interaction assertion lost — original proof premise no longer exists now that all 4 fields are real)
- [x] Gate check passes
- [x] Test count: 3+ (Filters stores in order, OwnFilters defensive copy, placeholder-type-removal doesn't break compilation)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(http): Controller.Filters stores real Filter, add OwnFilters, remove dead placeholder type`

---

### T3: `Module.Filters` (new) + `OwnFilters` [P] ✅ DONE (evaluator: PASS, commit `c65909d`)

**What**: `internal/module/module.go` gains a BRAND NEW method (does not exist yet): `func (m *Module) Filters(items ...*filter.Filter)`, storing into a new `filters []*filter.Filter` field. `func (m *Module) OwnFilters() []*filter.Filter` — defensive copy. Per design.md: ANY `*Module` can call `Filters` (Go can't restrict to root), but only the ROOT module's filters are actually consulted (T4's concern, not this task's) — mirrors `Module.Use`'s exact precedent from "Middleware".
**Where**: `internal/module/module.go` (existing, extend), `internal/module/module_test.go` (existing — add tests)
**Depends on**: T1
**Reuses**: `filter.Filter` from T1, the exact `Use`/`OwnMiddleware` pattern this file already grew in "Middleware"
**Requirement**: FLT-06 (storage part — root-only behavior is T4)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Module.Filters(f1, f2, ...)` stores real `*filter.Filter` values in registration order (new method)
- [x] `OwnFilters()` returns a defensive copy — test proves mutating the returned slice does not affect internal state (mirror `TestModule_OwnMiddleware_ReturnsCopyNotInternalSlice`)
- [x] Gate check passes
- [x] Test count: 2+ (Filters stores in order, OwnFilters defensive copy)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(module): add Module.Filters and OwnFilters accessor`

---

### T4: Stage 2.5 `filteredHandler` in `internal/app`

**What**: `internal/app/app.go`'s `registerRoutes` gains a NEW OUTERMOST layer, `filteredHandler`, wrapping the ENTIRE existing per-route chain (`withRoute` → `composedHandler` → `gatedHandler` → `interceptedHandler` → route Handler, all UNCHANGED). `filteredHandler` installs its own `defer`/`recover()`: on a recovered `exception.Exception`, checks the route's controller-level filters first (via `reflect.TypeOf`, exact match), then the root module's global filters; if a match is found, runs that Filter's handler and returns; if NOT found (or the panic isn't an `exception.Exception` at all), re-panics — letting the EXISTING adapter-level recover wrapper (`internal/adapter/fiber`, unchanged) apply the default `{name,message,details}` formatting exactly as it does today. Extend `routableController` with `OwnFilters() []*filter.Filter` (already satisfied by `*controller.Controller` post-T2).
**Where**: `internal/app/app.go` (existing, extended — new imports `internal/filter`, `reflect`), `internal/app/app_test.go` (existing — add tests)
**Depends on**: T2, T3
**Reuses**: `filter.Filter`/`Catch`/`HandlerFor` (T1), `Controller.OwnFilters()` (T2), `Module.OwnFilters()` (T3, consulted only on `root`), the EXISTING `withRoute`/`composedHandler`/`gatedHandler`/`interceptedHandler` chain (unmodified, just wrapped)
**Requirement**: FLT-01 through FLT-08 (all of them — this is where behavior actually happens)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when** (all require REAL `app.Test` dispatch):
- [ ] A controller-level Filter's `Catch`-registered handler runs when a route Handler panics with a matching concrete exception type — custom response (status+body) genuinely reaches the client
- [ ] An exception whose type does NOT match any registered `Catch` (controller or global) falls through to the EXISTING default `{name,message,details}` response, unchanged — explicit non-regression proof
- [ ] A global (root module) Filter's handler applies to a route whose OWN controller has zero `Filters()` of its own
- [ ] When BOTH a controller-level and a global Filter register `Catch` for the SAME exception type, the CONTROLLER-level handler wins (precedence proven — e.g. two distinguishable response bodies, confirm which one the client actually receives)
- [ ] A Filter can `Catch` a panic that originated from ANYWHERE in the dispatch chain, not just the route Handler — test with a panic from inside a Guard or Middleware, confirm the Filter still catches it (proves `filteredHandler` is genuinely the outermost layer)
- [ ] A non-`exception.Exception` panic (bare error, etc.) is NOT intercepted by any Filter (no `Catch` lookup even attempted) — still produces the existing generic 500, unchanged
- [ ] A controller with ZERO `Filters()` (and no global filters either) behaves EXACTLY as it did before this feature — an EXISTING pre-feature test (e.g. "Guard"'s or "Interceptor"'s own tests, or T9's `UserController` end-to-end) still passes UNMODIFIED
- [ ] Gate check passes
- [ ] Test count: 10+ (controller-level catch works, uncaught falls through to default, global-applies-without-controller-opt-in, controller-overrides-global precedence, catches panic from any pipeline stage not just Handler, non-Exception panic unaffected, zero-regression)

**Tests**: integration (real Fiber dispatch via `app.Test`)
**Gate**: full

**Commit**: `feat(app): add Filter interception (filteredHandler) as outermost Stage 2.5 layer, controller-overrides-global precedence`

---

### T5: Root re-exports

**What**: root `gonest` package gets `Filter` (type alias) and `NewFilter` (`var NewFilter = filter.New`, plain alias, `New` is not generic). ADD to the existing consolidated `gonest.go`/`gonest_test.go` (per AD-009 — do NOT create a new root-level file).
**Where**: `gonest.go` (existing, add a new `// Filter (Filter feature)` section), `gonest_test.go` (existing, add tests)
**Depends on**: T1, T2, T3, T4
**Reuses**: exact `type X = pkg.X` / `var Y = pkg.Y` idiom already used in `gonest.go` for every other pipeline-stage type
**Requirement**: FLT-01 through FLT-08 (surface-level completion)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `gonest.NewFilter(fn)`, `gonest.Filter` resolve and work at root
- [ ] INSIGHT.md's `FooExampleFilter` example (adapted per spec.md's Edge Cases: plain `int` status literal `418`, not the nonexistent `gonest.HttpStatusTeapot`) reproduced through root aliases, attached via `controller.Filters(...)` through root `Controller`/`Module`/`NewApp` aliases, dispatched via real `app.Test`: caught type → custom response, uncaught type → default response
- [ ] Gate check passes
- [ ] Test count: 2+ (root-level smoke test for `NewFilter`/`Filter` resolving, the adapted `FooExampleFilter` reproduction end-to-end through root aliases)

**Tests**: unit (integration-style dispatch, root-package convention)
**Gate**: quick

**Commit**: `feat(filter): re-export Filter/NewFilter at root`

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
| T1: internal/filter core | 1 arquivo novo, pacote novo, reflect + validação | ✅ Granular |
| T2: Controller.Filters + delete placeholder | 1 arquivo existente, mecânico + cleanup de dead code | ✅ Granular |
| T3: Module.Filters novo | 1 arquivo existente, mecânico | ✅ Granular |
| T4: Stage 2.5 filteredHandler | 1 arquivo existente, 1 responsabilidade nova coesa (denso em testes, camada mais externa nova) | ✅ Granular |
| T5: Root re-exports | Adiciona seção em `gonest.go`/`gonest_test.go` existentes (AD-009), mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Tipo isolado, sem HTTP real | unit | unit | ✅ OK |
| T2 | Builder isolado (Controller), sem dispatch real | unit | unit | ✅ OK |
| T3 | Builder isolado (Module), sem dispatch real | unit | unit | ✅ OK |
| T4 | Dispatch de rota via Fiber real (recover seletivo + precedência) | integration | integration | ✅ OK |
| T5 | Re-export + reprodução end-to-end via root | unit (com 1 caso integration-style embutido) | unit | ✅ OK |

Nenhuma violação.
