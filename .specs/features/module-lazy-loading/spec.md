# Module Lazy Loading Specification

## Problem Statement

`.examples/notification-driver`'s `notifier.ModuleForRoot(driver string) *gonest.Module`
(`.examples/notification-driver/notifier/module.go`) is a free-function workaround for a
real, recurring need: picking WHICH module to import based on a config value resolved
from `.env` (`NOTIFICATION_DRIVER` → `email.Module_` or `sms.Module_`). It works, but the
decision happens OUTSIDE the DI graph entirely — `main.go` calls `ModuleForRoot(...)`
itself and passes the result into `AppModule_`'s `Imports(...)`, so the config read
(`gonest.Dotenv()...`) happens ad hoc in `main()`, not through a `Provider`/`Schema`
the rest of the app already uses uniformly.

`.specs/insight/PROVIDER.md`'s `database/module.go` sketch (and NestJS's own
`DynamicModule.forRootAsync`, the feature it mirrors) shows the shape this SHOULD take:
a module's own `Config` `Provider` (validated the same way every other config in gonest
already is, via `Schema`/`MustParse`) decides — from INSIDE the DI graph, at bootstrap
time — which sibling module(s) it imports/exports. This was explicitly deferred from
`provider-interface-export` (AD note: "requires reordering bootstrap phases... this
project's historically highest-risk category of work", see `.specs/project/STATE.md`
AD-015) — this spec is that deferred work, now scoped narrowly enough to be safe.

## Goals

- [ ] `Module.Lazy(fn func(l *LazyModule))` — a new `*Module` method, called
      synchronously inside a module's own deferred `fn` (same timing as `Imports`/
      `Providers`/`Exports` — NOT itself deferred further, since Stage 1 assembly reads
      `m.imports` immediately after `fn(m)` returns, see design.md's Architecture
      Overview for why this timing constraint is load-bearing)
- [ ] `gonest.MustInject[T](l)` where `l` is `*LazyModule` — resolves `T` (a pointer
      type) by finding a provider ALREADY registered via `Module.Providers` on the SAME
      module, earlier in the same `fn` call, and constructing it EAGERLY AND
      SYNCHRONOUSLY right there (not through the normal placeholder/Stage-3 path, which
      would not have run yet) — restricted to providers with NO dependencies of their
      own (see Out of Scope; confirmed via `AskUserQuestion` this session)
- [ ] `l.Imports(mods ...*Module)` / `l.Exports(refs ...ExportableRef)` — same
      semantics as `Module.Imports`/`Module.Exports`, callable conditionally based on
      the value(s) obtained via the `MustInject[T](l)` above
- [ ] The provider injected via `MustInject[T](l)` is constructed EXACTLY ONCE for the
      whole bootstrap (not once during `Lazy` and again during the normal Stage 3 pass)
      — Stage 3 must recognize it was already resolved and reuse the cached value

## Out of Scope

