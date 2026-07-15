# Test App Bootstrap Design

**Spec**: `.specs/features/test-app-bootstrap/spec.md`
**Context**: `.specs/features/test-app-bootstrap/context.md`

**⚠️ This design was produced in a session that stopped BEFORE implementation, per explicit user instruction ("só especificar agora, execução depois"). The next session executing this MUST re-verify every concrete signature/field name against the actual current code before writing any implementation -- this document describes the INTENDED shape, but the codebase may have shifted since this was written, and some mechanics below (marked explicitly) are the orchestrator's own best design judgment, not user-confirmed line-by-line.**

## Architecture Overview

```
internal/inject (existing package, EXTENDED)
        │
        ├── MustInject[T any](owner module.Owner) T -- SIGNATURE UNCHANGED,
        │      but internal behavior branches on a NEW owner capability:
        │      if owner satisfies a new `directResolver` interface (see
        │      below), MustInject calls owner.ResolveDirect(t) instead of
        │      the OLD placeholder+PendingEdge path. Provider does NOT
        │      satisfy directResolver (keeps the OLD placeholder path,
        │      unchanged -- P0's AC4). Controller/Middleware/Guard/
        │      Interceptor/Filter DO satisfy it (once phase 1 is done for
        │      Controller, once phase 2 is done for the other 4).
        │
        ├── MustInjectAll[T any](owner module.Owner) []T -- NEW. T MUST be
        │      Kind()==Interface (panics otherwise). owner MUST satisfy
        │      directResolver (panics with a clear message otherwise --
        │      Provider never supports MustInjectAll, matching spec.md's
        │      Out of Scope: provider-to-provider deps stay single-value).
        │      Calls owner.ResolveDirectAll(t) []reflect.Value, converts
        │      each to T.
        │
        └── directResolver interface (NEW, unexported, satisfied by
               *controller.Controller post-phase-1 and by
               *middleware.Middleware/*guard.Guard/*interceptor.Interceptor/
               *filter.Filter post-phase-2):
                 ResolveDirect(t reflect.Type) (reflect.Value, bool)
                 ResolveDirectAll(t reflect.Type) []reflect.Value
               Both implemented by delegating to a SHARED helper (new file,
               likely internal/resolver/direct.go) that takes an explicit
               []*module.Module scope + t, and does exact-match-or-
               Implements() search against each scoped module's own
               resolved providers (see "Direct resolution scope" below)

internal/provider (existing package, EXTENDED -- needed by direct resolution)
        │
        + Provider gains resolvedValue reflect.Value (unexported) +
              SetResolvedValue(v reflect.Value) [called ONCE by Stage 3,
              right after callConstructor succeeds, in ADDITION to the
              existing placeholder-copy behavior -- purely additive] +
              ResolvedValue() (reflect.Value, bool) [getter, used by the
              direct-resolution helper above]

internal/resolver (existing package, EXTENDED)
        │
        ├── Stage 3 (stage3.go) UNCHANGED internally -- still resolves the
        │      FULL provider graph via placeholder+topological+errgroup,
        │      same as today; the ONLY addition is the SetResolvedValue
        │      call described above, right after each provider's
        │      Constructor succeeds
        │
        └── direct.go (NEW) -- the scoped exact-match-or-Implements()
               search: `func findDirect(scope []*module.Module, t reflect.Type) (reflect.Value, bool)`
               and `func findDirectAll(scope []*module.Module, t reflect.Type) []reflect.Value`
               -- walks each module in scope, both its OWN providers
               (module.OwnProviders()) and its EXPORTED-from-imports
               providers (existing findOwn/findExported logic in
               resolver.go is the reference implementation to REUSE/adapt,
               not reinvent) -- for pointer t: exact ResolvedType() match.
               For interface t: exact match first (an override registered
               AS the interface type itself), else Implements() match
               across every provider in scope, collecting ALL matches (the
               caller -- findDirect vs findDirectAll -- decides whether
               "more than 1" is an error or the whole point)

internal/middleware, internal/guard, internal/interceptor, internal/filter
(existing packages, EACH gets the SAME mechanical change -- AD-008
reversal)
        │
        ├── New(fn func(*T)) *T -- CHANGES from "run fn immediately,
        │      return already-built T" to "store fn, return a T whose fn
        │      has NOT run yet" (same shape as provider.New/controller.New
        │      TODAY -- these 4 packages are being brought in line with
        │      that existing pattern, not inventing a new one)
        │
        ├── Declare(scope []*module.Module) -- NEW (replaces the
        │      immediate-execution that used to happen inside New).
        │      Idempotent (same "no-op after first call" contract
        │      Pipe.Declare already established, per L-012 in STATE.md --
        │      a shared Middleware/Guard/etc value attached to MULTIPLE
        │      controllers must only run its OWN fn once, not once per
        │      attachment). Stores scope internally (so ResolveDirect/
        │      ResolveDirectAll, called from WITHIN fn while it runs, know
        │      what to search), THEN runs fn(self).
        │
        └── ResolveDirect/ResolveDirectAll (satisfies internal/inject's
               directResolver interface) -- delegate to
               internal/resolver's findDirect/findDirectAll using the
               scope stored by Declare

internal/controller (existing package, EXTENDED)
        │
        ├── Guards(gs ...*guard.Guard)/Use(ms ...*middleware.Middleware)/
        │      Interceptors(is ...*interceptor.Interceptor)/
        │      Filters(fs ...*filter.Filter) -- UNCHANGED signatures,
        │      still just append to OwnGuards()/etc -- the SIDE EFFECT of
        │      "this reference now needs its owning module recorded for
        │      phase 3" happens OUTSIDE Controller entirely, in the NEW
        │      phase-2-to-phase-3 transition step in internal/app (see
        │      below) -- Controller itself does not need to know about
        │      ownership discovery, it just keeps recording references
        │      the same way it always has
        │
        └── ResolveDirect/ResolveDirectAll (satisfies directResolver) --
               Controller's OWN scope is a SINGLE-element []*module.Module{c.OwnerModule()}
               (module-scoped, same encapsulation Controllers already have
               today via findOwn/findExported)

internal/app/app.go (existing, RESTRUCTURED -- the orchestration glue)
        │
        NewApp's sequence becomes:
        1. inject.Reset() (unchanged)
        2. modules, err := root.Assemble() (unchanged -- Stage 1)
        3. declareProviders(modules) -- NEW, replaces declareAll's
              provider-half: only runs OwnProviders()'s Declare()
        4. resolver.Resolve(ctx, modules) -- UNCHANGED (Stage 3), but now
              ALSO calls SetResolvedValue per provider (internal/provider's
              own new addition, invoked from within resolver's existing
              callConstructor success path)
        5. declareControllers(modules) -- NEW, replaces declareAll's
              controller-half: runs OwnControllers()'s Declare() (which,
              per Controller's own ResolveDirect scope, can now resolve
              MustInject/MustInjectAll DIRECTLY -- no placeholder)
        6. discoverPipelineStageOwnership(modules) -- NEW: walks EVERY
              module's OwnControllers() (now fully declared, step 5 done)
              + their OwnGuards()/OwnMiddleware()/OwnInterceptors()/
              OwnFilters(), PLUS every module's OWN Use()/Filters() (root-
              only global registration, existing Module-level methods) --
              builds map[pointer-identity]→[]*module.Module (the union-
              scope), deduplicated by pointer identity of the Middleware/
              Guard/Interceptor/Filter value itself
        7. declarePipelineStageTypes(discovered ownership map) -- NEW:
              for each DISTINCT value found in step 6, calls its own
              Declare(unionScope) exactly once
        8. adapter := newAdapter[T,PT]() (unchanged)
        9. registerRoutes(adapter, root, modules) (unchanged -- by this
              point every Route/Controller/Guard/Middleware/Interceptor/
              Filter is FULLY declared, so route composition works exactly
              as it does today, just later in the overall sequence)

internal/app (test-mode variant, NEW -- possibly internal/app itself, or a
        new internal/testapp package, TBD by the executing session)
        │
        Reuses steps 1-7 above, IDENTICALLY, with ONE injection point:
        an override registry (map[reflect.Type]reflect.Value, built from
        MustOverride[T] calls) consulted by resolver.Resolve (step 4)
        BEFORE calling a matching provider's real Constructor -- if an
        override exists for that provider's ResolvedType(), use the
        override's value via SetResolvedValue directly, skip
        callConstructor entirely for that one provider. Step 8-9 (adapter/
        registerRoutes) still run (so MustRequest, a future "HTTP Test
        Client" feature, has routes to dispatch against) but NO Listen call
        happens -- MustNewTestApp returns before binding any real port.
```

