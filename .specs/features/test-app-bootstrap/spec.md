# Test App Bootstrap Specification

## Problem Statement

Milestone 8's first feature was supposed to be `MustNewTestApp`/`TestBuilder`/`MustOverride[Interface]` (provider override for testing). Investigating how override-by-interface could work exposed that gonest's DI graph has NO interface-typed injection at all today (`MustInject[T]` panics unless `T` is a pointer kind), and that the existing placeholder+copy-in-place mechanism (which lets `MustInject[*Concrete]` return a usable value BEFORE the real dependency graph resolves) architecturally cannot generalize to interface types -- an interface value, once returned by value to a caller, has no further live indirection to a not-yet-resolved dependency the way a stable pointer does. See `context.md` for the full investigation and the 3-round decision trail with the user, which settled on reordering `NewApp`'s bootstrap into two phases (Provider resolution, then Consumer declaration) rather than any interface-injection workaround.

## Goals

- [ ] `NewApp`/`MustNewApp` bootstrap reordered into THREE phases (see context.md's Discovery 4 for why two isn't enough): (1) declare + fully resolve every `*Module`'s own `Providers()` (Stage 1-3, UNCHANGED internally -- same placeholder/topological/errgroup mechanism, same cycle detection); (2) declare every `*Module`'s own `Controllers()` (their builder closures run now, recording which `Middleware`/`Guard`/`Interceptor`/`Filter` values they reference via `Use()`/`Guards()`/`Interceptors()`/`Filters()`, WITHOUT yet running those referenced values' OWN builder closures); (3) for every DISTINCT `Middleware`/`Guard`/`Interceptor`/`Filter` value referenced anywhere in phase 2, run its OWN (now-deferred, per AD-008 reversal below) builder closure, with an effective `MustInject`/`MustInjectAll` search scope equal to the UNION of every referencing Controller's owning module's own+exported providers
- [ ] **AD-008 reversed**: `Middleware`/`Guard`/`Interceptor`/`Filter` change from immediate `New(fn)` execution (today, effectively at Go package-`init()` time for the typical `var X = gonest.NewGuard(...)` declaration style) to DEFERRED execution (`Declare()`-based, same pattern `Provider`/`Controller`/`Module` already use) -- run during phase 3 above, not before `NewApp` is even called
- [ ] `MustInject[T]`, called from a Controller's builder closure (phase 2) OR a Middleware/Guard/Interceptor/Filter's builder closure (phase 3), resolves DIRECTLY against the relevant already-resolved provider set -- no placeholder, no deferred copy-in-place -- for BOTH pointer `T` (existing behavior, same call-site shape, zero observable change) and (NEW) interface `T` (exact match OR `reflect.Type.Implements()` fallback against the relevant scoped provider set; panics if zero or 2+ matches)
- [ ] `MustInjectAll[T any](owner) []T` -- NEW, `T` MUST be an interface kind (panics otherwise); returns every resolved provider (as `T`, within the caller's scope -- module-scoped for Controller, union-scoped for Middleware/Guard/Interceptor/Filter) whose concrete type satisfies `T`, empty slice if none, never panics on 0 or 2+ matches (that's the whole point vs `MustInject[T]`)
- [ ] `Provider`'s OWN dependency declaration (a Provider's builder closure calling `MustInject[*OtherProvider](provider)` on ANOTHER provider) is UNCHANGED -- still phase-1, still placeholder-based, since providers may depend on each other in any topological order (cycles detected exactly as today)
- [ ] Zero observable regression for every EXISTING pointer-typed `MustInject` call site already shipped this session (every Middleware/Guard/Interceptor/Filter/Provider/Controller example, every test) -- including the TIMING change for Middleware/Guard/Interceptor/Filter (their side effects, if any, now happen during `NewApp`, not at package-init time -- audit every existing test that might depend on the OLD immediate-execution timing)

## Out of Scope

