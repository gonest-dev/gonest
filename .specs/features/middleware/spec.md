# Middleware Specification

## Problem Statement

Every route registered so far dispatches straight from Fiber to a single gonest `Handler` (T7-T8) — there is no way to run shared logic (request-ID tagging, logging, raw request/response observation) BEFORE a route's own `Handler` runs, the way Express/Nest middleware does. `internal/controller.Controller.Use(...)` already exists as a pure no-op stub (T6, "Controller & Route Registration") storing a placeholder `Middleware struct{}` — this feature gives that stub real behavior, and adds the missing `Module.Use(...)` for app-wide middleware (INSIGHT.md's `AppModule.Use(RequestIdMiddleware)` example).

## Goals

- [ ] `gonest.NewMiddleware(fn)` + `middleware.Handler(func(ctx, next))` lets a dev define reusable request-observation/mutation logic, the same way `NewProvider`/`NewPipe` already work
- [ ] `Controller.Use(...)` (currently a no-op stub) actually runs its registered middleware, in registration order, before that controller's route Handlers
- [ ] `Module.Use(...)` (new) lets the root module register middleware that runs before EVERY route in the app, regardless of which controller/module it belongs to — global middleware

## Out of Scope

| Feature | Reason |
| --- | --- |
| `Guard`/`Interceptor`/`Pipe`(the pipeline-stage kind, distinct from param-coercion `Pipe` already built)/`Filter` | Each is its own ROADMAP.md Milestone 3 feature, built after this one — `Middleware` is deliberately the first, simplest pipeline stage (raw observe/mutate, no authorization decision, no response-wrapping, no exception catching) |
| "Pipeline Ordering" validation (Middleware → Guard → Interceptor → Pipe → Handler running in the correct combined order) | Its own later ROADMAP.md feature, only meaningful once Guard/Interceptor exist too — this feature only has to get Middleware's OWN ordering right (registration order, global-before-controller) |
| `Use()` on a NON-ROOT module (an imported module registering its own middleware) | INSIGHT.md's only example is `AppModule.Use(...)` where `AppModule` is the root module passed to `NewApp`. Whether an imported module's own `Use()` should cascade to its own controllers only, to importers too, or is simply not meaningful, is genuinely ambiguous and not needed to satisfy any existing INSIGHT.md example — `Module.Use` in this feature is defined and tested ONLY for the root module (global middleware); calling `Use` on a non-root module is accepted (compiles, stores something) but this feature makes no behavioral promise about it. A future feature can define module-tree-scoped middleware inheritance if a real need arises. |
| Any change to `Guard`/`Interceptor`/`Filter` fields already stubbed on `Controller` (T6) | Those stay exactly as no-op stubs — this feature only gives `Use`/`Middleware` real behavior |

---

## User Stories

### P1: Define reusable middleware, wire it to a controller ⭐ MVP

