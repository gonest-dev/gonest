# HttpException Core Tasks

**Design**: `.specs/features/http-exception-core/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

```
T1 (Exception interface + HttpException core) → T2 (5 built-in exceptions) → T3 (root re-exports)
```

Sequential: T2 depends on `HttpException`/`NewHttpException` from T1 (same package, `internal/exception`, no real parallelism benefit for such a small feature — see L-003 precedent). T3 depends on everything existing to re-export.

---

## Task Breakdown

### T1: `Exception` interface + `HttpException` core type

**What**: `internal/exception/exception.go` (new file, new package) — `Exception` interface (`Status() int; Name() string; Message() string; Details() any`), `HttpException` struct (unexported fields: `status int; name, message string; details any`), `NewHttpException(status int, name, message string, details any) HttpException` (returns a VALUE, see design.md's Tech Decisions), and the 4 accessor methods (value receiver).
**Where**: `internal/exception/exception.go`, `internal/exception/exception_test.go`
**Depends on**: None
**Reuses**: nothing — first exception-shaped type in the codebase
**Requirement**: EXC-01, EXC-02, EXC-04, EXC-05, EXC-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `NewHttpException(status, name, message, details)`'s 4 accessors return exactly what was passed
- [ ] `details == nil` is accepted, `Details()` returns `nil` (not a panic, not a synthesized value)
- [ ] A locally-defined test type embedding `HttpException` by value (mirroring INSIGHT.md's `FooExampleError` pattern) satisfies `Exception` via promoted methods — proven by a type assertion in a test, not just "it compiles"
- [ ] A non-exception panic value (e.g. `errors.New("x")`, a bare `int`) does NOT satisfy `Exception` — proven by a failed type assertion in a test
- [ ] Gate check passes
- [ ] Test count: 6+ (4 accessors, nil details, embedding satisfies Exception, non-exception does not satisfy Exception — may combine some into table-driven tests)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(exception): add Exception interface and HttpException core type`

---

### T2: 5 built-in exceptions

**What**: `internal/exception/builtin.go` (new file) — `NotFoundException`/`BadRequestException`/`ConflictException`/`UnauthorizedException`/`ForbiddenException`, each `struct { HttpException }` with a `New*Exception(details any) *T` constructor (returns a POINTER, see design.md's Tech Decisions) that fixes `status` (via `net/http` stdlib constants: `http.StatusNotFound`/`StatusBadRequest`/`StatusConflict`/`StatusUnauthorized`/`StatusForbidden`) and `name` (the exception's own Go type name as a string, e.g. `"NotFoundException"`).
**Where**: `internal/exception/builtin.go`, `internal/exception/builtin_test.go`
**Depends on**: T1 (needs `HttpException`/`NewHttpException`)
**Reuses**: `HttpException`/`NewHttpException` from T1
**Requirement**: EXC-03

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] All 5 constructors return the correct fixed status code (404/400/409/401/403) and name string matching the type name exactly
- [ ] `New*Exception(nil)` works for all 5 (mirrors INSIGHT.md's `NewUnauthorizedException(nil)`)
- [ ] `panic(New*Exception(...))` + `recover().(*T)` round-trips correctly for all 5 (mirrors INSIGHT.md's own test pattern: `exc, ok := recover().(*gonest.NotFoundException)`)
- [ ] Each of the 5 satisfies `Exception` (table-driven, reusing T1's assertion pattern)
- [ ] Gate check passes
- [ ] Test count: 10+ (2 per exception: constructor correctness + panic/recover round-trip — may combine into table-driven tests covering all 5)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(exception): add 5 built-in HttpException subtypes`

---

### T3: Root re-exports

**What**: root `gonest` package gets `Exception`, `HttpException`, `NewHttpException`, and the 5 built-in types + constructors, all via type aliases (`type X = exception.X`) and plain `var` function aliases (`var NewX = exception.NewX` — no generic-wrapper needed, these aren't generic functions, see design.md's Tech Decisions and AD-004 in STATE.md for the general pattern).
**Where**: new file at repo root, e.g. `exception.go` (mirrors `internal/exception`'s own file name for discoverability), root-level test file
**Depends on**: T1, T2
**Reuses**: exact `type X = pkg.X` / `var Y = pkg.Y` idiom already used at root for other non-generic re-exports (check `app.go`'s `type App = app.App`-style aliases for the pattern; `var`-aliasing a plain func is new territory for this repo's root package but is standard, uncontroversial Go)
**Requirement**: EXC-01 through EXC-06 (surface-level completion)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `gonest.NewHttpException(...)`, `gonest.Exception`, `gonest.HttpException` all resolve and work at root
- [ ] All 5 `gonest.New*Exception(details)` constructors resolve and work at root, matching INSIGHT.md's exact call shapes
- [ ] A dev-defined type embedding `gonest.HttpException` (INSIGHT.md's `FooExampleError` example, reproduced verbatim in a test) compiles and satisfies `gonest.Exception`
- [ ] Gate check passes
- [ ] Test count: 3+ (root-level smoke test reproducing INSIGHT.md's `FooExampleError` example end-to-end, one panic/recover round-trip through a root-aliased built-in, one Exception-interface assertion through the root alias)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(exception): re-export Exception/HttpException/built-ins at root`

---

## Parallel Execution Map

```
Fully sequential: T1 → T2 → T3
```

**Nota de paralelismo (L-003):** T2 depende de tipos definidos em T1 no mesmo pacote (`internal/exception`) — sem paralelismo real. Feature pequena o suficiente (3 tasks) que não vale a pena forçar paralelismo artificial.

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: Exception + HttpException core | 1 arquivo novo, 1 pacote novo, tipo coeso | ✅ Granular |
| T2: 5 built-in exceptions | 1 arquivo novo, 5 tipos pequenos e paralelos entre si (mesmo shape) | ✅ Granular |
| T3: Root re-exports | 1 arquivo novo, mecânico | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Novo tipo de valor puro, sem I/O/HTTP | unit | unit | ✅ OK |
| T2 | Novo tipo de valor puro, sem I/O/HTTP | unit | unit | ✅ OK |
| T3 | Re-export surface, sem lógica nova | unit | unit | ✅ OK |

Nenhuma violação. **Nota:** `.specs/codebase/TESTING.md`'s Test Coverage Matrix não tem uma linha explícita pra "tipos de valor puro sem builder/graph" ainda — T1 pode adicionar uma linha pequena ("Exception/HttpException — construção e satisfação de interface") como housekeeping, não crítico o suficiente pra task própria.