| Feature | Reason |
| --- | --- |
| `MustNewTestApp`/`TestBuilder`/`MustOverride[T]` | Genuinely separate concern (test-mode bootstrap variant) -- this spec covers ONLY the underlying bootstrap-reorder + interface-injection PREREQUISITE; the test-app feature itself is `.specs/features/test-app-bootstrap`'s SECOND spec section below (P3), built directly on top of P0-P2 |
| `MustInject[T]`/`MustInjectAll[T]` callable from WITHIN `Route.Handler` (per-request) | User explicitly rejected this (context.md's Decision 2) -- resolution happens once, at builder-closure declare time (phase 2), never per-request |
| Provider constructors gaining new dependency-param signatures beyond the existing 4 (`func()`, `func()(T,error)`, `func(ctx)T`, `func(ctx)(T,error)`) | Not asked for, not needed by this feature -- Provider-to-Provider deps continue via `MustInject` inside the Provider's OWN builder closure, exactly as today |

---

## User Stories

### P0: Three-phase bootstrap (prerequisite, blocking, HIGHEST RISK OF THIS SESSION) ⭐ MVP

**User Story**: As the framework itself, every `New*`-builder type's closure body is meant to simulate a real constructor -- ANY dependency it grabs via `MustInject`/`MustInjectAll` should already be a REAL, resolved value at that point (context.md's "Underlying rationale"), never a placeholder that becomes valid later. This requires ALL of Provider, Controller, and (reversing AD-008) Middleware/Guard/Interceptor/Filter to defer their builder closures until their OWN dependencies are knowable and resolvable.

**Acceptance Criteria**:

