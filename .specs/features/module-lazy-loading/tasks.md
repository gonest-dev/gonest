# Module Lazy Loading Tasks

**Spec**: `.specs/features/module-lazy-loading/spec.md`
**Design**: `.specs/features/module-lazy-loading/design.md`
**Status**: In Tasks

## Milestone 24 — Module Lazy Loading

### T1 — `LazyModule` + `Module.Lazy` (internal/module)
- **What**: New `internal/module/lazy.go`. `LazyModule` struct with unexported `owner
  *Module`. `func (m *Module) Lazy(fn func(l *LazyModule))` constructs the wrapper and
  calls `fn(l)` IMMEDIATELY (not deferred). `LazyModule.Imports(mods ...*Module)` /
  `Exports(refs ...ExportableRef)` delegate to `owner`. `LazyModule.OwnProviders()
  []ProviderRef` delegates to `owner.OwnProviders()`.
- **Where**: `internal/module/lazy.go` (new), `internal/module/lazy_test.go` (new)
- **Depends on**: none
- **Reuses**: `Module.Imports`/`Exports`/`OwnProviders` unchanged
- **Done when**: unit tests prove `fn` runs synchronously (before `Lazy` returns),
  `Imports`/`Exports` called on `l` land on the SAME `owner` module's own storage
- **Tests**: unit
- **Gate**: `go test ./internal/module/... -race`

### T2 — `inject.Must[T]`'s Lazy dispatch branch (internal/inject)
- **What**: New branch in `Must[T]`, checked before the `directResolver` branch: if
  `owner` is `*module.LazyModule`, implement the 10-step algorithm from design.md's
  Components section (Declare-all-own-providers, exact-type match, already-resolved
  short-circuit, Singleton-only check, pending-edge-count snapshot/invoke/compare,
  SetResolvedValue). 3 new local unexported duck-typed interfaces (`declarable`,
  `constructable`, `resolvedSetter`) matching `*provider.Provider`'s exported methods,
  same naming precedent as `internal/resolver/stage3.go`'s own private mirrors.
