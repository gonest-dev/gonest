# Module Re-export Design

## Architecture Overview

One new piece of state (`Module.exportedModules []*Module`), one new builder
method (`ExportModules`), one new Stage 1 validation rule (mirrors the
existing provider-export rule), and ONE new centralized computation
(`Module.EffectiveExports() []ProviderRef`) that both existing resolution
paths (`resolver.Find`, `resolver.FindDirect`/`FindDirectAll`) call instead
of the raw `ExportedProviders()` getter they call today. No change to
`Module.Exports`'s existing signature (source-compatible, additive only).

```
Module.ExportModules(mods ...*Module)   -- NEW: register a re-exported module
        |
        v
assemble.go: validateExports            -- EXTENDED: re-exported module must
                                            be in the SAME module's Imports
        |
        v
Module.EffectiveExports() []ProviderRef -- NEW: own ExportedProviders() +
                                            recursive walk of exportedModules,
                                            cycle-guarded, deduped by identity
        |
        +--> resolver.Find (single-target, MustInject[T])
        |
        +--> resolver.FindDirect / FindDirectAll (multi-candidate direct)
```

## Components

### `internal/module/module.go`

- `Module` struct gains `exportedModules []*Module` (unexported field,
  mirrors `providers`/`exports`'s own storage shape).
- `func (m *Module) ExportModules(mods ...*Module)` -- appends to
  `exportedModules`, same no-return-value shape as `Providers`/`Exports`/
  `Controllers` (this codebase never makes these chainable; matching that
  exactly, not introducing a new convention).
- `func (m *Module) OwnExportedModules() []*Module` -- defensive-copy getter,
  same pattern as `OwnProviders()`/`ImportedModules()`.
- `func (m *Module) EffectiveExports() []ProviderRef` -- the new centralized
  computation. Delegates to an unexported recursive helper carrying a
  `visited map[*Module]bool` so a cycle (B re-exports C, C re-exports B, or a
  module re-exporting itself) terminates instead of infinite-looping:

  ```go
  func (m *Module) EffectiveExports() []ProviderRef {
      return effectiveExports(m, map[*Module]bool{})
  }

  func effectiveExports(m *Module, visited map[*Module]bool) []ProviderRef {
      if visited[m] {
          return nil
      }
      visited[m] = true

      seen := make(map[ProviderRef]bool, len(m.exports))
      var out []ProviderRef
      add := func(p ProviderRef) {
          if !seen[p] {
              seen[p] = true
              out = append(out, p)
          }
      }

      for _, p := range m.exports {
          add(p)
      }
      for _, re := range m.exportedModules {
          for _, p := range effectiveExports(re, visited) {
              add(p)
          }
      }
      return out
  }
  ```

  Dedup lives HERE (not pushed onto callers) so both `resolver.Find` and
  `direct.go`'s `candidateProviders` get a clean, already-deduped list --
  `direct.go` already dedups its OWN aggregate across multiple scope
  modules today; this just means a single `EffectiveExports()` call no
  longer hands back diamond-duplicated entries for that one module to begin
  with.

  `visited` is shared across the WHOLE recursive walk from one top-level
  `EffectiveExports()` call (not reset per-branch) -- correct for a DAG-like
  re-export graph, and is exactly what makes the cycle guard work: the
  first module to reach an already-visited module in the SAME walk stops
  there, same "each module visited once" invariant Stage 1's own BFS
  (`assemble.go`) already uses for the import graph.

### `internal/module/assemble.go`

`validateExports` gains a second loop, symmetric to the existing provider
one:

```go
func validateExports(m *Module) error {
    declaredProviders := make(map[ProviderRef]bool, len(m.providers))
    for _, p := range m.providers {
        declaredProviders[p] = true
    }
    for _, p := range m.exports {
        if !declaredProviders[p] {
            return fmt.Errorf("module %s exports provider %v it does not declare", moduleName(m), p)
        }
    }

    declaredImports := make(map[*Module]bool, len(m.imports))
    for _, im := range m.imports {
        declaredImports[im] = true
    }
    for _, re := range m.exportedModules {
        if !declaredImports[re] {
            return fmt.Errorf("module %s re-exports module %s it does not import", moduleName(m), moduleName(re))
        }
    }

    return nil
}
```

Runs at the same point in `assemble()` as today (after the whole import BFS
completes, over every visited module) -- no change to Stage 1's own
ordering/timing, only what it checks.

### `internal/resolver/resolver.go`

`findExported` changes its ONE call site from `m.ExportedProviders()` to
`m.EffectiveExports()`:

```go
func findExported(m *module.Module, target reflect.Type) module.ProviderRef {
	for _, p := range m.EffectiveExports() {
		if p.ResolvedType() == target {
			return p
		}
	}
	return nil
}
```

`Find`'s own 2-step search order (own providers, then imported modules'
[now-effective] exports) is UNCHANGED -- only what "an imported module's
exports" means got deeper.

