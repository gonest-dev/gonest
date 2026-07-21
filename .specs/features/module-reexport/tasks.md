# Module Re-export Tasks

**Spec**: `.specs/features/module-reexport/spec.md`
**Design**: `.specs/features/module-reexport/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: ✅ T1-T2 COMPLETE (evaluator: PASS, gate sem cache 2x). T3 COMPLETE com ressalva: `go build ./infra/...` real passou em `ctrl/api` (v0.23.0, `ExportModules` em uso) confirmando a API; verificação em RUNTIME (MustInject real através do reexport) ficou bloqueada por um panic pré-existente e não-relacionado no `init()` de `database/table` daquele projeto (nil pointer, fora do escopo desta feature) -- cobertura funcional já provada pelos próprios testes de T1/T2 (cenário transitivo de 3 módulos via Find/FindDirect).

---

## Execution Plan

```
T1 (internal/module: ExportModules/OwnExportedModules/EffectiveExports + validateExports)
  → T2 (internal/resolver: Find + FindDirect/FindDirectAll use EffectiveExports)
  → T3 (real consumer file: C:\dev\leandroluk\erc\ctrl\api\infra\module.go)
```

Sequential -- T2 needs T1's `EffectiveExports()` to exist; T3 needs the real,
working API from both.

---

## Task Breakdown

### T1: `internal/module` -- ExportModules, EffectiveExports, validateExports

**What**: `internal/module/module.go`'s `Module` struct gains an unexported
`exportedModules []*Module` field (same storage shape as `providers`/
`exports`). Add:
- `func (m *Module) ExportModules(mods ...*Module)` -- appends to
  `exportedModules`. NO return value (match `Providers`/`Controllers`/
  `Exports`'s own existing shape exactly -- this codebase never makes these
  chainable, do not introduce that here).
- `func (m *Module) OwnExportedModules() []*Module` -- defensive-copy
  getter, same pattern as `ImportedModules()`/`OwnProviders()` (copy the
  slice before returning, never the internal one directly).
- `func (m *Module) EffectiveExports() []ProviderRef` -- own
  `m.exports` (NOT `m.providers` -- only what's actually exported) PLUS,
  for each module in `m.exportedModules`, that module's OWN
  `EffectiveExports()`, recursively. Delegates to an unexported
  `effectiveExports(m *Module, visited map[*Module]bool) []ProviderRef`
  helper carrying the visited set across the WHOLE recursive walk (not
  reset per branch) so a cycle (mutual re-export, or a module re-exporting
  itself) terminates: if `visited[m]` is already true, return `nil`
  immediately without recursing further. Dedup the RETURNED list by
  `ProviderRef` identity (a `map[ProviderRef]bool` local to each
  `effectiveExports` call, exact same shape as `direct.go`'s existing
  `candidateProviders` dedup) so a diamond re-export chain never returns
  the same provider twice. See design.md's Components section for the
  exact reference implementation -- follow it, this is not open-ended.

`internal/module/assemble.go`'s `validateExports` gains a second check,
symmetric to the existing provider one: every module in `m.exportedModules`
must also be present in `m.imports` (compare by `*Module` identity, a
`map[*Module]bool` built from `m.imports` same shape as the existing
`declared` map built from `m.providers`). Return an error (not panic) in the
SAME style as the existing one: `"module %s re-exports module %s it does
not import"`.

**Where**: `internal/module/module.go` (existing, extended),
`internal/module/assemble.go` (existing, extended),
`internal/module/module_test.go` (existing, extended) or a new
`internal/module/reexport_test.go` if that reads cleaner for this many new
cases -- caller's judgment, match whichever this package's existing test
file organization already leans toward.

**Depends on**: none
**Reuses**: `providers`/`exports`/`imports` fields' own storage shape,
`ImportedModules()`/`OwnProviders()`'s own defensive-copy getter pattern,
`assemble.go`'s existing `validateExports` structure (extend, don't rewrite)
**Requirement**: REEXPORT-01, REEXPORT-02, REEXPORT-05, REEXPORT-06 (spec.md)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `ExportModules(mods...)` stores them, `OwnExportedModules()` returns a
  defensive copy (mutating the returned slice does not affect internal
  state -- same test shape as `TestOwnProperties_ReturnsCopyNotInternalSlice`
  precedent elsewhere in this codebase if one already exists for `Module`,
  otherwise a fresh equivalent)
- [ ] `EffectiveExports()` on a module with NO re-exported modules returns
  exactly its own `OwnProviders()`-that-are-in-`exports`... i.e. exactly what
  `ExportedProviders()` already returns today (regression-safety baseline)
- [ ] `EffectiveExports()` on a 2-hop chain (B re-exports C) includes C's own
  exported providers
- [ ] `EffectiveExports()` on a 3-hop chain (B re-exports C, C re-exports D)
  includes D's own exported providers (transitivity, not just 1 extra hop)
- [ ] A provider C declared via `Providers` but NEVER passed to `Exports` is
  NOT present in B's `EffectiveExports()` even though B re-exports C
  (re-export only propagates what's ALREADY exported, never bypasses C's own
  encapsulation)
- [ ] Diamond (B re-exports C and D, both C and D re-export shared module E)
  -- E's own exported provider appears exactly ONCE in B's
  `EffectiveExports()`, not twice
- [ ] Self re-export (`B.ExportModules(B)`) and mutual cycle (B re-exports C,
  C re-exports B, both real via mutual `Imports`) do not hang or
  stack-overflow -- test with a timeout-guarded goroutine or simply prove the
  call returns (Go's own test runner default timeout is enough of a guard,
  no need for a manual one)
- [ ] `assemble()` returns an error (not panic, not silent success) when a
  module calls `ExportModules(X)` without `Imports(X)`
