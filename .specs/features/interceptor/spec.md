# Interceptor Specification

## Problem Statement

`internal/controller.Controller.Interceptors(...)` is the last remaining pure no-op stub from T6 ("Controller & Route Registration") that still uses the placeholder `Middleware struct{}` type (`Use`→real via "Middleware", `Guards`→real via "Guard"; `Filters` is out of scope, a separate future feature). This feature gives `Interceptors` real behavior: an AOP-style wrapper that runs code BEFORE and AFTER the route Handler (and anything nested inside it, per ROADMAP.md's pipeline order), matching Nest's `Interceptor` concept — e.g. INSIGHT.md's `TimingInterceptor`, which measures and logs how long a request took.

## Goals

- [ ] `gonest.NewInterceptor(fn)` + `interceptor.Handler(func(ctx, next))` lets a dev wrap Handler execution with before/after logic, reusable across controllers
- [ ] `Controller.Interceptors(...)` (currently a no-op stub) actually runs its registered interceptors, wrapping the Handler (and any Guards already gating it), in registration order
- [ ] Interceptors run in the correct pipeline position: AFTER Middleware, AFTER Guards, wrapping the Handler directly — matching ROADMAP.md's documented order ("Middleware → Guard → Interceptor → Pipe → Handler")

## Out of Scope

| Feature | Reason |
| --- | --- |
| `MustInject` support inside `NewInterceptor`'s builder fn | Per AD-008 (STATE.md, established by the "Guard" feature's explicit user decision): a `*Interceptor`, like `*Guard`/`*Middleware`, can be attached to multiple controllers across different modules — no clean single "owner module" to resolve `MustInject` against. `NewInterceptor(fn)` runs `fn` IMMEDIATELY, same as `middleware.New`/`guard.New`. INSIGHT.md's own `TimingInterceptor` example (which calls `gonest.MustInject[*LoggerService](interceptor)`) is adapted for this feature's tests/examples to capture its dependency some other way (closing over an already-constructed value) — NOT by adding real DI support. |
| `Module.Interceptors(...)` (global, app-wide interceptors) | Neither ROADMAP.md nor INSIGHT.md show a module-level Interceptor registration point (same reasoning as `Guard`, which also has no module-level counterpart, unlike `Middleware`'s `Module.Use`) — `Interceptors` in this feature is Controller-scoped only |
| Response mutation/caching capabilities beyond "run code before/after" (e.g. actually rewriting the response body from an interceptor, matching Nest's `NestInterceptor.intercept` returning an `Observable` that can `map`/transform the response) | INSIGHT.md's own example (`TimingInterceptor`) only demonstrates the "measure time around next()" pattern — no response-transformation example exists to build against. This feature's `next(ctx)` continuation gives an interceptor everything it needs to run code before/after (INCLUDING mutating `ctx` if it wants, since `ctx` is the same shared instance per "Middleware"'s established mutation-visibility guarantee) — but nothing about "intercepting and replacing the response value itself" is in scope, since no concrete requirement exists for it yet |
| Full "Pipeline Ordering" validation (Middleware → Guard → Interceptor → Pipe → Handler running in the exact combined order across ALL 5 stages) | Its own later ROADMAP.md feature, only meaningful once `Pipe`(the pipeline-stage kind) exists too. This feature only has to get "Interceptor runs after Guard, wraps the Handler" right — the three stages that exist after this feature ships. |
| `Pipe`(pipeline-stage)/`Filter` | Each is its own ROADMAP.md Milestone 3 feature, built after this one |

---

## User Stories

### P1: Define an interceptor, wrap Handler execution with before/after logic ⭐ MVP

**User Story**: As a gonest user, I want `gonest.NewInterceptor(fn)` with `interceptor.Handler(func(ctx *gonest.Context, next gonest.Next))` so that I can write AOP-style logic once (e.g. measuring how long a request took, per INSIGHT.md's own `TimingInterceptor`) and attach it to a controller via `controller.Interceptors(TimingInterceptor)`, running before AND after the route Handler (and any Guards already gating it).

**Why P1**: This is the entire feature — without real evaluation wired into dispatch, `Interceptors()` stays the inert stub it's been since T6.

**Acceptance Criteria**:

1. WHEN `interceptor.Handler(func(ctx, next))` is registered and the interceptor is attached via `Controller.Interceptors(i)` THEN a real request dispatched to any route on that controller SHALL run code BEFORE `next(ctx)` is called, THEN run the Handler (and any Guards gating it), THEN run code AFTER `next(ctx)` returns — matching INSIGHT.md's `TimingInterceptor` exactly (`start := time.Now(); next(ctx); logger.Log(...)`)
2. WHEN the interceptor function does NOT call `next(ctx)` THEN system SHALL NOT run the Handler (or any subsequent interceptor) — same short-circuit semantics already established for Middleware (an interceptor's `next` has the identical shape/contract as Middleware's `Next`)
3. WHEN multiple interceptors are registered via `Interceptors(i1, i2, ...)` (variadic, mirrors `Controller.Guards`'s established signature) THEN system SHALL compose them in registration order, each wrapping the next — same composition shape already established for Middleware
4. WHEN a controller has BOTH `Guards()` and `Interceptors()` registered THEN system SHALL run: Middleware (already established order) → Guards → Interceptors wrapping the Handler → Handler — matching ROADMAP.md's documented pipeline order exactly
5. WHEN an interceptor panics (before or after calling `next`) THEN system SHALL be caught by the SAME existing recover wrapper already handling Middleware/Guard/Handler panics — no new recovery mechanism needed

**Independent Test**: reproduce INSIGHT.md's `TimingInterceptor` example (adapted per Out of Scope: no `MustInject`, close over a stand-in logger instead), attach via `controller.Interceptors(...)`, dispatch a real request via `app.Test`, confirm the "before" and "after" logic both ran (e.g. via an order-recorder proving before-Handler and after-Handler execution, not just "both ran at some point").

---

## Edge Cases

- WHEN a controller has zero `Interceptors()` calls THEN system SHALL dispatch exactly as it does today (Middleware + Guard features, unchanged) — pure addition, zero regression
- WHEN an interceptor's "after" logic runs (after `next(ctx)` returns) but the Handler (or something nested inside `next`) already panicked THEN the interceptor's own "after" code, if it exists AFTER the `next(ctx)` call in the interceptor's own function body, will NOT run — Go's own panic-unwinds-the-call-stack semantics apply here exactly as they would for any function that panics mid-call, no special handling needed or expected (an interceptor wanting guaranteed "after" behavior even on panic would need its own `defer`, same as any Go code — not a gonest-specific concern)
- WHEN `next` is called MORE THAN ONCE by an interceptor THEN system SHALL NOT crash — same "not defended against, matches how this would ALSO misbehave in Express/Nest" stance already taken for Middleware

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| ITC-01 | P1: NewInterceptor + Handler(ctx, next), before/after wrapping works | Design | Pending |
| ITC-02 | P1: omitting next() short-circuits | Design | Pending |
| ITC-03 | P1: multiple interceptors compose in registration order | Design | Pending |
| ITC-04 | P1: pipeline order Middleware → Guard → Interceptor → Handler | Design | Pending |
| ITC-05 | P1: interceptor panic caught by existing recovery | Design | Pending |

**ID format:** `ITC-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 5 total, 0 mapped to tasks yet, 5 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] INSIGHT.md's `TimingInterceptor` example (adapted per Out of Scope's no-DI decision) works end-to-end: before-Handler and after-Handler logic both provably run, in the right order relative to Middleware/Guard
- [ ] Zero regressions in the existing test suite (Milestones 1-2 complete + "Middleware" + "Guard", ~15 packages before this feature starts)
- [ ] The combined Middleware → Guard → Interceptor → Handler ordering is provably correct (sequence proven, not just presence)
