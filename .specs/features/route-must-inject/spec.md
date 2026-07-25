# Spec: Route-level MustInject support

## Problem

`MustInject[T](owner)` dispatches to direct resolution only when `owner` satisfies
`internal/inject`'s unexported `directResolver` interface (`ResolveDirect`/`ResolveDirectAll`) --
today only `*controller.Controller`, `*middleware.Middleware`, `*guard.Guard`,
`*interceptor.Interceptor`, `*filter.Filter` do. `*route.Route` (the `r` param of
`Controller.RouteGet`/`RoutePost`/etc.'s callback) has no owner reference and satisfies neither
`directResolver` nor `module.Owner` -- calling `MustInject[T](r)` inside a route callback panics:

```
gonest: MustInject[T] requires owner to be a *provider.Provider when T is a pointer type and
owner is not a Controller/Middleware/Guard/Interceptor/Filter
```

Confirmed live via a real consumer (`C:\dev\leandroluk\erc\ctrl\api\app\system\controller.go`)
resolving a per-route usecase inside `c.RouteGet("/health", func(r *gonest.Route) { ... })`.

## Requirements

- **R1**: `MustInject[T](r)` MUST work when called from inside a Route's callback (the `fn` passed
  to `Controller.Route`/`RouteGet`/`RoutePost`/etc.), resolving `T` from the SAME scope as
  `MustInject[T](c)` would for the owning Controller (single-module scope, no new resolution
  rules).
- **R2**: `MustInjectAll[T](r)` (interface `T`) MUST work the same way, mirroring
  `MustInjectAll[T](c)`.
- **R3**: No breaking change to `Controller`'s existing public API (`RouteGet`, `RoutePost`, ...)
  or to `MustInject[T](c)`'s existing behavior.
- **R4**: No import cycle between `internal/route` and `internal/controller` (`controller` already
  imports `route`).

## Non-goals

- A per-route resolution scope different from the owning Controller's (e.g. considering
  per-route guards/middleware for resolution) -- confirmed out of scope via `AskUserQuestion`
  during brainstorming; no real use case today.
- Any change to `MustInject`'s dispatch rules for `*provider.Provider` or `*module.LazyModule`
  owners.

## Approach (approved during brainstorming)

`route.Route` gains an `owner any` field, set at construction, plus a local (package-private)
structural interface mirroring `inject.directResolver`'s shape:

```go
type resolver interface {
    ResolveDirect(t reflect.Type) (reflect.Value, bool)
    ResolveDirectAll(t reflect.Type) []reflect.Value
}
```

`Route` implements `ResolveDirect`/`ResolveDirectAll` by type-asserting `owner` against this local
`resolver` interface and delegating; returns not-found/nil if `owner` is nil or doesn't satisfy it.
Since Go interfaces are structurally satisfied, `*controller.Controller` (which already implements
this exact method pair) satisfies it with zero changes to `internal/controller` beyond passing
itself in.

`route.New` gains `owner any` as its first parameter: `New(owner any, method HttpMethod, path
string, fn func(*Route)) *Route`. `Controller.Route` calls `route.New(c, method, path, fn)`. All
other existing callers (tests) pass `nil` -- same no-resolution behavior as today, just a
different panic message if `MustInject` is attempted on such a Route (interface-kind path panics
"no provider implements interface ..."; pointer-kind path panics "no provider registered for type
...", instead of the old "owner not Controller/Middleware/.../Provider" message).

## Acceptance Criteria

- AC1: A Route callback (`c.RouteGet("/x", func(r *gonest.Route) { usecase :=
  gonest.MustInject[*SomeUsecase](r) ... })`) resolves `usecase` from the owning Controller's
  module without panicking, when a matching provider is registered.
- AC2: `MustInject[T](r)` panics with "no provider registered for type ..." when no matching
  provider exists in scope -- same message shape as `MustInject[T](c)` today.
- AC3: `MustInjectAll[T](r)` (interface T) resolves all matching providers in scope, mirroring
  `MustInjectAll[T](c)`.
- AC4: Existing tests calling `route.New(method, path, fn)` compile after being updated to
  `route.New(nil, method, path, fn)`; behavior unchanged.
- AC5: `go build ./...`, `go vet ./...`, `go test ./... -race -count=1` all green.
- AC6: `.examples/` gets (or an existing example gets updated with) a route resolving a usecase
  via `MustInject[T](r)`, verified live (real HTTP dispatch), per project convention.
- AC7: README.md's relevant section reflects the new capability (Route can now MustInject),
  per project convention -- since this changes an existing public API's usable surface (Route
  gains a real capability it didn't have), not just an addition.

## Traceability

| Req | Status |
| --- | --- |
| R1 | Verified |
| R2 | Verified |
| R3 | Verified |
| R4 | Verified |
