# Terminus/Health Check Specification

**Status: COMPLETE (2026-07-15, commit `5c2fde4`).**

**Status previously: REVISED (2026-07-15) -- scope shrunk to near-zero new framework code.** The user rewrote INSIGHT.md's own "# exemplo de Probes / health" section, replacing the original `NewHealthCheck`/`Module.HealthChecks`/`App.UseHealthCheck` design (a dedicated bootstrap-participating type, same family as Guard/Scheduler) with NestJS's OWN actual mental model for `@nestjs/terminus`: a health/probe endpoint is NOT a special framework concept -- it is JUST a regular `Controller`, using primitives this codebase already has (`NewController`, `MustInjectAll[Connectable]`, `Route`, `ctx.Status`/`ctx.Json`). See INSIGHT.md's own explanatory note right above the code block: "No NestJS, o Terminus não introduz uma estrutura nova de bootstrap; ele é simplesmente um Controller padrão... apenas usamos um `NewController` normal!"

**Blocking dependency REMOVED**: the previous spec (`NewHealthCheck` calling `MustInject` inside its own builder) genuinely depended on "Test App Bootstrap"'s 3-phase reorder (AD-015). The new design has NO new `New*`-builder type at all -- `HealthController` is a plain `gonest.NewController`, which already fully supports `MustInjectAll`/`MustInject` (built and shipped this session, see Milestone 8/9). There is nothing left to block on.

## Problem Statement

Milestone 11 (equivalente `@nestjs/terminus`): expose Kubernetes-style `/readyz` (readiness -- pings every registered `Connectable`, 503 if any is down) and `/livez` (liveness -- a static 200, "if the Go process can respond at all, it isn't deadlocked") probes, as a completely ordinary Controller -- no new bootstrap type, no new Module-level registration method.

## Goals

This feature's ENTIRE goal is now: confirm the pattern INSIGHT.md documents actually compiles and runs against this codebase's real API, and close the 2 small primitive gaps it exposes (both are general-purpose additions, not Terminus-specific machinery):

