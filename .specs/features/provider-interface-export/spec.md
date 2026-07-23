# Provider Interface Export Specification

## Problem Statement

Two independent, real pains surfaced from `INSIGHT-PROVIDER.md` (a hypothesis draft
written against a swappable-backend pattern the user already applies in NestJS today
— e.g. a `Person` repository backed by `memory` or `postgres`, or a notification
sender backed by `sendgrid`/`smtp`/`aws`, `.examples/notification-driver` being the
real gonest instance of this pattern):

1. **No explicit module→interface mapping.** `MustInject[T](owner)` today resolves
   an interface type `T` by scanning every registered concrete provider in scope and
   matching via `reflect.Type.Implements()` (`internal/resolver/direct.go`,
   `internal/resolver/direct_test.go`'s `TestFindDirect_Interface_*` tests) — no
   explicit declaration anywhere states "this module exports interface `X`,
   implemented by struct `Z`". This has 3 concrete costs: (a) reading a `module.go`
   never reveals which interfaces it exposes — only the concrete struct registered;
   (b) if a struct implements more than one interface there is no way to restrict or
   choose which one a module exposes as "the" export — whatever structurally matches
   resolves, whether intended or not; (c) purely structural matching produces real
   false positives — a generic interface shape like `type Ex[T any] interface{
   Get() T }` collides with `gonest.Accessor[T]` (`internal/value` /
   `gonest.Accessor`) even though the two are unrelated, because `Implements()` only
   checks method sets, not intent.
2. **Naming collision forces an inconsistent workaround.** A provider's exported
   package-level var conventionally shares its struct's name (`Person` struct,
   `Person` provider var) — but Go forbids two package-level identifiers of the same
   name in the same package. The existing workaround, seen throughout
   `INSIGHT-PROVIDER.md`'s sketch and already used ad hoc elsewhere in the project,
   is a trailing underscore (`Person_`). It is currently informal/inconsistent
   rather than a documented convention.

## Goals

