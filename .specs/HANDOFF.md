# Handoff

**Date:** 2026-07-13
**Feature:** Middleware (Milestone 3) — ✅ COMPLETE (T1-T5)
**Task:** Nenhuma em progresso. Próxima: especificar "Guard" (2ª feature de Milestone 3, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-2 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, contrato de exceção `{name,message,details}` com recovery real. Ver STATE.md pra histórico completo.
- **Feature "Middleware" (Milestone 3, T1-T5) — COMPLETE**
  - T1 (`internal/middleware`: `Next`/`Middleware`/`New`/`Handler`, execução imediata tipo `route.New`) — DONE, evaluator PASS-WITH-NOTE (pequena imprecisão de contagem no self-report do dev, não bloqueante), commit `804f440`
  - T2 [P] (`Controller.Use` real, era stub desde T6; `OwnMiddleware()`) — DONE, evaluator PASS, commit `39b969e`
  - T3 [P] (`Module.Use` novo — só módulo raiz é consultado globalmente; `OwnMiddleware()`) — DONE, evaluator PASS, commit `bb4558f`
  - T4 (composição da chain em Stage 2.5, `internal/app`: global (root) sempre outermost, depois controller, depois Handler; panic de middleware cai no mesmo recover de "Panic Recovery") — DONE, evaluator PASS, commit `d349fa1`. Achou L-010 (STATE.md): `Context.Header()`/`SetHeader()` são stores diferentes (request vs response), técnica de teste sugerida não funcionava, dev adaptou pra 3 técnicas alternativas via dispatch real
  - T5 (re-exports raiz: `Middleware`/`Next`/`NewMiddleware`) — DONE, evaluator PASS, commit `1d70b8c`. Achou mais gaps de re-export raiz (`Context`/`Route`/`HttpGet` também faltam, registrado em STATE.md Deferred Ideas)
- **Feature "Middleware" COMPLETE.** `RequestIdMiddleware` do INSIGHT.md funciona verbatim, per-controller e global.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Guard** (`NewGuard`, retorno bool → 403 automático, panic pra exception custom, ver ROADMAP.md Milestone 3) — provavelmente reusa boa parte do padrão de `Middleware` (execução imediata, sem MustInject por enquanto, a menos que INSIGHT.md's `AuthGuard` exija — ver `gonest.MustInject[*AuthService](guard)` no exemplo, que PODE exigir Guard suportar injeção de verdade, diferente de Middleware — investigar isso na fase de Specify)
- Housekeeping de baixa prioridade (não bloqueia): revisar re-exports raiz faltantes (`FiberApp`, `Context`, `Route`, `HttpMethod`+constantes) — ver STATE.md Deferred Ideas, provavelmente 1 task só quando alguém for mexer na raiz de novo

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez, ver STATE.md.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `httpctx.Context.route any` + type assertion deveria ser interface tipada (`paramHost`) — barato de trocar agora, mais caro depois.
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response) — não dá pra "ler o que acabei de setar" via API pública hoje.
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes, registrados em STATE.md Deferred Ideas.
- `HttpStatus` enum completo não existe ainda — considerar se Guard/Interceptor/Filter precisam disso.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético, sem prioridade.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator, 2 sub-agents por task), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz — root files são a ÚNICA porta pública já que Go bloqueia import externo de `internal/*`), L-007 (`git commit -- <arquivos>` sempre com pathspec, especialmente crítico em tasks `[P]` paralelas — T2/T3 dessa feature rodaram em paralelo sem colisão, confirmado por ambos evaluators)
- Pra retomar: ler STATE.md inteiro primeiro (tem todo histórico de decisões/lições), depois ROADMAP.md Milestone 3 pra especificar "Guard" (Specify phase, feature nova).
