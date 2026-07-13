# HttpException Core Specification

## Problem Statement

Every route `Handler` built so far can only fail in one way from the caller's perspective: any panic becomes a generic, detail-free 500 (`internal/fiberapp`'s recover wrapper, T7). There is no way for domain code (a `Provider`'s service method, a route `Handler`) to signal "this specific, expected failure happened" with a status code and a structured body — the Nest equivalent of `throw new NotFoundException(...)`. This feature builds the exception TYPE SYSTEM only (`HttpException`, `NewHttpException`, 5 built-in subtypes); wiring panic-recovery to actually detect and format these into an HTTP response is the next feature ("Panic Recovery & Default Handler", also Milestone 2) — see Out of Scope.

## Goals

- [ ] `gonest.NewHttpException(status, name, message, details)` constructs a value carrying everything needed to render `{name, message, details}` at a given HTTP status
- [ ] A dev can define their own exception type (`type FooExampleError struct { gonest.HttpException }`) the exact same way the framework defines its own 5 built-ins — no special-casing between "framework exception" and "domain exception"
- [ ] All 5 built-in exceptions (`NotFoundException`→404, `BadRequestException`→400, `ConflictException`→409, `UnauthorizedException`→401, `ForbiddenException`→403) exist with `New*Exception(details any)` constructors

## Out of Scope

| Feature | Reason |
| --- | --- |
| Panic recovery actually detecting an Exception and writing `{name,message,details}` to the real HTTP response | Next feature, "Panic Recovery & Default Handler" (Milestone 2) — this feature only builds the value types or `internal/fiberapp`'s existing generic-500 recover wrapper (T7) untouched |
| `Filter`/`Catch(exceptionType, handler)` (per-exception custom response override) | Milestone 3 ("Request Pipeline" → `Filter`) |
| Full `HttpStatus` enum (`HttpStatusOk`, `HttpStatusCreated`, `HttpStatusTeapot`, etc. — every status INSIGHT.md's examples eventually use) | Only the 5 status codes the 5 built-in exceptions need are in scope here (400/401/403/404/409); a broader `HttpStatus` enum is not named in any ROADMAP.md milestone and would be speculative scope — add codes as concrete features need them |
| `MustJsonBody[T]`/Pipe validation raising `BadRequestException` automatically on invalid input | Milestone 6 (Runtime Validation) — this feature's `BadRequestException` exists as a type devs can `panic()` manually; automatic validation wiring is later |

---

## User Stories

### P1: Define and construct exceptions ⭐ MVP

**User Story**: As a gonest user, I want `gonest.NewHttpException(status, name, message, details)` and the 5 built-in `New*Exception(details)` constructors so that I can `panic()` a structured, typed failure from anywhere in my domain code (Provider/Controller/route Handler), the same way I'd `throw new NotFoundException()` in Nest.

**Why P1**: Without a concrete exception type to construct, there's nothing for the next feature (Panic Recovery & Default Handler) to detect and format — this is the foundational value type every later exception-handling feature builds on.

**Acceptance Criteria**:

1. WHEN `gonest.NewHttpException(status, name, message, details)` is called THEN system SHALL return a value whose `Status()`/`Name()`/`Message()`/`Details()` accessors return exactly what was passed
2. WHEN a dev defines `type FooExampleError struct { gonest.HttpException }` and constructs it by embedding a `gonest.NewHttpException(...)` call in their own constructor (mirroring INSIGHT.md's exact example) THEN `FooExampleError`'s promoted `Status()`/`Name()`/`Message()`/`Details()` methods SHALL work identically to a built-in exception's
3. WHEN any of the 5 built-in constructors (`NewNotFoundException(details)`, `NewBadRequestException(details)`, `NewConflictException(details)`, `NewUnauthorizedException(details)`, `NewForbiddenException(details)`) is called THEN system SHALL return a value with the correct fixed status code (404/400/409/401/403 respectively) and a name matching the exception's own type (e.g. `"NotFoundException"`)
4. WHEN `details` is `nil` (see INSIGHT.md's `NewUnauthorizedException(nil)` example) THEN system SHALL accept it without error — `Details()` returns `nil`, not a panic or a synthesized empty value

**Independent Test**: construct each of the 5 built-ins plus one custom `FooExampleError`-style exception, call `panic()` on each in a `defer/recover()`-wrapped test, `recover()` the value, type-assert it back to its concrete type (mirroring INSIGHT.md's own test example: `exc, ok := recover().(*gonest.NotFoundException)`), and confirm every accessor returns the expected value.

---

### P2: A common interface identifies "this panic value is an Exception"

**User Story**: As the framework's own future panic-recovery code (next feature) and as a dev writing a custom `Filter` (Milestone 3), I want a single interface that any `HttpException`-embedding type structurally satisfies, so that recovery/filter code can do one type-assertion (`recover().(gonest.Exception)`) instead of maintaining a growing list of every concrete exception type by name.

**Why P2**: Not needed to construct/use exceptions today (P1 covers that), but is a foundational design decision the NEXT feature depends on — get the interface shape right now, before code exists that has to match it, rather than retrofitting later.

**Acceptance Criteria**:

1. WHEN any value embedding `gonest.HttpException` (built-in or custom) is type-asserted against the common interface THEN system SHALL report a successful assertion (structural satisfaction via promoted methods)
2. WHEN a plain Go panic value that does NOT embed `HttpException` (e.g. a bare `error`, a string, `nil`-pointer dereference) is type-asserted against the same interface THEN system SHALL report a failed assertion — the interface must not accidentally match unrelated types

**Independent Test**: write a table-driven test asserting `ok` is true for all 6 exception values (5 built-ins + 1 custom) against the common interface, and `ok` is false for at least 2 non-exception panic values (`errors.New("x")`, a bare `int`).

---

## Edge Cases

- WHEN a custom exception embeds `HttpException` but ALSO defines its own colliding method name (e.g. its own `Name()` that shadows the promoted one) THEN Go's normal method-shadowing rules apply — not a gonest-specific concern, no special handling needed, just don't break if a dev does this
- WHEN `status` passed to `NewHttpException` is a value outside typical HTTP status conventions (e.g. `0`, `999`) THEN system SHALL NOT validate/reject it — this is a low-level constructor, the 5 built-ins already guarantee sane values, validation is not this feature's job (matches Go's general "trust internal code" convention already used elsewhere in this codebase, see CLAUDE.md-equivalent conventions)
- WHEN `message` is empty string THEN system SHALL accept it as-is, no special-casing

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| EXC-01 | P1: NewHttpException constructs with correct accessors | Design | Pending |
| EXC-02 | P1: Custom exception via embedding works identically to built-ins | Design | Pending |
| EXC-03 | P1: 5 built-in constructors, correct status+name | Design | Pending |
| EXC-04 | P1: nil details accepted | Design | Pending |
| EXC-05 | P2: Common interface, structural satisfaction for any embedder | Design | Pending |
| EXC-06 | P2: Common interface does NOT match non-exception panic values | Design | Pending |

**ID format:** `EXC-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 6 total, 0 mapped to tasks yet, 6 unmapped ⚠️ (Design phase next)

---

## Success Criteria

- [ ] All 5 built-in exceptions + `NewHttpException`/custom-embedding pattern work exactly as INSIGHT.md's own examples show (`gonest.NewNotFoundException(...)`, `type FooExampleError struct { gonest.HttpException }`, `recover().(*gonest.NotFoundException)`)
- [ ] Zero regressions in the existing 12-package test suite (T1-T6 of "App Bootstrap & Listen" and everything before it)
- [ ] The common interface (P2) is provably narrow — doesn't accidentally match unrelated panic values — so the next feature can trust a single type-assertion
