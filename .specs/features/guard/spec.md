# Guard Specification

## Problem Statement

`internal/controller.Controller.Guards(...)` exists as a pure no-op stub since T6 ("Controller & Route Registration"), storing the placeholder `Middleware struct{}` type — same as `Use`/`Interceptors`/`Filters` were before "Middleware" gave `Use` real behavior. This feature does the same for `Guards`: a `bool`-returning gate that runs before a route's Handler, short-circuiting to `403 Forbidden` if it returns `false`, or letting a custom `panic(exception.Exception)` produce a different response — matching Nest's `CanActivate` guard concept.

## Goals

- [ ] `gonest.NewGuard(fn)` + `guard.Handler(func(ctx) bool)` lets a dev define an authorization check, reusable across controllers
- [ ] `Controller.Guards(...)` (currently a no-op stub) actually runs its registered guards before that controller's route Handlers — ALL guards must return `true` for the request to proceed
- [ ] A guard returning `false` produces an automatic `403 Forbidden` response; a guard that panics with a custom `exception.Exception` (e.g. `NewUnauthorizedException(nil)`, per INSIGHT.md's own `AuthGuard` example) produces THAT exception's response instead

## Out of Scope

| Feature | Reason |
| --- | --- |
| `MustInject` support inside `NewGuard`'s builder fn | **User decision (2026-07-13, this session):** unlike `Provider`/`Controller`/`Module` (each with exactly ONE owner module, enforced by the DI graph), a `*Guard` can be attached to multiple controllers across DIFFERENT modules via `Controller.Guards(sameGuardVar)` — there is no clean single "owner module" to resolve `MustInject` against without inventing new ambiguous semantics. `Guard` in this feature mirrors `Middleware`'s existing scope decision: `New(fn)` runs `fn` IMMEDIATELY, no deferred/Declare/Owner wiring, no `MustInject` inside the builder. INSIGHT.md's own `AuthGuard` example (which calls `gonest.MustInject[*AuthService](guard)`) is adapted for this feature's tests/examples to capture its dependency some other way (e.g. closing over an already-constructed value, or a package-level singleton) — NOT by adding real DI support this feature deliberately excludes. |
| `Module.Guards(...)` (global, app-wide guards) | Neither ROADMAP.md nor INSIGHT.md show a module-level Guard registration point (unlike `Middleware`, which explicitly had `Module.Use`) — `Guards` in this feature is Controller-scoped only |
| Full "Pipeline Ordering" validation (Middleware → Guard → Interceptor → Pipe → Handler running in the exact combined order across ALL pipeline stages) | Its own later ROADMAP.md feature, only meaningful once Interceptor/Pipe(-the-pipeline-stage-kind) exist too. This feature only has to get "Guard runs after Middleware, before the Handler" right — the two stages that currently exist. |
| `Interceptor`/`Pipe`(pipeline-stage)/`Filter` | Each is its own ROADMAP.md Milestone 3 feature, built after this one |

---

## User Stories

### P1: Define a guard, attach it to a controller ⭐ MVP

**User Story**: As a gonest user, I want `gonest.NewGuard(fn)` with `guard.Handler(func(ctx *gonest.Context) bool)` so that I can write an authorization check once and attach it to a controller via `controller.Guards(AuthGuard)`, blocking unauthorized requests before any route Handler on that controller runs.

**Why P1**: This is the entire feature — without real evaluation wired into dispatch, `Guards()` stays the inert stub it's been since T6.

**Acceptance Criteria**:

1. WHEN `guard.Handler(func(ctx) bool)` is registered and the guard is attached via `Controller.Guards(g)` THEN a real request dispatched to any route on that controller SHALL run the guard's function BEFORE the route's own Handler
2. WHEN the guard function returns `true` THEN system SHALL proceed to the next guard (if any) or the route Handler
3. WHEN the guard function returns `false` THEN system SHALL respond with `403 Forbidden` and SHALL NOT run the route Handler (or any subsequent guard) — matches INSIGHT.md's own documented default ("retorna bool; false = 403 Forbidden automático")
4. WHEN the guard function panics with a value satisfying `exception.Exception` (e.g. `panic(gonest.NewUnauthorizedException(nil))`, INSIGHT.md's own `AuthGuard` example) THEN system SHALL respond with THAT exception's own status/body — reusing "Panic Recovery & Default Handler"'s existing recover wrapper, no new recovery logic needed (matches INSIGHT.md's own comment: "pra mensagem custom, panica com Exception própria em vez de retornar false")
5. WHEN multiple guards are registered via `Guards(g1, g2, ...)` (variadic, mirrors `Controller.Use`'s established signature) THEN system SHALL evaluate them in registration order, short-circuiting at the FIRST one that returns `false` (subsequent guards do not run) — matches boolean AND / Nest's own `CanActivate` chain semantics

**Independent Test**: reproduce INSIGHT.md's `AuthGuard` example (adapted per Out of Scope: no `MustInject`, some other way to reach an `AuthService`-equivalent check), attach via `controller.Guards(...)`, dispatch 3 real requests via `app.Test`: (a) no `Authorization` header → 401 (custom exception path), (b) header present but invalid → 403 (plain `false` path, using a second simpler guard for this case since `AuthGuard` itself always panics rather than returning false on empty token per the literal example — or a second, distinguishable guard purpose-built to prove the `false`→403 path), (c) header present and valid → 200, Handler ran.

---

## Edge Cases

- WHEN a controller has zero `Guards()` calls THEN system SHALL dispatch exactly as it does today (Middleware feature, unchanged) — pure addition, zero regression for controllers that never call `Guards`
- WHEN BOTH `Use()` (Middleware) and `Guards()` are registered on the same controller THEN system SHALL run ALL middleware (global then controller-level, per the "Middleware" feature's established order) FIRST, then guards, then the route Handler — matches ROADMAP.md's documented pipeline order ("Middleware → Guard → Interceptor → Pipe → Handler")
- WHEN a guard panics with something that is NOT an `exception.Exception` (a genuine bug, nil pointer etc.) THEN system SHALL fall through to the SAME generic-500 path any other panic already does (unchanged behavior, no new handling) — a buggy guard is not different from a buggy Handler or a buggy middleware from the recovery wrapper's perspective

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| GRD-01 | P1: NewGuard + Handler(ctx) bool works | Design | Pending |
| GRD-02 | P1: true proceeds to next guard/Handler | Design | Pending |
| GRD-03 | P1: false → automatic 403, Handler doesn't run | Design | Pending |
| GRD-04 | P1: panic(Exception) → that exception's response | Design | Pending |
| GRD-05 | P1: multiple guards, registration order, short-circuit on first false | Design | Pending |

**ID format:** `GRD-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 5 total, 0 mapped to tasks yet, 5 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] INSIGHT.md's `AuthGuard` example (adapted per Out of Scope's no-DI decision) works end-to-end: missing/invalid auth → correct exception response, valid auth → route Handler runs
- [ ] Zero regressions in the existing test suite (Milestones 1-2 complete + "Middleware", ~14 packages before this feature starts)
- [ ] Guard-then-Handler AND Middleware-then-Guard-then-Handler orderings are both provably correct (sequence proven, not just presence)
