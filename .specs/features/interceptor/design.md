# Interceptor Design

**Spec**: `.specs/features/interceptor/spec.md`

## Architecture Overview

```
internal/interceptor (new package, AD-004 pattern)
        │
        ├── Next          -- type Next func(ctx *httpctx.Context)  (own type, NOT reused
        │                     from internal/middleware -- see Tech Decisions)
        └── Interceptor    -- New(fn) runs fn IMMEDIATELY (AD-008: no MustInject)
                              Handler(func(ctx, next)), HandlerFunc()

internal/controller (existing) ──imports──> internal/interceptor
        Controller.Interceptors(items ...*interceptor.Interceptor)  -- was stub, real now
        Controller.OwnInterceptors() []*interceptor.Interceptor      -- new accessor

internal/app's Stage 2.5 (existing, registerRoutes/composeHandler,
extended by "Middleware" then "Guard") ──extended again──
        innermost chain, built in this exact order (each wraps the previous):
          1. routeHandler := route.HandlerFunc()
          2. gatedHandler  := guards wrap routeHandler                    (from "Guard")
          3. interceptedHandler := interceptor chain wraps gatedHandler   (THIS feature)
          4. composedHandler := middleware chain wraps interceptedHandler (from "Middleware",
                                                                             loop itself unchanged)
        adapter.RegisterRoute(method, path, composedHandler)
```

Matches ROADMAP.md's documented order exactly: Middleware (outermost) → Guard → Interceptor → Handler (innermost). No change to `internal/fiberapp`/`HttpAdapter`/panic-recovery — an interceptor panic (before or after `next`) is caught by the same existing recover wrapper, same as every other stage's panics already are.

---

## Components

### `Next` / `Interceptor` (new package)

- **Purpose**: the reusable before/after-Handler-execution unit — structurally identical shape to `middleware.Middleware` (`Handler(ctx, next)` continuation-passing), but semantically distinct (AOP wrapping vs raw pre-routing observation) and composed at a different pipeline position.
- **Location**: `internal/interceptor/interceptor.go` (new file, new package)
- **Interfaces**:
  - `type Next func(ctx *httpctx.Context)` (own type — see Tech Decisions for why not reused from `internal/middleware.Next`)
  - `type Interceptor struct { handler func(ctx *httpctx.Context, next Next) }` (unexported field)
  - `func New(fn func(*Interceptor)) *Interceptor` — runs `fn` IMMEDIATELY, not deferred (AD-008 in STATE.md: no `MustInject` support for pipeline-stage types, same reasoning as `middleware.New`/`guard.New`)
  - `func (i *Interceptor) Handler(h func(ctx *httpctx.Context, next Next))`
  - `func (i *Interceptor) HandlerFunc() func(ctx *httpctx.Context, next Next)` — `nil` if `Handler` never called
- **Dependencies**: `internal/httpctx` only
- **Reuses**: the exact "immediate execution" + `Handler`/`HandlerFunc` zero-value pattern already established by `internal/middleware`/`internal/guard` — this is now the FOURTH type following that precedent (route.New, middleware.New, guard.New, interceptor.New)

### `Controller.Interceptors` (extended, was a stub since T6)

- **Purpose**: give the existing no-op `Interceptors(items ...Middleware)` stub real storage of REAL interceptors.
- **Location**: `internal/controller/controller.go` (existing, extended)
- **Interfaces**:
  - `func (c *Controller) Interceptors(items ...*interceptor.Interceptor)` — signature changes from the stub's `...Middleware` (placeholder) to `...*interceptor.Interceptor` (real type)
  - `func (c *Controller) OwnInterceptors() []*interceptor.Interceptor` — new accessor, defensive copy (same pattern as `OwnGuards`/`OwnMiddleware`)
- **NOT changed**: `Filters` keeps its EXISTING stub signature (`...Middleware`, the local placeholder struct) — out of scope for this feature, separate future feature. After THIS feature ships, `Filters` is the ONLY remaining stub on `Controller` still using the placeholder type.
- **Dependencies**: adds `internal/interceptor`
- **Reuses**: `interceptor.Interceptor`, the exact `Guards`/`OwnGuards` pattern this file already grew in "Guard" (mechanical repeat of the same shape for a third pipeline-stage type)

### `internal/app`'s Stage 2.5 (`registerRoutes`/`composeHandler`, extended a third time)

