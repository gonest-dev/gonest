# Terminus/Health Check Specification

## Problem Statement

Milestone 11 (equivalente `@nestjs/terminus`, ver ROADMAP.md): endpoint `/health` automático a partir de `HealthCheck`s registrados no módulo. INSIGHT.md's "# exemplo de Terminus/health" já especifica o call shape completo.

**Dependência bloqueante**: `NewHealthCheck(fn)` é mais um tipo `New*`-builder que chama `MustInject` dentro do próprio builder (`db, redis := gonest.MustInject[*Db](health), gonest.MustInject[*Redis](health)`) -- precisa do bootstrap de 3 fases especificado (não implementado ainda) em "Test App Bootstrap" (`.specs/features/test-app-bootstrap/`, ver AD-015 em STATE.md). Esta feature NÃO deve ser executada antes daquela.

## Goals

- [ ] `NewHealthCheck(fn func(health *HealthCheck)) *HealthCheck` -- mesmo padrão `New*`-deferred (AD-015)
- [ ] `Module.HealthChecks(...*HealthCheck)` -- registro no bootstrap
- [ ] `health.Check(name string, fn func(ctx context.Context) error) *HealthCheck` -- registra 1 checagem nomeada; `fn` devolvendo `nil` = saudável, erro = falhou (mensagem do erro vira o detalhe)
- [ ] `App.UseHealthCheck(path string)` -- monta rota `GET {path}` automaticamente a partir de TODOS os `HealthCheck`s registrados em TODO o módulo (raiz + imports), rodando TODAS as checagens
- [ ] Resposta: status 200 + `{"status":"ok","checks":{"database":"ok","redis":"ok"}}` se TODAS passarem; status 503 + detalhe (nome + erro) de QUAL(is) checagem(ns) falharam se alguma falhar

## Out of Scope

| Feature | Reason |
| --- | --- |
| Timeout configurável por checagem individual | INSIGHT.md's exemplo não especifica -- pode herdar algum timeout padrão do processo, sem API extra por enquanto |
| Checagens "readiness" vs "liveness" separadas (2 endpoints distintos, padrão Kubernetes) | Nenhum exemplo pede isso -- só 1 endpoint (`/health`) com todas as checagens juntas |
| Cache do resultado (não re-rodar checagem a cada request) | Não especificado -- assumir "roda toda vez que `/health` é chamado", comportamento mais simples e correto por padrão |

---

## User Stories

### P1: HealthCheck + UseHealthCheck, matching INSIGHT.md verbatim ⭐ MVP

**User Story**: Como usuário do gonest, quero declarar `AppHealth` com 2 checagens (`database`, `redis`), montar `/health` via `app.UseHealthCheck(path)`, e receber 200 ou 503 conforme o resultado, reproduzindo INSIGHT.md's exemplo verbatim.

**Acceptance Criteria**:

1. WHEN `GET /health` é chamado E toda checagem registrada devolve `nil` THEN sistema SHALL responder 200 com `{"status":"ok","checks":{...}}` (uma entrada `"ok"` por checagem)
2. WHEN QUALQUER checagem devolve erro THEN sistema SHALL responder 503 com detalhe de QUAL(is) checagem(ns) falharam (nome + mensagem do erro), mas AINDA reportando o resultado das checagens que passaram
3. WHEN `Module.HealthChecks(h)` é chamado THEN `h`'s builder SHALL seguir o MESMO bootstrap de 3 fases que Guard/Middleware/Interceptor/Filter/Listener/Scheduler (AD-015)
4. WHEN `App.UseHealthCheck(path)` é chamado THEN sistema SHALL registrar a rota `GET {path}` DIRETO no `app.Adapter()` (mesmo mecanismo de baixo nível que `SetupSwagger` já usa, ver Milestone 7's "Swagger UI Setup"), agregando TODO `HealthCheck` de TODO módulo (raiz + imports, via `App.Root()`, já existe)

**Independent Test**: reproduzir `AppHealth`/`UseHealthCheck` do INSIGHT.md verbatim, com um `Db`/`Redis` fake controlável (ping bem-sucedido vs falho); confirmar 200 no caso feliz, 503 com detalhe correto no caso de falha de uma das duas checagens.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| TH-01 | P1: GET /health 200 quando tudo passa | Tasks | Pending |
| TH-02 | P1: GET /health 503 com detalhe quando algo falha | Tasks | Pending |
| TH-03 | P1: UseHealthCheck agrega todo módulo via App.Root() | Tasks | Pending |

**ID format:** `TH-[NUMBER]`

**Coverage:** 3 total, 0 mapped yet.

---

## Success Criteria

- [ ] Exemplo do INSIGHT.md reproduzido end-to-end
- [ ] Zero regressões na suite existente

---

## Blocking Dependency

**Não executar antes de "Test App Bootstrap" (Milestone 8) estar implementado.** Mesma razão de Emitter/Scheduler -- `HealthCheck` precisa do bootstrap de 3 fases (AD-015) pra `MustInject` dentro do seu builder funcionar.
