# App Bootstrap & Listen Specification

## Problem Statement

`NewApp[T]`/`MustNewApp[T]` (built in "Controller & Route Registration") already bootstrap the DI graph and register every route on the adapter, but the app never actually starts serving: `HttpAdapter.Listen` exists on the contract yet nothing in the public API calls it, and there is no way to configure basic bootstrap options (matching Nest's `NestFactory.create(AppModule, { bufferLogs, logger })`) or to know exactly when the server is ready to accept traffic (`OnListen`, since Go's `Listen` blocks the calling goroutine unlike Nest's `await app.listen()`).

## Goals

- [ ] `gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})` bootstraps the DI graph (unchanged from T1-T9) AND accepts basic app-level options
- [ ] `app.MustListen(addr string, onListen gonest.OnListen)` blocks the calling goroutine serving real HTTP traffic (Go-idiomatic, like `http.ListenAndServe`), and `onListen`'s callback (if non-nil) runs exactly once, right after the bind succeeds, before the block becomes effectively permanent

## Out of Scope

Explicitly excluded — present in INSIGHT.md's full bootstrap example but not part of this feature (either a later milestone, or not roadmapped at all yet):

| Feature | Reason |
| --- | --- |
| `UseLogger(logger)`, real log emission gated by `LogLevels` | No `Logger` interface/implementation exists anywhere in ROADMAP.md yet — `AppOptions.LogLevels`/`BufferLogs` are captured as config (Nest parity for people porting bootstrap code) but do not drive any actual logging behavior in this feature. Precedent: T6's `Use`/`Guards`/`Interceptors`/`Filters` stubs (store-only, no-op) — same treatment here. |
| `UseGlobalFilters`, `SetGlobalPrefix`, `EnableCors`, `Use(Helmet)` | Belong to Milestone 2 (Exceptions) / Milestone 3 (Middleware) per ROADMAP.md — not part of "App Bootstrap & Listen"'s stated scope (`NewApp`/`MustNewApp`, `AppOptions`, `MustListen`, `OnListen` only, see ROADMAP.md Milestone 1) |
| `NewOpenApiDocument`/`SetupSwagger` | Milestone 7 |
| `app.Close()`, `MustInject[T](app)` (App as an injection Owner) | Not mentioned in ROADMAP.md's scope line for this feature; App is not currently a `module.Owner` and making it one is a separate design decision with its own blast radius (would let arbitrary code resolve providers post-bootstrap outside any Controller/Provider builder fn) — deferred |

---

## User Stories

### P1: Configure and start the server ⭐ MVP

**User Story**: As a gonest user, I want to pass `AppOptions` to `NewApp`/`MustNewApp` and then call `app.MustListen(addr, onListen)` so that my process actually serves HTTP traffic, the same way `NestFactory.create` + `app.listen()` does in Nest.

**Why P1**: Without this, every feature built so far (DI graph, routes, real Fiber dispatch) is exercised only through `app.Test(req)` in tests — there is no way to actually run a gonest app as a real process. This is the last piece of Milestone 1's stated goal ("primeiro `go run` funcional... respondendo `/user/:id`").

**Acceptance Criteria**:

1. WHEN `NewApp[T, PT](root, opts)` is called with a valid `AppOptions` value THEN system SHALL bootstrap exactly as `NewApp` already does (Stages 1-3 + 2.5, unchanged) and additionally store `opts` on the returned `*App`
2. WHEN `app.MustListen(addr, onListen)` is called after a successful `NewApp` THEN system SHALL bind the real adapter (e.g. Fiber) to `addr` and block the calling goroutine for the lifetime of the process (or until the underlying `Listen` returns/errors)
3. WHEN the bind succeeds THEN system SHALL invoke `onListen`'s callback exactly once, before the blocking phase becomes the caller's only observable state (i.e. logging/side-effects inside the callback are guaranteed to run)
4. WHEN `onListen` is `nil` (no callback wanted) THEN system SHALL skip invoking anything and just block normally, same as if no hook were configured (see INSIGHT.md's own `app.MustListen(":3000", nil)` example)
5. WHEN the underlying bind fails (e.g. port already in use) THEN system SHALL panic with a clear message (matching `MustListen`'s "Must"-prefixed panic-on-error convention already established by `MustNewApp`/`MustInject`/`MustParam`)

**Independent Test**: bootstrap a real app with `AppModule` (reuse T9's `UserController` example), call `app.MustListen` in a goroutine against a free port, confirm the `OnListen` callback fires, then confirm a real HTTP request against that port (not `app.Test`, an actual `net/http` client dial) gets a correct response; shut down cleanly at test end.

---

### P2: `AppOptions` captures Nest-parity bootstrap config

**User Story**: As a gonest user porting bootstrap code from Nest, I want `AppOptions{BufferLogs, LogLevels}` to exist with the same field shape as Nest's `NestFactory.create(AppModule, { bufferLogs, logger })`, so that my bootstrap function signature looks familiar even though the logging behavior isn't wired yet.

**Why P2**: Not required for the server to actually start (P1 covers that with a zero-value `AppOptions{}`, per INSIGHT.md's own `gonest.AppOptions{}` example) — this is about API-surface parity so later milestones can wire real behavior into an already-stable struct shape without a breaking change.

**Acceptance Criteria**:

1. WHEN `AppOptions{}` (zero value) is passed to `NewApp`/`MustNewApp` THEN system SHALL bootstrap successfully with no behavior difference from omitting logging config entirely
2. WHEN `AppOptions{BufferLogs: true, LogLevels: []gonest.LogLevel{...}}` is passed THEN system SHALL accept and store the value without error (no validation beyond the type system — no real logging pipeline exists yet to validate against)
3. WHEN `LogLevel` values are referenced (e.g. `gonest.LogLevelError`, `gonest.LogLevelWarn`) THEN system SHALL provide them as a defined enum type, consistent with the existing `HttpMethod` enum pattern (`internal/route/method.go`) — debug-friendly `String()`, no runtime semantics attached yet

**Independent Test**: construct `AppOptions{BufferLogs: true, LogLevels: []gonest.LogLevel{gonest.LogLevelError, gonest.LogLevelWarn}}`, pass to `NewApp`, confirm bootstrap succeeds identically to passing `AppOptions{}` (same routes registered, same DI graph resolved) — proves the fields are captured but inert.

---

## Edge Cases

- WHEN `MustListen` is called on an `*App` whose `NewApp` call already failed (i.e. code reaches `MustListen` on a nil `*App`, which should be unreachable given `MustNewApp` already panics on bootstrap failure) THEN this is a caller-error scenario, not handled specially — same "don't call methods on a nil pointer" contract as the rest of Go
- WHEN `addr` is malformed or the port is unavailable THEN system SHALL panic via `MustListen` with the underlying adapter error wrapped in a clear message (see AC5 above) — no swallowed errors
- WHEN `NewApp` (non-generic-panic variant, i.e. the `(*App, error)`-returning form, not `MustNewApp`) bootstraps successfully but the caller never calls `Listen`/`MustListen` at all THEN system SHALL behave exactly as it does today (T8) — `Listen`/`MustListen` are additive, no forced coupling

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| BOOT-01 | P1: NewApp accepts AppOptions | Design | Pending |
| BOOT-02 | P1: MustListen blocks, binds real adapter | Design | Pending |
| BOOT-03 | P1: OnListen callback fires once, before block | Design | Pending |
| BOOT-04 | P1: nil OnListen is a no-op | Design | Pending |
| BOOT-05 | P1: bind failure panics clearly | Design | Pending |
| BOOT-06 | P2: AppOptions{} zero value works | Design | Pending |
| BOOT-07 | P2: AppOptions fields captured, inert | Design | Pending |
| BOOT-08 | P2: LogLevel enum, HttpMethod-pattern | Design | Pending |

**ID format:** `BOOT-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 8 total, 0 mapped to tasks yet, 8 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] A real `go run` of an app built with `AppModule`/`UserController` (T9's example) actually serves HTTP requests on a real port, confirmed via a real `net/http` client dial in a test
- [ ] `OnListen` callback is proven to run before the block, not after (test asserts observable side-effect from the callback before doing anything else)
- [ ] Zero regressions in T1-T9's existing 12-package test suite (`NewApp[T, PT]`'s existing signature/behavior for DI+route registration must not change from the caller's perspective when `AppOptions{}` is passed)
