# Lifecycle Hooks Context

**Gathered:** 2026-07-19
**Spec:** `.specs/features/lifecycle-hooks/spec.md`
**Status:** Ready for design

---

## Feature Boundary

`Provider` gains 5 lifecycle hook registration methods (`OnModuleInit`/`OnApplicationBootstrap`/
`OnModuleDestroy`/`BeforeApplicationShutdown`/`OnApplicationShutdown`), matching NestJS's real
lifecycle contract exactly (confirmed via Context7 against `/nestjs/docs.nestjs.com`, not assumed).

## Research: NestJS's real behavior (Context7-confirmed)

- **Trigger**: shutdown hooks are OFF by default; `app.enableShutdownHooks()` (or equivalent) must be
  called explicitly before `listen()`. Once enabled, they fire on an OS termination signal
  (`SIGTERM`/`SIGINT`/etc.) OR `app.close()`. `app.close()` itself does NOT kill the process — it only
  runs the hooks; a long-running task can still keep the process alive.
- **Windows caveat**: `SIGTERM` does not work on Windows; only `SIGINT`, `SIGBREAK`, `SIGHUP` do
  (relevant — user develops on Windows).
- **Init/Bootstrap order**: `onModuleInit()` then (once ALL modules are done) `onApplicationBootstrap()`
  — both run in MODULE IMPORT ORDER, sequential, each hook awaited before the next starts (not
  concurrent).
- **Destroy order**: `onModuleDestroy()` → `beforeApplicationShutdown()` → `onApplicationShutdown()`,
  sequential, signal passed as a parameter. Destroy-phase ordering is the REVERSE of init order
  (NestJS v11 change): if modules initialized `C → B → A`, they destroy as `A → B → C`. Global modules
  are treated as a dependency of every other module — initialized first, destroyed last.
- **Scope applicability**: lifecycle hooks are called on modules, providers, and controllers during
  bootstrap/shutdown — **explicitly excluded for request-scoped classes** ("not tied to the
  application lifecycle... created for each request, garbage-collected after the response").
- **Signal parameter**: `onApplicationShutdown(signal: string)` confirmed directly in docs.
  `beforeApplicationShutdown` very likely shares the same `(signal string)` shape (grouped in the same
  sentence in the sourced doc, and matches this agent's general knowledge of the real Nest API) — flagged
  here as reasonably-confident-but-not-verbatim-doc-confirmed, per Knowledge Verification Chain step 5.
  `onModuleDestroy()`/`onModuleInit()`/`onApplicationBootstrap()` take no signal/extra parameter.
- **Error handling**: no explicit "swallow vs fail" doc found for hook errors specifically. Given the
  documented mechanics are a plain sequential `await` loop per phase, the natural (and only
  Nest-consistent) behavior is: an error thrown/returned by a hook propagates immediately and the
  remaining hooks in that phase do not run — same as any other unhandled exception in Nest, no special
  collect-all or swallow behavior documented.

---

## Implementation Decisions

### Hook set

- 5 hooks on `*gonest.Provider`, matching Nest 1:1: `OnModuleInit(fn)`, `OnApplicationBootstrap(fn)`,
  `OnModuleDestroy(fn)`, `BeforeApplicationShutdown(fn)`, `OnApplicationShutdown(fn)` — supersedes
  `INSIGHT-ON.md`'s 4-hook sketch (missing `BeforeApplicationShutdown`).
- Each `fn` receives the Provider's own resolved instance (mirrors `Constructor`'s pattern), optionally
  `context.Context`, optionally returns `error`. `OnApplicationShutdown`/`BeforeApplicationShutdown`
  additionally receive the triggering signal as a string (exact Go signature — `func(T, string) error`
  shape vs. an options-struct — is a Design-phase decision, not fixed here).

### Shutdown trigger

- Opt-in, exactly like Nest: a new `App.EnableShutdownHooks()` must be called explicitly (before
  `Listen`) for `OnModuleDestroy`/`BeforeApplicationShutdown`/`OnApplicationShutdown` to ever fire.
  Without it, only `OnModuleInit`/`OnApplicationBootstrap` exist (bootstrap-time hooks always run,
  same as Nest — those two are not gated by `enableShutdownHooks`).
- Once enabled: OS signal handling registers for whatever signals are meaningful on the current OS
  (`SIGINT` universally; `SIGTERM` on non-Windows; Design phase resolves the exact Go `os/signal` set
  per platform) AND a manual `App.Close()`/`App.Shutdown(ctx)` path (Design decides the exact method
  name) triggers the same sequence.

### Ordering

- `OnModuleInit`/`OnApplicationBootstrap`: sequential, module-declaration order (gonest's existing
  module tree assembly order — the order `Module.Imports`/`Module.Providers` were declared in, not a
  new concept), each hook awaited before the next starts. This runs AFTER Stage 3's existing concurrent
  Constructor resolution completes — hook sequencing is a new, separate, sequential pass over an
  already-fully-resolved graph, not a change to Stage 3's own concurrency.
- `OnModuleDestroy` → `BeforeApplicationShutdown` → `OnApplicationShutdown`: sequential within each
  phase; the `OnModuleDestroy` phase itself runs in REVERSE module order relative to init.

### Scope applicability

- Singleton only. Request-scoped providers are explicitly excluded (directly confirmed for Nest).
  Transient is also excluded in gonest's version — same underlying reasoning (not tied to a single,
  predictable application-lifecycle instant) extended by this agent's judgment, since gonest's Transient
  semantics (a new instance per pending edge, potentially concurrent) has no clean single "moment" to
  fire a hook against, and Nest's own docs don't cover Transient explicitly either.

### Error handling

- Fail-fast/propagate, no swallow: a hook returning a non-nil `error` stops the REMAINING hooks in that
  same phase from running and the error propagates (bootstrap: `NewApp` returns it / `MustNewApp`
  panics, matching `Constructor`'s existing contract; shutdown: surfaces from
  `App.Close()`/`App.Shutdown(ctx)`'s own return value — exact propagation path is Design's call).

### Agent's Discretion

- Exact Go method names/signatures for the shutdown trigger (`App.Close()` vs `App.Shutdown(ctx)`,
  `EnableShutdownHooks()` vs another name) — Design phase picks the option most consistent with
  existing gonest naming (`MustListen`/`Listen` pairing, etc.).
- Exact signal set registered per OS and how `os/signal.Notify` is wired into `Listen`/a new method.
- Whether `BeforeApplicationShutdown`'s signature takes `signal string` (treated as near-certain per
  research above, but Design should double check if a stronger source surfaces).

---

## Specific References

`INSIGHT-ON.md`'s sketch (`p.OnApplicationBootstrap(...)`, `p.OnApplicationShutdown(...)`,
`p.OnModuleInit(...)`, `p.OnModuleDestroy(...)`) is the base shape being extended with the 5th hook.

---

## Deferred Ideas

None — discussion stayed within feature scope.
