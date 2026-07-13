# App Bootstrap & Listen Design

**Spec**: `.specs/features/app-bootstrap-listen/spec.md`

## Architecture Overview

```
gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})
        │
        ▼
internal/app.NewApp[T, PT](root, opts)   -- Stages 1-3 + 2.5 unchanged (T1-T9)
        │                                    opts stored on *App, otherwise inert
        ▼
     *App{root, adapter, opts}
        │
        │  app.MustListen(addr, gonest.OnListen(fn))
        ▼
   adapter.Listen(addr, onListenFunc)   -- HttpAdapter.Listen gains a 2nd param
        │
        ▼
   *fiberapp.FiberApp.Listen            -- wires fiber's native Hooks().OnListen
        │                                  BEFORE the blocking fiber.App.Listen(addr) call
        ▼
   real HTTP server bound, onListenFunc fires once, then blocks
```

No new DI-graph concept here (no Provider/Module/Controller-shaped type) — this feature only extends the existing `App`/`HttpAdapter` surface built in T1-T9. No new `internal/*` package: `AppOptions`/`LogLevel`/`OnListen` are App's own bootstrap config, co-located with `internal/app` the same way `HttpMethod` is co-located with `internal/route` (config for the thing, not a graph participant of its own).

---

## Components

### AppOptions / LogLevel