| Item | Reason |
| --- | --- |
| Providers with their own dependencies (nested `MustInject` inside a Lazy-injected provider's `Constructor`) | Confirmed via `AskUserQuestion` this session: only self-contained providers (e.g. `Config` reading `.env`/schema, zero further `MustInject` calls) are supported. A Lazy-injected provider's `Constructor` recording any new `PendingEdge` during its eager invocation is a hard error (see Edge Cases), not silently supported. Generalizing to a full synchronous sub-graph resolver was explicitly rejected as much higher risk for no confirmed real use case beyond Config. |
| Injecting a provider registered on an IMPORTED module (not the same module) inside `Lazy` | The module's own `Imports` are not yet finalized at the point `Lazy` runs (that is literally what `Lazy` is deciding) — a provider from an import cannot be available yet. Only this module's OWN `Providers(...)` (registered earlier in the same `fn`) are visible. |
| `Lazy` providers with `ScopeTransient`/`ScopeRequest` | A bootstrap-time decision value must be a single stable instance — restricting to `ScopeSingleton` (panic otherwise) avoids a meaningless "which instance" question. |
| Retrofitting `.examples/notification-driver` to ALSO keep `ModuleForRoot` as a documented alternative pattern | This feature's whole point is to replace it; the example is fully migrated (see Migration Impact), no dual-pattern documentation. |

---

## User Stories

### P1: Pick which module to import based on a config value resolved inside the DI graph ⭐ MVP

**User Story**: As a gonest user with a swappable-backend pattern
(`.examples/notification-driver`'s `email`/`sms` `Notifier`, or `.specs/insight/PROVIDER.md`'s
`memory`/`postgres` repository), I want to write:

```go
var AppModule_ = gonest.NewModule(func(m *gonest.Module) {
  m.Providers(notifier.Config_)
  m.Lazy(func(l *gonest.LazyModule) {
    config := gonest.MustInject[*notifier.Config](l)
    switch config.Driver {
    case "sms":
      l.Imports(sms.Module_)
      l.Exports(sms.Module_)
    default:
      l.Imports(email.Module_)
      l.Exports(email.Module_)
    }
  })
})
```

instead of a free function called from `main()` deciding which `*gonest.Module` to pass
to `Imports` from outside the graph.

**Why P1**: This is the entire feature — replaces `ModuleForRoot`, the one real
consumer confirmed to want this pattern (per this session's `AskUserQuestion`).

**Acceptance Criteria**:

1. WHEN `Module.Lazy(fn)` is called inside a module's own deferred `fn` THEN the
   `LazyModule` callback SHALL run synchronously, before Stage 1 assembly reads that
   module's `Imports`/`Exports`
2. WHEN `MustInject[T](l)` is called for a `T` matching a provider already registered
   via `Providers(...)` earlier in the SAME module's `fn`, AND that provider's
   `Constructor` records no dependency of its own THEN it SHALL return the constructed
   value synchronously
3. WHEN that same provider is later reached by Stage 3's normal resolution pass THEN
   its `Constructor` SHALL NOT run a second time — the cached value from step 2 is
   reused
4. WHEN `l.Imports(...)`/`l.Exports(...)` are called conditionally inside the `Lazy`
   callback THEN the resulting module tree SHALL behave identically (route registration,
   DI resolution, `Module.Exports` encapsulation) to the same `Imports`/`Exports` having
   been called unconditionally with the same final arguments
5. WHEN `.examples/notification-driver` is migrated to this pattern THEN `curl` against
   the running example SHALL still dispatch through both `email`/`sms` drivers
   correctly, config now read via a real `Provider`/`Schema` instead of ad hoc in `main()`

**Independent Test**: `.examples/notification-driver`'s `AppModule_` migrated from
`ModuleForRoot(driver)` (called from `main.go`) to `m.Lazy(...)` reading
`notifier.Config_` (a real `Schema`-validated `Provider`, `env:"NOTIFICATION_DRIVER"`) —
`main.go` no longer picks the module itself.

---

## Edge Cases

- WHEN `MustInject[T](l)` is called for a `T` with no matching provider registered on
  the SAME module before `Lazy` was called THEN it SHALL panic with a message naming
  `T` and stating `Lazy` only sees this module's own `Providers(...)`, registered
  earlier in the same `fn`
- WHEN the provider matched by `MustInject[T](l)` records ANY new `PendingEdge` (i.e.
  its `Constructor` itself calls `MustInject`) during its eager invocation THEN it
  SHALL panic — self-contained-only is a hard constraint, not a soft preference (see
  Out of Scope)
- WHEN the provider matched by `MustInject[T](l)` is NOT `ScopeSingleton` THEN it SHALL
  panic — `Lazy` injection requires a single stable bootstrap-time value
  scope.Singleton is the default, if not explicit the panic is unreachable/no-op
- WHEN the SAME provider is targeted by `MustInject[T](l)` more than once (e.g. 2
  separate `Lazy` calls on the same module, or 2 modules independently injecting a
  provider — cannot happen across modules since `Lazy` only sees own-module providers,
  but CAN happen with 2+ calls inside one `Lazy` callback) THEN the second call SHALL
  reuse the already-resolved value, not re-invoke `Constructor`
- WHEN `Module.Lazy` is never called on a module THEN nothing changes — 100%
  backward-compatible, `Imports`/`Providers`/`Exports` keep working exactly as today
- WHEN `l.Imports`/`l.Exports` are called from OUTSIDE the `Lazy` callback (via a
  captured `*LazyModule` reference used after `fn` returns) THEN behavior is undefined
  by this spec — `LazyModule` is only meant to be used synchronously within the
  callback, same implicit contract `*gonest.Module` itself already has for `fn`

## Migration Impact (informational, not requirements)

- `.examples/notification-driver/main.go`: stops calling `notifier.ModuleForRoot(driver)`
  itself — `AppModule_` (or a new `notifier.Module_`) decides via `Lazy` instead
- `.examples/notification-driver/notifier/module.go`: `ModuleForRoot` removed, replaced
  by a `Lazy`-driven module wiring `Config_` → `email.Module_`/`sms.Module_`
- `.examples/notification-driver/config.go`: `NOTIFICATION_DRIVER` env parsing, if done
  ad hoc today, migrates to a proper `Schema`-validated `Provider` (`notifier.Config_`)
  the `Lazy` callback injects
- `.specs/insight/PROVIDER.md`: status blockquote updated once this ships — its
  `database/module.go` sketch becomes real, not hypothetical

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| LAZY-01 | P1: Module.Lazy runs synchronously before Imports/Exports are read | Pending | Pending |
| LAZY-02 | P1: MustInject[T](l) eagerly constructs a same-module, self-contained provider | Pending | Pending |
| LAZY-03 | P1: eagerly-resolved provider is never constructed twice | Pending | Pending |
| LAZY-04 | P1: conditional Imports/Exports inside Lazy behave identically to unconditional calls | Pending | Pending |
| LAZY-05 | Edge: MustInject[T](l) with no matching own-module provider panics, names T | Pending | Pending |
| LAZY-06 | Edge: provider with its own MustInject dependency panics (self-contained-only) | Pending | Pending |
| LAZY-07 | Edge: non-Singleton provider panics | Pending | Pending |
| LAZY-08 | Edge: repeated MustInject[T](l) for the same provider reuses the cached value | Pending | Pending |

**ID format:** `LAZY-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 8 total, 0 Verified — spec just written, Design/Tasks/Execute not started.

---

## Success Criteria

- [ ] `.examples/notification-driver` migrated to `Module.Lazy`, `curl` against the
      running example still dispatches correctly through both `email`/`sms` drivers,
      config now read via a real `Provider`
- [ ] `go test ./... -race -count=1` green, zero unintended regression
- [ ] `.specs/insight/PROVIDER.md`'s `Module.Lazy` tangent status updated to SHIPPED
