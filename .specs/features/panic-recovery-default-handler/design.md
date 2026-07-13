# Panic Recovery & Default Handler Design

**Spec**: `.specs/features/panic-recovery-default-handler/spec.md`

## Architecture Overview

```
route Handler panics
        │
        ▼
internal/fiberapp.RegisterRoute's wrapper (T7, existing) — defer/recover()
        │
        ▼
   r := recover()
        │
        ├── exc, ok := r.(exception.Exception)?
        │       │
        │       ├── ok == true  → ctx.Status(exc.Status()).Json(map[string]any{
        │       │                     "name": exc.Name(), "message": exc.Message(), "details": exc.Details(),
        │       │                 })
        │       │
        │       └── ok == false → existing generic-500 path (T7, UNCHANGED):
        │                          c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
        ▼
    response sent, process survives either way
```

Single-file change: `internal/fiberapp/fiberapp.go`'s existing `RegisterRoute` wrapper (T7) gains one new import (`internal/exception`) and a type-assertion branch inside its already-existing `defer func() { if r := recover(); r != nil { ... } }()`. No new package, no new public type — this feature is pure behavior change to an existing internal recover path.

---

## Components

### `internal/fiberapp.RegisterRoute`'s recover branch (extended)

- **Purpose**: distinguish an intentional `Exception` panic from a genuine bug, respond accordingly.
- **Location**: `internal/fiberapp/fiberapp.go` (existing file, existing function — extend the `defer/recover()` block already there, don't restructure the rest of `RegisterRoute`)
- **Interfaces**: no new exported surface — this is entirely inside the existing unexported `wrapped` closure
- **Dependencies**: adds `github.com/gonest-dev/gonest/internal/exception` (new import for this file — does not create a cycle: `internal/exception` has zero dependencies of its own, doesn't import `internal/fiberapp` or anything HTTP-related)
- **Reuses**: the `*httpctx.Context` (`ctx`) already constructed earlier in the wrapper (before the Handler call) — the Exception-branch response is written via `ctx.Status(...).Json(...)`, the SAME methods any ordinary Handler already uses, not a raw `fiber.Ctx` call. This keeps exactly one HTTP-response code path (`httpctx.Context`), rather than the recover branch reaching around it to touch `fiber.Ctx` directly like the generic-500 path (T7) already does out of necessity (T7's generic path uses raw `c.Status/SendString` specifically BECAUSE it must be resilient even if something about `ctx`/`httpctx` itself is what's broken — see Tech Decisions below for why the Exception branch doesn't need that same defensiveness).

---

## Data Models

No new types. The response body is an ad-hoc `map[string]any{"name": ..., "message": ..., "details": ...}` passed straight to `ctx.Json(...)` — matches how every other route Handler in this codebase already produces a JSON body (T9's `UserController` example does the same thing with `*UserEntity` structs; a map is just as valid an argument to `Json(value any)`).

```go
// inside the recover branch, conceptually:
if exc, ok := r.(exception.Exception); ok {
    ctx.Status(exc.Status()).Json(map[string]any{
        "name":    exc.Name(),
        "message": exc.Message(),
        "details": exc.Details(),
    })
    return nil
}
// ...existing generic-500 fallback, unchanged
```

---

## Error Handling Strategy

| Scenario | Treatment | Impact |
| --- | --- | --- |
| Panic value satisfies `exception.Exception` | Its own `Status()`, JSON body `{name,message,details}` | Client gets a structured, actionable error — spec.md P1 |
| Panic value does NOT satisfy `exception.Exception` | UNCHANGED from T7: generic 500, `"Internal Server Error"` body, no leaked detail | Non-regression — spec.md P2 |
| `panic(nil)` (Go 1.21+ turns this into `*runtime.PanicNilError` on recover) | `recover()`'s value will not satisfy `exception.Exception` — falls to the generic-500 path automatically, no special-casing written | Same guarantee as any other non-Exception panic — spec.md Edge Cases |
| `ctx.Json(...)` itself fails while writing the Exception body (e.g. unmarshalable `Details()`) | Not this feature's concern — same pre-existing failure class as any ordinary Handler's own `ctx.Json` call (T2's `httpctx.Context.Json` just returns whatever `Responder.JSON` returns; the fiberapp wrapper today does not check that return value for the generic path either) | Out of scope per spec.md's Edge Cases — no new handling added |

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Exception branch uses `ctx.Status(...).Json(...)` (the `httpctx.Context` API), NOT raw `fiber.Ctx` calls | `httpctx.Context`-mediated | Unlike T7's generic-500 fallback (which intentionally reaches around `ctx`/`httpctx` via raw `fiber.Ctx` calls, precisely because that path exists to be resilient even against bugs INSIDE `httpctx`/`ctx` construction itself), the Exception branch only runs when we already have a well-formed, already-successfully-recovered `Exception` value with known-good `Status()`/`Name()`/`Message()`/`Details()` — there's no defensive reason to avoid the normal `ctx` API here, and using it keeps exactly one JSON-serialization code path in the whole adapter rather than two (raw Fiber JSON call vs `ctx.Json`) |
| Detection is a single type-assertion against the `Exception` INTERFACE, never a type-switch over the 5 concrete built-ins | `r.(exception.Exception)`, one line | This is the entire reason "HttpException Core" built `Exception` as a structural interface rather than a closed set of known types — a type-switch listing 5 cases would silently fail to recognize any FUTURE built-in or, more importantly, any DEV-DEFINED exception (INSIGHT.md's `FooExampleError` pattern), defeating half the point of the embedding design. Directly satisfies spec.md's PANIC-04. |
| No new exported API, no new package | Pure internal behavior change | Nothing about the public contract of `RegisterRoute`/`HttpAdapter` needs to change — Exception detection is an internal implementation detail of how the Fiber adapter maps panics to responses, same category of change as "the adapter now also checks X before falling back to Y", not a new capability callers configure |

---

## Open Questions pra Tasks

- None — this is a narrow, single-file behavior change with no ambiguity left after this design; the 6 acceptance criteria in spec.md map directly onto 1 code change + 1 comprehensive test suite.