- **Purpose**: Nest-parity bootstrap config struct (`BufferLogs`, `LogLevels`), captured but inert per spec.md's Out of Scope (no `Logger` exists yet).
- **Location**: `internal/app/options.go` (new file), re-exported at root via `app.go`'s existing type-alias pattern.
- **Interfaces**:
  - `type AppOptions struct { BufferLogs bool; LogLevels []LogLevel }`
  - `type LogLevel int` with `LogLevelError`/`LogLevelWarn`/`LogLevelLog`/`LogLevelDebug`/`LogLevelVerbose` (Nest's 5 standard levels) + `String()` (same debug-friendly pattern as `HttpMethod.String()` in `internal/route/method.go`)
- **Dependencies**: none
- **Reuses**: `HttpMethod`'s enum+`String()` shape (T1)

### App (extended)

- **Purpose**: `internal/app.App` gains an `opts AppOptions` field (stored, unused beyond storage) and a new `MustListen` method.
- **Location**: `internal/app/app.go` (existing, extended)
- **Interfaces**:
  - `NewApp[T any, PT httpAdapterPtr[T]](root *module.Module, opts AppOptions) (*App, error)` — **breaking signature change** from T8's `NewApp[T, PT](root)`; every existing call site (T8/T9's tests, root `app.go`'s wrapper) must pass `AppOptions{}` at minimum
  - `MustNewApp[T, PT](root *module.Module, opts AppOptions) *App` — same signature extension
  - `func (a *App) MustListen(addr string, onListen OnListen)` — new method, panics if the adapter's `Listen` returns an error
- **Dependencies**: unchanged (`internal/module`, `internal/resolver`, `internal/inject`, `internal/route`, `internal/httpctx`) plus its own new `options.go`
- **Reuses**: existing `HttpAdapter`/`httpAdapterPtr` generic-constraint machinery from T8 — extended, not replaced

### OnListen

- **Purpose**: nil-able functional-option type wrapping the "bind succeeded" callback — matches INSIGHT.md's exact two call shapes: `gonest.OnListen(func(){...})` and literal `nil`.
- **Location**: `internal/app/options.go` (same file as AppOptions — same "app bootstrap config" grouping)
- **Interfaces**: `type OnListen func()`
- **Dependencies**: none

### HttpAdapter.Listen (extended contract)

- **Purpose**: the adapter's `Listen` needs a way to signal "bind succeeded" back to `App.MustListen` before the call blocks for good.
- **Location**: `internal/app/app.go`'s `HttpAdapter` interface (existing, T8) — signature changes from `Listen(addr string) error` to `Listen(addr string, onListen func()) error`. `onListen` may be `nil` (no hook wanted); implementations must guard against calling a nil func.
- **Fiber implementation**: `internal/fiberapp/fiberapp.go`'s `(*FiberApp).Listen` registers `onListen` via Fiber's own `f.app.Hooks().OnListen(func(fiber.ListenData) error { onListen(); return nil })` (only if `onListen != nil`) BEFORE calling the blocking `f.app.Listen(addr)` — Fiber's hook fires synchronously once its own bind succeeds, from inside `Listen`'s own goroutine setup, which is exactly the "before the block becomes effectively permanent" semantics spec.md's AC3 requires. Verified this hook exists via Context7 during T1's original Fiber v3 confirmation; re-verify exact `Hooks().OnListen` signature against `go.sum`'s vendored Fiber v3 source during implementation (Knowledge Verification Chain Step 1/3) since this feature hasn't touched it yet.

---

## Data Models

```go
type AppOptions struct {
    BufferLogs bool
    LogLevels  []LogLevel
}

type LogLevel int
const (
    LogLevelError LogLevel = iota
    LogLevelWarn
    LogLevelLog
    LogLevelDebug
    LogLevelVerbose
)

type OnListen func()
```

**Relationships**: `AppOptions` is a pure value passed once into `NewApp`/`MustNewApp` and stored on `*App` — no other component reads it in this feature. `OnListen` is passed once into `MustListen` per call, never stored.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| `adapter.Listen(addr, onListen)` returns an error (e.g. port in use) | `MustListen` panics with the wrapped error, same "Must"-prefixed convention as `MustNewApp`/`MustInject`/`MustParam` | Process exits (or caller's own `recover`, if any) — no silent failure |
| `onListen` is `nil` | `HttpAdapter` implementations (Fiber) skip registering any hook; `App.MustListen` passes `nil` straight through, no wrapping needed since `func()` is already nil-safe to check | Identical behavior to not having Listen/OnListen at all, just blocks |
| `NewApp`'s existing Stage 1-3 + 2.5 errors (unchanged from T1-T9) | Unchanged — `AppOptions` is captured after all bootstrap stages, doesn't influence any of them | No behavior change from T9 baseline |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Where `AppOptions`/`LogLevel`/`OnListen` live | `internal/app/options.go`, not a new `internal/appoptions` package | AD-004's "1 package per DI-graph type" applies to graph participants (Provider/Module/Controller/Route) that get resolved/walked/collided against each other. `AppOptions` is inert config with zero relationships to the graph — same category as `HttpMethod` living inside `internal/route` rather than its own package. A dedicated package would be over-engineering for 3 small types with no cross-package reuse need. |
| `HttpAdapter.Listen` signature change (breaking, not additive) | Change `Listen(addr string) error` → `Listen(addr string, onListen func()) error` directly, rather than adding a second method (`ListenWithHook` or similar) | `Listen` was declared in T8 but never called by any shipped code path (T8's own doc comment says "Not exercised by NewApp[T] itself -- reserved for the App Bootstrap & Listen feature that follows") — there is no external caller to break. A second method would leave a dead, never-right `Listen(addr string) error` on the public contract forever. |
| `NewApp`/`MustNewApp` signature change (breaking) | Add `opts AppOptions` as a required 2nd positional param, matching INSIGHT.md's own call sites exactly (`gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{...})` and `gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})` — always passed, never omitted, even as a zero value) | Ground truth is INSIGHT.md's literal call sites — not optional/variadic. Every T8/T9 test call site needs a one-line update (`AppOptions{}`) — small, mechanical, called out explicitly as a Tasks-phase migration item so it isn't missed. |
| `OnListen` fire timing implementation | Rely on Fiber v3's own `Hooks().OnListen` rather than gonest spawning its own goroutine + polling/racing against `fiber.App.Listen`'s internal bind | Fiber already solves "run this exactly when the listener is bound, before the blocking accept loop takes over" correctly and race-free internally; reimplementing that with a goroutine+channel in gonest would duplicate Fiber's own synchronization for no benefit and risk a race Fiber doesn't have. |

---

## Open Questions pra Tasks

- Exact Fiber v3 `Hooks().OnListen` callback signature (`func(fiber.ListenData) error` vs something else) needs re-confirming against the vendored version in `go.sum` (or Context7) at implementation time — design above is written from T1's Context7 memory, not re-verified this session.
- Whether `MustListen`'s panic message should include the addr (`"gonest: failed to listen on %q: %v"`) — follow existing panic-message conventions from `MustInject`/`MustParam` at implementation time, no need for a separate decision round.
