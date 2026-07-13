# Handoff

**Date:** 2026-07-13
**Feature:** Pipe (Milestone 3) — ✅ COMPLETE (housekeeping + 2 bug fixes reais, não feature nova)
**Task:** Nenhuma em progresso. Próxima: especificar "Filter" (5ª e última feature-tipo de Milestone 3 antes de "Pipeline Ordering", ver ROADMAP.md).

## Completed ✓

- **Milestones 1-2 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, contrato de exceção `{name,message,details}` com recovery real.
- **Milestone 3 em progresso:**
  - Feature "Middleware" (T1-T5) — COMPLETE.
  - Feature "Guard" (T1-T4) — COMPLETE. AD-008 registrada.
  - Feature "Interceptor" (T1-T4) — COMPLETE. L-011 registrada (erro de ordem Guard/Interceptor no design, corrigido).
  - **"Pipe" (Milestone 3) — COMPLETE.** Descoberta: `internal/pipe` já existia desde T3 de "Controller & Route Registration" — só faltava re-export raiz. Ao adicionar, achei e corrigi 2 bugs reais de integração nunca antes testados end-to-end:
    - `b305e70` — `Route.Param` não chamava `Pipe.Declare()`, Handler do Pipe customizado nunca era registrado em produção
    - `2d3e0c3` — `ctx.WithRoute()` nunca era chamado em `internal/app`'s `registerRoutes`, `MustParam` sempre caía no `defaultCoerce` genérico, ignorando qualquer Pipe customizado
    - `a153ba8` — `pipe.go`/`pipe_test.go` na raiz, incluindo o teste que expôs os 2 bugs acima
    - Verificação independente (evaluator): PASS, confirmou ambas correções + checou criticamente se os testes provam mesmo a correção (sim — subteste "caminho inválido → 400 estruturado" é a prova inequívoca, já que só o Pipe customizado produz Exception estruturada)
  - **L-012 registrada (STATE.md):** padrão a vigiar — testes que só validam peça isolada (chamando `Declare()`/`WithRoute()` manualmente) nunca pegam bug de fiação entre peças; só teste que sobe app inteira + dispara request HTTP real prova conexão de verdade. Testes raiz não são redundantes com testes de `internal/*` por causa disso.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Filter** (`NewFilter`, `Catch(exceptionType, handler)`, registro por controller/módulo/global — último stub de `Controller` ainda no placeholder `Middleware struct{}`, ver ROADMAP.md Milestone 3)
- Depois: "Pipeline Ordering" (valida ordem combinada Middleware → Guard → Interceptor → Pipe → Handler com TODOS os 5 estágios existindo — **atenção especial dado L-011**: traçar ordem de execução real antes de despachar)
- Ao especificar Filter, considerar se precisa de composição em Stage 2.5 (`internal/app`) igual Middleware/Guard/Interceptor, ou se plugga em cima do recover wrapper já existente (`internal/fiberapp`) de forma diferente — Filter intercepta EXCEPTIONS específicas, não decora o request/response como os outros, pode ter formato de integração distinto

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `httpctx.Context.route any` + type assertion deveria ser interface tipada (`paramHost`).
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response).
- L-011: cuidado ao desenhar composição de pipeline — traçar ordem de EXECUÇÃO resultante, não só ordem sintática de wrapping.
- L-012: tipos com `New(fn)` deferido precisam de chamada real de `Declare()` em produção, não só em teste; testes raiz end-to-end são o único lugar que pega bugs de fiação entre peças.
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes.
- `HttpStatus` enum completo não existe ainda.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents + correções diretas do orquestrador quando achados durante housekeeping, sempre verificadas por evaluator independente depois; docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz — root files são a ÚNICA porta pública já que Go bloqueia import externo de `internal/*`, arquivo `package gonest` fisicamente TEM que estar na raiz do módulo), AD-008 (pipeline-stage types sem MustInject), L-007 (`git commit -- <arquivos>` sempre com pathspec), L-011/L-012 (cuidados de composição de pipeline e de fiação Declare/WithRoute)
- Pra retomar: ler STATE.md inteiro primeiro, depois ROADMAP.md Milestone 3 pra especificar "Filter" (Specify phase, feature nova).
