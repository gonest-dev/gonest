# Handoff

**Date:** 2026-07-13
**Feature:** Controller & Route Registration (Milestone 1)
**Task:** T8 done — próxima: T9 (exemplo end-to-end `UserController`, última task da feature)

## Completed ✓

- Feature "Provider & DI Graph" (T1-T11) — COMPLETE, 95 testes, depois rename `MustResolve`→`MustInject` (AD-006)
- Feature "Module Composition" — COMPLETE (já vinha pronta do DI Graph + `Module.Name()`, commit `13aeec0`, 99 testes)
- Feature "Controller & Route Registration" — spec.md + design.md (Fiber v3 confirmado via Context7) + tasks.md prontos (9 tasks)
  - T1 (Fiber dep + `HttpMethod`) — DONE, evaluator PASS, commit `e7c277b`
  - T2 (`Context` shell, sem Fiber) — DONE, evaluator PASS, commit `c767832`
  - T3 (`Pipe` reflect-validado) — DONE, evaluator PASS, commit `3cb48c6`
  - T4 (`defaultCoerce` reflect+strconv) — DONE, evaluator PASS, commit `47afc26`
  - T5 (`Route` builder + integração Pipe/MustParam) — DONE, evaluator PASS-WITH-NOTE, commit `96cbb4c` (ver L-008: `Context.route any` deveria ser interface tipada, débito não-bloqueante)
  - Migração consistência AD-004 (`app.go`→`internal/app`, `param.go` já compliant) — DONE, evaluator PASS, commit `4d2d7c9`
  - T6 (`Controller` estendido: Path/Route/OwnRoutes + stubs Use/Guards/Interceptors/Filters) — DONE, evaluator PASS, commit `4e99255`
  - T7 (`internal/fiberapp` — adapter Fiber v3 real, recover próprio) — DONE, evaluator PASS, commit `53cd63f`
  - T8 (`NewApp[T]` genérico + Stage 2.5: coleta/registro de rota + detecção de colisão) — DONE, evaluator PASS, commit `129d2da` (ver AD-007 em STATE.md: idiom de 2 type param pra resolver T por valor + método pointer-receiver)
- Total: 42+ testes nos pacotes tocados por T8, todos os pacotes verdes, `-race` limpo (12 pacotes)

## In Progress

- Nada em execução agora — sessão pausada por pedido do usuário.

## Pending

- **T9**: Exemplo end-to-end `UserController` do INSIGHT.md via `app.Test` — próxima task, depende de T8 (feito), última da feature
- **T8**: `NewApp[T]` genérico + Stage 2.5 (coleta/registro de rota + detecção de colisão) — `Where` já aponta pra `internal/app` (migração já rodou)
- **T9**: Exemplo end-to-end `UserController` do INSIGHT.md via `app.Test`

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez, ver STATE.md.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `httpctx.Context.route any` + type assertion deveria ser interface tipada (`paramHost`) — barato de trocar agora, mais caro depois.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético, sem prioridade.

## Context

- Branch: `master`
- Uncommitted: `.specs/*` (docs de spec/design/tasks/STATE/ROADMAP editados ao longo da sessão, nunca commitados — só código Go foi commitado via sub-agents). `.vscode/` untracked (não gerado por mim).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator, 2 sub-agents por task), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz), L-007 (`git commit -- <arquivos>` em dispatches paralelos)
- Pra retomar: ler STATE.md inteiro primeiro (tem todo histórico de decisões/lições), depois `.specs/features/controller-route-registration/tasks.md` pra pegar T9 de onde parou.