1. WHEN `NewApp`/`MustNewApp` runs THEN EVERY `*Module`'s own `Providers()` SHALL be declared and FULLY resolved (Stage 1-3, unchanged mechanism) BEFORE any `Controller` builder closure anywhere in the module tree runs (phase 1 → phase 2 ordering)
2. WHEN every `Controller` builder closure has run (phase 2) THEN system SHALL have recorded, for every DISTINCT `*Middleware`/`*Guard`/`*Interceptor`/`*Filter` value referenced (via `Use()`/`Guards()`/`Interceptors()`/`Filters()`) by ANY Controller, the FULL SET of modules that referenced it (context.md's Decision 4 -- ownership = union of referencing modules)
3. WHEN phase 2 completes THEN system SHALL run each DISTINCT Middleware/Guard/Interceptor/Filter value's OWN (now-deferred) builder closure EXACTLY ONCE (even if referenced by multiple Controllers/modules), with `MustInject`/`MustInjectAll` calls inside it searching the UNION of every referencing module's own+exported providers (phase 3)
4. WHEN a Provider's own builder closure calls `MustInject[*OtherProvider]` (Provider-to-Provider, phase 1) THEN behavior SHALL be IDENTICAL to today -- placeholder allocated, resolved via Stage 3's existing topological/errgroup/cycle-detecting mechanism (this is the ONE case that still needs placeholder-based deferred resolution, since providers may depend on each other in unpredictable topological order)
5. WHEN every EXISTING test in this codebase that exercises `MustInject[*Concrete]` from a Controller/Guard/Middleware/Interceptor/Filter builder closure, OR that depends on the OLD immediate-execution timing of `NewGuard`/`NewMiddleware`/`NewInterceptor`/`NewFilter` (e.g. any side effect expected to happen at package-init time rather than during `NewApp`), is re-run AFTER this reorder THEN it SHALL show ZERO regression in FINAL observable behavior -- TIMING of side effects may legitimately shift (now during `NewApp`, not before it), but no test's own ASSERTIONS should need to change to accommodate this (if any DO need to change, that is itself a signal this reorder broke something and needs investigation, not a assertion to silently update)

**Independent Test**: build a module tree with a Provider chain (A depends on B depends on C), a Controller depending on A (concrete pointer, existing style), AND a `*Guard` referenced by TWO Controllers in TWO DIFFERENT modules (one module's own provider, one imported/exported provider from the other) -- confirm: (a) the Controllers' builder closures run strictly after A/B/C are constructed; (b) the Guard's own builder closure runs exactly ONCE (not twice, despite 2 references), with visibility into BOTH referencing modules' provider sets (union-scoped `MustInject` succeeds for a provider from EITHER module). Also: full existing suite (`go test ./... -race`) green, with a documented accounting of every test whose TIMING assumptions needed updating (if any) vs. every test that needed a genuine ASSERTION change (target: zero of the latter).

---

### P1: Interface-typed `MustInject[T]`

**User Story**: As a gonest user, I want `userService := gonest.MustInject[IUserService](controller)` (an interface type argument) to resolve to whichever registered Provider's concrete type implements `IUserService`, panicking if zero or more than one provider qualifies.

**Acceptance Criteria**:

1. WHEN `MustInject[T]` is called with `T` an interface kind, from a Controller builder closure (phase 2, module-scoped: own+exported providers of the Controller's OWN module) OR a Middleware/Guard/Interceptor/Filter builder closure (phase 3, union-scoped: own+exported providers of EVERY referencing module, context.md's Decision 4) THEN system SHALL search the relevant scoped provider set for ones whose concrete `ResolvedType()` satisfies `reflect.Type.Implements(T)` (or is an EXACT match for `T` itself, e.g. an override provider whose constructor's OWN declared return type is `T`)
2. WHEN exactly ONE provider matches THEN system SHALL return its resolved value, converted/asserted to `T`
3. WHEN ZERO providers match THEN system SHALL panic with a clear "no provider implements interface X" message
4. WHEN 2+ providers match THEN system SHALL panic with a clear "ambiguous: N providers implement interface X, use MustInjectAll" message
5. WHEN `MustInject[T]` is called with `T` a POINTER kind THEN behavior SHALL be UNCHANGED from today's exact-match semantics (P0's own zero-regression requirement)

**Independent Test**: register 2 providers, one implementing `Animal` (only one of them), `MustInject[Animal]` resolves to it; register a SECOND provider also implementing `Animal`, `MustInject[Animal]` now panics ambiguous; remove both, `MustInject[Animal]` panics not-found.

---

### P2: `MustInjectAll[T]`

**User Story**: As a gonest user, I want `animals := gonest.MustInjectAll[Animal](controller)` to return every registered provider implementing `Animal` as a slice, for a plugin/strategy pattern where the exact count is intentionally open-ended.

**Acceptance Criteria**:

1. WHEN `MustInjectAll[T]` is called with `T` an interface kind THEN system SHALL return `[]T` containing every resolved provider whose concrete type satisfies `T`, in a stable (registration-order-based) sequence
2. WHEN ZERO providers match THEN system SHALL return an empty (non-nil vs nil is an implementation detail, but SHALL NOT panic)
3. WHEN `MustInjectAll[T]` is called with `T` a POINTER kind (not interface) THEN system SHALL panic with a clear message -- multi-binding only makes sense for interfaces

**Independent Test**: reproduce INSIGHT.md's `Animal`/`Cat`/`Dog` example verbatim -- `MustInjectAll[Animal]` returns exactly 2 entries, `Talk()` on each produces the expected sounds regardless of registration order edge cases.

---

### P3: `MustNewTestApp`/`TestBuilder`/`MustOverride[T]`

**User Story**: As a gonest user, I want `tester := gonest.MustNewTestApp(UserModule, func(b *gonest.TestBuilder) { gonest.MustOverride[IUserService](b, mock) })` (INSIGHT.md's own "exemplo de Testing") to run the SAME 3-phase bootstrap as `NewApp`, except that ANY provider whose resolved type EXACTLY matches (or, for an interface override, is satisfied by) an overridden type gets the override's value SUBSTITUTED in place of running its real Constructor.

**Acceptance Criteria**:

1. WHEN `MustOverride[T](b, mockValue)` is called (T either a concrete pointer or an interface) THEN system SHALL register a synthetic override entry keyed by `T`'s own `reflect.Type`
2. WHEN phase 1 (Provider resolution) runs AND a real Provider's `ResolvedType()` exactly matches an override's key THEN system SHALL use the override's `mockValue` DIRECTLY instead of invoking that Provider's real `Constructor` (the real constructor never runs -- no wasted work, no unwanted side effects from the real dependency)
3. WHEN a Controller's (or Middleware/Guard/Interceptor/Filter's) `MustInject[T]`/`MustInjectAll[T]` call resolves against an overridden interface THEN system SHALL return the override's `mockValue` wherever the real implementation would have been found via `Implements()` matching
4. WHEN `MustNewTestApp(module, nil)` is called (no overrides) THEN system SHALL behave identically to `NewApp` in every observable way except NOT starting an HTTP listener (no `adapter.Listen`) -- routes are still registered on the adapter (for `MustRequest`, a SEPARATE "HTTP Test Client" feature, to dispatch against later) but nothing binds a real network port
5. WHEN the returned tester value is used with `MustInject[T]` DIRECTLY (unit-test style, no HTTP involved, per INSIGHT.md's `TestUserService_Get_NotFound` example) THEN it SHALL resolve exactly like a Controller's own `MustInject` would (module-scoped, override-aware)

**Independent Test**: reproduce INSIGHT.md's `TestUserController_Get` (HTTP-level, override + dispatch) AND `TestUserService_Get_NotFound` (unit-level, no override, direct `MustInject`) verbatim.

---

## Edge Cases

- WHEN `MustInject[T]`/`MustInjectAll[T]` is called from WITHIN `Route.Handler` (per-request) rather than a builder closure THEN behavior is UNSPECIFIED by this feature (Out of Scope -- not defended against with a special panic message, though a future feature might add one; for now, whatever the direct-lookup mechanism naturally does when called at that time, which should still WORK correctly since providers are already resolved by then too, just wastefully re-executed per request -- not a correctness bug, a documented perf anti-pattern)
- WHEN an override provider's constructor declares its return type as EXACTLY the interface `T` (not a concrete type implementing it) THEN it SHALL be found via the EXACT-match path (not `Implements()`), same precedent as every other exact-match provider lookup in this codebase

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| TB-00 | P0: three-phase bootstrap, zero regression for existing pointer MustInject | Tasks | Pending |
| TB-00a | P0: AD-008 reversed -- Middleware/Guard/Interceptor/Filter defer builder to phase 3 | Tasks | Pending |
| TB-00b | P0: ownership = union of referencing modules, discovered post-phase-2 | Tasks | Pending |
| TB-01 | P1: MustInject[T] interface support, exact + Implements() fallback, scoped correctly per caller type | Tasks | Pending |
| TB-02 | P1: MustInject[T] panics on 0 or 2+ interface matches | Tasks | Pending |
| TB-03 | P2: MustInjectAll[T], interface-only, returns all matches, empty slice not panic on zero, scoped correctly | Tasks | Pending |
| TB-04 | P3: MustOverride[T] substitutes value, real Constructor never runs for overridden provider | Tasks | Pending |
| TB-05 | P3: MustNewTestApp behaves like NewApp minus Listen, override-aware resolution throughout | Tasks | Pending |
| TB-06 | P3: MustNewTestApp result usable both via HTTP dispatch and direct MustInject (unit-style) | Tasks | Pending |

**ID format:** `TB-[NUMBER]`

**Coverage:** 9 total, 0 mapped yet.

---

## Success Criteria

- [ ] Full existing test suite (`go test ./... -race`) green with ZERO assertion changes to any pre-existing test file
- [ ] INSIGHT.md's `Animal`/`Cat`/`Dog` `MustInjectAll` example works end-to-end
- [ ] INSIGHT.md's "exemplo de Testing" (`TestUserController_Get`/`TestUserService_Get_NotFound`) works end-to-end verbatim
- [ ] Provider-to-Provider dependency resolution (phase 1) behaves identically to today, including cycle detection
- [ ] A `*Guard` (or Middleware/Interceptor/Filter) referenced by Controllers in 2 different modules runs its OWN builder closure exactly once, with correct union-scoped visibility into both modules' providers
- [ ] Provider-to-Provider dependency resolution (phase 1) behaves identically to today, including cycle detection