- [ ] `gonest.ProviderAs[TInterface any](ref ProviderRef) ProviderRef` — a new
      generic free function (not a method: Go does not support generic methods, so
      this cannot live as `provider.As[T]()`) that wraps an existing `ProviderRef`
      and exposes it as `TInterface`. The returned `ProviderRef`'s own
      `ResolvedType()` reports `TInterface` immediately (no dependency on the
      wrapped provider's Constructor having run yet); it resolves as `TInterface`
      and can be passed to `Module.Providers` (local + same-module resolution) and
      `Module.Exports` (cross-module — same `ExportableRef` value, reusing the
      unified accept-providers-or-modules signature from `unified-exportable-refs`).
      The wrapped ref's CONCRETE registration (e.g. `Person_`) must ALSO be present
      in the module's own `Providers(...)` — the wrapper never drives Stage 3
      construction itself, only the concrete registration does (see Migration
      Impact / Design for why: `internal/resolver/stage3.go`'s `callConstructor`
      hard-errors on any registered `ProviderRef` that isn't independently
      constructable, so the wrapper must be excluded from Stage 3's construction
      pass, not given a fake Constructor)
- [ ] `reflect`-based validation that the wrapped ref actually implements
      `TInterface` cannot run inside `ProviderAs` itself at call time: a
      `*provider.Provider`'s `Constructor` is not registered until its deferred
      builder fn runs at `Declare()` (Stage 2, inside `NewApp`/`MustNewApp`) — at
      package var-init time (when `ProviderAs[T](Person_)` would actually execute,
      if called the way `INSIGHT-PROVIDER.md`'s sketch shows) `Person_.ResolvedType()`
      is still `nil`. The check instead runs as a new validation pass in
      `internal/app`, positioned after `declareProviders` (Stage 2 — every
      registered provider's `ResolvedType()` is reliable by then) and before
      `resolver.Resolve` (Stage 3) — still fails at `NewApp`/`MustNewApp` call time,
      well before any request, just not at var-init
- [ ] `MustInject[T]`/`MustInjectAll[T]`/`FindDirect`/`FindDirectAll` interface
      resolution becomes EXCLUSIVELY explicit: a type is resolvable as interface `T`
      if and only if some `ProviderAs[T](...)` registered it that way. The existing
      `reflect.Type.Implements()` structural fallback is removed entirely — this is
      a breaking change to current behavior (see Migration Impact)
- [ ] Formalize the `Thing_` trailing-underscore suffix as the project-wide naming
      convention for every exported package-level var produced by a gonest builder
      (`Provider_`, `Controller_`, `Module_`, `Listener_`, `Scheduler_`,
      `Resolver_`), applied unconditionally — not only when a same-name struct
      collision actually exists, for one predictable rule instead of a
      conditional one

## Out of Scope

| Feature | Reason |
| --- | --- |
| `Module.Lazy` / config-driven conditional `Imports` (replacing `.examples/notification-driver`'s `ModuleForRoot(driver string) *gonest.Module` free-function workaround) | Real, confirmed need — aims to replicate NestJS's `DynamicModule.forRootAsync` (inject an already-resolved config value, e.g. from the SAME module, to decide which sub-module to import) — but requires reordering bootstrap phases (a `Module`'s own providers, e.g. `Config`, would need to resolve before that module's `Imports`/`Exports` are finalized). Bootstrap-phase changes are this project's historically highest-risk category of work (see `.specs/project/STATE.md` AD-015, "maior risco do projeto até agora" for the 3-phase bootstrap rewrite). Deliberately deferred to its own future spec, discussed but not designed here. |
| Automatic migration tooling for the `Thing_` rename across the whole repo | This spec formalizes the convention going forward; retrofitting existing internal/example code that doesn't yet follow it is a mechanical follow-up task, not a design decision |
| Multi-interface `ProviderAs` (one call exposing a provider as 2+ interfaces at once) | Not requested — call `ProviderAs[A](ref)` and `ProviderAs[B](ref)` separately, both wrapping the same underlying `ProviderRef`, if a provider genuinely needs 2 explicit interface views |

---

## User Stories

### P1: Explicitly declare which interface a provider is exported as ⭐ MVP

**User Story**: As a gonest user with a `memory`/`postgres`-style swappable
implementation (or `sendgrid`/`smtp`/`aws` for a notification sender), I want to
write `gonest.ProviderAs[repository.Person](memory.Person_)` in a module's
`Providers`/`Exports` call and have that be the ONLY thing that makes
`MustInject[repository.Person]` resolve — not an incidental structural match.

**Why P1**: This is the entire feature — the concrete gap named in
`INSIGHT-PROVIDER.md` and confirmed against the real `.examples/notification-driver`
example (`controller.go:28`'s `gonest.MustInject[notifier.Port](controller)`
currently resolves implicitly against whichever of `email.Service`/`sms.Service` was
wired, per the example's own code comment documenting this as deliberate-but-fragile).

**Acceptance Criteria**:

1. WHEN `gonest.ProviderAs[TInterface](ref)` is called AND `ref`'s resolved type
   implements `TInterface` THEN it SHALL return a `ProviderRef` that resolves as
   `TInterface` wherever registered (`Providers` and/or `Exports`)
2. WHEN `gonest.ProviderAs[TInterface](ref)` is called AND `ref`'s resolved type
   does NOT implement `TInterface` THEN it SHALL panic immediately (at call time,
   not deferred to `MustInject` resolution) with a message naming both the concrete
   type and the target interface
3. WHEN a concrete provider is registered via `Providers` WITHOUT ever being wrapped
   in `ProviderAs` THEN `MustInject[SomeInterfaceItHappensToImplement]` SHALL fail to
   resolve it — structural `reflect.Implements()` matching is no longer a fallback
4. WHEN the same underlying `ProviderRef` is wrapped by `ProviderAs` for 2 different
   interfaces (2 separate calls) THEN both interfaces SHALL independently resolve to
   the same singleton instance
5. WHEN a `ProviderAs`-wrapped ref is passed to `Module.Exports` THEN an importing
   module SHALL be able to `MustInject[TInterface]` it, following the same
   encapsulation/re-export rules `Exports` already enforces for plain providers

**Independent Test**: `.examples/notification-driver` migrated to use
`gonest.ProviderAs[port.Notifier](email.Service_)` /
`gonest.ProviderAs[port.Notifier](sms.Service_)` (wrapped inside the `notifier`
package, which already imports both `port` and the `email`/`sms` impls — no new
import edge, no cycle) in place of the current bare implicit match; `curl` against
the running example still dispatches through whichever driver `NOTIFICATION_DRIVER`
selected, unchanged from the user's point of view.

### P2: Consistent `Thing_` naming convention for builder vars

**User Story**: As a gonest user, I want every exported package-level var produced
by `gonest.NewProvider`/`NewController`/`NewModule`/`NewListener`/`NewScheduler`/
`NewResolver` to follow one unconditional naming rule (`Thing_`), so I never have to
decide case-by-case whether a suffix is needed.

**Why P2**: Directly requested, tied to the same reflection — real driver: Go
forbids `type Person struct{}` and `var Person = ...` colliding in the same package,
and the user wants ONE rule instead of a collision-dependent one.

**Acceptance Criteria**:

1. WHEN documenting or writing a new gonest builder var THEN its name SHALL be the
   domain concept name plus a trailing underscore (`Person_`, `NotificationController_`),
   regardless of whether a same-named struct/type exists in that package
2. WHEN `.specs/features/provider-interface-export` ships THEN the convention SHALL
   be written down as project documentation (this spec + a follow-up note in
   whichever doc governs contributor conventions), consumed later by
   `C:\dev\gonest-dev\site` per the user's stated intent to reflect it there

---

## Edge Cases

- WHEN `ProviderAs[TInterface]` wraps a `ProviderRef` that is ITSELF the result of
  another `ProviderAs` call (chaining) THEN behavior SHALL be well-defined — either
  explicitly supported (resolves as both interfaces) or explicitly rejected with a
  clear panic; Design must pick one, not leave it silently undefined
- WHEN 2 DIFFERENT concrete providers are each wrapped via `ProviderAs[TInterface]`
  and BOTH end up in the same resolution scope (e.g. both imported into the same
  consumer) THEN `MustInject[TInterface]` SHALL fail with the existing ambiguity
  error (mirrors today's `TestFindDirect_Interface_TwoImplementors_AmbiguousNotFoundHere`
  behavior, just against explicit registrations instead of implicit matches)
- WHEN `MustInjectAll[TInterface]` is used (multi-binding, e.g. health-check-style
  `Connectable`/`Pingable` aggregation in `gonest_test.go`) THEN every provider
  explicitly wrapped via `ProviderAs[TInterface]` in scope SHALL be included — this
  path currently also relies partly on implicit matching and needs its own
  migration, not just single-resolution `MustInject`
- WHEN a `ProviderAs[TInterface]`-wrapped ref is registered via `Providers`/`Exports`
  but its underlying CONCRETE ref was never separately registered anywhere THEN the
  new post-Stage-2 validation pass SHALL fail loud with a clear message (not leave
  the interface silently unresolvable at first `MustInject[TInterface]` call, which
  would read as "provider not found" instead of "setup mistake")
- WHEN Stage 3 (`internal/resolver/stage3.go`'s `allProviders`/`resolveGraph`)
  encounters a registered `ProviderRef` that is a `ProviderAs` wrapper (not
  independently `constructable`) THEN it SHALL exclude it from the construction
  pass rather than hard-error via `callConstructor`'s existing
  `"does not expose a Constructor"` path — that error path remains a real invariant
  violation for any OTHER kind of non-constructable `ProviderRef`, so the exclusion
  must be scoped to wrapper refs specifically (e.g. via a dedicated marker), not a
  blanket "skip anything non-constructable"

## Migration Impact (informational, not requirements)

- `internal/resolver/direct_test.go`: ~6 tests assert implicit `Implements()`
  matching directly (`TestFindDirect_Interface_SingleImplementor_Resolves`,
  `TestFindDirect_Interface_TwoImplementors_AmbiguousNotFoundHere`,
  `TestFindDirect_Interface_ExactMatchTakesPrecedenceOverImplements`,
  `TestFindDirectAll_Interface_ReturnsEveryImplementor`,
  `TestFindDirectAll_Interface_ZeroImplementors_ReturnsEmpty`,
  `TestResolveWithOverrides_InterfaceOverride_SkipsRealConstructor`) — all need
  rewriting against explicit `ProviderAs` registration instead of bare structs
- `gonest_test.go`: `MustInjectAll[insightConnectable]`/`MustInjectAll[insightPingable]`
  multi-binding integration tests currently rely on implicit matching too — need
  review, likely partial rewrite
- `.examples/notification-driver`: the ONLY real (non-test) call site relying on
  implicit interface injection in the whole repo
  (`controller.go:28`, `gonest.MustInject[notifier.Port](controller)`) — confirmed
  via full-repo search. Migration path already sketched in P1's Independent Test
  above, no import cycle introduced

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| PROVAS-01 | P1: ProviderAs wraps a ref, validates via reflect, panics on mismatch | Pending | Pending |
| PROVAS-02 | P1: ProviderAs is the ONLY path to interface resolution (implicit fallback removed) | Pending | Pending |
| PROVAS-03 | P1: same ref wrapped for 2 interfaces resolves both to same singleton | Pending | Pending |
| PROVAS-04 | P1: ProviderAs-wrapped ref respects Module.Exports encapsulation rules | Pending | Pending |
| PROVAS-05 | Edge: ProviderAs chaining behavior explicitly defined | Pending | Pending |
| PROVAS-06 | Edge: 2 explicit ProviderAs registrations for same interface in scope still ambiguous | Pending | Pending |
| PROVAS-07 | Edge: MustInjectAll multi-binding migrated to explicit ProviderAs too | Pending | Pending |
| PROVAS-08 | Edge: post-Stage-2 validation fails loud when wrapped concrete never separately registered | Pending | Pending |
| PROVAS-09 | Edge: Stage 3 excludes ProviderAs wrapper refs from construction pass without weakening the existing non-constructable hard-error for real invariant violations | Pending | Pending |
| NAMING-01 | P2: Thing_ suffix convention documented unconditionally | Pending | Pending |

**ID format:** `PROVAS-[NUMBER]` (ProviderAs), `NAMING-[NUMBER]` (naming convention)

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 10 total, 0 Verified — spec just written, Design/Tasks/Execute not started.

---

## Success Criteria

- [ ] `.examples/notification-driver` migrated to `ProviderAs`, `curl` against the
      running example still dispatches correctly through both `email`/`sms` drivers
- [ ] `go test ./... -race -count=1` green, zero unintended regression (all
      intentional test rewrites tracked in Migration Impact above)
- [ ] Project convention doc updated with the `Thing_` naming rule (exact location —
      README.md / a new CONVENTIONS note / `.specs/insight/PROVIDER.md` — decided in
      Design)
- [ ] `INSIGHT-PROVIDER.md` reflection folded into `.specs/insight/PROVIDER.md`
      (mirrors the in-progress `INSIGHT-LAZY.md` → `.specs/insight/LAZY.md` reorg
      already staged in the working tree), Module.Lazy tangent kept as a forward
      pointer for its own future spec
