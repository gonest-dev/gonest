# Provider Interface Export Tasks

**Spec**: `.specs/features/provider-interface-export/spec.md`
**Design**: `.specs/features/provider-interface-export/design.md`
**Status**: In Tasks

Split into 2 execution milestones (Roadmap 22/23): P1 (ProviderAs, breaking) ships and
is verified end-to-end BEFORE P2 (naming convention) touches any file, so a bisect
never lands on a half-migrated state.

---

## Milestone 22 — Provider Interface Export (P1: ProviderAs)

### T1 — `providerAsRef` + `ProviderAs[T]` (internal/module)
- **What**: New file `internal/module/provider_as.go`. Unexported `providerAsRef` struct
  wrapping a `ProviderRef` + `reflect.Type` (the target interface). Implements
  `ProviderRef` (`IsProvider`, `IsExportable`, `ResolvedType` returns stored interface
  type, `SetOwnerModule`/`OwnerModule`), plus `ResolvedValue() (reflect.Value, bool)`
  (delegates to wrapped ref if it exposes the same duck-typed method), `isProviderAsView()`
  marker, `innerRef() ProviderRef` accessor.
  `func ProviderAs[T any](ref ProviderRef) ProviderRef`: panics if `T` is not an
  interface kind; panics if `ref` already satisfies `isProviderAsView` (chaining
  rejected, PROVAS-05).
- **Where**: `internal/module/provider_as.go` (new), `internal/module/provider_as_test.go` (new)
- **Depends on**: none
- **Reuses**: `ProviderRef`/`ExportableRef` shapes already in `module.go`
- **Done when**: unit tests cover — wraps and reports T as ResolvedType; panics on
  non-interface T; panics on chaining; ResolvedValue delegates when wrapped ref has one
- **Tests**: unit
- **Gate**: `go test ./internal/module/... -race`

### T2 — `validateProviderAsRefs` (internal/app)
- **What**: New function walking every module's `OwnProviders()`, type-asserting
  `isProviderAsView`; for each, call `innerRef().ResolvedType()` — nil → error naming
  the missing `Providers(...)` registration; non-nil but `!Implements(T)` → error naming
  both types. Wire into `NewApp`/`MustNewApp` between `declareProviders(modules)` and
  `resolver.Resolve(ctx, modules)`.
- **Where**: `internal/app/provider_as_validate.go` (new), wired in `internal/app/app.go`
  (both `NewApp` and `MustNewApp` paths, ~line 391-396)
- **Depends on**: T1
- **Reuses**: existing module-walk pattern from `declareProviders`
- **Done when**: bootstrap fails loud (returns error / MustNewApp panics) for both
  the "never registered" and "wrong type" cases; succeeds silently otherwise
- **Tests**: unit + integration (real `NewApp`/`MustNewApp` call)
- **Gate**: `go test ./internal/app/... -race`

### T3 — Stage 3 exclusion filter (internal/resolver)
- **What**: In `stage3.go`'s `allProviders`, skip any `ProviderRef` satisfying a local
  `isProviderAsView` marker interface (structural, same pattern as `constructable`/
  `scoped`/`resolvedSetter`) before appending to `out`.
- **Where**: `internal/resolver/stage3.go` (`allProviders`, ~line 263)
- **Depends on**: T1
- **Reuses**: existing dedup-by-identity loop
- **Done when**: a `ProviderAs`-wrapped ref registered via `Providers` never reaches
  `callConstructor` (no "does not expose a Constructor" error for it)
- **Tests**: unit
- **Gate**: `go test ./internal/resolver/... -race`

### T4 — Remove implicit `Implements()` fallback (internal/resolver/direct.go)
- **What**: Delete the `if t.Kind() != reflect.Interface { return nil }` / `implementing`
  block in `findDirectMatches` (lines ~78-92). Function becomes exact-match only.
- **Where**: `internal/resolver/direct.go`
- **Depends on**: T1, T3 (ProviderAs must exist and be constructible-excluded before the
  fallback it replaces is removed, so existing green tests can be re-pointed at it)
- **Reuses**: nothing new — deletion only
- **Done when**: `findDirectMatches` has no `reflect.Type.Implements()` call left; exact
  match with 2+ candidates still resolves to "not found" (unchanged ambiguity contract)
- **Tests**: unit (existing tests rewritten, see T6)
- **Gate**: `go test ./internal/resolver/... -race`

### T5 — `gonest.ProviderAs[T]` public export
- **What**: Add `ProviderAs[T any](ref ProviderRef) ProviderRef` to `gonest.go`,
  delegating to `module.ProviderAs[T]`. Follow the file's existing generic-function
  re-export convention (see `MustInject`'s own comment on why generics can't be a
  `var`-aliased function).