- [ ] **`Context.SendString(s string) error`** (NEW, `internal/execution`) -- writes `s` as a raw, uninterpreted response body (no JSON-encoding, no HTML content-type) -- INSIGHT.md's liveness probe does `ctx.Status(gonest.HttpStatusOk).SendString("OK")`. Mirrors `Context.HTML`'s existing shape (delegates to `Responder`, needs a new `Responder.SendString(s string) error` method every adapter implements -- Fiber's own version delegates to `fiber.Ctx.SendString`, already used internally by `internal/adapter/fiber`'s own generic-500 fallback path) but withOUT setting any Content-Type (plain `text/plain` implied by the absence of a header, matching Fiber's own default for `SendString`).
- [ ] **`HttpStatusOk`/`HttpStatusServiceUnavailable` root constants** (NEW, `gonest.go`) -- INSIGHT.md's example (and the earlier "exemplo de Testing" section, `gonest.HttpStatusOk`) assumes these already exist; they don't -- every existing test in this codebase uses raw `http.StatusOK` from `net/http` directly. This is a genuinely CROSS-CUTTING gap (not Terminus-specific), but Terminus's own example is what surfaces it concretely enough to finally close. Scope: add ONLY the 2 status codes INSIGHT.md's examples actually reference (`HttpStatusOk` = `http.StatusOK`, `HttpStatusServiceUnavailable` = `http.StatusServiceUnavailable`) as typed `int` constants -- NOT a full HTTP status enum (no example demonstrates needing more than these 2, same "don't invent unused API surface" stance as every prior feature).
- [ ] Reproduce INSIGHT.md's `HealthController`/`AppModule` example verbatim: `/health/readyz` (aggregates every `Connectable` via `MustInjectAll`, 200 if all `Ping(ctx)` succeed, 503 + per-connectable up/down detail if any fails) and `/health/livez` (static 200 "OK") -- both via a completely ordinary `Controller`/`Module`, zero new bootstrap-participating type

## Out of Scope

| Feature | Reason |
| --- | --- |
| `NewHealthCheck`/`Module.HealthChecks`/`App.UseHealthCheck` (the ORIGINAL design) | Superseded entirely by the user's own INSIGHT.md rewrite -- health checks are just a Controller now, no dedicated framework type |
| A full HTTP status code enum (`HttpStatusNotFound`, `HttpStatusBadRequest`, etc.) | Only the 2 status codes INSIGHT.md's examples actually use are in scope here -- a fuller enum is a separate, later housekeeping task if a real need appears (same stance as every prior "don't invent unused API surface" decision this session) |
| Timeout configurável por probe individual | Não especificado pelo novo exemplo -- cada `Connectable.Ping(ctx)` já recebe um `ctx` que o CALLER (o Handler da rota) poderia, no futuro, derivar com timeout via `context.WithTimeout` -- não é uma preocupação desta feature |
| Separação liveness/readiness em módulos distintos | O exemplo usa os 2 na MESMA `HealthController` -- é assim que fica especificado, sem necessidade de generalizar |

---

## User Stories

### P1: `HealthController` pattern, matching INSIGHT.md verbatim ⭐ MVP

**User Story**: Como usuário do gonest, quero declarar um `Controller` comum que injeta `MustInjectAll[Connectable]` e expõe `/readyz`/`/livez`, sem precisar de nenhum tipo novo de bootstrap, reproduzindo INSIGHT.md's exemplo verbatim.

**Acceptance Criteria**:

1. WHEN `GET /health/readyz` é chamado E todo `Connectable.Ping(ctx)` registrado devolve `nil` THEN sistema SHALL responder `HttpStatusOk` com `{"status":"ok","checks":{"<name>":"up", ...}}`
2. WHEN QUALQUER `Connectable.Ping(ctx)` devolve erro THEN sistema SHALL responder `HttpStatusServiceUnavailable`, com esse `Connectable`'s entrada marcada `"down"` no mesmo mapa `checks` (as que passaram continuam `"up"`)
3. WHEN `GET /health/livez` é chamado THEN sistema SHALL responder `HttpStatusOk` com corpo bruto `"OK"` (via `Context.SendString`, sem JSON-encoding)
4. WHEN `HealthController` é registrado via `Module.Controllers(HealthController)` THEN NENHUM mecanismo de bootstrap novo é necessário -- `MustInjectAll[Connectable](controller)` já resolve via o mecanismo padrão de Controller (fase 2, ver Milestone 8)

**Independent Test**: reproduzir `HealthController`/`AppModule` do INSIGHT.md verbatim, com 2 `Connectable` fakes controláveis (um sempre "up", outro alternável "up"/"down"); confirmar 200 no caso feliz de `/readyz`, 503 com `checks` correto quando um fica "down"; confirmar `/livez` sempre 200 com corpo `"OK"`.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| TH-01 | Context.SendString (novo primitivo, Responder gains SendString) | Tasks | Pending |
| TH-02 | HttpStatusOk/HttpStatusServiceUnavailable root constants | Tasks | Pending |
| TH-03 | GET /readyz 200 quando tudo passa, 503 + detalhe quando algo falha | Tasks | Pending |
| TH-04 | GET /livez sempre 200, corpo "OK" via SendString | Tasks | Pending |

**ID format:** `TH-[NUMBER]`

**Coverage:** 4 total, 0 mapped yet.

---

## Success Criteria

- [ ] Exemplo do INSIGHT.md (`HealthController`/`AppModule`) reproduzido end-to-end via dispatch HTTP real
- [ ] Zero regressões na suite existente
- [ ] `Context.SendString`/`HttpStatusOk`/`HttpStatusServiceUnavailable` re-exportados e testados isoladamente (não só através do exemplo de Terminus, já que são primitivos de uso geral)

---

## Blocking Dependency

**Nenhuma.** Diferente da versão anterior deste spec, a nova abordagem não depende de nenhum tipo `New*`-builder novo -- `Controller`/`MustInjectAll` já existem e funcionam (Milestone 8/9). Pode ser executada a qualquer momento.