- [ ] Gate check passes: `go test ./... -race -count=1` (full suite, run
  from repo root)
- [ ] Test count: 10+ covering the bullets above

**Tests**: unit
**Gate**: full

**Commit**: `feat(module): add ExportModules for transitive re-export of a whole imported module`

---

### T2: `internal/resolver` -- Find and FindDirect/FindDirectAll use EffectiveExports

**What**: `internal/resolver/resolver.go`'s `findExported` changes its ONE
call from `m.ExportedProviders()` to `m.EffectiveExports()`. No other change
to `Find`'s own 2-step search order (own providers, then imported modules'
exports) -- only what "an imported module's exports" resolves to gets
deeper. `internal/resolver/direct.go`'s `candidateProviders` inner loop
(`for _, p := range imported.ExportedProviders()`) changes the same way to
`imported.EffectiveExports()`. `add`'s own existing identity-dedup (already
present in `candidateProviders`) stays as-is -- it's now a second,
outer layer of dedup on top of `EffectiveExports()`'s own inner one (across
modules in `scope`, not just within one module's re-export chain), the 2
layers do not conflict.

**Where**: `internal/resolver/resolver.go` (existing, extended),
`internal/resolver/direct.go` (existing, extended),
`internal/resolver/resolver_test.go` (existing, extended),
`internal/resolver/direct_test.go` (existing, extended)

**Depends on**: T1 (needs `Module.EffectiveExports()` to exist)
**Reuses**: `Find`/`findOwn`/`hasOwnUnexported`'s existing structure
(extend, don't rewrite), `candidateProviders`/`findDirectMatches`'s existing
structure
**Requirement**: REEXPORT-03, REEXPORT-04 (spec.md)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Reproduce spec.md's own Independent Test with BOTH `Find` (via a
  `MustInject[T]`-shaped call, or `Find` directly if that's more direct to
  unit-test at this layer) AND `FindDirect`/`FindDirectAll`: A imports ONLY
  B, B imports+re-exports C, C owns+exports a provider -- resolving that
  provider's type FROM A's scope succeeds through both paths
- [ ] `Find` still panics "no provider registered" when the target is
  reachable only through a module that was imported but NOT re-exported
  (regression-safety: re-export stays strictly opt-in, matching today's
  plain-import-without-export behavior)
- [ ] `Find`'s OTHER existing panic ("exists in module %s but is not
  exported") still fires for its own original (non-re-export) scenario --
  explicit regression test, not just "no failure"
- [ ] Gate check passes: `go test ./... -race -count=1` (full suite)
- [ ] Test count: 6+ covering the bullets above

**Tests**: unit
**Gate**: full

**Commit**: `feat(resolver): Find and FindDirect walk re-exported modules transitively`

---

### T3: fix the real consumer file (external validation)

**What**: `C:\dev\leandroluk\erc\ctrl\api\infra\module.go` currently has
`m.Exports(database.Module)`, which does not compile against gonest's
CURRENT (pre-T1/T2) API and was never the intended call anyway (`Exports`
takes providers, not modules). Change it to use the new `ExportModules`:

```go
var Module = gonest.NewModule(func(m *gonest.Module) {
	m.Imports(database.Module)
	m.ExportModules(database.Module)
})
```

This consumer project is OUTSIDE this repo (its own `go.mod`) -- it needs to
pick up gonest's new version (the tag T4-equivalent work, i.e. the version
bump this feature's own STATE.md/commit workflow produces) before this
compiles. If the consumer's `go.mod` pins an older gonest version or uses a
`replace` directive pointing at a local checkout, update/confirm whichever
applies so `go build ./infra/...` in that project actually exercises the
NEW code, not a stale cached version.

**Where**: `C:\dev\leandroluk\erc\ctrl\api\infra\module.go` (existing,
1-line-shape change, outside this repo)

**Depends on**: T1, T2
**Reuses**: nothing new -- this is the feature's own real-world proof, not
new gonest code
**Requirement**: spec.md's own Success Criteria ("the exact scenario...
compiles and resolves correctly using the new API")

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `go build ./infra/...` (and ideally `./...`, if the rest of that
  project already builds cleanly independent of this change) succeeds in
  `C:\dev\leandroluk\erc\ctrl\api`
- [ ] Confirm (via a real `MustInject`/bootstrap path if one already exists
  in that project, e.g. an existing test or a quick throwaway `main`) that a
  DAO type is actually resolvable through `infra.Module` without anything
  importing `database.Module` directly -- not just "it compiles", the
  RUNTIME behavior this whole feature exists for
- [ ] No other file in that project needed to change

**Tests**: manual (external consumer project, not part of gonest's own
`go test ./...`)
**Gate**: quick (build + a real resolve check)

**Commit**: none in THIS repo (the change lives in the consumer project) --
if that project is under its own git control, commit there per its own
convention, not gonest's.

---

## Parallel Execution Map

```
Sequential: T1 → T2 → T3
```

**Papéis por task (Subagent workflow convention em STATE.md):** Implementer
sub-agent implementa, Evaluator sub-agent (ou eu mesmo, escopo pequeno o
bastante) confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: ExportModules/EffectiveExports/validateExports | 2 arquivos existentes (module.go, assemble.go) + testes, 1 pacote | ✅ Granular |
| T2: resolver usa EffectiveExports | 2 arquivos existentes (resolver.go, direct.go) + testes, 1 pacote | ✅ Granular |
| T3: consumer real | 1 arquivo, fora deste repo, verificação manual | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | DI graph builder puro (`internal/module`) | unit | unit | ✅ OK |
| T2 | Motor de resolução (`internal/resolver`) | unit | unit | ✅ OK |
| T3 | Consumer externo, fora da suite `go test` deste repo | manual | manual | ✅ OK |

Nenhuma violação.
