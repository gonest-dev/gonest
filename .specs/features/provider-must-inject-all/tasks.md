# Provider-side MustInjectAll Tasks

**Spec**: `.specs/features/provider-must-inject-all/spec.md`
**Design**: `.specs/features/provider-must-inject-all/design.md`
**Status**: Complete (T1-T11 executed via PO→DEV→QA subagent pipeline, 2026-08-20)

## Milestone — Provider-side MustInjectAll

### T1 — `PendingAllEdge` bookkeeping (internal/inject)
- **REQ**: REQ-002, REQ-007
- **What**: New struct `PendingAllEdge{Owner module.Owner, InterfaceType reflect.Type,
  Matches []module.ProviderRef, Slice reflect.Value}` in `internal/inject/inject.go` --
  multi-valor counterpart of the existing `PendingEdge`. New process-global storage
  (`pendingAllEdgesMu sync.Mutex`, `pendingAllEdges []PendingAllEdge`), same shape as
  `pendingEdges`. `PendingAllEdges() []PendingAllEdge` exported getter (defensive copy,
  mirrors `PendingEdges()`). `Reset()` clears `pendingAllEdges` too.
- **Where**: `internal/inject/inject.go`, `internal/inject/inject_test.go`
- **Depends on**: none
- **Execution note**: logically independent from T2, but SAME FILE as T2/T3/T4 -- run
  sequentially (T1→T2→T3→T4), not as parallel agents, to avoid edit conflicts (PO review)
- **Reuses**: exact shape/pattern of `PendingEdge`/`pendingEdges`/`PendingEdges()`/`Reset()`
- **Done when**: unit tests prove `PendingAllEdges()` returns a defensive copy (mutating
  the returned slice does not affect internal state, mirrors existing `PendingEdges()`
  test) and `Reset()` clears it
- **Tests**: unit
- **Gate**: `go test ./internal/inject/... -race`

### T2 — `findAllRefs` structural search (internal/inject)
- **REQ**: REQ-002, REQ-006
- **What**: New unexported `findAllRefs(ownerModule *module.Module, t reflect.Type)
  []module.ProviderRef` in `internal/inject/inject.go` -- collects every candidate visible
  from `ownerModule` (own `OwnProviders()` + each import's `EffectiveExports()`, deduped by
  identity) whose `ResolvedType() == t` (EXACT match, same convention as
  `resolver.findDirectMatches`/`Find` -- no structural `Implements()` fallback, per AD-053).
  Any matched ref implementing a new local duck-typed interface `allAsView{ InnerRef()
  module.ProviderRef }` (mirrors `module.providerAsRef.InnerRef()`, already exported) is
  UNWRAPPED to the concrete ref it wraps before being returned -- callers must never see a
  `providerAsRef` in the result, only constructable refs. Duplicated here (not reused from
  `internal/resolver`) because `internal/resolver` already imports `internal/inject`
  (`graph.go`/`stage3.go`) -- the inverse import would cycle; same precedent as `mustLazy`'s
  own duplicated `constructable`/`scoped`/etc interfaces in this same file.
