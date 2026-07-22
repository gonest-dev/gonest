# Unified Exports — Design

## Approach

New marker interface `ExportableRef` (mirrors `ProviderRef`/`ControllerRef`'s own marker
pattern in `internal/module/module.go`):

```go
type ExportableRef interface {
    IsExportable()
}
```

`ProviderRef` embeds it (so every `ProviderRef` already satisfies `ExportableRef`, no extra
work at call sites):

```go
type ProviderRef interface {
    ExportableRef
    IsProvider()
    ResolvedType() reflect.Type
    SetOwnerModule(m *Module)
}
```

`*Module` gets a trivial `IsExportable() {}` — same shape as `(*Provider) IsProvider()`,
`(*Controller) IsController()`.

`Exports` signature changes from `...ProviderRef` to `...ExportableRef`, type-switches per arg:

```go
func (m *Module) Exports(refs ...ExportableRef) {
    for _, ref := range refs {
        switch v := ref.(type) {
        case *Module:
            m.exportedModules = append(m.exportedModules, v)
        case ProviderRef:
            m.exports = append(m.exports, v)
        }
    }
}
```

`ExportModules` deleted. `OwnExportedModules`/`ExportedProviders`/`EffectiveExports`/
`validateExports` untouched (read from the same 2 fields, populated differently now).

## Concrete provider type

`internal/provider/provider.go`'s `*Provider` needs `IsExportable() {}` to keep satisfying
`ProviderRef` (interface grew a method). Same for any test fakes implementing `ProviderRef`
(`internal/module/module_test.go`'s `fakeProvider`, `internal/resolver/{resolver,direct}_test.go`'s
`fakeProvider`).

## Callsites to migrate (ExportModules → Exports)

- `internal/module/reexport_test.go` (all `m.ExportModules(x)` → `m.Exports(x)`)
- `internal/resolver/{resolver_test.go,direct_test.go}` (same)
- `C:\dev\leandroluk\erc\ctrl\api\app\module.go` (real consumer, already migrated once to
  `ExportModules` in this same session — reverts to `Exports`)
- `.specs/features/module-reexport/{spec,design,tasks}.md` — left as historical record
  (AD-051 happened, was real, was superseded — not rewritten), but STATE.md gets a new AD
  entry (AD-052) documenting the reversal and pointing back at AD-051.

## Tech Decision (reverses design.md:219 of module-reexport)

| Decision | Rationale |
| --- | --- |
| Unify into `Exports(...ExportableRef)`, `*Module` implements a marker interface purely to qualify | Previously rejected as "fake abstraction" under a Go-idiomatic-purity lens. Reversed because PROJECT.md's own Goals now state explicitly: Nest-parity wins over Go-purity when they conflict. Nest's `exports: [...]` is genuinely one array accepting both — replicating that shape lowers friction for the TypeScript dev audience this whole project targets, which outweighs the "Module is not a kind of Provider" purity objection. |
| Type switch inside `Exports`, not compile-time separation | Go has no union types — this is the only way to get one call accepting two disjoint types. Trade real: a caller CAN write `m.Exports(wrongThing)` where `wrongThing` implements `ExportableRef` in an unexpected way only if they hand-roll a fake `IsExportable()` — not realistic in practice since concrete types are internal (`*Provider`, `*Controller` don't implement it, only `ProviderRef`-satisfying types and `*Module` do). |

## Testing Strategy

`go test ./... -race -count=1` after the change — same gate as every prior AD in this repo.
No new test *files* needed beyond updating existing `ExportModules(...)` calls to `Exports(...)`
in place (behavior being tested is unchanged, only the entry point).