**User Story**: As a gonest user, I want `gonest.NewMiddleware(fn)` with `middleware.Handler(func(ctx *gonest.Context, next gonest.Next))` so that I can write request-observation logic once (e.g. tagging every response with an `X-Request-Id` header, per INSIGHT.md's own example) and attach it to a controller via `controller.Use(RequestIdMiddleware)`, running before every route on that controller.

**Why P1**: This is the entire feature — without a working `Handler(ctx, next)` continuation and real wiring into dispatch, `Use()` stays the inert stub it's been since T6.

**Acceptance Criteria**:

1. WHEN `middleware.Handler(func(ctx, next))` is registered and the middleware is attached via `Controller.Use(m)` THEN a real request dispatched to any route on that controller SHALL run the middleware's function BEFORE the route's own `Handler`
2. WHEN the middleware function calls `next(ctx)` THEN system SHALL continue to the next middleware in the chain (or the route `Handler`, if this was the last one) — mirrors INSIGHT.md's own `RequestIdMiddleware` example calling `next(ctx)` after mutating a header
3. WHEN the middleware function does NOT call `next(ctx)` THEN system SHALL NOT run the route Handler (or any subsequent middleware) — the chain stops there, matching Express/Nest middleware semantics where omitting `next()` short-circuits the request
4. WHEN multiple middleware are registered via `Use(m1, m2, ...)` (variadic, mirrors `Controller.Use`'s existing stub signature) THEN system SHALL run them in registration order, each wrapping the next
5. WHEN a middleware mutates `ctx` (e.g. `ctx.SetHeader(...)`) before calling `next(ctx)` THEN that mutation SHALL be visible to every subsequent middleware AND the route Handler — same `*httpctx.Context` instance threaded through, not a copy

**Independent Test**: reproduce INSIGHT.md's `RequestIdMiddleware` example verbatim (generates a UUID, sets `X-Request-Id` response header, calls `next`), attach via `controller.Use(...)`, dispatch a real request via `app.Test`, confirm the response header is present and the route Handler's own response body is unaffected.

---

### P2: `Module.Use(...)` — global middleware for the whole app

**User Story**: As a gonest user, I want `AppModule.Use(RequestIdMiddleware)` (root module) so that a single registration point applies middleware to every route in the app, regardless of which controller declares it — matching Nest's `app.use(...)`/global middleware.

**Why P2**: Not needed for a single-controller app (P1 covers that), but is the second concrete usage INSIGHT.md itself shows (`module.Use(RequestIdMiddleware) // global middleware`) — global middleware is a common real-world need (request-ID tagging, CORS-adjacent headers) that shouldn't require attaching to every controller manually.

**Acceptance Criteria**:

1. WHEN middleware is registered via `Use(...)` on the ROOT module passed to `NewApp` THEN system SHALL run it before EVERY route in the app, across every controller in every (imported) module
2. WHEN both global (root `Module.Use`) and controller-level (`Controller.Use`) middleware are registered for a route THEN system SHALL run global middleware FIRST, then controller-level middleware, then the route Handler (global-before-local, matching Nest's own middleware application order: global middleware always runs first)
3. WHEN a route's controller has NO `Use()` middleware but the root module does THEN system SHALL still run the global middleware for that route (global application does not depend on the controller opting in)

**Independent Test**: register `RequestIdMiddleware` globally on the root module AND a second, distinguishable middleware (e.g. one appending a fixed marker to a response header) on one specific controller; dispatch requests to (a) a route on that controller and (b) a route on a DIFFERENT controller with no middleware of its own; confirm (a) has both markers in the correct order and (b) has only the global marker.

---

## Edge Cases

- WHEN a controller has zero `Use()` calls and the root module has zero `Use()` calls THEN system SHALL dispatch exactly as it does today (T7/T8, unchanged) — this feature must be a pure addition, zero regression for apps that never call `Use`
- WHEN a middleware function itself panics (instead of calling or not calling `next`) THEN system SHALL be caught by the SAME recover wrapper already in place (`internal/fiberapp`'s `RegisterRoute`, extended by "Panic Recovery & Default Handler") — a panicking middleware behaves exactly like a panicking route Handler from the response's perspective (Exception → structured response, non-Exception → generic 500). No new recovery mechanism needed; the middleware chain (including the final Handler) all runs inside the SAME already-existing recover scope.
- WHEN `next` is called MORE THAN ONCE by a middleware THEN system SHALL NOT crash — behavior for a double-call is unspecified/last-write-wins is acceptable (matches how a bug like this would ALSO misbehave in Express/Nest; this feature does not need to defend against a middleware author's own bug beyond "doesn't crash the process")

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| MW-01 | P1: NewMiddleware + Handler(ctx, next) works | Design | Pending |
| MW-02 | P1: next(ctx) continues chain | Design | Pending |
| MW-03 | P1: omitting next() short-circuits | Design | Pending |
| MW-04 | P1: multiple middleware run in registration order | Design | Pending |
| MW-05 | P1: ctx mutation visible downstream | Design | Pending |
| MW-06 | P2: root Module.Use is global | Design | Pending |
| MW-07 | P2: global runs before controller-level | Design | Pending |
| MW-08 | P2: global applies even without controller opt-in | Design | Pending |

**ID format:** `MW-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 8 total, 0 mapped to tasks yet, 8 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] INSIGHT.md's `RequestIdMiddleware` example works verbatim, both attached per-controller and globally via the root module
- [ ] Zero regressions for any existing route/controller that never calls `Use()` — full existing test suite (Milestones 1-2 complete, ~14 packages) stays green
- [ ] Global-before-local ordering is provably correct, not just "both run" — a test proves the SEQUENCE, not just presence
