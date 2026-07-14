# Pipeline Ordering Specification

## Problem Statement

Each pipeline stage (Middleware, Guard, Interceptor, Filter) was built and tested in isolation, one feature at a time — each feature's own tests prove ITS piece of the ordering correctly (e.g. "Interceptor" proved Guard-then-Interceptor, "Filter" proved it catches panics from anywhere in the chain). No single test has ever exercised ALL 5 pipeline concepts (Middleware, Guard, Interceptor, Pipe, Filter) together on the SAME route, reproducing INSIGHT.md's own full example (`UserController` with `Use`/`Guards`/`Interceptors`/`Filters` all registered, plus a route using `Route.Param`). This feature closes that gap: prove the documented ROADMAP.md order ("Middleware → Guard → Interceptor → Pipe → Handler") holds when every stage is present at once, on one real request.

## Goals

- [ ] A single integration test reproduces INSIGHT.md's full `UserController` pipeline example (Middleware + Guard + Interceptor + Filter registered on the controller, a route using a custom `Pipe` via `Route.Param`) and proves the combined execution order matches ROADMAP.md's documented sequence
- [ ] Confirm where `Pipe` actually sits in that order: unlike Middleware/Guard/Interceptor/Filter (which are Stage-2.5-composed WRAPPING layers around the Handler), `Pipe` is invoked EXPLICITLY by the dev's own Handler code via `MustParam[T]` — there is no separate "Pipe stage" in the composed chain the way there is for the other four. This feature documents and proves that distinction is correct, not a gap.
- [ ] Confirm Filter still catches a panic that originates from INSIDE a Pipe's custom `Handler` (invoked via `MustParam[T]` from within the route Handler) — proving Filter's "catches from anywhere in the chain" guarantee (already proven for Guard/Middleware in "Filter"'s own T4) extends to Pipe-triggered panics too, since nothing about Pipe's invocation mechanism should be special-cased differently from any other panic source inside the Handler's own call stack

## Out of Scope

| Feature | Reason |
| --- | --- |
| Any NEW production code in `internal/app`'s Stage 2.5 composition | Every ordering guarantee this feature needs to prove is ALREADY implemented — Middleware (from "Middleware"), Guard-before-Interceptor (from "Interceptor", L-011 fix), Filter-wraps-everything (from "Filter"). This feature is validation-only, unless testing reveals a genuine gap (if so, that becomes its own fix, documented as a finding, not silently absorbed into this feature's original scope) |
| A formal `Pipe` "stage" added to the Stage 2.5 composition chain | Per Goals, `Pipe` deliberately stays invoked-from-within-the-Handler (via `MustParam[T]`), not composed as a wrapping layer — this matches how it was originally built (T4/T5 of "Controller & Route Registration") and how INSIGHT.md's own example uses it (`route.Param("user_id", ParseIntPipe)` + `MustParam[int64](ctx, "user_id")` inside the Handler body, not a separate registration point Stage 2.5 would need to know about) |
| Testing every PAIRWISE combination of the 5 stages exhaustively | Each pairwise interaction was already proven in the feature that introduced the SECOND stage of the pair (e.g. Guard-before-Interceptor proven in "Interceptor", Filter-catches-Guard-panic proven in "Filter") — this feature's job is the single ALL-5-AT-ONCE scenario INSIGHT.md itself shows, not a combinatorial re-proof of pairs already covered |

---

## User Stories

### P1: All 5 pipeline stages together, correct combined order ⭐ MVP

**User Story**: As a gonest maintainer, I want one integration test reproducing INSIGHT.md's full `UserController` pipeline example (Middleware + Guard + Interceptor + Filter on the controller, a route using a custom Pipe) so that the documented order (ROADMAP.md: "Middleware → Guard → Interceptor → Pipe → Handler") is provably correct end-to-end, not just correct pairwise in isolation.

**Why P1**: This is the entire feature — a single comprehensive proof that closes Milestone 3, giving future maintainers (and future features that might touch Stage 2.5 composition again) a regression net that would catch an ordering mistake like L-011's before it ships.

**Acceptance Criteria**:

1. WHEN a route has Middleware, Guard, Interceptor, and Filter ALL registered on its controller (plus a global Middleware/Filter on the root module, matching INSIGHT.md's `AppModule.Use(...)`/`AppModule.Filters(...)` global registrations) AND the route itself uses `Route.Param` with a custom Pipe THEN a real request that exercises the "happy path" (all guards pass, Pipe parses successfully) SHALL run every stage in the documented order: global Middleware → controller Middleware → Guard → Interceptor(before) → Handler (which itself invokes the Pipe via `MustParam[T]`) → Interceptor(after)
2. WHEN the SAME setup's Guard rejects the request (returns `false`) THEN Interceptor's "before" logic and the Handler (and therefore the Pipe) SHALL NOT run — matches "Guard"'s own already-proven short-circuit semantics, re-confirmed here in the full combined scenario
3. WHEN the SAME setup's Pipe panics (invalid param, e.g. `BadRequestException`) from INSIDE the Handler THEN the registered Filter SHALL still be able to catch it (if it registers a `Catch` for that exception type) or, if uncaught, fall through to the default `{name,message,details}` response — proving Filter's catch-from-anywhere guarantee genuinely covers Pipe-triggered panics too

**Independent Test**: this story'S OWN acceptance criteria ARE the independent test — a single test function (or a small table of subtests) reproducing the full INSIGHT.md `UserController` example, dispatched via real `app.Test`, covering: (a) full happy path with an order-recorder proving the exact sequence, (b) Guard-rejects short-circuit within the full setup, (c) Pipe-panic caught by Filter within the full setup.

---

## Edge Cases

- WHEN this feature's own testing reveals the CURRENT implementation does NOT match the documented order in some combined scenario not previously tested (a genuine regression or gap, not expected) THEN system SHALL treat that as a real bug to fix (same rigor as L-011's own discovery-and-fix cycle in "Interceptor"), not silently work around it in the test — flag it clearly, fix the actual composition code in `internal/app`, re-verify
- WHEN INSIGHT.md's full example uses `gonest.MustInject[*UserService](controller)` (real DI, unrelated to pipeline-stage ordering) THEN this feature's test SHALL use a simple in-memory service constructed directly (matching the established `UserService`/`UserProvider` precedent already used in "App Bootstrap & Listen"'s and "Controller & Route Registration"'s own end-to-end tests) — MustInject itself is out of scope, already proven elsewhere

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| ORD-01 | P1: full happy-path order, all 5 stages | Design | Pending |
| ORD-02 | P1: Guard-reject short-circuits within full setup | Design | Pending |
| ORD-03 | P1: Filter catches Pipe-triggered panic within full setup | Design | Pending |

**ID format:** `ORD-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 3 total, 0 mapped to tasks yet, 3 unmapped ⚠️

---

## Success Criteria

- [ ] INSIGHT.md's full `UserController` pipeline example works end-to-end with the documented ROADMAP.md order provably correct in one combined scenario
- [ ] Milestone 3 (Request Pipeline) is COMPLETE — every ROADMAP.md-listed feature for this milestone (Middleware, Guard, Interceptor, Pipe, Filter, Pipeline Ordering) done
- [ ] If testing reveals a genuine ordering bug not caught by any prior feature's own tests, it's found and fixed here, with the same rigor (independent evaluator verification) as every other fix this session