- **Purpose**: compose an interceptor chain around the already-guard-gated handler (from "Guard"), BEFORE the existing middleware-composition loop (from "Middleware") wraps the whole thing.
- **Location**: `internal/app/app.go` (existing, extended)
- **Interfaces**: no new exported surface — `routableController` gains one more method requirement, `OwnInterceptors() []*interceptor.Interceptor`, already implemented by `*controller.Controller` post-this-feature's `Controller.OwnInterceptors()`
- **Dependencies**: adds `internal/interceptor`
- **Reuses**: the EXISTING guard-gating (`gatedHandler`, from "Guard") and middleware-composition loop (from "Middleware") — NEITHER is restructured, this feature only inserts a new composition step BETWEEN them (interceptor chain wraps `gatedHandler`'s output, and THAT becomes what the middleware loop wraps, instead of `gatedHandler` directly)

---

## Data Models

```go
// internal/interceptor
type Next func(ctx *httpctx.Context)
type Interceptor struct {
    handler func(ctx *httpctx.Context, next Next)
}
```

**Composition change** (inside `registerRoutes`/`composeHandler`, per route — this is a THIRD layer inserted between the existing `gatedHandler` and the existing middleware-wrap loop; neither of those two pieces of logic changes internally, only what gets passed between them):

```go
// UNCHANGED from "Guard":
routeHandler := route.HandlerFunc()
gatedHandler := func(ctx *httpctx.Context) {
    for _, g := range controllerGuards { /* ...panics ForbiddenException on false... */ }
    routeHandler(ctx)
}

// NEW this feature: wrap gatedHandler with the interceptor chain, same composition
// shape as middleware's own chain-building loop (registration order, outward composition)
interceptorChain := controllerRC.OwnInterceptors()
interceptedNext := interceptor.Next(gatedHandler)
for i := len(interceptorChain) - 1; i >= 0; i-- {
    it := interceptorChain[i]
    captured := interceptedNext
    interceptedNext = func(ctx *httpctx.Context) { it.HandlerFunc()(ctx, captured) }
}
interceptedHandler := func(ctx *httpctx.Context) { interceptedNext(ctx) }

// UNCHANGED from "Middleware", only the argument changes (was gatedHandler, now interceptedHandler):
chain := append(append([]*middleware.Middleware{}, root.OwnMiddleware()...), controllerMiddleware...)
next := middleware.Next(interceptedHandler)
for i := len(chain) - 1; i >= 0; i-- { /* ...unchanged... */ }
composedHandler := func(ctx *httpctx.Context) { next(ctx) }
```

**Relationships**: `interceptor.Next`'s underlying shape (`func(ctx *httpctx.Context)`) is identical to `middleware.Next`'s and to a bare route Handler's — all three are structurally interchangeable at the `func(ctx *httpctx.Context)` level, which is exactly what makes chaining `gatedHandler` → `interceptedHandler` → `composedHandler` a series of direct assignments, no adapter/conversion code needed anywhere in this composition.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| Interceptor panics (before OR after calling `next`) | Propagates unchanged through the composition (no `recover()` anywhere in `internal/app`'s composition code), caught by the SAME existing recover wrapper (`internal/fiberapp`) that already handles Middleware/Guard/Handler panics | Zero new recovery code — matches spec.md AC5 |
| Interceptor's "after" logic doesn't run because something inside `next(ctx)` panicked | Standard Go panic-unwind semantics — the interceptor's own function body stops executing at the `next(ctx)` call site, any code physically after it in that function never runs (unless the interceptor itself used its own `defer`) | Not a gonest-specific concern, matches spec.md's Edge Cases explicitly |
| Zero interceptors registered on a controller | `interceptorChain` is empty, the loop runs zero iterations, `interceptedHandler` behaves identically to `gatedHandler` directly | Zero regression — spec.md's Edge Cases |
| `next` called more than once by an interceptor | Whatever the second call's target does runs a second time — not defended against, matches Middleware's own established stance | Consistent with existing precedent, not new to this feature |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `interceptor.New(fn)` runs `fn` IMMEDIATELY, no `MustInject` support | Same as `middleware.New`/`guard.New` | AD-008 (STATE.md) — established by "Guard"'s explicit user decision, applies identically here: an `*Interceptor` can be attached to multiple controllers across modules, no clean single owner for `MustInject` to resolve against |
| `internal/interceptor` defines its OWN `Next` type, does NOT import/reuse `internal/middleware.Next` | Separate, structurally-identical-but-distinct type | AD-004's "1 package per concept" — `internal/interceptor` and `internal/middleware` are parallel, independent pipeline-stage concepts (different composition position, different semantic framing per Nest/spec.md) that happen to share a shape today. Making `internal/interceptor` import `internal/middleware` just to reuse `Next` would create an artificial coupling between two conceptually-independent packages for a type that's a 12-character one-liner (`type Next func(ctx *httpctx.Context)`) to redeclare — cheaper to duplicate the type declaration than to couple the packages. (Both `Next` types remain freely interchangeable at the underlying-function level for composition purposes, per Go's structural function-type compatibility — no runtime cost to this choice.) |
| Interceptor composition inserted BETWEEN the existing `gatedHandler` (Guard) and the existing middleware-wrap loop (Middleware), neither of which is restructured | Additive insertion, not a rewrite | Keeps this feature's diff minimal and low-risk — "Guard"'s own composition logic and "Middleware"'s own composition loop are both already thoroughly tested (T3/T4 of their respective features); touching either would mean re-verifying logic this feature doesn't actually need to change. Matches the established pattern from "Guard" itself, which inserted `gatedHandler` between the route Handler and the (then-unmodified) middleware loop the exact same way. |
| `Module.Interceptors` NOT added | Controller-scoped only | Same reasoning as `Guard`'s `Module.Guards` exclusion — no ROADMAP.md/INSIGHT.md ground truth for a module-level Interceptor registration point |

---

## Open Questions pra Tasks

- None — every decision follows directly from "Middleware"/"Guard"'s own established precedent (AD-004, AD-008, the additive-insertion composition pattern), with no new ambiguity introduced by this feature.
