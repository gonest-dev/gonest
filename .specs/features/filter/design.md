# Filter Design

**Spec**: `.specs/features/filter/spec.md`

## Architecture Overview

```
internal/filter (new package, AD-004 pattern)
        │
        └── Filter  -- New(fn) runs fn IMMEDIATELY (AD-008: no MustInject)
                       Catch(exemplar any, handler any) -- reflect-validated,
                       exact-type match (reflect.TypeOf(exemplar))

internal/controller (existing) ──imports──> internal/filter
        Controller.Filters(items ...*filter.Filter)   -- was stub, real now
        Controller.OwnFilters() []*filter.Filter        -- new accessor

internal/module (existing) ──imports──> internal/filter
        Module.Filters(items ...*filter.Filter)         -- NEW method, mirrors Module.Use
        Module.OwnFilters() []*filter.Filter             -- new accessor

internal/app's Stage 2.5 (existing, registerRoutes -- extended a FOURTH time,
after Middleware/Guard/Interceptor) ──extended──
        filteredHandler becomes the OUTERMOST layer of the whole per-route
        chain (wraps withRoute -> composedHandler -> gatedHandler ->
        interceptedHandler -> routeHandler, i.e. EVERYTHING):

        filteredHandler := func(ctx) {
            defer func() {
                r := recover()
                if r == nil { return }
                exc, isException := r.(exception.Exception)
                if isException {
                    excType := reflect.TypeOf(exc)
                    if h, ok := controllerFilters.find(excType); ok { h.call(ctx, exc); return }
                    if h, ok := globalFilters.find(excType); ok { h.call(ctx, exc); return }
                }
                panic(r) // re-panic: no Filter caught it, let the adapter's
                          // EXISTING recover wrapper handle it exactly as
                          // it does today (Exception -> default
                          // {name,message,details}, non-Exception -> 500)
            }()
            withRoute(ctx) // wraps everything else, unchanged from Pipe fix
        }

        adapter.RegisterRoute(method, fullPath, filteredHandler)
```

The adapter's OWN recover wrapper (`internal/adapter/fiber`, from T7/"Panic Recovery & Default Handler") is completely UNCHANGED -- it's still there, still catches whatever `filteredHandler` re-panics with, still applies the exact same default-formatting logic it always has. Filter's recover is a NEW, MORE INNER layer that sits between the adapter's recover and everything else, catching first and selectively, re-throwing what it doesn't handle.

---

## Components

### `Filter` (new package)

- **Purpose**: registers per-exception-type custom response handlers, reusable across controllers.
- **Location**: `internal/filter/filter.go` (new file, new package)
- **Interfaces**:
  - `type Filter struct { catches map[reflect.Type]reflect.Value }` (unexported field)
  - `func New(fn func(*Filter)) *Filter` -- runs `fn` IMMEDIATELY (AD-008, same as `middleware.New`/`guard.New`/`interceptor.New`)
  - `func (f *Filter) Catch(exemplar any, handler any)` -- `exemplar` is a value whose CONCRETE TYPE identifies what to catch (e.g. `&FooExampleError{}`, mirroring `Pipe.Handler`'s reflect-validation style, not `Middleware`/`Guard`/`Interceptor`'s typed-func-parameter style, since the exact exception type varies per call). `handler` must be `func(ctx *execution.Context, exc T)` where `T` is EXACTLY `reflect.TypeOf(exemplar)` -- panics at registration time (reflect-validated, same "clear message, fail fast" convention as `Pipe.Handler`) if the signature doesn't match.
  - `func (f *Filter) HandlerFor(excType reflect.Type) (reflect.Value, bool)` -- exact map lookup, used by Stage 2.5's `filteredHandler`
- **Dependencies**: `reflect`, `internal/execution` (for the `*execution.Context` type used in signature validation)
- **Reuses**: `Pipe.Handler`'s reflect-validation pattern (`internal/pipe/pipe.go`) -- closest existing precedent for "validate a caller-supplied func's signature via reflect, panic clearly if wrong"

### `Controller.Filters` (extended, was the LAST remaining stub since T6)

