# Handoff

**Date:** 2026-07-13
**Feature:** Interceptor (Milestone 3) — ✅ COMPLETE (T1-T4)
**Task:** Nenhuma em progresso. Próxima: especificar "Pipe" (pipeline-stage, 4ª feature de Milestone 3, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-2 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, contrato de exceção `{name,message,details}` com recovery real. Ver STATE.md pra histórico completo.
- **Milestone 3 em progresso:**
  - Feature "Middleware" (T1-T5) — COMPLETE.
  - Feature "Guard" (T1-T4) — COMPLETE. AD-008 registrada.
  - Feature "Interceptor" (T1-T4) — COMPLETE
    - T1 (`internal/interceptor`: `Next` PRÓPRIO, `Interceptor`/`New`/`Handler`, execução imediata) — DONE, evaluator PASS, commit `81b32ba`
    - T2 (`Controller.Interceptors` real, era stub; `OwnInterceptors()`) — DONE, evaluator PASS, commit `2345f95`
    - T3 (composição em Stage 2.5: Interceptor envolve `routeHandler` puro, Guard envolve o RESULTADO — Guard mais externo) — DONE, evaluator PASS, commits `2f7fbc4`+`d74ec71` (segundo commit = correção de bug real de ordem, ver L-011 em STATE.md)
    - T4 (re-exports raiz: `Interceptor`/`NewInterceptor`) — DONE, evaluator PASS, commit `f357df0`
  - **L-011 registrada (STATE.md):** erro real no MEU design.md — descrevi Interceptor envolvendo `gatedHandler` (Guard+Handler já compostos), produzindo ordem errada (Interceptor-before rodando antes do Guard decidir). Corrigido: Interceptor envolve o Handler puro, Guard envolve isso. Ordem final correta: Middleware → Guard → Interceptor → Handler, batendo com ROADMAP.md. Dev sub-agent que implementou T3 originalmente seguiu o design errado corretamente (não era escopo dele reordenar sozinho) e reportou a divergência como SPEC_DEVIATION — foi isso que permitiu pegar o erro rápido.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Pipe** (pipeline-stage, distinto do `Pipe` de coerção de param já existente — `NewPipe`, transforma/valida param antes do handler, panic `BadRequestException` se inválido, ver ROADMAP.md Milestone 3). Mesmo padrão AD-008 provavelmente aplica (sem MustInject).
- Depois: `Filter` (`NewFilter`, `Catch(exceptionType, handler)` — último stub de `Controller` ainda no placeholder), depois "Pipeline Ordering" (valida ordem combinada Middleware → Guard → Interceptor → Pipe → Handler com TODOS os 5 estágios existindo)
- **Atenção especial pra "Pipeline Ordering":** dado L-011, ao especificar essa feature validar explicitamente a ordem de execução resultante de qualquer composição proposta contra ROADMAP.md ANTES de despachar pra developer — não só a ordem "de quem chama quem no código", mas a ordem real observável em runtime.

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez, ver STATE.md.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `httpctx.Context.route any` + type assertion deveria ser interface tipada (`paramHost`).
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response).
- L-011: cuidado ao desenhar composição de pipeline — traçar ordem de EXECUÇÃO resultante, não só ordem sintática de wrapping, contra ROADMAP.md antes de despachar.
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes.
- `HttpStatus` enum completo não existe ainda.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator, 2 sub-agents por task), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz), AD-008 (pipeline-stage types sem MustInject, execução imediata), L-007 (`git commit -- <arquivos>` sempre com pathspec), L-011 (traçar ordem de execução real de composições de pipeline antes de despachar, não só sintaxe)
- Pra retomar: ler STATE.md inteiro primeiro (tem todo histórico de decisões/lições), depois ROADMAP.md Milestone 3 pra especificar "Pipe" (Specify phase, feature nova) — provável reaproveitamento quase mecânico do padrão de Middleware/Guard/Interceptor.
