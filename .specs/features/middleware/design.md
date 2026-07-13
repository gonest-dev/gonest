# Middleware Design

**Spec**: `.specs/features/middleware/spec.md`

## Architecture Overview

```
internal/middleware (new package, AD-004 pattern)
        │
        ├── Next               -- type Next func(ctx *httpctx.Context)
        └── Middleware          -- New(fn) runs fn IMMEDIATELY (like route.New, not deferred
                                    -- see Tech Decisions for why), Handler(func(ctx, next))

internal/controller (existing, T6) ──imports──> internal/middleware
        Controller.Use(items ...*middleware.Middleware)   -- was Use(items ...Middleware) stub,
                                                              BREAKING signature change, real storage now
        Controller.OwnMiddleware() []*middleware.Middleware -- new accessor, defensive copy

internal/module (existing) ──imports──> internal/middleware
        Module.Use(items ...*middleware.Middleware)        -- NEW method, does not exist yet
        Module.OwnMiddleware() []*middleware.Middleware    -- new accessor, defensive copy

internal/app's Stage 2.5 (existing, registerRoutes, T8) ──extended──
        for each route: compose chain = root.OwnMiddleware() (global) + controller.OwnMiddleware() (local)
        wrap route.HandlerFunc() as the innermost Next, build outward
        register the COMPOSED handler with adapter.RegisterRoute (not the bare route Handler anymore)
```