- **Purpose**: give the existing no-op `Filters(items ...Middleware)` stub real storage.
- **Location**: `internal/controller/controller.go` (existing, extended)
- **Interfaces**:
  - `func (c *Controller) Filters(items ...*filter.Filter)` -- signature changes from `Filters(items ...Middleware)`
  - `func (c *Controller) OwnFilters() []*filter.Filter` -- defensive copy, mirrors `OwnGuards`/`OwnInterceptors`/`OwnMiddleware`
- **Note**: after this task, the placeholder `Middleware struct{}` type declared in `controller.go` since T6 has ZERO remaining consumers (`Use`→real, `Guards`→real, `Interceptors`→real, `Filters`→real) -- it becomes dead code, safe to delete as part of this feature (see Tech Decisions)
- **Dependencies**: adds `internal/filter`
- **Reuses**: the exact `Guards`/`OwnGuards` pattern this file already grew 3 times (Middleware, Guard, Interceptor)

### `Module.Filters` (new -- did not exist before this feature)

- **Purpose**: root-module-only global filter registration, mirrors `Module.Use`'s exact scoping precedent from "Middleware".
- **Location**: `internal/module/module.go` (existing, extended)
- **Interfaces**:
  - `func (m *Module) Filters(items ...*filter.Filter)` -- new method, any `*Module` can call it (same "compiles everywhere, behaviorally consulted only for root" caveat as `Module.Use`)
  - `func (m *Module) OwnFilters() []*filter.Filter` -- defensive copy
- **Dependencies**: adds `internal/filter`
- **Reuses**: the exact `Use`/`OwnMiddleware` pattern this file already grew in "Middleware"

### `internal/app`'s Stage 2.5 (`registerRoutes`, extended a fourth time)

