# HTTP Test Client Specification

**Status: COMPLETE (2026-07-15, commit `96f3d57`).** Design gap noted below (HttpAdapter needing a Test method) resolved exactly as flagged: `Test(req *http.Request) (*http.Response, error)` added to `internal/app.HttpAdapter`, `internal/adapter/fiber.FiberApp.Test` delegates to `*fiber.App.Test`. `AssertStatus`/`AssertJsonPath`'s own "prove it fails the test" cases could not be tested cleanly (a failing `t.Run` sub-test unconditionally marks the parent test failed too, no clean way to intercept that with a real `*testing.T`) -- covered instead by a positive-path test proving the real dispatched status/body flow through correctly.

## Problem Statement

Milestone 8's second and last feature: `tester.MustRequest(method, path, body)` dispatches an HTTP request against a `MustNewTestApp`-built app WITHOUT starting a real network listener, and the returned response gains `AssertStatus`/`AssertJsonPath` test-assertion helpers. INSIGHT.md's own "exemplo de Testing" section already specifies the full call shape verbatim. This feature is BLOCKED on "Test App Bootstrap" (`.specs/features/test-app-bootstrap/`) actually being built first -- it has NOT been implemented yet (spec-only, execution deferred to a future session, see that feature's own HANDOFF note).

## Goals

- [ ] `Tester.MustRequest(method HttpMethod, path string, body any) *TestResponse` -- dispatches an in-memory HTTP request against the app `MustNewTestApp` built (no real port bound), `body` marshaled as JSON if non-nil (mirrors `ctx.Json`'s own encoding), panics on a transport-level failure (never on a non-2xx status -- that's for the caller's own `AssertStatus` to check)
- [ ] `TestResponse.AssertStatus(t *testing.T, want int)` -- fails the test (via `t.Fatalf`/`t.Errorf`, TBD by design) if the actual status doesn't match
- [ ] `TestResponse.AssertJsonPath(t *testing.T, path string, want any)` -- decodes the response body as JSON, extracts the value at `path` (dot-notation, e.g. `"id"`, matching INSIGHT.md's own example -- nested paths like `"address.zip"` are a reasonable extension but not explicitly demonstrated anywhere yet), compares against `want`
- [ ] `Tester.Close()` -- already exists as part of "Test App Bootstrap"'s own `MustNewTestApp` contract (`defer tester.Close()`, INSIGHT.md) -- this feature does not redefine it, just confirms `MustRequest`/response assertions work within its lifecycle

## Out of Scope

| Feature | Reason |
| --- | --- |
| Real network dispatch (an actual bound port) | `MustNewTestApp` explicitly does NOT `Listen` (per "Test App Bootstrap" spec.md P3 AC4) -- `MustRequest` dispatches in-memory, same technique this codebase's OWN test suite already uses throughout (Fiber's `*fiber.App.Test(req)`) |
| Assertion helpers beyond `AssertStatus`/`AssertJsonPath` (e.g. header assertions, raw body string match) | INSIGHT.md's own example is the only concrete requirement source; nothing else is demonstrated |
| `AssertJsonPath`'s dot-notation path syntax supporting array indices (`"tags[0]"`) or wildcards | No example demonstrates this; can be added later if a real need appears, same "don't invent unused API surface" stance as every prior feature this session |

## Known Design Gap (flagged, not resolved -- depends on "Test App Bootstrap" internals not yet built)

`internal/app.HttpAdapter` (the abstract interface `NewApp`/`MustNewApp` bootstrap against) today only exposes `Init()`/`RegisterRoute(...)`/`Listen(...)` -- no in-memory dispatch method. Every existing test in this codebase that dispatches an HTTP request without a real listener does so by reaching the CONCRETE `*fiber.App` (via `internal/adapter/fiber`'s own `.Test(req)`, Fiber's real method) rather than through the abstract `HttpAdapter` interface. For `MustRequest` to work generically (not hardcoded to Fiber), `HttpAdapter` likely needs a NEW method, e.g. `Test(req *http.Request) (*http.Response, error)`, that every adapter implementation (today only Fiber) provides -- Fiber's own version would just delegate to `fiber.App.Test(req)`. This is a real design decision for whoever specifies/designs this feature's own `design.md` (not resolved here, since it depends on reading "Test App Bootstrap"'s ACTUAL implementation once that exists, not this session's still-unbuilt design).

---

## User Stories

### P1: `MustRequest`/`AssertStatus`/`AssertJsonPath`, matching INSIGHT.md verbatim ⭐ MVP

**User Story**: As a gonest user, I want `res := tester.MustRequest(gonest.HttpGet, "/user/42", nil); res.AssertStatus(t, gonest.HttpStatusOk); res.AssertJsonPath(t, "id", int64(42))` (INSIGHT.md's own `TestUserController_Get`) to dispatch against my test app and let me assert on the result.

**Acceptance Criteria**:

1. WHEN `MustRequest` is called with a nil `body` THEN system SHALL dispatch a request with no body (e.g. a `GET`)
2. WHEN `MustRequest` is called with a non-nil `body` THEN system SHALL JSON-encode it as the request body, same encoding `ctx.Json`/`MustJsonBody` already use elsewhere in this codebase
3. WHEN `AssertStatus(t, want)` is called AND the actual status differs THEN the test SHALL fail with a clear message showing both values
4. WHEN `AssertJsonPath(t, path, want)` is called THEN system SHALL decode the response body as JSON, extract the value at `path`, and fail the test (clear message) if it doesn't equal `want`

**Independent Test**: reproduce INSIGHT.md's `TestUserController_Get` verbatim (override via "Test App Bootstrap"'s `MustOverride`, dispatch, assert both status and JSON path) end-to-end, once "Test App Bootstrap" itself is implemented.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| HTC-01 | P1: MustRequest dispatches in-memory, JSON-encodes non-nil body | Tasks | Pending |
| HTC-02 | P1: AssertStatus fails test on mismatch | Tasks | Pending |
| HTC-03 | P1: AssertJsonPath decodes+extracts+compares | Tasks | Pending |

**ID format:** `HTC-[NUMBER]`

**Coverage:** 3 total, 0 mapped yet.

---

## Success Criteria

- [ ] INSIGHT.md's `TestUserController_Get` reproduced end-to-end
- [ ] Zero regressions in existing test suite

---

## Blocking Dependency

**This feature cannot be designed in full detail (design.md) or implemented until "Test App Bootstrap" (`.specs/features/test-app-bootstrap/`) is actually built** -- `Tester`'s real shape, `HttpAdapter`'s real interface, and how `MustNewTestApp` actually registers routes are all currently only DESIGNED (not implemented). Whoever picks up "Test App Bootstrap" execution should specify THIS feature's `design.md` immediately after "Test App Bootstrap"'s own T4 closes, while the fresh implementation details are still front-of-mind -- not from this spec.md alone, which was deliberately kept at the requirements level for exactly this reason.