No change to `internal/fiberapp`/`HttpAdapter`/panic-recovery — the composed handler is still just a `func(ctx *httpctx.Context)`, indistinguishable from a bare route Handler to everything downstream of Stage 2.5. Middleware panics are caught by the SAME recover wrapper as before (spec.md's Edge Cases).

---

## Components

### `Next` / `Middleware` (new package)

- **Purpose**: the reusable request-observation/mutation unit — `Handler(ctx, next)` continuation-passing shape, mirrors Express/Nest middleware.
- **Location**: `internal/middleware/middleware.go` (new file, new package)
- **Interfaces**:
  - `type Next func(ctx *httpctx.Context)`
  - `type Middleware struct { handler func(ctx *httpctx.Context, next Next) }`
  - `func New(fn func(*Middleware)) *Middleware` — runs `fn` IMMEDIATELY, not deferred (see Tech Decisions)
  - `func (m *Middleware) Handler(h func(ctx *httpctx.Context, next Next))`
  - `func (m *Middleware) HandlerFunc() func(ctx *httpctx.Context, next Next)` — returns nil if `Handler` was never called (caller's problem, mirrors `Pipe.HandlerFunc()`'s zero-value contract)
- **Dependencies**: `internal/httpctx` only
- **Reuses**: nothing — first pipeline-stage type with a `next`-continuation shape (Guard/Interceptor/Filter, later features, will likely follow a similar shape but are NOT built here)

### `Controller.Use` (extended, was a stub since T6)

- **Purpose**: give the existing no-op `Use(items ...Middleware)` stub (placeholder `Middleware struct{}`) real storage of REAL middleware.
- **Location**: `internal/controller/controller.go` (existing, extended)
- **Interfaces**:
  - `func (c *Controller) Use(items ...*middleware.Middleware)` — signature changes from the T6 stub's `...Middleware` (placeholder) to `...*middleware.Middleware` (real type) — BREAKING, but this is exactly what T6's own doc comment predicted ("it exists so those methods have a plausible signature to grow into")
  - `func (c *Controller) OwnMiddleware() []*middleware.Middleware` — new accessor, defensive copy (same pattern as `OwnRoutes`/`Module.OwnProviders`)
- **NOT changed**: `Guards`/`Interceptors`/`Filters` keep their EXISTING stub signature (`...Middleware`, the local placeholder struct) — those are out of scope for this feature (spec.md's Out of Scope), only `Use` graduates to the real type
- **Dependencies**: adds `internal/middleware`
- **Reuses**: `middleware.Middleware`

### `Module.Use` (new — did not exist before this feature)

- **Purpose**: root-module-only global middleware registration (spec.md P2, scoped explicitly to the root module per Out of Scope).
- **Location**: `internal/module/module.go` (existing, extended)
- **Interfaces**:
  - `func (m *Module) Use(items ...*middleware.Middleware)` — new method, any `*Module` can call it (Go can't restrict "only the root" at the type-system level), but ONLY the root module's registered middleware is actually consulted by Stage 2.5 (see below) — calling `Use` on a non-root/imported module compiles and stores the value, but has no defined behavior per spec.md's Out of Scope
  - `func (m *Module) OwnMiddleware() []*middleware.Middleware` — defensive copy, same pattern as every other `Own*` accessor on `Module`
- **Dependencies**: adds `internal/middleware`
- **Reuses**: `middleware.Middleware`, exact same field/accessor pattern as `Module.providers`/`OwnProviders`

### `internal/app`'s Stage 2.5 (`registerRoutes`, extended)

- **Purpose**: compose the middleware chain (global + controller-level) around each route's own Handler BEFORE registering with the adapter, instead of registering the bare route Handler directly (T8's current behavior).
- **Location**: `internal/app/app.go` (existing `registerRoutes` function, extended)
- **Interfaces**: no new exported surface — `registerRoutes` itself is already unexported; `routableController` (the existing local interface used to type-assert `module.ControllerRef` down to `PathPrefix`/`OwnRoutes`, added in T8) gains one more method requirement, `OwnMiddleware() []*middleware.Middleware`, already implemented by `*controller.Controller` as of this feature's `Controller.OwnMiddleware()` addition above
- **Dependencies**: adds `internal/middleware`
- **Reuses**: `root.OwnMiddleware()` (the SAME `*module.Module` already passed into `NewApp`/`registerRoutes` as `root` — no new parameter needed, `registerRoutes`'s existing signature already receives it), `routableController.OwnMiddleware()` per controller

---

## Data Models

```go
// internal/middleware
type Next func(ctx *httpctx.Context)
type Middleware struct {
    handler func(ctx *httpctx.Context, next Next)
}
```

**Composition algorithm** (conceptually, inside `registerRoutes`, per route):

```go
chain := append(append([]*middleware.Middleware{}, root.OwnMiddleware()...), controllerMiddleware...)
// build from the route Handler outward, so chain[0] (global-first) ends up OUTERMOST
next := middleware.Next(route.HandlerFunc())
for i := len(chain) - 1; i >= 0; i-- {
    mw := chain[i]
    captured := next // capture per-iteration, avoid the classic Go closure-loop-variable bug
    next = func(ctx *httpctx.Context) { mw.HandlerFunc()(ctx, captured) }
}
finalHandler := func(ctx *httpctx.Context) { next(ctx) }
// finalHandler, not route.HandlerFunc(), is what gets passed to adapter.RegisterRoute
```

**Relationships**: `Next`'s underlying type (`func(ctx *httpctx.Context)`) is IDENTICAL in shape to a route `Handler` (`func(ctx *httpctx.Context)`, established T2/T5) — this is what makes wrapping the innermost route Handler as the initial `Next` a direct, no-adapter-needed assignment, not a type-conversion shim.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| Middleware function panics (with or without calling `next`) | Caught by the EXISTING `internal/fiberapp` recover wrapper (unchanged this feature) — the whole composed chain (global mw → controller mw → route Handler) runs inside that one recover scope, same as a bare route Handler always has | `Exception` → structured response, non-Exception → generic 500 (from "Panic Recovery & Default Handler", already built) — zero new recovery code needed |
| Middleware never calls `next(ctx)` | Chain simply stops — nothing downstream runs, route Handler never executes | Matches spec.md AC3 (intentional short-circuit support) and Express/Nest's own semantics |
| Middleware calls `next(ctx)` more than once | Whatever the SECOND call's target does, runs a second time (last effect wins, no dedup/guard added) | Matches spec.md's Edge Cases — not defended against, matches how a real bug like this would ALSO misbehave in Express/Nest |
| App with zero `Use()` calls anywhere (neither root module nor any controller) | `chain` is empty, `next := middleware.Next(route.HandlerFunc())` stays as-is, `finalHandler` behaves identically to registering `route.HandlerFunc()` directly | Zero regression — spec.md's Edge Cases, this feature must be a pure addition for apps that never call `Use` |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `middleware.New(fn)` runs `fn` IMMEDIATELY, not deferred | Immediate execution, same as `route.New` | Unlike `Provider`/`Controller`/`Module`/`Pipe` (all of which defer `fn` until their own `Declare()` runs), `Middleware.Handler(...)` registration has NO dependency on the module tree being assembled first — INSIGHT.md's own `RequestIdMiddleware` example calls no `MustInject`, needs no `Owner`/module-scope knowledge at all. Deferring would require wiring a `Declare()` call somewhere (Controller.Use time? Module.Use time?) that nothing in this feature's scope actually needs — immediate execution sidesteps that complexity entirely, matching `route.New`'s own precedent ("no further stage left to usefully defer to" when the thing being configured needs no module-tree context) |
| `Controller.Use`'s signature changes (breaking) rather than adding a second method | Change `Use(items ...Middleware)` → `Use(items ...*middleware.Middleware)` directly | T6's own doc comment on the `Middleware struct{}` placeholder explicitly says it "exists so those methods have a plausible signature to grow into once a later feature defines what middleware actually does" — this IS that later feature. `Use` was never called by any shipped code path with the placeholder type (same "safe to change directly" reasoning already used for `HttpAdapter.Listen` in T3 of "App Bootstrap & Listen") |
| `Guards`/`Interceptors`/`Filters` are NOT touched | Keep exactly as-is (still the placeholder `Middleware struct{}` stub) | Out of scope per spec.md — those are separate ROADMAP.md Milestone 3 features (`Guard`, `Interceptor`, `Filter`), each likely needs its OWN distinct type (a `Guard` returns `bool`, per INSIGHT.md, not `func(ctx, next)` — reusing `middleware.Middleware`'s shape for them would be actively wrong) |
| `Module.Use` behaviorally scoped to the ROOT module only, not module-tree-cascading | Any `*Module` can call `Use` (compiles), but `registerRoutes` only ever reads `root.OwnMiddleware()` — the SAME `root` parameter it already receives, never walks the tree looking for `Use()` on imported modules | See spec.md's Out of Scope — module-tree-scoped middleware inheritance is genuinely ambiguous (does an imported module's `Use()` apply to its own controllers only? cascade to importers?) and not needed by any INSIGHT.md example (`AppModule.Use(...)` is always the literal root). Scoping tightly now avoids guessing at semantics nothing currently needs. |
| Composition happens in `internal/app`'s `registerRoutes` (Stage 2.5), not inside `internal/fiberapp` | Chain built as a plain `func(ctx *httpctx.Context)` BEFORE calling `adapter.RegisterRoute` | Keeps `HttpAdapter`'s contract completely unchanged (`RegisterRoute(method, path, h func(ctx *httpctx.Context)) error` — same signature as before this feature) — from the adapter's perspective, a composed middleware+Handler chain is indistinguishable from a bare Handler, so no adapter-level changes, no `internal/fiberapp` changes, and the "only 2 packages touch Fiber directly" boundary (established since T7) stays intact |
| Global-before-local ordering achieved via `append(root.OwnMiddleware(), controllerMiddleware...)` then composing outward from the END of that slice | Global middleware ends up OUTERMOST in the composed chain (runs first) | Matches spec.md AC MW-07 exactly (global runs before controller-level) — composing "outward from the end" means the LAST element in the slice becomes the innermost wrapper (closest to the route Handler), so putting global middleware FIRST in the slice makes it the OUTERMOST (first-to-run) layer |

---

## Open Questions pra Tasks

- None — spec.md's Out of Scope explicitly resolves the one genuine ambiguity (non-root `Module.Use` semantics) by scoping it out; every other decision follows directly from existing precedent in this codebase (`route.New`'s immediate-execution reasoning, T3's "safe to change an interface no shipped code calls yet" reasoning, the existing `Own*` defensive-copy accessor pattern).
