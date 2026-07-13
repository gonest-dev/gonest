# Handoff

**Date:** 2026-07-13
**Feature:** Guard (Milestone 3) — ✅ COMPLETE (T1-T4)
**Task:** Nenhuma em progresso. Próxima: especificar "Interceptor" (3ª feature de Milestone 3, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-2 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, contrato de exceção `{name,message,details}` com recovery real. Ver STATE.md pra histórico completo.
- **Milestone 3 em progresso:**
  - Feature "Middleware" (T1-T5) — COMPLETE. `internal/middleware`, `Controller.Use`/`Module.Use` reais, composição em Stage 2.5 (global sempre outermost). Ver STATE.md L-010 (Header()/SetHeader() são stores diferentes).
  - Feature "Guard" (T1-T4) — COMPLETE
    - T1 (`internal/guard`: `Guard`/`New`/`Handler`, execução imediata) — DONE, evaluator PASS, commit `4e8d03f`
    - T2 (`Controller.Guards` real, era stub; `OwnGuards()`) — DONE, evaluator PASS, commit `4f29ed8`
    - T3 (`gatedHandler` em Stage 2.5: guards avaliados em ordem, `false`→403 automático via `panic(NewForbiddenException)`, encaixado como innermost do wrap de Middleware já existente) — DONE, evaluator PASS, commit `2f7fbc4`
    - T4 (re-exports raiz: `Guard`/`NewGuard`) — DONE, evaluator PASS (nota menor não-bloqueante: teste raiz confere status code mas não body completo, já coberto em T3), commit `97ba40c`
  - **AD-008 registrada (STATE.md):** decisão explícita do usuário — Middleware/Guard (e Interceptor/Filter futuros) NÃO suportam `MustInject` no builder, porque podem ter múltiplos "donos" (anexados a vários controllers/módulos), diferente de Provider (1 dono só). `New(fn)` roda imediato, sempre.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Interceptor** (`NewInterceptor`, envolve execução do handler antes/depois — AOP, ver ROADMAP.md Milestone 3). Segue o mesmo padrão de AD-008 (sem MustInject, execução imediata) até decisão em contrário. INSIGHT.md's `TimingInterceptor` também usa `MustInject` no exemplo original — vai precisar da mesma adaptação que Guard/Middleware já tiveram (capturar dependência via closure, não injeção real)
- Depois: `Pipe` (pipeline-stage, distinto do `Pipe` de coerção de param já existente) e `Filter`, depois "Pipeline Ordering" (valida ordem combinada Middleware → Guard → Interceptor → Pipe → Handler)

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez, ver STATE.md.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `httpctx.Context.route any` + type assertion deveria ser interface tipada (`paramHost`) — barato de trocar agora, mais caro depois.
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response) — não dá pra "ler o que acabei de setar" via API pública hoje.
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes, registrados em STATE.md Deferred Ideas.
- `HttpStatus` enum completo não existe ainda — considerar se Interceptor/Filter precisam disso.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético, sem prioridade.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator, 2 sub-agents por task), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz), AD-008 (pipeline-stage types sem MustInject, execução imediata), L-007 (`git commit -- <arquivos>` sempre com pathspec)
- Pra retomar: ler STATE.md inteiro primeiro (tem todo histórico de decisões/lições), depois ROADMAP.md Milestone 3 pra especificar "Interceptor" (Specify phase, feature nova) — provável reaproveitamento quase mecânico do padrão de Middleware/Guard (internal/interceptor, Controller.Interceptors real, composição em Stage 2.5, root re-export).