This is the largest, riskiest architectural change of this entire session -- it touches Milestone 1's foundational bootstrap AND reverses AD-008 (a decision made earlier THIS session, for Milestone 3's 4 pipeline-stage types). Every existing test in this codebase that exercises Middleware/Guard/Interceptor/Filter, Controller, or Provider needs to be re-verified for zero-assertion-change regression -- this is not a small refactor.

---

## Direct resolution scope (the "union of referencing modules" mechanic, spelled out precisely)

Given a `*guard.Guard` value `g` referenced by `controller.Guards(g)` inside TWO different Controllers, `cA` (owned by module `MA`) and `cB` (owned by module `MB`):

1. During phase 2 (`declareControllers`), both `cA` and `cB` run their OWN builder closures, each calling `controller.Guards(g)` -- `g`'s reference now appears in BOTH `cA.OwnGuards()` and `cB.OwnGuards()`.
2. Step 6 (`discoverPipelineStageOwnership`) walks every module's every controller's `OwnGuards()`, and for `g` specifically, finds it referenced from `cA` (module `MA`) and `cB` (module `MB`) -- records `ownership[g] = []*module.Module{MA, MB}` (deduplicated if the SAME module references it multiple times, e.g. two different controllers in `MA` both using `g`).
3. Step 7 calls `g.Declare([]*module.Module{MA, MB})` EXACTLY ONCE (pointer-identity dedup across the whole discovery map, regardless of how many controllers/modules referenced it).
4. Inside `g`'s now-running `fn` (the original builder closure body), any `MustInject[T](g)` call resolves against BOTH `MA` and `MB`'s own+exported providers, unioned -- a provider that exists in `MA` but not `MB` is still found; if BOTH modules happen to have a matching provider for an interface `T`, that's 2 matches → `MustInject[T]` panics ambiguous (legitimate, not a bug -- spec.md's own "trust the caller" stance).

