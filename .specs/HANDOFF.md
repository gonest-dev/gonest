# Handoff

**Date:** 2026-07-13
**Feature:** Pipeline Ordering (Milestone 3) — ✅ COMPLETE (T1). **Milestone 3 (Request Pipeline) inteiro COMPLETE.**
**Task:** Nenhuma em progresso. Próxima: especificar primeira feature de Milestone 4 (Metadata Builder — Primitivos: `Metadata Registration Core`, `NewMetadata[T]`/`Property(&t.X)`, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-2 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, contrato de exceção `{name,message,details}` com recovery real.
- **Milestone 3 (Request Pipeline) COMPLETE — todas as 6 features:**
  - "Middleware" (T1-T5) — COMPLETE.
  - "Guard" (T1-T4) — COMPLETE. AD-008 registrada (pipeline-stage types sem MustInject).
  - "Interceptor" (T1-T4) — COMPLETE. L-011 registrada (erro de ordem Guard/Interceptor achado e corrigido).
  - "Pipe" (housekeeping) — COMPLETE. L-012 registrada (Declare/WithRoute nunca chamados em produção, 2 bugs reais corrigidos).
  - "Filter" (T1-T5) — COMPLETE. Último stub de `Controller` (`Middleware struct{}` placeholder) eliminado.
  - **"Pipeline Ordering" (T1) — COMPLETE.** Teste de integração único (`internal/app/pipeline_ordering_test.go`, commit `52cff89`) reproduz o `UserController` completo do INSIGHT.md — Middleware global+controller, Guard, Interceptor, Pipe customizado, Filter, TODOS na mesma rota. Ordem observada: `global-middleware → controller-middleware → guard → interceptor-before → handler (roda Pipe via MustParam) → interceptor-after`, batendo exatamente com ROADMAP.md. **Zero bugs de composição encontrados** — cada peça já garantia sua própria ordem desde a feature que a construiu. Evaluator reproduziu o experimento de inverter a ordem esperada (dev tinha feito isso e revertido) e confirmou independentemente que a asserção falha genuinamente quando deveria — não é uma prova frouxa.
- **Reorganizações estruturais desta sessão** (fora do fluxo de feature, pedidas pelo usuário):
  - **AD-009 (STATE.md):** pacote raiz consolidado de 13 arquivos pra `gonest.go`/`gonest_test.go` único, commit `be09f7e`.
  - **AD-010 (STATE.md):** `internal/httpctx`→`internal/execution`, `internal/fiberapp`→`internal/adapter/fiber`. Commit `f2bdff3` + limpeza `52ce511`.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Metadata Registration Core** (primeira feature de Milestone 4) — `NewMetadata[T]`, `Property(&t.X)`, base comum `Description`/`Required`/`Nullable`/`Examples`. Ver ROADMAP.md Milestone 4 inteiro (Metadata Builder — Primitivos): depois vêm String-family/Numeric/Boolean/Date-Time branches.
- **Nota de escopo importante pra Milestone 4:** ROADMAP.md's INSIGHT.md (`.specs`... não, é `INSIGHT.md` na raiz do repo) já documenta a decisão AD-002 (STATE.md) sobre builder linear vs callback aninhado — reler antes de especificar, já que essa decisão já foi tomada em sessão anterior a esta e não deve ser re-litigada.

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `execution.Context.route any` + type assertion deveria ser interface tipada (`paramHost`).
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response).
- L-011: cuidado ao desenhar composição de pipeline — traçar ordem de EXECUÇÃO resultante, não só ordem sintática de wrapping.
- L-012: tipos com `New(fn)` deferido precisam de chamada real de `Declare()` em produção, não só em teste.
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes.
- `HttpStatus` enum completo não existe ainda — considerar se Milestone 4/5 (Metadata Builder) precisa disso pra representar responses OpenAPI.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents + correções diretas do orquestrador quando achados durante housekeeping, sempre verificadas por evaluator independente depois; docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (`internal/execution` era `httpctx`, `internal/adapter/fiber` era `fiberapp`), L-007 (`git commit -- <arquivos>` sempre com pathspec), L-011/L-012 (cuidados de composição de pipeline e de fiação Declare/WithRoute)
- Pra retomar: ler STATE.md inteiro primeiro, depois ROADMAP.md Milestone 4 pra especificar "Metadata Registration Core" (Specify phase, feature nova — primeira de uma milestone diferente de tudo que veio antes, mais próxima de schema/validação do que DI/HTTP).
