# Lifecycle Hooks Tasks

**Design**: `.specs/features/lifecycle-hooks/design.md`
**Status**: Done -- T1-T7 all Complete, `go test ./... -race` green (25 packages), `.examples/lifecycle-hooks` demonstrates the flow live (real curl + real hook console output). See STATE.md's AD-044.

---

## Execution Plan

### Phase 1: Foundation (Parallel OK)

```
T1 [P] ──┐
T3 [P] ──┤ (independent files/packages, no shared state)
```

### Phase 2: Provider-Hook Consumers (Sequential)

```
T1 ──→ T2
T1 ──→ T4
```

### Phase 3: Shutdown Orchestrator (Sequential)

```
T2, T3, T4 ──→ T5
```

### Phase 4: Wiring (Sequential)

```
T4, T5 ──→ T6
```

### Phase 5: End-to-End Proof (Sequential)

```
T6 ──→ T7
```

---

## Task Breakdown

### T1: Provider no-signal lifecycle hooks (OnModuleInit/OnApplicationBootstrap/OnModuleDestroy) [P]

**What**: New file `internal/provider/lifecycle.go` adding 3 registration methods
(`OnModuleInit(fn any)`, `OnApplicationBootstrap(fn any)`, `OnModuleDestroy(fn any)`) on `*Provider`,
each validating `fn` against the 4 accepted shapes (`func(T)` / `func(T) error` /
`func(T, context.Context)` / `func(T, context.Context) error`), plus the 3 corresponding invocation
methods (`RunModuleInit(ctx) error`, `RunApplicationBootstrap(ctx) error`, `RunModuleDestroy(ctx) error`)
that call the stored hook against `p.ResolvedValue()`.

**Where**: `internal/provider/lifecycle.go` (new), `internal/provider/lifecycle_test.go` (new)

**Depends on**: None

**Reuses**:
- `isValidConstructorSignature`'s reflect-based shape-check pattern (`internal/provider/provider.go:181-208`)
  — same `NumIn`/`NumOut`/`contextType`/`errorType` checks, extended to accept a bare `func(T)` (no
  error return) in addition to Constructor's own 2-of-4 no-context shapes (Constructor never allows
  zero-arg + zero-return; these hooks must, since `func(T)` is a valid hook shape)
