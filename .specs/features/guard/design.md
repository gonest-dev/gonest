# Guard Design

**Spec**: `.specs/features/guard/spec.md`

## Architecture Overview

```
internal/guard (new package, AD-004 pattern)
        │
        └── Guard  -- New(fn) runs fn IMMEDIATELY (same reasoning as middleware.New:
                       no MustInject support this feature, see spec.md's Out of Scope,
                       so nothing needs the module tree assembled first)
                       Handler(func(ctx) bool), HandlerFunc() func(ctx) bool

internal/controller (existing) ──imports──> internal/guard
        Controller.Guards(items ...*guard.Guard)   -- was Guards(items ...Middleware) stub,
                                                        BREAKING signature change, real storage now
        Controller.OwnGuards() []*guard.Guard       -- new accessor, defensive copy

internal/app's Stage 2.5 (existing, registerRoutes/composeHandler, extended by "Middleware") ──extended──
        gatedHandler := func(ctx) {
            for each controller guard, in order:
                if !guard.HandlerFunc()(ctx) { panic(exception.NewForbiddenException(nil)) }
            route.HandlerFunc()(ctx)
        }
        -- gatedHandler REPLACES route.HandlerFunc() as the innermost Next that the
           EXISTING middleware-chain composition (from "Middleware") wraps around.
           Middleware composition logic itself is UNCHANGED -- this feature only
           changes what the innermost function IS.
```

No change to `internal/fiberapp`/`HttpAdapter`/panic-recovery, no change to `internal/middleware`'s own composition algorithm — a guard-panic (`exception.NewForbiddenException` on `false`, or a custom `panic(exception.Exception)` from inside the guard's own handler) is caught by the SAME recover wrapper already in place, exactly like "Middleware"'s own panics already are.

---

## Components

### `Guard` (new package)

