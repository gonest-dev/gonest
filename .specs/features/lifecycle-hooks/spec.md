# Lifecycle Hooks Specification

## Problem Statement

Providers today have no way to run code at a well-defined moment in the app's life besides their
own `Constructor` (build-time only). A `Provider` wrapping a real resource (DB pool, message broker
connection, cache client) has no hook to run extra setup AFTER its own dependencies are ready but
BEFORE the app starts serving (`OnModuleInit`), no hook to run once EVERY provider in the whole graph
is ready (`OnApplicationBootstrap`), and — critically — no hook at all for teardown
(`OnModuleDestroy`/`OnApplicationShutdown`), because gonest has no shutdown mechanism whatsoever yet
(no `App.Close`, no OS signal handling). This mirrors NestJS's 4 lifecycle hooks
(`OnModuleInit`/`OnApplicationBootstrap`/`OnModuleDestroy`/`OnApplicationShutdown`), adapted to
gonest's existing `Provider` builder pattern per `INSIGHT-ON.md`'s sketch (methods on `*gonest.Provider`,
not a struct implementing marker interfaces like Nest's TS decorators).

## Goals

- [ ] `Provider` gains 5 new registration methods (`OnModuleInit`/`OnApplicationBootstrap`/
      `OnModuleDestroy`/`BeforeApplicationShutdown`/`OnApplicationShutdown`), matching NestJS's real
      hook set 1:1 (confirmed via Context7, supersedes `INSIGHT-ON.md`'s 4-hook sketch), each accepting
      a callback shaped like `Constructor` (receives the provider's own resolved instance, optionally
      `context.Context`, returns optionally `error`) -- the 2 shutdown-phase hooks additionally receive
      the triggering signal as a string
- [ ] `OnModuleInit`/`OnApplicationBootstrap` run automatically during `NewApp`/`MustNewApp`, before
      `Listen` can be called
- [ ] `OnModuleDestroy`/`OnApplicationShutdown` run automatically on a well-defined shutdown trigger
      (mechanism to be settled in Discuss)
- [ ] A hook returning a non-nil `error` is surfaced the same way a `Constructor` error is today
      (bootstrap: `NewApp` returns the error / `MustNewApp` panics; shutdown: TBD in Discuss)

## Out of Scope

| Feature | Reason |
| --- | --- |
| Module-level hooks (a hook on `*module.Module` itself, not a Provider) | `INSIGHT-ON.md`'s sketch only shows Provider-level hooks; NestJS's own hooks are class-level (= Provider-level here), not Module-level |
| Hooks on Controllers/Resolvers/Schedulers/Listeners | Only `Provider` builds long-lived resources; the sketch and the motivating use case (DB/broker connections) are Provider-scoped |
| Request/Transient-scoped provider hooks | DECIDED in Discuss: Singleton only. NestJS explicitly excludes request-scoped classes ("not tied to the application lifecycle"); Transient excluded here by the same reasoning, extended by agent judgment (see context.md) |

---

## User Stories

### P1: Setup a resource after its dependencies are ready ⭐ MVP

**User Story**: As a gonest user, I want my `Provider`'s `OnModuleInit`/`OnApplicationBootstrap`
callback to run automatically during bootstrap, so I can call `Connect()`/`Ping()` on a resource
without wiring a manual bootstrap step.

**Why P1**: This is the entire value proposition — `INSIGHT-ON.md`'s sketch exists specifically for
this (DB `Connect`/`Ping` example).

**Acceptance Criteria**:

1. WHEN a `Provider` registers `OnModuleInit(fn)` THEN `fn` SHALL run exactly once, after that
   Provider's own `Constructor` has resolved successfully
2. WHEN a `Provider` registers `OnApplicationBootstrap(fn)` THEN `fn` SHALL run exactly once, after
   EVERY Singleton provider in the whole assembled graph has finished resolving (a global barrier,
   not per-provider)
3. WHEN `OnModuleInit`/`OnApplicationBootstrap` returns a non-nil `error` THEN `NewApp` SHALL return
   that error (and `MustNewApp` SHALL panic), same contract as a `Constructor` error today
4. WHEN a `Provider` never calls `OnModuleInit`/`OnApplicationBootstrap` THEN nothing changes from
   today's behavior (purely additive, zero-value safe)

**Independent Test**: Register a Provider with a `Constructor` + `OnModuleInit` that flips a bool;
`NewApp` returns; assert the bool is true and no route was reachable before it flipped.

---

### P2: Graceful teardown on shutdown

**User Story**: As a gonest user, I want my `Provider`'s `OnModuleDestroy`/`OnApplicationShutdown`
callback to run when the app shuts down, so I can close a DB pool/broker connection without leaking
resources.

**Why P2**: Real value, but gated on a shutdown mechanism gonest does not have today — needs Discuss
before this can be scoped precisely (see Edge Cases / open questions below).

**Acceptance Criteria**:

1. WHEN the app shuts down (trigger TBD) THEN every Provider's `OnModuleDestroy` SHALL run
2. WHEN every `OnModuleDestroy` has run THEN every Provider's `OnApplicationShutdown` SHALL run (global
   barrier, mirroring `OnApplicationBootstrap`'s relationship to `OnModuleInit`)
3. WHEN a destroy/shutdown hook returns an `error` THEN the app SHALL NOT stop processing remaining
   hooks because of it (destruction must be best-effort, not fail-fast like bootstrap)

**Independent Test**: Register a Provider with `OnModuleDestroy` that flips a bool; trigger shutdown;
assert the bool is true.

---

### P3: Ordering guarantees across independent Providers

**User Story**: As a gonest user, I want documented (even if not strictly configurable) ordering
between different Providers' hooks, so I know whether `OnModuleInit` A always runs before/after B.

**Why P3**: Nice-to-have clarity; gonest's Stage 3 resolves Singletons concurrently via `errgroup`
respecting only dependency edges — exact ordering between UNRELATED providers is not guaranteed today
and this feature should not invent a stronger guarantee than the underlying resolver already provides.

**Acceptance Criteria**:

1. WHEN Provider B depends on Provider A (via `MustInject` inside A... wait, B depends on A) THEN A's
   `OnModuleInit` SHALL complete before B's `Constructor` starts (same ordering Stage 3 already
   guarantees for Constructors themselves)
2. WHEN two Providers have no dependency relationship THEN their `OnModuleInit` order SHALL be
   documented as unspecified (matches today's concurrent Constructor resolution)

---

## Edge Cases

- WHEN a Provider has no `Constructor` at all (should not happen given existing validation, but
  defensive) THEN `OnModuleInit` SHALL not run (nothing resolved to pass it)
- WHEN `OnApplicationBootstrap` is registered but `OnModuleInit` is not THEN it still SHALL run (the
  4 hooks are independent, not a required chain)
- WHEN a Request or Transient scoped Provider registers any of the 4 hooks THEN behavior is currently
  UNDEFINED pending Discuss (see Out of Scope)
- WHEN shutdown is triggered mid-bootstrap (should be near-impossible given `NewApp` is synchronous)
  THEN out of scope — not a real scenario

## Open Questions — RESOLVED (see `context.md`, Context7-confirmed against real NestJS docs)

1. **Shutdown trigger**: opt-in, exactly like Nest — `App.EnableShutdownHooks()` explicit call before
   `Listen`. Disabled by default; without it only `OnModuleInit`/`OnApplicationBootstrap` ever fire.
2. **Hook scope applicability**: Singleton only. Request explicitly excluded per Nest docs; Transient
   excluded by extension.
3. **Destroy hook error handling**: fail-fast/propagate (no swallow) — a returned error stops the
   remaining hooks in that phase; matches the plain sequential-await mechanics Nest itself documents.
4. **`OnApplicationShutdown`/`BeforeApplicationShutdown` argument**: both receive the triggering signal
   as a string, mirroring Nest's `onApplicationShutdown(signal: string)`.
5. **Ordering for `OnApplicationBootstrap`/shutdown phases**: sequential, module-declaration order
   (confirmed — Nest is sequential, not concurrent, despite gonest's own Stage 3 Constructor resolution
   being concurrent). Destroy-phase order is the REVERSE of init order.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| LIFEC-01 | P1: Setup a resource after its dependencies are ready | Verified | Verified |
| LIFEC-02 | P1: Setup a resource after its dependencies are ready | Verified | Verified |
| LIFEC-03 | P1: Setup a resource after its dependencies are ready | Verified | Verified |
| LIFEC-04 | P2: Graceful teardown on shutdown | Verified | Verified |
| LIFEC-05 | P2: Graceful teardown on shutdown | Verified | Verified |
| LIFEC-06 | P3: Ordering guarantees across independent Providers | Verified | Verified |
| LIFEC-07 | P2: Graceful teardown on shutdown (`BeforeApplicationShutdown`, added post-Discuss) | Verified | Verified |

**ID format:** `LIFEC-NN`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 7 total, 7 mapped to tasks (T1-T7), 0 unmapped -- feature COMPLETE, see STATE.md's AD-044.

---

## Success Criteria

- [ ] `INSIGHT-ON.md`'s exact sketch (`p.OnApplicationBootstrap(...)`/`p.OnApplicationShutdown(...)`/
      `p.OnModuleInit(...)`/`p.OnModuleDestroy(...)`) compiles and runs end-to-end via a real example
- [ ] `go test ./... -race` stays green, zero pre-existing assertion changed
- [ ] Shutdown mechanism (whatever Discuss settles on) is demonstrated with a real process signal or
      explicit call, not just a unit test mock