### `internal/resolver/direct.go`

`candidateProviders`'s inner loop changes the same way:

```go
for _, imported := range m.ImportedModules() {
    for _, p := range imported.EffectiveExports() {
        add(p)
    }
}
```

`add`'s own identity-based dedup (already present) stays as the SECOND,
outer layer of dedup (across every module in `scope`, not just within one
`EffectiveExports()` call) -- both layers coexist without conflict, same
"idempotent under re-application" property a working dedup should have.

## Data Flow

1. Dev writes `database.Module` (owns+exports DAO providers),
   `infra.Module` (`Imports(database.Module)`, `ExportModules(database.Module)`),
   root `AppModule` (`Imports(infra.Module)` only -- never touches
   `database.Module` directly).
2. Stage 1 (`assemble`) BFS-walks `AppModule -> infra.Module -> database.Module`
   (via `Imports`, unchanged), runs every module's `fn`, wires
   `OwnerModule`, then validates every module's exports (providers AND now
   re-exported modules) via the extended `validateExports`.
3. A Controller owned by (or a provider constructed within) `AppModule`
   calls `MustInject[*SomeDao]`. `internal/inject` resolves `AppModule`'s
   scope, calls `resolver.Find(AppModule, reflect.TypeFor[*SomeDao]())`.
4. `Find` checks `AppModule`'s own providers (none match) $\to$ walks
   `AppModule.ImportedModules()` = `[infra.Module]` $\to$
   `findExported(infra.Module, target)` $\to$ calls
   `infra.Module.EffectiveExports()` $\to$ recurses into
   `database.Module` (since `infra.Module.exportedModules` contains it)
   $\to$ finds `*SomeDao` in `database.Module.exports`.

## Error Handling Strategy

| Scenario | Behavior |
| --- | --- |
| `B.ExportModules(C)` called without `B.Imports(C)` | Stage 1 `assemble()` returns an error (not a panic) -- same class/timing as today's "exports provider it does not declare" |
| Target type only reachable through a module NOT re-exported (declared+exported by C, but B never called `B.ExportModules(C)`, only `B.Imports(C)`) | `Find` panics "no provider registered for type X" -- same as today, re-export is opt-in per hop, exactly like a plain (non-re-exported) import already hides an un-exported provider |
| Cyclic re-export chain | `EffectiveExports()`'s `visited` guard returns an empty slice for the module encountered a second time in the same walk -- no panic, no hang; if the target genuinely only lived behind the cycle's second entry, `Find`/`FindDirect` still correctly report "not found" (a real, if unusual, DI-graph shape) |

## Testing Strategy

- `internal/module`: unit tests for `ExportModules`/`OwnExportedModules`
  (basic storage), `EffectiveExports()` (single hop, 2-hop transitive chain,
  diamond dedup, self-cycle, mutual cycle -- each a dedicated test per
  Edge Case in spec.md), `assemble()`'s new validation error (re-export
  without import).
- `internal/resolver`: unit tests for `Find` and `FindDirect`/`FindDirectAll`
  each proving the SAME 3-module (A/B/C) transitive scenario spec.md's own
  Independent Test describes, plus the "re-exported module has an
  un-exported provider" negative case (REEXPORT-04).
- Full suite gate (`go test ./... -race -count=1`) after both tasks, per
  this repo's own established convention -- no test type beyond `unit`
  needed (pure in-memory DI-graph logic, no HTTP dispatch involved, matching
  `.specs/codebase/TESTING.md`'s existing "Schema/reflection puro" /
  DI-graph-builder classification).
- Manual verification against the REAL consumer file
  (`C:\dev\leandroluk\erc\ctrl\api\infra\module.go`) as this feature's own
  Success Criteria requires -- `go build ./infra/...` in that project must
  compile clean using the new `ExportModules` API once T4 (below) updates
  it.

## Tech Decisions

| Decision | Rationale |
| --- | --- |
| Separate `ExportModules` method, not a mixed-type `Exports` | Go has no union types; a shared marker interface `*Module` would implement purely to satisfy `Exports`'s existing `ProviderRef` parameter would be a fake abstraction (a Module is not "a kind of provider"). Matches this codebase's own precedent of one method per concept. |
| Dedup + cycle-guard live INSIDE `EffectiveExports()`, not pushed onto each of the 2 callers | Single implementation, both resolution paths automatically correct and never able to diverge -- this repo's own STATE.md (AD-036/037) already documents real bugs from 2 independently-hand-written engines drifting apart; this design structurally prevents that class of bug for this feature. |
| `visited` shared across one whole top-level walk, not per-branch-reset | Matches Stage 1's own BFS "visit each module at most once" invariant (`assemble.go`) -- consistent mental model across the whole package, and is what actually makes a cycle terminate rather than merely slow down. |
