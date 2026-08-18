# Logger Tasks

**Status: COMPLETE** (T1-T5, this session). `go build ./...`/`go vet ./...`/`go test ./... -race
-count=1` green, 25 packages, zero pre-existing assertion changed (T5's format upgrade had no
existing test asserting the old message string, confirmed via grep before editing).


Core mechanism (`gonest.Logger`, `internal/logger`'s swappable `active`, `AppOptions.Logger`,
`GetLogger`/`GetLoggerFor`) already **COMPLETE** -- see spec.md's Goals, checked items. Remaining
work is wiring `logger.Error`/`GetLoggerFor` into every silent `recover()` site traced in spec.md's
Ecosystem Trace table. Execute in the order below (HIGH before MEDIUM before LOW) -- each task ends
with `go build ./... && go vet ./... && go test ./... -race -count=1` green before starting the next.

## T1 -- `internal/adapter/fiber/fiber.go:180` (HTTP dispatch, HIGH)

Wrap the raw-panic branch (the `else` after the `exception.Exception` type-assert fails) with
`logger.Error(...)` before writing the generic 500 -- include the panic value and, if derivable, the
request method/path (`fc.Method()`/`fc.Path()`) as `meta`. Do NOT log the `exception.Exception`
branch -- those are expected business errors (400/404/etc), not bugs; logging every one would be
noise, not diagnostic signal. New regression test in `internal/adapter/fiber/fiber_test.go`: a route
Handler that panics a bare `error` (non-Exception) → assert `logger`'s captured output (via
`SetOutput`, same pattern `internal/logger/logger_test.go` already uses) contains the panic message,
AND the HTTP response is still the unchanged 500 text.

## T2 -- `internal/graphql/generate.go:143` (GraphQL resolver, HIGH)

Same split as T1: log only the raw-panic branch (`resolveErr = fmt.Errorf("gonest: panic in resolver
%q: %v", name, r)`), not the `exception.Exception` branch. Include `name` (the field name) as context
via `logger.GetLoggerFor` is not applicable here (no Go type to key on) -- use
`logger.Error(fmt.Sprintf("panic in resolver %q: %v", name, r))` or equivalent structured `meta`.
Regression test in `internal/graphql/generate_test.go`.

## T3 -- GraphQL Subscription transports, discarded recover (MEDIUM)

4 sites, same fix each: `internal/graphql/sse_distinct.go:129`, `sse_distinct.go:212`,
`sse_single.go:142`, `ws_protocol.go:126`. Currently `defer func() { _ = recover() }()` -- change to
capture the value and call `logger.Error` when non-nil, keep the "must not crash the process"
behavior (still recovers, still doesn't re-panic). One regression test per file is enough (4 total),
same "trigger the panic inside Handler, assert captured log line" shape as T1.

## T4 -- Bootstrap-time panic sites, format-only (LOW)

`internal/resolver/stage3.go:402` (`callConstructor`) and `internal/provider/lifecycle.go:328`
(`invokeHook`) already convert panic to `err` and it already propagates loudly (bootstrap fails). Add
a `logger.Error` call alongside the existing `err = fmt.Errorf(...)` so the SAME message also reaches
the structured logger before the process exits -- no behavior change, purely additive. No new test
required beyond confirming existing tests for these 2 functions still pass (their error-return
contract is unchanged).

## T5 -- Emitter/Scheduler, format upgrade (LOW)

`internal/emitter/emitter.go:90` and `internal/scheduler/scheduler.go:148` already call
`logger.Error(...)` (old, contextless). Upgrade both to `logger.GetLoggerFor[T]()` (`T` = the
listener/job's own type, already available at each call site) so their panic lines get the same
`[TypeName]` context prefix every other ecosystem site now has. Existing tests for these 2 packages
already assert on the log line content (per STATE.md's Milestone 9/10 history) -- update assertions
for the new `[TypeName]` prefix, do not weaken them.

## Gate (every task)

`go build ./...` / `go vet ./...` / `go test ./... -race -count=1` all green, zero pre-existing
assertion changed except the 2 named in T5. Update `.specs/project/STATE.md` (Current Work + a new AD
entry) and this file's checkboxes in the SAME commit as the last task, per the Spec Gate guarantee.
