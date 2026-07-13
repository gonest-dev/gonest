# Panic Recovery & Default Handler Tasks

**Design**: `.specs/features/panic-recovery-default-handler/design.md`
**Testing**: `.specs/codebase/TESTING.md`
**Status**: Draft

---

## Execution Plan

```
T1 (Exception detection + formatted response in RegisterRoute's recover branch)
```

Single task — this feature is a narrow, single-file behavior change (design.md's Open Questions confirms no ambiguity left to split across tasks). No parallelism to plan for.

---

## Task Breakdown

### T1: Exception detection in `RegisterRoute`'s recover branch

**What**: extend `internal/fiberapp/fiberapp.go`'s `RegisterRoute` wrapper (existing `defer/recover()` block, T7) — add `github.com/gonest-dev/gonest/internal/exception` import, and inside the recover branch, type-assert the recovered value against `exception.Exception`. If it satisfies the interface: `ctx.Status(exc.Status()).Json(map[string]any{"name": exc.Name(), "message": exc.Message(), "details": exc.Details()})`. If not: fall through to the EXISTING generic-500 path (`c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")`), unchanged.
**Where**: `internal/fiberapp/fiberapp.go` (existing file, existing function extended), `internal/fiberapp/fiberapp_test.go` (existing — add tests)
**Depends on**: None (both `internal/exception`, from the prior feature, and `internal/fiberapp`'s existing recover wrapper, from T7, are already complete and committed)
**Reuses**: `exception.Exception` interface + all 5 built-ins + the embeddable `HttpException` pattern (prior feature, "HttpException Core"), `httpctx.Context.Status`/`Json` (T2), the existing `ctx` already constructed earlier in the wrapper
**Requirement**: PANIC-01 through PANIC-06 (all of them — this is the only task)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Panicking with each of the 5 built-in exceptions produces the correct status code + `{"name","message","details"}` JSON body, dispatched via a REAL `app.Test` request (not a unit-level check of the recover logic alone)
- [ ] Panicking with a dev-defined exception (a locally-declared test type embedding `HttpException`, mirroring INSIGHT.md's `FooExampleError`) is treated identically — proves detection is interface-based, not a type-switch over known built-ins
- [ ] `Details() == nil` on the panicked exception serializes as JSON `null` in the response body (not omitted, not empty object)
- [ ] Panicking with a non-Exception value (bare `error`, `nil`-pointer deref, `index out of range`, raw string) still produces the EXACT SAME generic 500 + `"Internal Server Error"` body as before this task — write an explicit test proving the response body does NOT contain the panic value's own message content
- [ ] `panic(nil)` (Go 1.21+ `*runtime.PanicNilError` on recover) falls through to the generic-500 path without crashing the test process — explicit test
- [ ] Existing T7 tests (`internal/fiberapp/fiberapp_test.go`'s original panic→500 test) still pass unmodified — proves this is additive, not a regression
- [ ] Gate check passes
- [ ] Test count: 8+ (5 built-ins + 1 custom exception + nil-details-serializes-as-null + non-Exception-still-generic-500 + panic(nil)-edge-case — combine into table-driven tests where it reads cleanly, but every bullet above needs its own genuine assertion)

**Tests**: integration (real Fiber dispatch via `app.Test`, per this codebase's established pattern for anything touching `internal/fiberapp`'s actual request/response cycle — see TESTING.md's Test Coverage Matrix)
**Gate**: full

**Commit**: `feat(http): detect Exception in panic recovery, format {name,message,details} response`

---

## Parallel Execution Map

```
Single task, no parallelism to plan.
```

**Papéis por task (AD-001 em STATE.md):** developer sub-agent implementa, evaluator sub-agent separado confere Done-when + roda Gate antes de marcar `[x]`.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: Exception detection in recover branch | 1 arquivo existente estendido, 1 responsabilidade coesa (mesmo sendo o "fecho" de toda a feature) | ✅ Granular |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | Dispatch de rota via Fiber real (`internal/fiberapp`) — mesma camada que T7/T3 já tocaram | integration | integration | ✅ OK |

Nenhuma violação.
