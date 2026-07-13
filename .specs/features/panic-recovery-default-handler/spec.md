# Panic Recovery & Default Handler Specification

## Problem Statement

`internal/fiberapp`'s route wrapper (T7 of "Controller & Route Registration") already recovers any panic from a route `Handler` and turns it into a generic 500 — but it has no way to tell "this was an intentional `panic(gonest.NewNotFoundException(...))`" apart from "this was a real bug (nil pointer, index out of range)". Now that `internal/exception`'s `Exception` interface exists (previous feature, "HttpException Core"), the recover wrapper can finally distinguish the two: an `Exception` gets its own status code and a `{name,message,details}` JSON body; anything else still gets the existing generic, detail-free 500. This is the last remaining Milestone 2 feature — it closes the loop `panic(exception) → real HTTP response` that "HttpException Core" only built the vocabulary for.

## Goals

- [ ] Any route `Handler` that panics with a value satisfying `gonest.Exception` (built-in or dev-defined, via embedding `HttpException`) gets a response at that exception's own `Status()`, body `{"name":..., "message":..., "details":...}`
- [ ] Any route `Handler` that panics with anything else (a bare `error`, `nil`-pointer dereference, `index out of range`, a raw string, etc.) keeps today's exact behavior: generic 500, no internal detail leaked in the body (T7's existing guarantee, unchanged)

## Out of Scope

