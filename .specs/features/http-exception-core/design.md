# HttpException Core Design

**Spec**: `.specs/features/http-exception-core/spec.md`

## Architecture Overview

```
internal/exception (new package, AD-004 pattern: 1 package for this concept)
        │
        ├── Exception interface        -- Status()/Name()/Message()/Details()
        │        ▲ (structural satisfaction via promoted methods)
        ├── HttpException struct       -- concrete carrier, embeddable
        │        │
        │        ├── NotFoundException struct { HttpException }      -- 404
        │        ├── BadRequestException struct { HttpException }    -- 400
        │        ├── ConflictException struct { HttpException }      -- 409
        │        ├── UnauthorizedException struct { HttpException }  -- 401
        │        └── ForbiddenException struct { HttpException }     -- 403
        │
        └── net/http status constants reused directly (StatusNotFound etc.)
                 -- no custom HttpStatus enum, see spec.md's Out of Scope

root gonest package: type aliases + var-aliased constructors (no generics here,
unlike MustInject/MustParam/NewApp -- plain functions re-export via `var` directly)
```

No wiring into `internal/fiberapp`'s recover path in this feature — that stays a generic 500 (T7, unchanged) until the next feature ("Panic Recovery & Default Handler") teaches it to check `recover()`'s value against `Exception`.

---

## Components

### Exception (interface)

- **Purpose**: the single assertion point future code (panic recovery, `Filter`) uses to answer "is this panic value a structured exception". Satisfied structurally by ANY type that embeds `HttpException` (built-in or dev-defined) — no explicit `implements`/registration needed, matching Go's usual embedding-promotes-methods behavior.
- **Location**: `internal/exception/exception.go` (new file)
- **Interfaces**: `type Exception interface { Status() int; Name() string; Message() string; Details() any }`
- **Dependencies**: none
- **Reuses**: nothing — this is the new foundational interface for this concept

### HttpException (struct)

