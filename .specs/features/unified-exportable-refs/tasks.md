# Unified Exports — Tasks

Single atomic task (small scope: 1 interface addition, 1 method signature change, marker
methods on 2 concrete types + fakes, callsite migration). Runs via 1 Implementer subagent +
Evaluator (this session) review, same 3-persona pattern as AD-051 (module-reexport).

## T1: ExportableRef unification

**What:**
1. `internal/module/module.go`: add `ExportableRef interface { IsExportable() }`; make
   `ProviderRef` embed it; rewrite `Exports(refs ...ExportableRef)` with a type switch
   (`*Module` → `m.exportedModules`, `ProviderRef` → `m.exports`); add
   `func (m *Module) IsExportable() {}`; delete `ExportModules`; update doc comments on
   `Exports`, `OwnExportedModules`, `ExportedProviders`, `EffectiveExports` that reference the
   old 2-method split.
2. `internal/provider/provider.go`: add `func (p *Provider) IsExportable() {}` next to the
   existing `IsProvider()`.
3. `internal/module/assemble.go`: update the doc comment on `validateExports` that mentions
   `ExportModules` by name (logic itself unchanged, reads the same 2 fields).
4. Test fakes implementing `ProviderRef` (`internal/module/module_test.go`,
   `internal/resolver/resolver_test.go`, `internal/resolver/direct_test.go`) get
   `IsExportable() {}` added to their `fakeProvider`.
5. `internal/module/reexport_test.go`, `internal/resolver/{resolver_test.go,direct_test.go}`:
   replace every `m.ExportModules(x)` call with `m.Exports(x)`. Test names mentioning
   `ExportModules` may keep the name (historical) or be renamed to `Exports` — reviewer's call,
   not worth churn either way.
6. Real consumer `C:\dev\leandroluk\erc\ctrl\api\app\module.go`: `m.ExportModules(system.Module)`
   → `m.Exports(system.Module)`.
7. `.specs/project/STATE.md`: new `AD-052` entry documenting the reversal (references AD-051,
   cites the new PROJECT.md Goals premise as the reason).

**Where:** `internal/module/{module.go,assemble.go,module_test.go,reexport_test.go}`,
`internal/provider/provider.go`, `internal/resolver/{resolver_test.go,direct_test.go}`,
`C:\dev\leandroluk\erc\ctrl\api\app\module.go`, `.specs/project/STATE.md`.

**Depends on:** nothing (AD-051 already merged).

**Reuses:** `EffectiveExports`, `validateExports`, `OwnExportedModules`, `ExportedProviders` —
all read-side logic, zero changes needed.

**Done when:** `ExportModules` no longer exists anywhere in `internal/module`; `Exports`
accepts both a `ProviderRef` and a `*Module` in the same call; `go build ./...` (gonest) and
`go build ./...` (erc's `app` package) both compile clean.

**Tests:** existing `reexport_test.go`/`resolver_test.go`/`direct_test.go` suites, migrated
in place — same assertions, new entry point. No new test cases required (behavior identical,
only the call surface changed per EXPORT-05).

**Gate:** `go test ./... -race -count=1` (gonest repo) green, 24+ packages, zero pre-existing
assertion changed.