- `callConstructor`'s panic-recovery + optional-context-arg + optional-error-return call convention
  (`internal/resolver/stage3.go:344-380`) — mirror into a shared unexported `invokeHook(ctx
  context.Context, fn reflect.Value, resolved reflect.Value, phaseName string) error` helper in the new
  file, called by all 3 `RunXxx` methods (and reused by T2's 2 signal hooks)
- `Provider.ResolvedValue()` (existing, `internal/provider/provider.go:128-135`) — read-only source of
  the instance passed as the hook's first argument
- `Provider.ResolvedScope()` (existing) + `internal/scope.Singleton` — every `RunXxx` no-ops
  (`return nil`) when `p.ResolvedScope() != scope.Singleton`, or when the hook was never registered, or
  when `ResolvedValue()` reports not-yet-set (defensive)

**Requirement**: LIFEC-01, LIFEC-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `OnModuleInit(fn any)` / `OnApplicationBootstrap(fn any)` / `OnModuleDestroy(fn any)` panic with
      `"gonest: invalid OnModuleInit signature"` (etc., one message per method name) on any shape outside
      the 4 accepted forms
- [ ] `RunModuleInit`/`RunApplicationBootstrap`/`RunModuleDestroy` invoke the stored hook exactly once
      with the provider's `ResolvedValue()` (and `ctx`, if the registered shape accepts it), return `nil`
      when never registered, and return `nil` without invoking anything when
      `ResolvedScope() != scope.Singleton`
- [ ] A hook panic is recovered and converted to
      `"gonest: provider for type %s panicked during OnModuleInit: %v"` (etc., phase name substituted)
- [ ] A hook returning a non-nil `error` is propagated unchanged by the corresponding `RunXxx`
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: existing `internal/provider` test count + new tests, all passing (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(provider): add OnModuleInit/OnApplicationBootstrap/OnModuleDestroy hooks`

---

### T2: Provider signal lifecycle hooks (BeforeApplicationShutdown/OnApplicationShutdown)

**What**: Extend `internal/provider/lifecycle.go` (from T1) with 2 more registration methods
(`BeforeApplicationShutdown(fn any)`, `OnApplicationShutdown(fn any)`) accepting the same 4 shapes as T1
PLUS a trailing `signal string` parameter (`func(T, string)` / `func(T, string) error` /
`func(T, context.Context, string)` / `func(T, context.Context, string) error`), plus their invocation
counterparts `RunBeforeApplicationShutdown(ctx, signal string) error` /
`RunApplicationShutdown(ctx, signal string) error`.

**Where**: `internal/provider/lifecycle.go` (modify, same file as T1), `internal/provider/lifecycle_test.go`
(modify, same file as T1)

**Depends on**: T1 (same file; reuses T1's `invokeHook` helper, extended to accept an optional trailing
signal argument)

**Reuses**: T1's `invokeHook` helper (extend its signature to take an optional `signal *string`, or add a
sibling `invokeSignalHook` that mirrors it with the extra parameter — implementer's choice, whichever
avoids duplicating the panic-recovery/optional-context logic)

**Requirement**: LIFEC-01, LIFEC-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `BeforeApplicationShutdown(fn any)` / `OnApplicationShutdown(fn any)` panic with
      `"gonest: invalid BeforeApplicationShutdown signature"` (etc.) on any shape outside the 4 accepted
      signal-carrying forms
- [ ] `RunBeforeApplicationShutdown`/`RunApplicationShutdown` invoke the stored hook exactly once with
      the provider's `ResolvedValue()`, `ctx` (if accepted), and `signal` (always, as the last arg),
      same no-op rules as T1 (never registered / non-Singleton scope)
- [ ] A hook panic is recovered and converted to
      `"gonest: provider for type %s panicked during OnApplicationShutdown: %v"` (etc.)
- [ ] A hook returning a non-nil `error` is propagated unchanged
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: T1's test count + new tests for the 2 signal hooks, all passing (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(provider): add BeforeApplicationShutdown/OnApplicationShutdown hooks`

---

### T3: HttpAdapter.Shutdown + FiberApp.Shutdown + test fake updates [P]

**What**: Add `Shutdown(ctx context.Context) error` to the `HttpAdapter` interface
(`internal/app/app.go`), implement it on `*fiber.FiberApp` via Fiber v3's real
`f.app.ShutdownWithContext(ctx)`, and add the same method (no-op or recording) to every existing
`HttpAdapter`-satisfying test fake so the package keeps compiling.

**Where**: `internal/app/app.go` (modify, `HttpAdapter` interface only), `internal/adapter/fiber/fiber.go`
(modify), `internal/app/app_test.go` (modify: `recordingFakeAdapter` and `listenSpyAdapter`),
`internal/adapter/fiber/*_test.go` (new or extended test proving real shutdown)

**Depends on**: None

**Reuses**: `FiberApp.Listen`'s existing real-bind pattern (`internal/adapter/fiber/fiber.go:264-277`) as
the setup half of the new test — bind a real port, then call `Shutdown(ctx)` and confirm `Listen`
unblocks, same "never `time.Sleep`, always channel/waitgroup + `t.Cleanup`" convention TESTING.md's
"Bind/Listen real" row documents

**Requirement**: LIFEC-04, LIFEC-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `HttpAdapter` interface has a new `Shutdown(ctx context.Context) error` method
- [ ] `FiberApp.Shutdown` calls `f.app.ShutdownWithContext(ctx)` and returns its error unchanged
- [ ] `recordingFakeAdapter` and `listenSpyAdapter` (`internal/app/app_test.go`) both gain a
      `Shutdown(ctx context.Context) error` method so `internal/app` keeps compiling with the wider
      interface
- [ ] A real test binds a real port via `FiberApp.Listen`, calls `Shutdown(ctx)` from another goroutine,
      and asserts `Listen` returns (no error, or `http.ErrServerClosed`-equivalent — whatever Fiber's
      `ShutdownWithContext` actually returns on a clean shutdown, confirmed by running it, not assumed)
      within a bounded wait (channel-synchronized, no `time.Sleep`)
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: existing `internal/adapter/fiber` test count + 1 new Shutdown test, all passing (no
      silent deletions)

**Tests**: integration
**Gate**: full

**Commit**: `feat(adapter): add HttpAdapter.Shutdown, implement on FiberApp`

---

### T4: internal/app bootstrap-time phase runners (Init/Bootstrap)

**What**: New file `internal/app/lifecycle.go` with 2 functions:
`runModuleInitPhase(ctx context.Context, modules []*module.Module) error` and
`runApplicationBootstrapPhase(ctx context.Context, modules []*module.Module) error`. Both walk
`modules` LEAF-FIRST (a reversed copy of the slice — `modules` itself comes root-first from
`root.Assemble()`), and within each module, its `OwnProviders()` in declaration order; for each
provider that satisfies a small unexported interface exposing `RunModuleInit(ctx) error` (or
`RunApplicationBootstrap`), call it and return immediately on the first non-nil error (fail-fast,
aborts the WHOLE phase, not just the current module's remaining providers).
`runApplicationBootstrapPhase` only ever runs after `runModuleInitPhase` has already returned nil (that
sequencing is wired in T6, not here — this task only defines the 2 functions).

**Where**: `internal/app/lifecycle.go` (new), `internal/app/lifecycle_test.go` (new)

**Depends on**: T1 (needs `Provider.RunModuleInit`/`RunApplicationBootstrap`, via an unexported
interface type-assertion, same style as `declarable` in `app.go:740`)

**Reuses**:
- `declareProviders`'s "walk modules, walk `OwnProviders()`, type-assert, call" loop shape
  (`internal/app/app.go:749-757`)
- `Module.OwnProviders()` (existing, `internal/module/module.go:207-209`)
- The reversed-copy-of-`root.Assemble()`'s-output technique described in design.md's "Module traversal
  order source" Tech Decision — no new traversal, just `for i := len(modules) - 1; i >= 0; i--`

**Requirement**: LIFEC-01, LIFEC-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `runModuleInitPhase`/`runApplicationBootstrapPhase` visit `modules` in reverse (leaf-first) order
- [ ] Within one module, providers run in `OwnProviders()` declaration order
- [ ] The first provider hook to return a non-nil error stops the phase immediately — later
      providers/modules in the SAME phase call do not run
- [ ] A provider with no matching hook registered is silently skipped (no panic, no error)
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: existing `internal/app` test count + new tests for both phase runners, all passing (no
      silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(app): add runModuleInitPhase/runApplicationBootstrapPhase`

---

### T5: internal/app shutdown orchestrator + destroy-phase runners

**What**: Extend `internal/app/lifecycle.go` (from T4) with 3 more phase-runner functions
(`runModuleDestroyPhase`, `runBeforeApplicationShutdownPhase`, `runApplicationShutdownPhase` — the last
taking an extra `signal string` param), all walking `modules` ROOT-FIRST (as-is, no reversal), same
fail-fast shape as T4's 2 functions. Also add `(a *App) EnableShutdownHooks() *App`,
`(a *App) Close(ctx context.Context) error`, and the unexported `(a *App) shutdown(ctx context.Context,
signal string) error` orchestrator described in design.md's "internal/app phase runners" component:
`sync.Once`-guarded, always calls `a.adapter.Shutdown(ctx)` first, then — only if
`a.shutdownHooksEnabled` — runs the 3 destroy phases in strict sequence (an error in any phase aborts
the remaining phases too), stores the result in `a.shutdownErr`, and closes `a.shutdownDone`.
`EnableShutdownHooks` additionally starts a goroutine registering `os.Interrupt` + `syscall.SIGTERM` via
`signal.Notify`, invoking `a.shutdown(context.Background(), signalName)` on receipt.

**Where**: `internal/app/lifecycle.go` (modify, same file as T4), `internal/app/lifecycle_test.go`
(modify, same file as T4)

**Depends on**: T2 (needs `Provider.RunModuleDestroy`/`RunBeforeApplicationShutdown`/
`RunApplicationShutdown`), T3 (needs `HttpAdapter.Shutdown` on `a.adapter`), T4 (same file; `App` struct
fields `shutdownHooksEnabled`/`shutdownOnce`/`shutdownDone`/`shutdownErr` referenced here are added to
`App` itself in T6, but this task's functions/methods reference them by name — coordinate field names
exactly as design.md lists them so T6's struct literal lines up)

**Reuses**:
- T4's leaf-first/root-first traversal pattern, mirrored with NO reversal for these 3
- `declareProviders`-style walk-and-type-assert loop shape, same as T4
- `sync.Once` (stdlib) for the guard, matching the "no double-execution" rule in design.md's Error
  Handling Strategy table

**Requirement**: LIFEC-04, LIFEC-05, LIFEC-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `runModuleDestroyPhase`/`runBeforeApplicationShutdownPhase`/`runApplicationShutdownPhase` visit
      `modules` root-first (unreversed), same fail-fast contract as T4
- [ ] `shutdown(ctx, signal)` always calls `a.adapter.Shutdown(ctx)` first, regardless of
      `shutdownHooksEnabled`
- [ ] When `shutdownHooksEnabled` is true, the 3 destroy phases run in strict sequence
      (`ModuleDestroy → BeforeApplicationShutdown → ApplicationShutdown`), and an error from any one of
      them aborts the remaining phases (verify via a hook returning an error in phase 1 and asserting
      phase 2/3's hooks never ran)
- [ ] When `shutdownHooksEnabled` is false, `shutdown` still calls `adapter.Shutdown(ctx)` but none of
      the 3 destroy phases run
- [ ] Calling `shutdown` (directly, or via `Close`) twice runs the destroy sequence only once (`sync.Once`
      proven via a call counter on a fake), and the second call returns the SAME error the first computed
- [ ] `EnableShutdownHooks()` returns the same `*App` it was called on (chainable)
- [ ] `Close(ctx)` calls `shutdown(ctx, "")` and returns its error
- [ ] `syscall.SIGTERM` reference compiles clean on this Windows dev/CI environment (confirmed by the
      gate check below actually building/running, not assumed per design.md's Open Follow-ups)
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: T4's test count + new tests for the 3 destroy runners + `Close`/`EnableShutdownHooks`/
      `shutdown`, all passing (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(app): add shutdown orchestrator and destroy-phase runners`

---

### T6: Wire lifecycle hooks into NewApp/Listen

**What**: Modify `internal/app/app.go`: (1) add `modules []*module.Module`, `shutdownHooksEnabled bool`,
`shutdownOnce sync.Once`, `shutdownDone chan struct{}`, `shutdownErr error` fields to the `App` struct;
(2) in `NewApp`, after `resolver.Resolve(ctx, modules)` succeeds and before `declareControllers(modules)`,
call `runModuleInitPhase(ctx, modules)` then `runApplicationBootstrapPhase(ctx, modules)`, returning
early on either's error; (3) initialize `shutdownDone` (always, via `make(chan struct{})`) and set
`modules: modules` in the returned `*App` literal; (4) in `Listen`, after `a.adapter.Listen(addr,
onListenFunc)` returns a NIL error, if `a.shutdownHooksEnabled`, block on `<-a.shutdownDone` before
returning, then return `a.shutdownErr` instead of `nil`.

**Where**: `internal/app/app.go` (modify: `App` struct, `NewApp`, `Listen`)

**Depends on**: T4, T5 (calls the phase-runner functions and reads the struct fields both tasks
introduced/reference)

**Reuses**: The existing `NewApp` structure at `internal/app/app.go:317-384` (insertion point already
identified between `resolver.Resolve` and `declareControllers`) and `Listen` at `internal/app/app.go:120-142`
(insertion point after `a.adapter.Listen(...)` returns)

**Requirement**: LIFEC-01, LIFEC-02, LIFEC-03, LIFEC-04, LIFEC-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] A `Provider` with a passing `OnModuleInit`/`OnApplicationBootstrap` hook has it invoked during
      `NewApp`, before `NewApp` returns
- [ ] A hook returning a non-nil error during either bootstrap phase makes `NewApp` return that error
      (and, via existing `MustNewApp` behavior, `MustNewApp` panics — no new code needed there, just
      confirm the existing panic-on-error wrapper still fires)
- [ ] `Listen`, called on an `*App` with `shutdownHooksEnabled == false`, returns as soon as
      `adapter.Listen` returns (no new blocking) — zero behavior change from before this feature for the
      opt-in-not-taken path
- [ ] `Listen`, called on an `*App` with `shutdownHooksEnabled == true`, blocks past `adapter.Listen`'s
      own return until `shutdownDone` closes, then returns `shutdownErr`
- [ ] A non-nil `Listen` error (e.g. port already in use) returns immediately, without waiting on
      `shutdownDone` at all
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: existing `internal/app` test count + new tests for the `NewApp`/`Listen` wiring, all
      passing (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(app): wire lifecycle hook phases into NewApp and Listen`

---

### T7: End-to-end test proving the full lifecycle sequence

**What**: A single new test (or small test file) registering a `Provider` with a `Constructor` and all 5
hooks, each appending a distinct label to a shared, mutex- or channel-synchronized ordered slice (or
flipping ordered bools) instead of just booleans, so ORDER is directly assertable, not just
"eventually true". Covers: (a) `NewApp` runs `OnModuleInit` then `OnApplicationBootstrap`, in that order,
before returning; (b) with `EnableShutdownHooks()` called and `Close(ctx)` invoked, `OnModuleDestroy` →
`BeforeApplicationShutdown` → `OnApplicationShutdown` run in that order, `OnApplicationShutdown`
receiving the expected signal string (`""` for a manual `Close`); (c) a SEPARATE assertion, on an `*App`
that never called `EnableShutdownHooks()`, that calling `Close(ctx)` still returns (adapter drains) but
none of the 3 destroy-phase hooks ever fire.

**Where**: `internal/app/lifecycle_e2e_test.go` (new)

**Depends on**: T6

**Reuses**: T3's `recordingFakeAdapter`/`listenSpyAdapter`-style fake (or a new minimal fake) as the
`HttpAdapter` type argument to `NewApp[T]`, so this test needs no real network port; `provider.New`/
`module.New` builder patterns already used throughout `internal/app`'s existing test suite

**Requirement**: LIFEC-01, LIFEC-02, LIFEC-03, LIFEC-04, LIFEC-05, LIFEC-06, LIFEC-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Test asserts the ordered slice reads exactly
      `["OnModuleInit", "OnApplicationBootstrap"]` after `NewApp` returns, before `Close`/shutdown is
      ever triggered
- [ ] Test asserts the ordered slice reads exactly
      `[..., "OnModuleDestroy", "BeforeApplicationShutdown", "OnApplicationShutdown"]` after
      `EnableShutdownHooks()` + `Close(ctx)`, with the destroy-phase entries appended strictly AFTER the
      2 bootstrap-phase entries already there
- [ ] Test asserts `OnApplicationShutdown`'s received signal argument is `""` for the manual `Close(ctx)`
      path
- [ ] A separate sub-test/case proves that on an `*App` that never called `EnableShutdownHooks()`,
      `Close(ctx)` returns successfully but the 3 destroy-phase labels never appear in the ordered slice
- [ ] Gate check passes: `go test ./... -race`
- [ ] Test count: existing `internal/app` test count + this new e2e test, all passing (no silent
      deletions)

**Verify**:
```
go test ./internal/app/... -run TestLifecycle -race -v
```
Expected: test passes, printing the ordered slice/labels in assertion failure messages only if it
mismatches (no output on success beyond PASS).

**Tests**: integration
**Gate**: full

**Commit**: `test(app): add end-to-end lifecycle hooks sequencing test`

---

## Parallel Execution Map

```
Phase 1 (Parallel):
  ├── T1 [P]  (internal/provider/lifecycle.go: 3 no-signal hooks)
  └── T3 [P]  (internal/app + internal/adapter/fiber: Shutdown plumbing)

Phase 2 (Sequential, both depend only on T1):
  T1 done, then:
    T2  (internal/provider/lifecycle.go: 2 signal hooks -- same file as T1)
    T4  (internal/app/lifecycle.go: Init/Bootstrap phase runners)

Phase 3 (Sequential):
  T2, T3, T4 done, then:
    T5  (internal/app/lifecycle.go: destroy phases + orchestrator -- same file as T4)

Phase 4 (Sequential):
  T4, T5 done, then:
    T6  (internal/app/app.go: wire into NewApp/Listen)

Phase 5 (Sequential):
  T6 done, then:
    T7  (internal/app/lifecycle_e2e_test.go: full sequence proof)
```

**Note on T2/T4 in Phase 2**: both depend only on T1 and touch different files, but neither is marked
`[P]` — TESTING.md's Parallelism Assessment marks "Testes do motor de resolução (Stage 1-3 rodando
NewApp completo)" as **Parallel-Safe: No** (package-level example vars risk cross-test state leakage),
and T4's tests live in that same `internal/app` bucket. Per the Diagram-Definition Cross-Check rules, a
task whose required test type is not parallel-safe never gets `[P]`, regardless of what else is or
isn't running alongside it. They are still independent of EACH OTHER and may be executed in either
order.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: Provider no-signal hooks | 1 new file (3 registration + 3 invocation methods + shared helper) | ✅ Granular |
| T2: Provider signal hooks | 1 file extension (2 registration + 2 invocation methods) | ✅ Granular |
| T3: HttpAdapter.Shutdown + FiberApp + fakes | 1 interface method threaded through 1 real impl + 2 fakes | ✅ Granular (cohesive: a single interface addition, mechanically applied) |
| T4: Init/Bootstrap phase runners | 1 new file, 2 functions | ✅ Granular |
| T5: Destroy phases + orchestrator | 1 file extension, 3 functions + 3 methods | ✅ Granular (cohesive: one shutdown sequencing concern) |
| T6: Wire into NewApp/Listen | 1 file, 2 existing functions + 1 struct modified | ✅ Granular |
| T7: End-to-end sequencing test | 1 new test file, 1 test scenario (with sub-cases) | ✅ Granular |

**Granularity check**: every task is 1 file (or 1 file-extension of a file a prior task in the same
chain created) with 1 cohesive concern — no task spans unrelated components.

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| --- | --- | --- | --- |
| T1 | None | No incoming arrow (Phase 1 start) | ✅ Match |
| T2 | T1 | T1 → T2 | ✅ Match |
| T3 | None | No incoming arrow (Phase 1 start, parallel with T1) | ✅ Match |
| T4 | T1 | T1 → T4 | ✅ Match |
| T5 | T2, T3, T4 | T2, T3, T4 → T5 | ✅ Match |
| T6 | T4, T5 | T4, T5 → T6 | ✅ Match |
| T7 | T6 | T6 → T7 | ✅ Match |

**Parallel-flag check**: T1 and T3 are the only tasks marked `[P]`; neither depends on the other, and
they touch disjoint files/packages (`internal/provider` vs `internal/app` + `internal/adapter/fiber`) —
no shared mutable state. No task marked `[P]` depends on another task in the same phase. ✅

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1: Provider no-signal hooks | Builders públicos (new `*Provider` methods, same bucket as `NewProvider`) | unit | unit | ✅ OK |
| T2: Provider signal hooks | Builders públicos (new `*Provider` methods) | unit | unit | ✅ OK |
| T3: HttpAdapter.Shutdown + FiberApp | Bind/Listen real (`internal/adapter/fiber`, `HttpAdapter.Listen`/now `.Shutdown`) | integration | integration | ✅ OK |
| T4: Init/Bootstrap phase runners | Motor interno de resolução (`internal/app` bootstrap orchestration, closest existing matrix bucket) | unit | unit | ✅ OK |
| T5: Destroy phases + orchestrator | Motor interno de resolução (`internal/app` bootstrap/shutdown orchestration) | unit | unit | ✅ OK |
| T6: Wire into NewApp/Listen | Motor interno de resolução (`internal/app`, `NewApp`/`Listen`) | unit | unit | ✅ OK |
| T7: End-to-end sequencing test | Full feature integration across `internal/provider` + `internal/app` (no single matrix row covers this combination; treated as the "close the feature" full-suite proof TESTING.md's `full` gate exists for) | integration | integration | ✅ OK |

**Rules applied**: no task uses `Tests: none` — every task's code layer maps to a required test type in
TESTING.md (or, for T7, to the explicit "before closing the feature inteira" full-gate case TESTING.md's
own Gate Check Commands table calls out). No task defers its own tests to a later task — T3 tests
`FiberApp.Shutdown` itself (a real bind + real shutdown), not merely "wait for T7's e2e test to cover it
indirectly."

---

## Tips (carried from template, unchanged)

- **[P] = Parallel OK** — only T1/T3 qualify here, per the Parallelism Assessment.
- **Reuses = Token saver** — every task above cites the exact existing function/pattern it mirrors.
- **One commit per task** — commit messages are listed per task; use them verbatim.
