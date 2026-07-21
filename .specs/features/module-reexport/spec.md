# Module Re-export Specification

## Problem Statement

`Module.Exports(ps ...ProviderRef)` only accepts individual providers. NestJS's
own `exports: [...]` also accepts a whole imported `Module` class, re-exposing
that module's own exports transitively to whoever imports the re-exporting
module (confirmed via Context7 against `/nestjs/docs.nestjs.com`: *"Modules
can not only export their internal providers but also re-export other
modules they import... making CommonModule available to any other modules
that import CoreModule"*). gonest has no equivalent today -- confirmed with a
REAL compile error in a consumer project
(`C:\dev\leandroluk\erc\ctrl\api\infra\module.go`):
`m.Exports(database.Module)` fails with `*gonest.Module does not implement
module.ProviderRef (missing method IsProvider)`. Even if that compiled,
`internal/module/assemble.go`'s `validateExports` would still reject it (only
allows exporting a provider present in the SAME module's own `Providers`).

Both of gonest's 2 independent resolution paths -- `internal/resolver.Find`
(single-target `MustInject[T]`) and `internal/resolver.FindDirect`/
`FindDirectAll` (multi-candidate direct resolution, `internal/inject`'s
Provider-to-Provider dependency + Controller/Middleware/etc direct lookups)
-- currently walk exactly ONE hop: owner's own providers, then each directly
imported module's OWN `ExportedProviders()`. Neither recurses into a module
a directly-imported module itself re-exports.

## Goals

- [ ] `Module` gains a way to re-export a whole imported `*Module` (not just
      individual providers), collected via a new method (see Design's own
      naming decision -- `Exports` itself keeps its current
      `...ProviderRef` signature, source-compatible, no breaking change)
- [ ] Stage 1 assembly (`assemble.go`) validates a re-exported module the
      same way it already validates a re-exported PROVIDER: the module being
      re-exported must be present in the re-exporter's own `Imports` list
      (mirrors "you can only export a provider you declared")
- [ ] `internal/resolver.Find` (single-target `MustInject[T]`) and
      `internal/resolver.FindDirect`/`FindDirectAll` (multi-candidate direct
      resolution) both walk re-exported modules TRANSITIVELY: if module A
      imports B, and B imports+re-exports C, A sees C's own exported
      providers through B without importing C directly -- and this chains
      to any depth (A -> B -> C -> D...), matching NestJS's own real
      behavior (a re-export is not shallow)
- [ ] A cyclic re-export chain (B re-exports C, C re-exports B -- reachable
      only if both also import each other, itself unusual but not
      independently forbidden) does not infinite-loop or stack-overflow the
      resolver

## Out of Scope

| Feature | Reason |
| --- | --- |
| Re-exporting a Controller/Resolver/Middleware/Filter/Listener/Scheduler's OWNING module | NestJS's own re-export semantics apply to `providers`/`exports` only -- Controllers are never importable/exportable in Nest either. Out of scope here for the same reason. |
| Changing `Exports`'s existing signature to accept a mixed `ProviderRef \| *Module` union in one call | Go has no sum/union types; forcing one polymorphic method would need a marker interface `*Module` has no natural reason to implement (a Module is not conceptually "a kind of provider"). A separate, explicitly-named method is more idiomatic Go and matches this codebase's own existing pattern of one method per concept (`Providers`/`Controllers`/`Resolvers`/`Listeners`/`Schedulers` are already separate, never one polymorphic `Register(...)`). |
| Re-exporting a module NOT already imported | Mirrors the existing provider-export rule 1:1 (`validateExports` already requires an exported provider to be declared via `Providers` first) -- no new precedent, just the same rule applied to the new re-export kind. |

---

## User Stories

### P1: Re-export an imported module's providers to my own importers ⭐ MVP

**User Story**: As a gonest user, I want `infraModule.ExportModules(databaseModule)` (after `infraModule.Imports(databaseModule)`) to make every provider `databaseModule` itself exports visible to anyone who imports `infraModule`, without `infraModule` needing to re-declare each provider individually.

**Why P1**: This is the entire feature -- the real gap found in a consumer project (a `database` module with 25+ DAO providers, an `infra` module meant to be the single import point for the rest of the app).

**Acceptance Criteria**:

1. WHEN module B imports module C and calls `B.ExportModules(C)` THEN a module A that imports B (but does NOT import C) SHALL be able to `MustInject[T]` any type C itself exports
2. WHEN B calls `ExportModules(C)` but did NOT call `B.Imports(C)` THEN Stage 1 assembly SHALL return an error (same class as "exports provider it does not declare"), not silently succeed or panic unhelpfully at resolve time
3. WHEN B re-exports C, and C re-exports D (C imports D, calls `C.ExportModules(D)`) THEN A (importing only B) SHALL be able to resolve a type D exports -- the chain has no fixed depth limit
4. WHEN B re-exports C but C does NOT export a given provider (C declared it via `Providers` but never exported it) THEN A SHALL NOT be able to resolve it through B -- re-export only propagates what C *itself* already exports, never C's un-exported internals
5. WHEN the re-export chain contains a cycle (reachable only via mutual import+re-export) THEN resolution SHALL terminate (not hang/stack-overflow), treating each module as visited at most once per search

**Independent Test**: 3 modules, C (owns+exports a provider), B (imports C, re-exports C, contributes no providers of its own), A (imports ONLY B, root module). `MustInject[T]` from a Controller owned by A resolves the type C exports. Confirm A never imports C directly.

---

## Edge Cases

- WHEN a module re-exports the SAME module more than once (`ExportModules(C, C)` or 2 separate calls) THEN system SHALL treat it as idempotent -- no duplicate resolution results, no panic
- WHEN a module re-exports itself (`B.ExportModules(B)`, degenerate/unusual) THEN system SHALL not infinite-loop -- same cycle guard as the mutual-cycle case
- WHEN 2 DIFFERENT re-export chains both eventually reach the same provider (a diamond: B re-exports C and D, both C and D re-export the same shared module E) THEN resolution SHALL still find it exactly once (dedup by identity, same as `direct.go`'s existing `candidateProviders` dedup)

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| REEXPORT-01 | P1: ExportModules registers a re-exported module | Implementing | Verified |
| REEXPORT-02 | P1: Stage 1 validates re-exported module is in own Imports | Implementing | Verified |
| REEXPORT-03 | P1: Find (single-target) walks re-exports transitively | Implementing | Verified |
| REEXPORT-04 | P1: FindDirect/FindDirectAll (multi-candidate) walk re-exports transitively | Implementing | Verified |
| REEXPORT-05 | P1: cycle guard -- no infinite loop on a cyclic re-export chain | Implementing | Verified |
| REEXPORT-06 | Edge: idempotent duplicate re-export, self re-export, diamond dedup | Implementing | Verified |

**ID format:** `REEXPORT-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 6 total, 6 Verified (T1/T2 evaluator-approved, gate rerun without cache). T3 (real consumer) pending.

---

## Success Criteria

- [ ] The exact scenario from `C:\dev\leandroluk\erc\ctrl\api\infra\module.go` compiles and resolves correctly using the new API (verified against that real consumer file, not just gonest's own test suite) -- T3, in progress
- [x] `go test ./... -race -count=1` stays green, 24+ core packages, zero regression in either existing resolution path
- [ ] Both `Find` and `FindDirect`/`FindDirectAll` share ONE transitive-walk implementation (not 2 independently-written, divergence-prone copies -- this repo's own STATE.md documents past bugs from exactly that kind of duplication, e.g. AD-036/037)