- **Purpose**: wrap the ENTIRE existing per-route dispatch chain (which already handles Middleware→Guard→Interceptor→Handler composition, plus the `withRoute` wiring from the Pipe fix) in a new, selective recover layer that intercepts a panicked `exception.Exception` whose concrete type matches a registered `Catch` -- controller-level filters checked first (spec.md's precedence rule, FLT-07), then global (root module) filters, falling through (re-panic) to the UNCHANGED adapter-level recover if nothing catches it.
- **Location**: `internal/app/app.go` (existing, extended)
- **Interfaces**: no new exported surface -- `routableController` gains one more method requirement, `OwnFilters() []*filter.Filter`, already implemented by `*controller.Controller` post-this-feature
- **Dependencies**: adds `internal/filter`, `reflect` (already indirectly available via other stdlib usage in this file)
- **Reuses**: `exception.Exception` (already imported for `gatedHandler`'s `NewForbiddenException`), the EXISTING `withRoute`/`composedHandler`/`gatedHandler`/`interceptedHandler` chain -- none of those four layers change AT ALL, `filteredHandler` is purely an additional wrapper around the outside of all of them

---

## Data Models

```go
// internal/filter
type Filter struct {
    catches map[reflect.Type]reflect.Value
}
```

**Dispatch algorithm** (conceptually, inside Stage 2.5's `filteredHandler`, per route):

```go
func filteredHandler(controllerFilters, globalFilters []*filter.Filter, next func(ctx *execution.Context)) func(ctx *execution.Context) {
    return func(ctx *execution.Context) {
        defer func() {
            r := recover()
            if r == nil {
                return
            }
            if exc, ok := r.(exception.Exception); ok {
                excType := reflect.TypeOf(exc)
                if h, found := findCatch(controllerFilters, excType); found {
                    h.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(exc)})
                    return
                }
                if h, found := findCatch(globalFilters, excType); found {
                    h.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(exc)})
                    return
                }
            }
            panic(r) // not caught by any Filter -- re-panic, adapter's own
                      // recover (unchanged) applies the existing default
        }()
        next(ctx)
    }
}

func findCatch(filters []*filter.Filter, excType reflect.Type) (reflect.Value, bool) {
    for _, f := range filters {
        if h, ok := f.HandlerFor(excType); ok {
            return h, true
        }
    }
    return reflect.Value{}, false
}
```

**Relationships**: `filteredHandler` wraps `withRoute` (which wraps everything else, unchanged) as the new OUTERMOST layer of the whole per-route chain built by `registerRoutes`. It is the FIRST thing that runs and the LAST thing that gets a chance to handle a panic before the adapter's own recover does.

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| Panicked `exception.Exception`'s concrete type matches a `Catch` on a controller-level Filter | That Filter's handler runs, produces whatever response it wants via `ctx` | Matches spec.md AC1/AC4 |
| Panicked `exception.Exception`'s concrete type matches a `Catch` on a GLOBAL (root module) Filter, but NOT any controller-level Filter | The global Filter's handler runs | Matches spec.md AC on global-applies-without-controller-opt-in |
| Panicked `exception.Exception`'s concrete type matches BOTH a controller-level AND a global Filter's `Catch` | Controller-level wins (checked first, short-circuits) | Matches spec.md AC "controller-level Filter overrides global" |
| Panicked `exception.Exception`'s concrete type matches NO registered `Catch` (controller or global) | Re-panic -- falls through to the adapter's EXISTING recover, which applies the default `{name,message,details}` formatting exactly as it does today | Zero regression -- spec.md AC2 |
| Panic is NOT an `exception.Exception` at all (bare error, nil-deref, `panic(nil)`, etc.) | `r.(exception.Exception)` assertion fails, `isException` false, falls straight to re-panic without even checking any Filter's `Catch` map | Matches spec.md's Out of Scope ("Filters only ever catch structured exception.Exception values") -- same generic-500 path as always |
| A Filter's own custom handler panics (a bug in the dev's Filter code) | NOT caught by this same `defer`/`recover` (it already returned by the time a second panic could happen, or -- if it panics DURING the handler call itself -- Go's own panic-during-defer semantics apply: the NEW panic replaces/chains with the old one and continues unwinding) -- ultimately still caught by the adapter's outer recover, generic 500 | Matches spec.md's Edge Cases -- no special defense needed |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Filter's own recover lives in `internal/app`'s Stage 2.5 composition (`filteredHandler`), NOT inside `internal/adapter/fiber`'s existing recover wrapper | New, separate, MORE INNER `defer`/`recover` layer, wrapping the whole composed chain, that selectively re-panics if nothing catches | The adapter's recover wrapper doesn't know anything about Controllers/Modules/Filters -- it only ever sees `(method, path, handler)`. Stage 2.5 is where all the per-route/per-controller context (guards, middleware, interceptors, and now filters) already lives and gets composed -- adding Filter's selective catch-or-rethrow logic there keeps that same locality, and keeps the adapter itself (only 2 packages allowed to touch Fiber, per T7's original boundary) completely unaware of a Filter concept it doesn't need to know about |
| `Catch(exemplar any, handler any)` uses reflect (like `Pipe.Handler`), not a typed generic method | `exemplar` identifies the type to match, `handler`'s signature is validated via reflect at registration time | Go generics can't express "a method whose accepted handler signature depends on a runtime value" cleanly -- `Filter.Catch[T any](handler func(ctx, T))` would need the caller to explicitly instantiate the type parameter (`f.Catch[*FooExampleError](...)`), which doesn't match INSIGHT.md's own call shape (`filter.Catch(&FooExampleError{}, func(ctx, exc *FooExampleError) {...})`, exemplar value first, type inferred from it) -- reflect-based validation, exactly like `Pipe.Handler` already does for a similar "accept any function with a specific-shaped signature" problem, is the established precedent to follow |
| Controller-level Filters checked BEFORE global (root module) Filters | Explicit precedence order in `filteredHandler`'s dispatch | Matches Nest's own filter-precedence convention (more specific scope overrides broader scope) and spec.md's explicit AC (FLT-07) -- this is a deliberate design choice being made now, not left ambiguous, since INSIGHT.md's examples don't show a conflicting-Filter scenario to derive it from directly |
| The placeholder `Middleware struct{}` type in `controller.go` (declared T6, unused as of this feature) gets DELETED | Dead code removal, not kept "just in case" | Per this codebase's convention (avoid backwards-compat shims for genuinely-dead code) -- once `Filters` graduates to the real `*filter.Filter` type, nothing references the placeholder anymore; keeping it around would be confusing residue, not a safety net |

---

## Open Questions pra Tasks

- None -- the one genuine design decision (WHERE Filter's recover lives, and controller-vs-global precedence) is resolved above with clear rationale; everything else mechanically follows the `Guards`/`OwnGuards`, `Use`/`OwnMiddleware` precedent already established three times over in this codebase.
