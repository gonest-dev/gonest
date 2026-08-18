# Logger Specification

## Problem Statement

gonest's own diagnostic output (startup banner, "Modules: N"/"Controllers: N"/"Routes: N",
"Listening on: ...") was hardcoded to a single console format (`internal/logger`, package-level
functions, `fmt.Fprintf(os.Stdout, ...)`) -- no way to plug a different implementation (structured
JSON, remote transport, etc), matching Nest's `NestFactory.create(AppModule, {logger: instance})`.

Worse, tracing every `recover()` in the codebase showed the mismatch runs deeper: gonest's own
internal error paths mostly do NOT log anything server-side when they recover a panic -- they
convert it to an HTTP/GraphQL response (or a bootstrap error) and stop, leaving the operator with
zero server-side trace of what happened. `internal/emitter`/`internal/scheduler` are the only two
call sites that already call `logger.Error` today.

## Goals

- [x] `gonest.Logger` -- public interface, 5 severities (`Error`/`Warn`/`Info`/`Debug`/`Verbose`),
      each accepting an optional structured `meta ...map[string]any`
- [x] `internal/logger` becomes a swappable-instance package (`active Logger`, default
      `consoleLogger` = today's exact format) instead of hardcoded functions -- existing
      package-level `Error`/`Warn`/`Info`/`Debug`/`Verbose` callers (`app.go`, `emitter`,
      `scheduler`) unchanged, now delegate to `active`
- [x] `AppOptions.Logger` -- factory-time swap (`NewApp(root, AppOptions{Logger: instance})`), nil
      keeps `consoleLogger`; wired in `NewApp`/reset to nil in `MustNewTestApp` (no `Options` param
      there, always resets so a custom Logger never leaks across bootstraps in the same process)
- [x] `gonest.GetLogger(optionalNamedContext ...string)` -- returns the active Logger, optionally
      prefixing every line with a caller-chosen string context. Direct package-function access, NOT
      `MustInject` -- callable from anywhere, including inside a Provider's own `Constructor` (where
      `MustInject[SomeInterface]` would panic, Provider-to-Provider dependencies only accept pointer
      `T`)
- [x] `gonest.GetLoggerFor[T any]()` -- same as `GetLogger`, context derived from `T`'s own type name
      via reflect instead of a literal string
- [x] Every `recover()` site across the gonest ecosystem that currently swallows a panic without any
      server-side trace calls `logger.Error`/`GetLogger` before converting it to a response/error --
      see Ecosystem Trace below for the exhaustive list and per-site priority (T3/T5 ended up using
      `logger.GetLogger(runtimeString)` rather than `GetLoggerFor[T]` where the context is a
      runtime-only value -- event `reflect.Type`, job `name` -- not a compile-time type param)

## Ecosystem Trace (every `recover()` in `internal/`, panic-swallowing behavior before this feature)

| # | Site | Swallows panic into | Server-side log today | Priority |
|---|------|---------------------|------------------------|----------|
| 1 | `internal/adapter/fiber/fiber.go:180` (`RegisterRoute`'s wrapped handler) | HTTP response (Exception JSON or generic 500 text) | **none** | HIGH -- every unhandled panic on every REST request in every app |
| 2 | `internal/graphql/generate.go:143` (`Field.Resolve`) | GraphQL `resolveErr` | **none** | HIGH -- every unhandled panic on every Query/Mutation |
| 3 | `internal/graphql/sse_distinct.go:129` (Distinct connection stream) | discarded (`_ = recover()`) | **none** | MEDIUM -- long-lived Subscription connection |
| 4 | `internal/graphql/sse_distinct.go:212` (Distinct connection, 2nd site) | discarded (`_ = recover()`) | **none** | MEDIUM |
| 5 | `internal/graphql/sse_single.go:142` (Single connection stream) | discarded (`_ = recover()`) | **none** | MEDIUM |
| 6 | `internal/graphql/ws_protocol.go:126` (WebSocket transport) | discarded (`_ = recover()`) | **none** | MEDIUM |
| 7 | `internal/resolver/stage3.go:402` (`callConstructor`) | `err`, propagates to bootstrap failure | none directly, but bootstrap panics loudly downstream | LOW -- already surfaces, just not via `Logger` |
| 8 | `internal/provider/lifecycle.go:328` (`invokeHook`) | `err`, propagates to bootstrap/shutdown failure | none directly, same as #7 | LOW |
| 9 | `internal/emitter/emitter.go:90` (listener panic) | already `logger.Error` | yes, old format | LOW -- format-only upgrade to `GetLoggerFor` |
| 10 | `internal/scheduler/scheduler.go:148` (job panic) | already `logger.Error` | yes, old format | LOW -- format-only upgrade to `GetLoggerFor` |

**Excluded on purpose**: `internal/app/app.go:689` (`filteredHandler`) re-panics anything not caught
by a Filter -- it never swallows, the panic always reaches site #1's recover eventually, so logging
there would double-log every unhandled panic.

## Out of Scope

| Item | Reason |
| --- | --- |
| Structured `context`/`trace` params on `Logger`'s methods (Nest's `error(msg, trace?, context?)`) | No concrete use case yet beyond `meta`; add when one appears |
| Async/buffered logging (Nest's `bufferLogs`) | gonest has no window between "container created" and "logger attached" -- `AppOptions.Logger` is available synchronously at `NewApp` |
| Per-request/request-scoped Logger instances | `Logger` is an app-wide singleton (`active`), matching Nest's own default `LoggerService` -- request-scoped logging is a different, unrequested feature |

---

## User Stories

**P1 -- Operator debugging a production panic**
As an operator running a gonest app, when a Handler/Resolver panics, I want a server-side log line
(timestamp, severity, message) so I can find the bug without reproducing it, instead of only seeing
the client-facing error response.

**P1 -- App author swapping the logger implementation**
As an app author, I want to pass my own `Logger` implementation at `NewApp(root, AppOptions{Logger:
myImpl})` so gonest's own diagnostic lines AND my own `GetLogger`/`GetLoggerFor` calls route through
my chosen transport (e.g. structured JSON to stdout, or a remote log sink).

## Independent Test

Given `AppOptions{Logger: spy}` and a route Handler that panics with a non-`Exception` value, when
the request is dispatched, then `spy` recorded exactly one `Error` line containing the panic value,
AND the HTTP response is still the existing generic 500 (response contract unchanged, logging is
additive).