- **Where**: `internal/inject/inject.go`, `internal/inject/inject_test.go`
- **Depends on**: none
- **Execution note**: same file as T1/T3/T4, run sequentially (see T1's execution note)
- **Reuses**: `module.Module.OwnProviders`/`ImportedModules`/`EffectiveExports` (unchanged)
- **Done when**: unit tests cover -- own-module match, import-exported match, unexported
  import provider NOT matched, diamond-import dedup (provider visible via 2 import paths
  returned once), and a `providerAsRef` match returned as its unwrapped concrete ref (not
  the view itself)
- **Tests**: unit
- **Gate**: `go test ./internal/inject/... -race`

### T3 — `mustAllProvider[T]` (internal/inject)
- **REQ**: REQ-002, REQ-003, REQ-005
- **What**: New unexported generic `mustAllProvider[T any](owner module.Owner, t
  reflect.Type) []T` in `internal/inject/inject.go`, implementing design.md's 5-step
  algorithm: (1) `matches := findAllRefs(owner.OwnerModule(), t)`; (2) for each match,
  duck-typed `ResolvedScope() scope.Scope` check (reuse the existing `lazyScoped` interface
  already declared in this file) -- panic immediately (fail-fast, still Stage 2) if any
  match is not `scope.Singleton`, message format `"gonest: MustInjectAll[T](p) matched
  provider for type %s is scoped %v, only ScopeSingleton providers can be members of a
  Provider-owned MustInjectAll slice"`; (3) `slice := reflect.MakeSlice(reflect.SliceOf(t),
  len(matches), len(matches))`; (4) record `PendingAllEdge{Owner: owner, InterfaceType: t,
  Matches: matches, Slice: slice}` into `pendingAllEdges`; (5) `return
  slice.Interface().([]T)`.
- **Where**: `internal/inject/inject.go`, `internal/inject/inject_test.go`
- **Depends on**: T1, T2
- **Reuses**: `lazyScoped` (existing interface in this file, same shape `mustLazy` already
  uses for its own LAZY-07 check)
- **Done when**: unit tests cover -- zero matches returns empty (len 0, non-nil) slice, no
  panic (REQ-003); N Singleton matches returns a slice of len N with a `PendingAllEdge`
  recorded; any Transient match panics before recording any edge
- **Tests**: unit
- **Gate**: `go test ./internal/inject/... -race`

### T4 — `MustAll[T]` dispatch branch (internal/inject)
- **REQ**: REQ-001, REQ-004, REQ-006, REQ-008
- **What**: In `MustAll[T]` (`internal/inject/inject.go`), after the existing
  `t.Kind() != reflect.Interface` panic and BEFORE the existing `directResolver` check,
  keep `directResolver` dispatch untouched (REQ-008). Add a new branch: if `owner` is
  `*module.LazyModule`, panic explicitly (`"gonest: MustInjectAll[T](l) is not supported
  from a Lazy owner"` -- out of scope per spec.md, guards against silently mis-dispatching
  if `LazyModule` ever structurally satisfies `module.Owner`). Else if `owner` satisfies
  `module.Owner` (i.e. `*provider.Provider`), dispatch to
  `mustAllProvider[T](moOwner, t)`. Else keep the existing final panic (owner unsupported).
- **Where**: `internal/inject/inject.go`, `internal/inject/inject_test.go`
- **Depends on**: T3
- **Also**: add a GoDoc comment on `MustAll[T]` (or `mustAllProvider`, whichever ends up
  documenting the Provider-owner path) explicitly stating the returned slice's element
  order is NOT guaranteed (REQ-004) -- this is the only artifact REQ-004 produces, must
  not be silently dropped.
- **Done when**: unit test proves a `*provider.Provider`-owned `MustAll[T]` call reaches
  `mustAllProvider` (returns a correctly-sized slice + records a `PendingAllEdge`); a
  `*module.LazyModule`-owned call panics with the new message; `directResolver`-owned
  calls (existing behavior) unchanged, proven by re-running existing tests unmodified;
  GoDoc comment documents no-order-guarantee (REQ-004)
- **Tests**: unit
- **Gate**: `go test ./internal/inject/... -race`

### T5 — `BuildGraph` PendingAllEdge expansion (internal/resolver/graph.go)
- **REQ**: REQ-007
- **What**: In `BuildGraph()`, after the existing loop over `inject.PendingEdges()`, add a
  loop over `inject.PendingAllEdges()`: for each edge, `ownerRef, ok :=
  edge.Owner.(module.ProviderRef)` (skip if not ok, same guard as the existing loop --
  should never trigger since only `*provider.Provider` can own a `PendingAllEdge`, but
  keep the defensive check for consistency); `graph[ownerRef] = append(graph[ownerRef],
  edge.Matches...)` -- no `Find()` call needed, `Matches` already holds the exact
  (unwrapped) target refs recorded at declare-time by T2's `findAllRefs`.
- **Where**: `internal/resolver/graph.go`, `internal/resolver/graph_test.go`
- **Depends on**: T1
- **[P]**: with T6
- **Done when**: unit test proves `BuildGraph()` includes an edge from a `PendingAllEdge`
  owner to every one of its `Matches`, alongside pre-existing `PendingEdge`-derived edges
  for the same owner (both kinds coexist in the same map entry)
- **Tests**: unit
- **Gate**: `go test ./internal/resolver/... -race`

### T6 — `invokeAndCopy` slot-write loop (internal/resolver/stage3.go)
- **REQ**: REQ-002
- **What**: In `invokeAndCopy` (Singleton path), after the existing loop over
  `placeholdersFor(node)`, add a new loop over `inject.PendingAllEdges()`: for each edge,
  for each `i, m := range edge.Matches`, if `m == node` and
  `real.Type().AssignableTo(edge.InterfaceType)`, `edge.Slice.Index(i).Set(real)`. Mirrors
  the existing `placeholdersFor` loop's type-guard-then-`Set` shape
  (`AssignableTo` instead of exact `Type() != Type()`, since the slice element type is an
  interface, not an exact pointer type).
- **Where**: `internal/resolver/stage3.go`, `internal/resolver/stage3_test.go`
- **Depends on**: T1
- **[P]**: with T5
- **Done when**: unit test proves a node listed as `Matches[i]` in a `PendingAllEdge` has
  its resolved value written into `edge.Slice.Index(i)` after `invokeAndCopy(node)` runs; a
  node NOT listed in any edge's `Matches` leaves every `PendingAllEdge`'s `Slice` untouched
- **Tests**: unit
- **Gate**: `go test ./internal/resolver/... -race`

### T7 — Integration: happy path, real NewApp bootstrap
- **REQ**: REQ-001, REQ-002
- **What**: Real `NewApp`/`MustNewApp` integration test: 2+ concrete Providers in
  different (imported) modules, each `gonest.ProviderAs[SomeInterface]`-exported, plus one
  Provider whose `Constructor` closure calls `gonest.MustInjectAll[SomeInterface](p)` and
  captures the result. Assert: the slice has the exact expected length, and each expected
  concrete value is present (order NOT asserted, per REQ-004) once the app finishes
  bootstrapping.
- **Where**: new `internal/app/must_inject_all_provider_test.go` (or added to an existing
  bootstrap integration test file -- Implementer's judgment based on file organization)
- **Depends on**: T4, T5, T6
- **Execution note**: logically independent from T8/T9/T10, but SAME FILE (created here) --
  run sequentially (T7→T8→T9→T10), not as parallel agents (PO review)
- **Tests**: integration (real NewApp bootstrap)
- **Gate**: `go test ./internal/app/... -race`

### T8 — Integration: zero matches
- **REQ**: REQ-003
- **What**: Real `NewApp`/`MustNewApp` integration test: a Provider calls
  `MustInjectAll[SomeInterface](p)` where zero providers in scope implement
  `SomeInterface`. Assert: `NewApp` succeeds (no panic), the captured slice has length 0.
- **Where**: same file as T7
- **Depends on**: T4, T5, T6
- **Execution note**: same file as T7, run sequentially (see T7's execution note)
- **Tests**: integration
- **Gate**: `go test ./internal/app/... -race`

### T9 — Integration: Transient member panics
- **REQ**: REQ-005
- **What**: Real `NewApp`/`MustNewApp` integration test: a `scope.Transient` Provider
  implements the target interface and is in scope of a `MustInjectAll` caller. Assert:
  `NewApp`/`MustNewApp` panics with the T3 message, no partial app returned.
- **Where**: same file as T7
- **Depends on**: T4
- **Execution note**: same file as T7, run sequentially (see T7's execution note)
- **Tests**: integration
- **Gate**: `go test ./internal/app/... -race`

### T10 — Integration: ordering guarantee
- **REQ**: REQ-007
- **What**: Real `NewApp`/`MustNewApp` integration test: every matched provider's
  `Constructor` appends its own name to a shared, mutex-guarded log BEFORE returning; the
  `MustInjectAll`-calling Provider's own `Constructor` appends `"owner"` to the same log.
  Assert `"owner"` is always the LAST entry, proving `waitDeps` actually blocks the owner
  until every matched provider has finished (not just "the slice was eventually correct" --
  a race that only shows up under `-race -count=N` repeated runs).
- **Where**: same file as T7
- **Depends on**: T5, T6
- **Execution note**: same file as T7, run sequentially (see T7's execution note)
- **Tests**: integration
- **Gate**: `go test ./internal/app/... -race -count=5`

### T11 — Close-out (ROADMAP.md / STATE.md)
- **What**: Add a milestone entry to `ROADMAP.md` (mirroring existing format). Update
  STATE.md's Current Work + a new AD-0XX entry (next number after the latest AD in STATE.md
  at execution time).
- **Where**: `.specs/project/{ROADMAP,STATE}.md`
- **Depends on**: T1-T10 complete
- **Tests**: none (docs)
- **Gate**: manual read-through

**Milestone close-out gate**: `go test ./... -race -count=1` green, all packages; commit.

---

## Requirement Traceability

| Requirement ID | Task |
| --- | --- |
| REQ-001 | T4, T7 |
| REQ-002 | T1, T2, T3, T6, T7 |
| REQ-003 | T3, T8 |
| REQ-004 | T4 (documented, not tested -- no ordering guarantee to assert) |
| REQ-005 | T3, T9 |
| REQ-006 | T2, T4 |
| REQ-007 | T1, T5, T10 |
| REQ-008 | T4 |