| Feature | Reason |
| --- | --- |
| `Filter`/`Catch(exceptionType, handler)` — per-exception custom response override (INSIGHT.md's `FooExampleFilter` example) | Milestone 3 ("Request Pipeline" → `Filter`). This feature only builds the DEFAULT handler every Exception falls back to when no Filter is registered — Filters don't exist as a concept yet |
| `UseGlobalFilters` | Same as above — Milestone 3 |
| Any change to how Exceptions are CONSTRUCTED (`HttpException`, `NewHttpException`, the 5 built-ins) | Already complete, previous feature — this feature only consumes `exception.Exception`, doesn't touch its definition |
| Structured logging of the recovered panic (stack trace capture, `AppLogger` integration) | No `Logger` exists anywhere in ROADMAP.md yet (same scoping reasoning as `AppOptions.LogLevels` in "App Bootstrap & Listen" — captured as config, never wired to real behavior). This feature may `fmt.Print`/no-op on the server side for now; real logging is a future concern |
| A full `HttpStatus` named-constant enum (`HttpStatusOk`, `HttpStatusBadRequest`, etc.) | Still not needed — the recover wrapper reads whatever `int` `Exception.Status()` returns, it doesn't need named constants to do that. Same Out-of-Scope reasoning as "HttpException Core" |

---

## User Stories

### P1: Exception panics produce a real, structured HTTP response ⭐ MVP

**User Story**: As a gonest user, I want `panic(gonest.NewNotFoundException(details))` inside a route `Handler` to produce a real `404` response with `{"name":"NotFoundException","message":"","details":{...}}` in the body, so that my API's error responses are usable by clients without me writing any response-formatting code myself — the same guarantee Nest gives for `throw new NotFoundException()`.

**Why P1**: This is the entire point of the feature — without it, `HttpException Core`'s types exist but are functionally inert from an HTTP client's perspective (a client still just sees a generic 500 no matter what got thrown).

**Acceptance Criteria**:

1. WHEN a route `Handler` panics with a value satisfying `gonest.Exception` THEN system SHALL respond with that exception's own `Status()` code
2. WHEN that happens THEN system SHALL respond with a JSON body shaped exactly `{"name": <Name()>, "message": <Message()>, "details": <Details()>}` (matching INSIGHT.md's own documented body shape verbatim — see the "resposta HTTP quando panica com Exception" comment block in INSIGHT.md's exceptions example)
3. WHEN `Details()` returns `nil` THEN the JSON body's `"details"` field SHALL serialize as JSON `null` (Go's standard `encoding/json` behavior for a nil `any` field — no special-casing needed, just don't accidentally omit or coerce it)
4. WHEN a dev-defined exception (embedding `HttpException`, not one of the 5 built-ins) panics THEN system SHALL treat it identically to a built-in — the recovery logic must key off the `Exception` INTERFACE, never a list of concrete built-in types (this is exactly why "HttpException Core" built `Exception` as a structural interface instead of a closed type-switch)

**Independent Test**: bootstrap a real app (reuse T9's `UserController` example or a new minimal controller) with a route `Handler` that panics with each of the 5 built-ins plus one custom exception type, dispatch a real request via `app.Test`, confirm status code and JSON body match exactly for all 6.

---

### P2: Non-Exception panics keep today's generic, safe 500

**User Story**: As a gonest user, I want a genuine bug (nil pointer, index out of range, an unwrapped `error` I forgot to handle) to still produce a generic 500 with no internal detail leaked, exactly like it does today, so that adding Exception support doesn't accidentally start leaking stack traces or Go-internal panic messages to API clients.

**Why P2**: This is a NON-regression requirement, not new behavior — T7 already guarantees this; this story exists to make the guarantee explicit and testable now that the recover wrapper's logic is more complex (branching on `Exception` vs not), so a future change can't accidentally weaken it.

**Acceptance Criteria**:

1. WHEN a route `Handler` panics with a value that does NOT satisfy `gonest.Exception` (bare `error`, `nil`-pointer deref, `index out of range`, raw string, raw int, etc.) THEN system SHALL respond with a generic 500 and a body that does NOT include the panic value's own message/content (matches T7's existing `"Internal Server Error"` behavior — this feature does not need to change the generic body's exact text, just confirm it stays generic)
2. WHEN this happens THEN system SHALL NOT crash the process (recover() must still catch it, per T7's original guarantee, unchanged)

**Independent Test**: dispatch a request against a route `Handler` that panics with `errors.New("some internal detail")`, confirm the response body does NOT contain the string `"some internal detail"` anywhere, confirm status is 500, confirm the test process doesn't crash.

---

## Edge Cases

- WHEN an `Exception`'s `Status()` returns something outside normal HTTP status conventions (e.g. `0`) THEN system SHALL pass it straight through to the adapter's status-setting call without validation — matches "HttpException Core"'s own decision not to validate `status` at construction time, consistent trust-the-caller posture
- WHEN a route `Handler` panics with a `nil` value (e.g. `panic(nil)`, which Go itself treats specially since Go 1.21 — it becomes a `*runtime.PanicNilError` when recovered, not a bare `nil`) THEN system SHALL treat it as a non-Exception panic (falls to the generic 500 path) — `recover()`'s returned value in this case will not satisfy `gonest.Exception`, so this falls out of the existing type-assertion logic naturally, no special-casing needed, but worth a test to prove it doesn't panic-recovery-loop or crash
- WHEN `ctx.Json(...)` itself fails while writing the Exception body (e.g. `Details()` holds something that can't marshal to JSON, like a channel or a func value) THEN this is the SAME failure mode T7's wrapper already has for `ctx.Json` calls made by ordinary Handler code (not new to this feature) — no new handling required, out of scope to solve here

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| PANIC-01 | P1: Exception panic → its own Status() code | Design | Pending |
| PANIC-02 | P1: Exception panic → {name,message,details} JSON body | Design | Pending |
| PANIC-03 | P1: nil Details() serializes as JSON null | Design | Pending |
| PANIC-04 | P1: dev-defined exceptions treated identically via interface, not type-switch | Design | Pending |
| PANIC-05 | P2: non-Exception panic → generic 500, no leaked detail (non-regression) | Design | Pending |
| PANIC-06 | P2: non-Exception panic → process doesn't crash (non-regression) | Design | Pending |

**ID format:** `PANIC-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 6 total, 0 mapped to tasks yet, 6 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] All 5 built-in exceptions + 1 custom exception, panicked from a real route Handler, produce correct status+body via a real `app.Test` dispatch (not a unit-level check of the recover logic in isolation)
- [ ] The non-regression story (P2) is proven with an explicit test, not just "we didn't touch that code path" — T7's original generic-500 test should still exist AND a new test should prove the branching logic correctly falls through to it
- [ ] Zero regressions in the existing test suite (Milestone 1 complete + "HttpException Core", ~16 packages before this feature starts)