- **Purpose**: concrete carrier of `status`/`name`/`message`/`details`, meant to be embedded (not used bare) by both the 5 built-ins and any dev-defined exception type — mirrors INSIGHT.md's exact pattern (`type FooExampleError struct { gonest.HttpException }`).
- **Location**: `internal/exception/exception.go` (same file as `Exception` — small, tightly coupled, no reason to split)
- **Interfaces**:
  - `type HttpException struct { status int; name string; message string; details any }`
  - `func NewHttpException(status int, name, message string, details any) HttpException` — returns a VALUE, not a pointer (embedding a value field is simpler for dev-defined types than embedding a pointer field that could be nil; matches INSIGHT.md's `HttpException: gonest.NewHttpException(...)` struct-literal field assignment, which requires a value, not `*HttpException`)
  - `func (e HttpException) Status() int`
  - `func (e HttpException) Name() string`
  - `func (e HttpException) Message() string`
  - `func (e HttpException) Details() any`
- **Dependencies**: none
- **Reuses**: nothing new — this is the first exception-shaped type in the codebase

### Built-in exceptions (5 types)

- **Purpose**: framework-provided exceptions for the 5 most common HTTP failure modes, constructed exactly like a dev would construct their own (no special internal shortcut) — proves `HttpException`'s embedding contract is real, not just a claim.
- **Location**: `internal/exception/builtin.go` (new file, separate from `exception.go` since these are 5 co-located but logically distinct declarations, not part of the core type's own definition)
- **Interfaces** (one pair per exception, `status` fixed per constructor, `name` fixed to the Go type's own name string):
  - `type NotFoundException struct { HttpException }` / `func NewNotFoundException(details any) *NotFoundException` → status `http.StatusNotFound` (404), name `"NotFoundException"`
  - `type BadRequestException struct { HttpException }` / `func NewBadRequestException(details any) *BadRequestException` → `http.StatusBadRequest` (400), `"BadRequestException"`
  - `type ConflictException struct { HttpException }` / `func NewConflictException(details any) *ConflictException` → `http.StatusConflict` (409), `"ConflictException"`
  - `type UnauthorizedException struct { HttpException }` / `func NewUnauthorizedException(details any) *UnauthorizedException` → `http.StatusUnauthorized` (401), `"UnauthorizedException"`
  - `type ForbiddenException struct { HttpException }` / `func NewForbiddenException(details any) *ForbiddenException` → `http.StatusForbidden` (403), `"ForbiddenException"`
  - Each constructor returns a POINTER (`*NotFoundException`, not `NotFoundException`) — matches INSIGHT.md's own recover-and-assert example (`recover().(*gonest.NotFoundException)`) and Go's usual convention of `panic`ing a pointer for a value that will be type-asserted back (pointer identity is cheap, avoids an unnecessary copy of the embedded struct on panic/recover)
- **Dependencies**: `net/http` (status constants only — `http.StatusNotFound` etc., stdlib, not a real HTTP dependency, no request/response types touched)
- **Reuses**: `HttpException`/`NewHttpException` from `exception.go`

---

## Data Models

```go
type Exception interface {
    Status() int
    Name() string
    Message() string
    Details() any
}

type HttpException struct {
    status  int
    name    string
    message string
    details any
}

func NewHttpException(status int, name, message string, details any) HttpException

// 5 built-ins, each: struct { HttpException } + New*Exception(details any) *T constructor
```

**Relationships**: `HttpException` is embedded by value inside each built-in AND inside any dev-defined exception type. `Exception` is never embedded/implemented explicitly — it's satisfied purely structurally by whatever embeds `HttpException` (or independently implements the same 4-method shape, though no code in this feature does that).

---

## Error Handling Strategy

Not applicable in the usual sense — this feature IS the error-handling vocabulary, not a consumer of it. No panics/errors originate from this feature's own code (constructors are pure, no validation per spec.md's Edge Cases — a `status` of `0` or `999` is accepted without complaint, that's the caller's problem).

---

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| `NewHttpException` returns a VALUE (`HttpException`), not `*HttpException` | Value, embedded by value in every consumer | INSIGHT.md's own dev-defined-exception example assigns it directly into a struct-literal field (`HttpException: gonest.NewHttpException(...)`) — that's a value assignment. A pointer-returning constructor would force `*HttpException` embedding, which changes the promoted-method receiver semantics (pointer-receiver methods only promote through a pointer-embedding, and `FooExampleError{}` as a bare struct literal — no `&`-then-deref dance — matches the example exactly with value embedding) |
| Built-in constructors (`NewNotFoundException` etc.) return POINTERS (`*NotFoundException`) | Pointer, despite `HttpException` itself being embedded by value inside them | Matches INSIGHT.md's own recover-and-type-assert example verbatim (`recover().(*gonest.NotFoundException)`) — the built-ins are meant to be `panic()`ed and `recover()`ed, where pointer identity keeps the recovered value's assertion target unambiguous and matches idiomatic Go error-value conventions (compare `errors.New` returning `error` wrapping a pointer internally) |
| Status codes sourced from `net/http` stdlib constants, not a custom `HttpStatus` enum | `http.StatusNotFound` etc. directly | Per spec.md's Out of Scope: a full custom `HttpStatus` enum (`HttpStatusOk`, `HttpStatusTeapot`, etc., as INSIGHT.md eventually uses across other examples) is not named in any ROADMAP.md milestone for THIS feature — reusing stdlib avoids inventing enum values this feature doesn't need, while not blocking a future feature from introducing `type HttpStatus = int` aliases of the same stdlib constants if Nest-parity naming becomes a real requirement later |
| `Exception` interface lives in `internal/exception`, not co-located with a future `internal/fiberapp`/panic-recovery package | `internal/exception` owns both the interface and its only current implementers | AD-004's "1 package per concept" — `Exception` is exception-vocabulary, not an HTTP-adapter concept; the next feature (Panic Recovery & Default Handler) will IMPORT `internal/exception`, not the other way around, keeping the dependency direction clean (exception vocabulary has zero HTTP-adapter knowledge) |
| Root re-export: plain `var` aliasing for constructors, not generic-wrapper functions | `var NewHttpException = exception.NewHttpException` (and same for the 5 built-in constructors) | None of these functions are generic (unlike `MustInject[T]`/`MustParam[T]`/`NewApp[T]`) — Go CAN re-export a non-generic function via `var` directly, no wrapper needed. Types re-export via `type X = exception.X` as usual (AD-004). |

---

## Open Questions pra Tasks

- None — spec.md's Out of Scope table and this design.md's Tech Decisions cover every INSIGHT.md example detail relevant to this feature's narrow scope (type system only, no HTTP wiring).