Global (root-module-only) `Module.Use(...)`/`Module.Filters(...)` registrations get an ownership set of exactly `[]*module.Module{root}` -- they were never attachable to multiple modules in the first place (existing constraint, unchanged), so there's no discovery needed there beyond "this middleware/filter belongs to root."

---

## Components

*(Signatures below are the orchestrator's best design judgment -- NOT verified against a fresh read of the current code at write-time for every single line, given this session stopped before implementation. The executing session MUST read the actual current `internal/middleware`, `internal/guard`, `internal/interceptor`, `internal/filter`, `internal/provider`, `internal/controller`, `internal/resolver`, `internal/app` source files in full before writing any code, and adjust field/method names to match reality where this document's guesses turn out wrong.)*

### `internal/provider.Provider` (extended)

- `resolvedValue reflect.Value` (new field), `func (p *Provider) SetResolvedValue(v reflect.Value)` (new, called once by Stage 3), `func (p *Provider) ResolvedValue() (reflect.Value, bool)` (new getter)

### `internal/resolver` (extended)

- `func findDirect(scope []*module.Module, t reflect.Type) (reflect.Value, bool)` -- for pointer `t`: exact `ResolvedType()` match within scope (reuse/adapt `findOwn`/`findExported`'s existing module-scoping logic, but iterate ALL modules in `scope`, not just one owner's single module + its imports). For interface `t`: exact match first, else iterate every provider in scope checking `provider.ResolvedType().Implements(t)`, collect ALL matches.
- `func findDirectAll(scope []*module.Module, t reflect.Type) []reflect.Value` -- `t` must be interface (caller's responsibility to check, per spec.md TB-03's panic requirement -- exact panic-raising happens in `internal/inject`, not here); returns every match from the interface branch above, no ambiguity error (that's the whole difference from `findDirect`).

### `internal/inject` (extended)

- `directResolver` interface (unexported): `ResolveDirect(t reflect.Type) (reflect.Value, bool)`, `ResolveDirectAll(t reflect.Type) []reflect.Value`
- `MustInject[T any](owner module.Owner) T` -- type-switches/interface-asserts `owner` against `directResolver`; if it satisfies it, call `ResolveDirect`, panic clear message on `!ok`, else convert+return. If `owner` does NOT satisfy `directResolver` (i.e. it's a `*provider.Provider`), fall through to the EXISTING placeholder+PendingEdge logic, UNCHANGED, including the existing "T must be Pointer" check (Providers never get interface injection, per spec.md's Out of Scope)
- `MustInjectAll[T any](owner module.Owner) []T` -- NEW. Panics if `T`'s `Kind() != reflect.Interface`. Panics if `owner` does not satisfy `directResolver` (Provider unsupported). Else calls `ResolveDirectAll`, converts each match to `T`.

### `internal/middleware`/`internal/guard`/`internal/interceptor`/`internal/filter` (each, mechanically identical change)

- `New(fn func(*T)) *T` -- stores `fn`, does NOT call it, returns `T` with `fn` pending (mirrors `provider.New`/`controller.New`'s existing shape)
- `Declare(scope []*module.Module)` -- idempotent (a `declared bool` guard, same as `Pipe.Declare`'s existing precedent per L-012), stores `scope` on the receiver, then calls `fn(self)` if not already declared
- `ResolveDirect(t reflect.Type) (reflect.Value, bool)` / `ResolveDirectAll(t reflect.Type) []reflect.Value` -- delegate to `resolver.findDirect(self.scope, t)` / `resolver.findDirectAll(self.scope, t)`

### `internal/controller.Controller` (extended)

- `ResolveDirect`/`ResolveDirectAll` -- delegate to `resolver.findDirect([]*module.Module{c.OwnerModule()}, t)` / `findDirectAll(...)` (single-module scope, unlike the 4 pipeline-stage types' union scope)
- `Declare()` signature STAYS zero-arg (unlike the 4 pipeline-stage types) -- a Controller's OWN module is already known via `OwnerModule()` (set during Stage 1 `Assemble`), no external scope needs to be PASSED IN the way Guard/etc's union scope does (which can only be computed AFTER Controllers finish declaring, hence the different timing)

### `internal/app` (restructured)

- `declareProviders(modules []*module.Module)` -- NEW, extracted from today's `declareAll` (the `OwnProviders()` half only)
- `declareControllers(modules []*module.Module)` -- NEW, extracted from today's `declareAll` (the `OwnControllers()` half only), runs AFTER `resolver.Resolve`
- `discoverPipelineStageOwnership(modules []*module.Module) map[any][]*module.Module` -- NEW (the map key is the pointer-identity of a `*middleware.Middleware`/`*guard.Guard`/`*interceptor.Interceptor`/`*filter.Filter` value, stored as `any` since these are 4 different concrete types sharing no common exported interface today -- the executing session should evaluate whether introducing a small shared unexported interface, e.g. `pipelineStageType interface { Declare([]*module.Module) }`, makes this cleaner than a raw `any` key; likely yes, but left as an implementation-time call)
- `declarePipelineStageTypes(ownership map[any][]*module.Module)` -- NEW, calls `.Declare(scope)` once per distinct key

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| `MustInject[T]` (T interface) finds 0 matches in scope | Panic, clear "no provider implements interface X (searched N module(s))" message | spec.md TB-02 |
| `MustInject[T]` (T interface) finds 2+ matches in scope | Panic, clear "ambiguous: N providers implement interface X, use MustInjectAll" message, listing the matched concrete types if feasible | spec.md TB-02 |
| `MustInjectAll[T]` called with T a pointer kind | Panic, clear message ("MustInjectAll requires an interface type") | spec.md P2 AC3 |
| `MustInject[T]`/`MustInjectAll[T]` called from a `*provider.Provider`'s own builder (T interface) | Panic (Provider never satisfies `directResolver`, AND Provider-side `MustInject` keeps its existing pointer-only check) -- same "providers stay pointer-only, unchanged" stance as spec.md's Out of Scope | Zero new capability for Provider-to-Provider deps, intentionally |
| A `*Guard` (etc) referenced by controllers whose owning modules' scoped provider sets are MUTUALLY INCOMPATIBLE for some OTHER (non-injected) reason | Not specially handled -- `findDirect`'s union search doesn't care about module "compatibility" beyond what each module's own+exported provider set naturally allows | Not a scenario spec.md's stories anticipate needing special handling |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `MustInject`'s public signature (`MustInject[T any](owner module.Owner) T`) stays EXACTLY the same -- the phase/mode dispatch happens INSIDE, via an interface-satisfaction check on `owner` | No new exported entry point, no `MustInjectDirect`-style parallel API | Every existing call site in this codebase (`gonest.MustInject[*AuthService](guard)` etc) must keep compiling UNCHANGED -- introducing a differently-named function for the "new" resolution mode would force every existing example/test to be rewritten for no behavioral reason. Dispatching on `owner`'s own capability (does it satisfy `directResolver`?) is transparent to every caller. |
| Provider gains `SetResolvedValue`/`ResolvedValue` as a NEW, ADDITIVE capability, rather than repurposing the EXISTING placeholder mechanism to also serve direct lookups | Two separate storage mechanisms coexist on `Provider` (placeholder-copy for Stage-3-era consumers, `resolvedValue` for post-Stage-3 direct consumers) | The placeholder mechanism's whole point is "give the CALLER a stable address to hold onto before the value exists" -- `findDirect`/`findDirectAll` (phase 2/3 callers) don't need a placeholder AT ALL, they call AFTER the value already exists and just want to READ it once. Repurposing placeholders for this would be backwards (allocating throwaway indirection for something that doesn't need to defer anything) -- a plain stored value + getter is simpler and matches the actual need. |
| Ownership discovery (`discoverPipelineStageOwnership`) is a SEPARATE, DEDICATED step in `internal/app`, not folded into `declareControllers` itself | Two clearly separated steps | `declareControllers` is naturally the thing that finds out WHICH Guards/etc got referenced (as a SIDE EFFECT of controllers declaring), but the interesting parte -- deduplicating by pointer identity across the WHOLE tree and computing per-value union scopes -- only makes sense to do ONCE all controllers have finished (can't correctly union a Guard's ownership set while some OTHER controller that also references it hasn't run yet). Keeping this as an explicit, separately-testable function (rather than interleaved state accumulated mid-loop) makes the "when is this complete" invariant obvious and independently unit-testable. |
| `internal/middleware`/`guard`/`interceptor`/`filter`'s new `Declare(scope []*module.Module)` takes scope as a PARAMETER (not looked up some other way, e.g. from a global registry keyed by the value's own identity) | Explicit parameter | Matches `Pipe.Declare()`'s own existing idempotent-call precedent (L-012, STATE.md) while adding the ONE piece of new information (scope) these 4 types specifically need that Pipe never did -- keeps the dependency (what scope to search) visible at the call site in `internal/app`'s `declarePipelineStageTypes`, rather than hidden behind another lookup a reader would need to trace separately. |

---

## Open Questions for the executing session (NOT settled here -- genuinely deferred)

- **Exact shared-interface shape for the ownership discovery map's key type** (`any` vs. a small new unexported interface) -- flagged above, a real implementation-time call, not a design gap that blocks starting.
- **Where `MustNewTestApp`/`TestBuilder`/`MustOverride[T]` physically live** (new `internal/testapp` package vs. extending `internal/app` itself) -- TBD, likely follows AD-004's "1 package per concept" precedent (`internal/testapp`), but not settled here.
- **Exact override-registry consultation point inside `resolver.Resolve`** (Stage 3) -- design.md sketches "consulted before calling a matching provider's real Constructor," but the PRECISE code location (which function, `invokeAndCopy`/`callConstructor`/elsewhere) needs the executing session's own fresh read of `stage3.go`'s CURRENT state (which may have shifted if T0-T-whatever of THIS feature itself changes `stage3.go` first).
- **Whether `MustJsonBody`/`MustParams`/`MustQuery`/`Custom(fn)` (Milestone 6) or `internal/openapi` (Milestone 7) have ANY latent dependency on `Middleware`/`Guard`/`Interceptor`/`Filter`'s OLD immediate-execution timing** -- not identified during this session's design work, but the executing session's T0 (bootstrap reorder) MUST run the ENTIRE test suite (not just Milestone 1/3's own tests) as its zero-regression bar, precisely because a timing change this fundamental could ripple into places this design session didn't think to check.