- **Purpose**: the reusable authorization-check unit — `Handler(ctx) bool`, no `next` (unlike `Middleware`, a Guard doesn't decorate/wrap, it gates: true=continue, false=stop).
- **Location**: `internal/guard/guard.go` (new file, new package)
- **Interfaces**:
  - `type Guard struct { handler func(ctx *httpctx.Context) bool }` (unexported field)
  - `func New(fn func(*Guard)) *Guard` — runs `fn` IMMEDIATELY, not deferred (same reasoning as `middleware.New`, see Tech Decisions — no `MustInject` support this feature per spec.md's Out of Scope, so no dependency on module-tree assembly timing)
  - `func (g *Guard) Handler(h func(ctx *httpctx.Context) bool)`
  - `func (g *Guard) HandlerFunc() func(ctx *httpctx.Context) bool` — `nil` if `Handler` never called
- **Dependencies**: `internal/httpctx` only
- **Reuses**: exact same "immediate execution" shape as `internal/middleware.Middleware`/`internal/route.Route` — this is now the THIRD type in this codebase following that precedent (route.New, middleware.New, guard.New), all for the same underlying reason (no module-tree dependency at construction time)

### `Controller.Guards` (extended, was a stub since T6)

- **Purpose**: give the existing no-op `Guards(items ...Middleware)` stub real storage of REAL guards.
- **Location**: `internal/controller/controller.go` (existing, extended)
- **Interfaces**:
  - `func (c *Controller) Guards(items ...*guard.Guard)` — signature changes from the T6/pre-"Middleware" stub's `...Middleware` (placeholder) to `...*guard.Guard` (real type) — BREAKING, same "never called by shipped code with the placeholder type" reasoning already used for `Use` in the "Middleware" feature
  - `func (c *Controller) OwnGuards() []*guard.Guard` — new accessor, defensive copy (same pattern as `OwnMiddleware`/`OwnRoutes`)
- **NOT changed**: `Interceptors`/`Filters` keep their EXISTING stub signature (`...Middleware`, the local placeholder struct) — those are out of scope for this feature (separate later features)
- **Dependencies**: adds `internal/guard`
- **Reuses**: `guard.Guard`, the exact `Use`/`OwnMiddleware` pattern this same file just grew in the "Middleware" feature (mechanical repeat of that same shape for a second pipeline-stage type)

### `internal/app`'s Stage 2.5 (`registerRoutes`/`composeHandler`, extended again)

- **Purpose**: evaluate a route's controller's guards (in order, short-circuit on first `false`) BEFORE calling the route's own Handler — inserted as the new innermost layer, with the EXISTING middleware-chain composition (from "Middleware") wrapping around it unchanged.
- **Location**: `internal/app/app.go` (existing `registerRoutes`/`composeHandler`, extended)
- **Interfaces**: no new exported surface — `routableController` (existing local interface) gains one more method requirement, `OwnGuards() []*guard.Guard`, already implemented by `*controller.Controller` as of this feature's `Controller.OwnGuards()` addition above
- **Dependencies**: adds `internal/guard`, adds `internal/exception` (for `exception.NewForbiddenException` — new dependency for `internal/app`, but `internal/exception` has zero dependencies of its own, no cycle risk)
- **Reuses**: the EXISTING middleware composition loop from "Middleware" — this feature does NOT modify that loop's logic at all, it only changes what gets passed in as the initial "innermost" function (previously `route.HandlerFunc()` directly, now `gatedHandler` which wraps `route.HandlerFunc()` behind a guard-evaluation loop)

---

## Data Models

```go
// internal/guard
type Guard struct {
    handler func(ctx *httpctx.Context) bool
}
```

**Composition change** (inside `registerRoutes`/`composeHandler`, per route — this REPLACES the previous "innermost = route.HandlerFunc()" line, everything else about the middleware-wrapping loop from "Middleware" stays exactly as it was):

```go
// NEW: build the guard-gated inner handler first
routeHandler := route.HandlerFunc()
controllerGuards := controllerRC.OwnGuards() // via routableController, extended
gatedHandler := func(ctx *httpctx.Context) {
    for _, g := range controllerGuards {
        if !g.HandlerFunc()(ctx) {
            panic(exception.NewForbiddenException(nil))
        }
    }
    routeHandler(ctx)
}

// UNCHANGED from "Middleware": compose the middleware chain around gatedHandler
// instead of around routeHandler directly
chain := append(append([]*middleware.Middleware{}, root.OwnMiddleware()...), controllerMiddleware...)
next := middleware.Next(gatedHandler) // <- only this line's argument changed
for i := len(chain) - 1; i >= 0; i-- {
    mw := chain[i]
    captured := next
    next = func(ctx *httpctx.Context) { mw.HandlerFunc()(ctx, captured) }
}
composedHandler := func(ctx *httpctx.Context) { next(ctx) }
```

**Relationships**: `Guard.HandlerFunc()`'s shape (`func(ctx *httpctx.Context) bool`) is intentionally NOT the same shape as `Next`/route-Handler (`func(ctx *httpctx.Context)`, no return value) — a Guard genuinely returns a decision, it does not itself produce/forward a response the way a `Next` continuation does. This is why `gatedHandler` (a plain `func(ctx *httpctx.Context)`, matching `Next`'s shape) is the adapter between "a list of bool-returning Guards" and "the single continuation-shaped function the existing Middleware composition already knows how to wrap."

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| A guard returns `false` | `gatedHandler` panics `exception.NewForbiddenException(nil)` — caught by the EXISTING recover wrapper (`internal/fiberapp`, from "Panic Recovery & Default Handler"), formatted as `{"name":"ForbiddenException","message":"","details":null}` at 403 | Matches spec.md AC3/INSIGHT.md's documented default ("false = 403 Forbidden automático") — zero new recovery code |
| A guard panics with a value satisfying `exception.Exception` | Propagates unchanged through `gatedHandler` (no `recover()` inside `gatedHandler` itself — pass-through), caught by the SAME existing recover wrapper, formatted per THAT exception's own status/body | Matches spec.md AC4/INSIGHT.md's `AuthGuard` example exactly — "pra mensagem custom, panica com Exception própria em vez de retornar false" |
| A guard panics with something NOT an `exception.Exception` (genuine bug) | Same generic-500 fallback any other panic already gets — `gatedHandler` does not distinguish guard-panics from Handler-panics or middleware-panics at the recovery layer, they're all just "a panic happened somewhere in the composed chain" | No new handling — matches spec.md's Edge Cases |
| Zero guards registered on a controller | `controllerGuards` is an empty slice, the `for` loop in `gatedHandler` runs zero iterations, `routeHandler(ctx)` runs immediately — behaviorally identical to calling `routeHandler` directly | Zero regression — spec.md's Edge Cases, this feature must be a pure addition for controllers that never call `Guards` |
| Multiple guards, one returns `false` partway through | Short-circuits at that guard — subsequent guards in the loop never run (Go's own `for` loop with an early `return`-equivalent via `panic`, which unwinds immediately) | Matches spec.md AC5 exactly |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `guard.New(fn)` runs `fn` IMMEDIATELY, no `MustInject` support | Same as `middleware.New` | **User decision, this session**: a `*Guard` can be attached to multiple controllers across different modules (unlike `Provider`, which has exactly one owner module enforced by the DI graph) — there's no clean single "owner" to resolve `MustInject` against without inventing new, currently-unneeded ambiguous ownership semantics. Mirrors the exact same scope decision already made for `Middleware`. |
| `403 Forbidden` on `false` implemented as `panic(exception.NewForbiddenException(nil))`, not a special adapter-level status-setting path | Reuse the EXISTING exception/recovery machinery entirely | Zero new recovery code, zero new response-writing code — "false means forbidden" becomes just another case the existing recover wrapper already knows how to format correctly, consistent with how a custom `panic(exception.Exception)` from inside a guard ALSO just flows through the same path. One recovery mechanism for the whole pipeline, not two. |
| Guard evaluation happens INSIDE `gatedHandler`, which becomes the new innermost layer the EXISTING middleware composition wraps around (middleware composition logic itself is untouched) | Guards are innermost (run last among "gate/decorate" stages, right before the Handler), Middleware stays outermost | Matches ROADMAP.md's documented pipeline order exactly: "Middleware → Guard → Interceptor → Pipe → Handler" — Middleware (already built) is leftmost/outermost, Guard (this feature) sits between Middleware and the Handler. Composing it as `gatedHandler` wrapping `routeHandler`, THEN feeding `gatedHandler` into the existing middleware-wrap loop (instead of `routeHandler` directly) achieves this ordering with a one-line change to the existing composition code, no restructuring. |
| `Module.Guards` NOT added (unlike `Module.Use` for Middleware) | Controller-scoped only | Neither ROADMAP.md nor INSIGHT.md show a module-level Guard registration point — no ground truth to build against, and no established need (unlike Middleware, which had `AppModule.Use(...)` as a literal INSIGHT.md example) |

---

## Open Questions pra Tasks

- None — the one genuine ambiguity (Guard's DI/ownership scope) was resolved by explicit user decision before this design was written; every other decision follows directly from the "Middleware" feature's own precedent (immediate execution, `Own*` defensive-copy accessors, composing into the existing Stage 2.5 pipeline without touching the adapter contract).
