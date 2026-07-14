# Filter Specification

## Problem Statement

`internal/controller.Controller.Filters(...)` is the LAST remaining no-op stub from T6 ("Controller & Route Registration") still using the placeholder `Middleware struct{}` type. "Panic Recovery & Default Handler" already gives every panicked `exception.Exception` a default `{name,message,details}` JSON response — this feature lets a dev OVERRIDE that default for specific exception types, matching Nest's `ExceptionFilter`/`@Catch()`: e.g. INSIGHT.md's `FooExampleFilter` catches `*FooExampleError` specifically and responds with a custom `{"custom":true,"name":...}` body instead of the default shape.

## Goals

- [ ] `gonest.NewFilter(fn)` + `filter.Catch(exemplar, handler)` lets a dev register a custom response for one specific exception TYPE (identified structurally by an exemplar value, e.g. `&FooExampleError{}`), reusable across controllers
- [ ] `Controller.Filters(...)` (currently a no-op stub) actually intercepts a panicked exception whose CONCRETE TYPE matches a registered `Catch`, running the custom handler instead of the default `{name,message,details}` formatting
- [ ] `Module.Filters(...)` (new, mirrors `Module.Use`'s root-only-global scoping from "Middleware") lets the root module register filters that apply across every route in the app
- [ ] Any exception NOT caught by a matching Filter still falls through to the EXISTING default handler (unchanged from "Panic Recovery & Default Handler") — this feature is additive, never removes the safety net

## Out of Scope

| Feature | Reason |
| --- | --- |
| `MustInject` support inside `NewFilter`'s builder fn | Per AD-008: same reasoning as Middleware/Guard/Interceptor — a `*Filter` can be attached to multiple controllers across modules, no clean single owner |
| Catching non-Exception panics (bare error, nil-pointer deref, etc.) | Filters only ever catch structured `exception.Exception` values — a non-Exception panic keeps falling straight through to the existing generic-500 path, unchanged, same as it does today with no Filter involved at all |
| `Filter.Catch` matching by INTERFACE or by a supertype (e.g. catching "any Exception", or "any 4xx") | INSIGHT.md's own example matches one CONCRETE type (`&FooExampleError{}`) — matching is exact-type only (via `reflect.TypeOf`), not hierarchical. A future feature could add broader matching if a real need shows up; nothing here requires it |
| Full "Pipeline Ordering" validation (Middleware → Guard → Interceptor → Pipe → Handler) | Filter isn't even part of that documented request-pipeline order (it operates on the RESPONSE side, after a panic, not before the Handler runs) — this feature doesn't touch or need to worry about that ordering at all |

---

## User Stories

### P1: Catch a specific exception type, override its default response ⭐ MVP

**User Story**: As a gonest user, I want `gonest.NewFilter(fn)` with `filter.Catch(&FooExampleError{}, func(ctx, exc *FooExampleError) {...})` so that I can customize the HTTP response for one specific, known exception type (e.g. wrapping it in extra fields, changing the status code) without touching the generic default-handler machinery for every OTHER exception type.

**Why P1**: This is the entire feature — without a working `Catch` + real interception, `Filters()` stays the inert stub it's been since T6, and devs have no way to customize responses per-exception-type at all (only the generic `{name,message,details}` shape exists today).

**Acceptance Criteria**:

1. WHEN `filter.Catch(exemplar, handler)` is registered and the filter is attached via `Controller.Filters(f)` THEN a route Handler (or anything nested inside its dispatch chain — middleware, guard, interceptor) panicking with a value whose CONCRETE TYPE matches `exemplar`'s type SHALL run `handler(ctx, typedExc)` instead of the default `{name,message,details}` response
2. WHEN the panicked value's concrete type does NOT match any registered `Catch` on that route's applicable filters THEN system SHALL fall through to the EXISTING default handler (Exception → `{name,message,details}` at its own status, or generic 500 for non-Exception panics) — unchanged from "Panic Recovery & Default Handler"
3. WHEN `handler(ctx, typedExc)` runs THEN `typedExc` SHALL be the exact concrete type registered (e.g. `*FooExampleError`, not the generic `exception.Exception` interface) so the handler can access type-specific fields/methods without a further type assertion
4. WHEN a Filter's custom handler itself writes a response via `ctx.Status(...).Json(...)` THEN that response SHALL be what the client receives — matching INSIGHT.md's own example (`ctx.Status(gonest.HttpStatusTeapot).Json(map[string]any{"custom": true, "name": exc.Name()})`, adapted per this feature's own scoping of `HttpStatus` — see Edge Cases)
5. WHEN multiple `Catch` calls are registered on the SAME Filter for DIFFERENT exemplar types THEN each type SHALL be dispatched to its own registered handler correctly (a Filter can catch more than one exception type)

**Independent Test**: reproduce INSIGHT.md's `FooExampleFilter` example (adapted per Edge Cases: plain `int` status literal, not a `gonest.HttpStatusTeapot` named constant that doesn't exist), attach via `controller.Filters(...)`, dispatch a route Handler that panics with the caught type, confirm the custom body/status land; dispatch a DIFFERENT route on the same controller whose Handler panics with a DIFFERENT (uncaught) exception type, confirm the default `{name,message,details}` response still applies.

---

### P2: `Module.Filters(...)` — global filters for the whole app

**User Story**: As a gonest user, I want `AppModule.Filters(FooExampleFilter)` (root module) so that a filter applies across every route in the app, matching INSIGHT.md's own comment ("`module.Filters(FooExampleFilter) // global exception filter`") and mirroring `Module.Use`'s existing root-only-global scoping from "Middleware".

**Why P2**: Not needed for a single-controller app (P1 covers that), but is the second concrete usage INSIGHT.md itself shows — a common real need (a domain-wide error-shape override) shouldn't require attaching the same Filter to every controller manually.

**Acceptance Criteria**:

1. WHEN a Filter is registered via `Use`... via `Filters(...)` on the ROOT module passed to `NewApp` THEN system SHALL apply it to every route in the app, across every controller in every (imported) module — same root-only scoping precedent as `Module.Use` (spec.md's Out of Scope in "Middleware": non-root `Module.Filters` compiles but has no defined behavior)
2. WHEN both a global (root `Module.Filters`) and a controller-level (`Controller.Filters`) Filter register a `Catch` for the SAME exemplar type THEN the CONTROLLER-level Filter's handler SHALL win (more specific override takes precedence over the global default) — matches Nest's own filter-precedence convention (method/controller-scoped filters override globally-scoped ones for the same exception)
3. WHEN a route's controller has NO `Filters()` of its own but the root module does, and the panicked exception matches a globally-registered `Catch` THEN system SHALL still apply the global filter's custom handler

**Independent Test**: register a global Filter catching `*FooExampleError` on the root module AND a controller-level Filter ALSO catching `*FooExampleError` (with a distinguishable response) on one specific controller; dispatch to (a) a route on that controller (confirm controller-level handler wins) and (b) a route on a DIFFERENT controller with no Filter of its own (confirm the global handler still applies).

---

## Edge Cases

- WHEN INSIGHT.md's example uses `gonest.HttpStatusTeapot` (a named `HttpStatus` constant that does not exist in this codebase — same scoping gap already noted in "HttpException Core"'s and "Panic Recovery & Default Handler"'s own specs) THEN this feature's own tests/examples SHALL use the equivalent plain `int` literal (`418`) instead — no new `HttpStatus` enum work is in scope here either
- WHEN a controller has zero `Filters()` calls and the root module has zero `Filters()` calls THEN system SHALL dispatch exactly as it does today (unchanged) — pure addition, zero regression for apps that never call `Filters`
- WHEN a Filter's own custom handler ITSELF panics THEN system SHALL NOT attempt to catch that second panic with the same or any other Filter — it propagates to whatever recovery is already wrapping the whole dispatch (the adapter's existing generic-500 fallback), same "don't defend against a Filter author's own bug beyond not crashing the process" stance already taken for Middleware/Guard/Interceptor
- WHEN the panicked value is `nil` (Go 1.21+ `*runtime.PanicNilError` on recover) THEN it does not satisfy `exception.Exception`, so no `Catch` can match it — falls straight through to the default non-Exception path, same as today

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| FLT-01 | P1: NewFilter + Catch(exemplar, handler), exact-type match | Design | Pending |
| FLT-02 | P1: uncaught exception falls through to existing default handler | Design | Pending |
| FLT-03 | P1: handler receives concrete typed exception, not generic interface | Design | Pending |
| FLT-04 | P1: Filter's custom response is what the client receives | Design | Pending |
| FLT-05 | P1: one Filter can Catch multiple distinct types | Design | Pending |
| FLT-06 | P2: root Module.Filters is global | Design | Pending |
| FLT-07 | P2: controller-level Filter overrides global for same type | Design | Pending |
| FLT-08 | P2: global Filter applies even without controller opt-in | Design | Pending |

**ID format:** `FLT-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 8 total, 0 mapped to tasks yet, 8 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] INSIGHT.md's `FooExampleFilter` example (adapted per Edge Cases: plain int status) works end-to-end: caught type → custom response, uncaught type → existing default response, both attached per-controller and globally via the root module
- [ ] Controller-level Filter provably overrides a global Filter's handler for the same caught type (sequence/precedence proven, not just "one of them ran")
- [ ] Zero regressions in the existing test suite (Milestones 1-2 complete + Middleware/Guard/Interceptor/Pipe, ~16 packages before this feature starts)