- **Where**: `gonest.go`
- **Depends on**: T1
- **Reuses**: existing re-export pattern for every other generic free function
- **Done when**: `gonest.ProviderAs[SomeInterface](someRef)` compiles and works from
  outside the module (proven by T7's example migration)
- **Tests**: covered by T7's live example
- **Gate**: `go build ./...`

### T6 — Rewrite tests relying on implicit Implements() matching
- **What**: Rewrite the ~6 tests in `internal/resolver/direct_test.go`
  (`TestFindDirect_Interface_*`, `TestFindDirectAll_Interface_*`,
  `TestResolveWithOverrides_InterfaceOverride_SkipsRealConstructor`) plus
  `gonest_test.go`'s `MustInjectAll[insightConnectable]`/`[insightPingable]`
  multi-binding tests against explicit `ProviderAs` registration.
- **Where**: `internal/resolver/direct_test.go`, `gonest_test.go`
- **Depends on**: T1-T5
- **Reuses**: existing test fixtures, just re-registered via `ProviderAs`
- **Done when**: every rewritten test still asserts the SAME behavioral contract
  (single match resolves, 2 matches ambiguous, zero matches not-found) against explicit
  registration instead of structural matching
- **Tests**: unit
- **Gate**: `go test ./... -race -count=1`

### T7 — Migrate `.examples/notification-driver`
- **What**: `email.Service_`/`sms.Service_` wrapped via `gonest.ProviderAs[port.Notifier](...)`
  inside the `notifier` package (per spec.md's Independent Test), replacing the current
  implicit-match registration. Verify live via the example's existing `curl` smoke pattern
  for both `NOTIFICATION_DRIVER` values.
- **Where**: `.examples/notification-driver/**`
- **Depends on**: T1-T5
- **Reuses**: example's existing structure/env-driver-swap wiring, only the registration
  line(s) change
- **Done when**: `go build`/`go vet` clean, `curl` against the running example dispatches
  through both drivers correctly (live evidence, not just compile)
- **Tests**: manual/live (project convention — no test harness for `.examples/*`)
- **Gate**: real HTTP dispatch via `curl`, both driver values

**Milestone 22 close-out gate**: `go test ./... -race -count=1` green, all packages;
`.examples/*` build; commit + push.

---

## Milestone 23 — Thing_ Naming Convention (P2)

### T8 — Document the convention
- **What**: Add a short "Naming Convention" note (builder vars get a trailing `_`,
  unconditionally — `Provider_`, `Controller_`, `Module_`, `Listener_`, `Scheduler_`,
  `Resolver_`) to README.md (new subsection near the existing API overview).
- **Where**: `README.md`
- **Depends on**: Milestone 22 complete (P1 examples already demonstrate the pattern)
- **Reuses**: nothing — documentation only
- **Done when**: rule stated once, unconditionally, with a 1-line rationale (collision
  avoidance, one rule instead of a conditional one)
- **Tests**: none (docs)
- **Gate**: manual read-through

### T9 — Apply convention to `.examples/notification-driver` (already touched in T7)
- **What**: Confirm T7's migration already names its builder vars with the trailing `_`
  convention (`Service_`, not `Service`) — fix if not.
- **Where**: `.examples/notification-driver/**`
- **Depends on**: T7, T8
- **Done when**: every exported builder var in the example follows `Thing_`
- **Tests**: `go build`/`go vet`
- **Gate**: `go vet ./...`

### T10 — Fold `INSIGHT-LAZY.md` → `.specs/insight/LAZY.md`, confirm `.specs/insight/PROVIDER.md` reflects shipped state
- **What**: Finish the already-staged reorg (root `INSIGHT-LAZY.md` deleted, new
  `.specs/insight/LAZY.md` in place — confirm content is current). Update
  `.specs/insight/PROVIDER.md`'s status header once ProviderAs has shipped (Milestone 22
  complete), keeping the `Module.Lazy` tangent as a forward pointer only.
- **Where**: `.specs/insight/{LAZY,PROVIDER}.md`
- **Depends on**: Milestone 22 complete
- **Done when**: no stray root `INSIGHT-LAZY.md`, `PROVIDER.md` status line says "shipped"
  not "part formalized"
- **Tests**: none (docs)
- **Gate**: manual read-through

### T11 — ROADMAP.md / STATE.md close-out
- **What**: Add Milestone 22 + 23 entries to `.specs/project/ROADMAP.md` (mirroring the
  existing entries' format), update `.specs/project/STATE.md`'s Current Work + a new
  AD-0XX decision entry.
- **Where**: `.specs/project/{ROADMAP,STATE}.md`
- **Depends on**: T1-T10 all complete
- **Done when**: roadmap status line says "Milestones 1-23 COMPLETE"
- **Tests**: none (docs)
- **Gate**: manual read-through

**Milestone 23 close-out gate**: `go test ./... -race -count=1` green; commit + push.

---

## Requirement Traceability

| Requirement ID | Task |
| --- | --- |
| PROVAS-01 | T1 |
| PROVAS-02 | T4, T6 |
| PROVAS-03 | T1 (test) |
| PROVAS-04 | T1 (test, Exports path) |
| PROVAS-05 | T1 |
| PROVAS-06 | T4 |
| PROVAS-07 | T6 |
| PROVAS-08 | T2 |
| PROVAS-09 | T3 |
| NAMING-01 | T8, T9 |