- **Where**: `internal/inject/inject.go`
- **Depends on**: T1
- **Reuses**: `PendingEdges()` (existing), `provider.Constructor`'s 4-signature shape
  (duplicated invocation logic per design.md's Tech Decisions)
- **Done when**: unit tests cover — successful eager resolve; panic on no matching
  provider (LAZY-05); panic when the provider's Constructor calls MustInject (LAZY-06);
  panic on non-Singleton provider (LAZY-07); 2nd call for the same type reuses the
  cached value without re-invoking Constructor (LAZY-08)
- **Tests**: unit
- **Gate**: `go test ./internal/inject/... -race`

### T3 — Stage 3 skip-if-already-resolved (internal/resolver/stage3.go)
- **What**: In `invokeAndCopy` (Singleton path), after the existing `overrideFor`
  check, add: if `node` already has a `ResolvedValue()` (via the existing local
  `resolvedGetter` interface in this file), reuse it and skip `callConstructor`. Exact
  code shown in design.md's "Stage 3 skip-if-already-resolved check" component.
- **Where**: `internal/resolver/stage3.go`
- **Depends on**: T2 (needs a provider that COULD already have a resolved value by the
  time Stage 3 runs, to test against)
- **Reuses**: `overrideFor`'s existing "skip real Constructor" pattern, `resolvedGetter`
  (already declared in this file)
- **Done when**: a real `NewApp`/`MustNewApp` integration test proves a Lazy-eager-
  resolved provider's Constructor runs EXACTLY ONCE across the whole bootstrap (LAZY-03)
  — e.g. a Constructor incrementing a counter, asserted == 1 after `NewApp` returns
- **Tests**: integration (real NewApp bootstrap)
- **Gate**: `go test ./internal/resolver/... -race`

### T4 — `gonest.LazyModule` root alias
- **What**: `type LazyModule = module.LazyModule` in `gonest.go`, same root-aliasing
  convention as `ProviderRef`/every other internal type re-exported at the root. `Lazy`
  itself needs NO new root wrapper (it's a method on the already-aliased `Module` type,
  calling `m.Lazy(fn)` on a `*gonest.Module` already resolves to `module.Module.Lazy`
  automatically via the type alias — confirm this compiles as expected, no wrapper
  function needed unlike `NewProvider`/`ProviderAs`, which are free functions not methods).
- **Where**: `gonest.go`
- **Depends on**: T1
- **Done when**: `gonest.LazyModule` compiles and `(*gonest.Module).Lazy(...)` works
  from outside the module (proven by T6's example migration)
- **Tests**: covered by T6's live example
- **Gate**: `go build ./...`

### T5 — LAZY-04 full integration proof
- **What**: A real `NewApp`/`MustNewApp` test with a module using `Lazy` to pick between
  2 sibling modules based on a config value (2 sub-tests, one per branch), asserting the
  resulting app behaves identically to the equivalent unconditional `Imports` (e.g. both
  sub-tests dispatch a real HTTP request through whichever module won, confirming route
  registration + DI resolution work end-to-end, not just that `Imports` was called with
  the right argument).
- **Where**: `internal/app/lazy_module_test.go` (new) or added to an existing bootstrap
  integration test file — Implementer's judgment based on existing file organization
- **Depends on**: T1-T4
- **Tests**: integration
- **Gate**: `go test ./internal/app/... -race`

### T6 — Migrate `.examples/notification-driver`
- **What**: New `notifier.Config_` (real `Schema`-validated `Provider`,
  `env:"NOTIFICATION_DRIVER"`, following this project's existing `env:"..."`/`Schema`
  conventions from `.examples/config-dotenv`). `notifier.ModuleForRoot` removed.
  `AppModule_` (or a new `notifier.Module_`) wires `email.Module_`/`sms.Module_` via
  `m.Lazy(...)`. `main.go` no longer reads the driver env var or picks a module itself
  — it just builds `AppModule_` and lets `Lazy` decide internally.
- **Where**: `.examples/notification-driver/**`
- **Depends on**: T1-T5
- **Done when**: `go build`/`go vet` clean, `curl` against the running example (both
  `NOTIFICATION_DRIVER` values) dispatches correctly — same live-verification pattern
  every previous example migration in this project has used
- **Tests**: manual/live (project convention — no test harness for `.examples/*`)
- **Gate**: real HTTP dispatch via `curl`, both driver values

### T7 — `.specs/insight/PROVIDER.md` close-out
- **What**: Update the status blockquote — `Module.Lazy` tangent moves from "OUT OF
  SCOPE... registered here as reflexão viva" to "SHIPPED (Milestone 24)".
- **Where**: `.specs/insight/PROVIDER.md`
- **Depends on**: T1-T6 complete
- **Tests**: none (docs)
- **Gate**: manual read-through

### T8 — ROADMAP.md / STATE.md close-out
- **What**: Add Milestone 24 entry to ROADMAP.md (mirroring existing format), update
  STATE.md's Current Work + a new AD-0XX entry (next number after AD-053).
- **Where**: `.specs/project/{ROADMAP,STATE}.md`
- **Depends on**: T1-T7 complete
- **Tests**: none (docs)
- **Gate**: manual read-through

**Milestone 24 close-out gate**: `go test ./... -race -count=1` green, all packages;
`.examples/*` build; commit + push.

---

## Requirement Traceability

| Requirement ID | Task |
| --- | --- |
| LAZY-01 | T1 |
| LAZY-02 | T2 |
| LAZY-03 | T3 |
| LAZY-04 | T5 |
| LAZY-05 | T2 |
| LAZY-06 | T2 |
| LAZY-07 | T2 |
| LAZY-08 | T2 |
