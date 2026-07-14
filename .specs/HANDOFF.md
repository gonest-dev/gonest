# Handoff

**Date:** 2026-07-13
**Feature:** Filter (Milestone 3) — ✅ COMPLETE (T1-T5). **Todas as 5 features-tipo de Milestone 3 completas.**
**Task:** Nenhuma em progresso. Próxima: especificar "Pipeline Ordering" (última feature de Milestone 3, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-2 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, contrato de exceção `{name,message,details}` com recovery real.
- **Milestone 3 — todas as 5 features-tipo COMPLETE:**
  - "Middleware" (T1-T5) — COMPLETE.
  - "Guard" (T1-T4) — COMPLETE. AD-008 registrada.
  - "Interceptor" (T1-T4) — COMPLETE. L-011 registrada (erro de ordem Guard/Interceptor no design, corrigido).
  - "Pipe" (housekeeping, não feature nova) — COMPLETE. L-012 registrada (Declare/WithRoute nunca chamados em produção, 2 bugs reais corrigidos).
  - **"Filter" (T1-T5) — COMPLETE**
    - T1 (`internal/filter`: `Filter`/`New`/`Catch` reflect-validado como `Pipe.Handler`, `HandlerFor`) — DONE, evaluator PASS, commit `f0fe0af`
    - T2 (`Controller.Filters` real, ÚLTIMO stub eliminado; placeholder `Middleware struct{}` DELETADO por completo) — DONE, evaluator PASS, commit `dd5ad1b`
    - T3 (`Module.Filters` novo, global só-root) — DONE, evaluator PASS, commit `c65909d`
    - T4 (`filteredHandler` em Stage 2.5 — camada MAIS EXTERNA de toda a chain, envolve middleware→guard→interceptor→handler; recover próprio, controller sobrepõe global, re-panica pro default se nada capturar) — DONE, evaluator PASS, commit `e22e0d8`
    - T5 (re-exports raiz: `Filter`/`NewFilter`, adicionados ao `gonest.go`/`gonest_test.go` consolidados per AD-009) — DONE, evaluator PASS, commit `df9b616`
- **Reorganizações estruturais desta sessão** (fora do fluxo de feature, pedidas pelo usuário):
  - **AD-009 (STATE.md):** pacote raiz consolidado de 13 arquivos pra `gonest.go`/`gonest_test.go` único, commit `be09f7e`. Regra pra daí em diante: novo re-export vai pros arquivos existentes, não arquivo novo.
  - **AD-010 (STATE.md):** `internal/httpctx`→`internal/execution` (tipo continua `Context`), `internal/fiberapp`→`internal/adapter/fiber` (pacote `fiber`, prepara multi-adapter futuro). Commit `f2bdff3` + limpeza de sujeira de índice em `52ce511`.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Pipeline Ordering** (última feature de Milestone 3) — valida a ordem combinada Middleware → Guard → Interceptor → Pipe → Handler com TODOS os 5 estágios existindo AGORA. A ordem de execução JÁ está implementada e testada individualmente em cada feature (Middleware T4, Guard T3, Interceptor T3/correção L-011, Filter T4) — essa feature provavelmente é só um teste de integração final combinando TODOS os estágios numa rota só, não deve exigir código de produção novo (a menos que a checagem revele alguma lacuna). **Atenção especial dado L-011**: traçar a ordem de execução REAL resultante antes de escrever qualquer teste/asserção, não confiar só na ordem sintática de composição.
- Depois de Milestone 3 fechado: começar Milestone 4 (Metadata Builder — Primitivos, ver ROADMAP.md)

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `execution.Context.route any` + type assertion deveria ser interface tipada (`paramHost`).
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response).
- L-011: cuidado ao desenhar composição de pipeline — traçar ordem de EXECUÇÃO resultante, não só ordem sintática de wrapping.
- L-012: tipos com `New(fn)` deferido precisam de chamada real de `Declare()` em produção, não só em teste; testes raiz end-to-end são o único lugar que pega bugs de fiação entre peças.
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes.
- `HttpStatus` enum completo não existe ainda.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents + correções diretas do orquestrador quando achados durante housekeeping, sempre verificadas por evaluator independente depois; docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz — root files são a ÚNICA porta pública já que Go bloqueia import externo de `internal/*`), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (`internal/execution` era `httpctx`, `internal/adapter/fiber` era `fiberapp`), L-007 (`git commit -- <arquivos>` sempre com pathspec), L-011/L-012 (cuidados de composição de pipeline e de fiação Declare/WithRoute)
- Pra retomar: ler STATE.md inteiro primeiro, depois ROADMAP.md Milestone 3 pra especificar "Pipeline Ordering" (Specify phase, feature nova, provavelmente pequena — validação, não construção).
