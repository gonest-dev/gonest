# Handoff

**Date:** 2026-07-13
**Feature:** App Bootstrap & Listen (Milestone 1) — ✅ COMPLETE (T1-T6). **Milestone 1 inteiro COMPLETE.**
**Task:** Nenhuma em progresso. Próxima: especificar primeira feature de Milestone 2 (Exceptions & Response Contract — `HttpException Core`, ver ROADMAP.md).

## Completed ✓

- Feature "Provider & DI Graph" (T1-T11) — COMPLETE, 95 testes, depois rename `MustResolve`→`MustInject` (AD-006)
- Feature "Module Composition" — COMPLETE (já vinha pronta do DI Graph + `Module.Name()`, commit `13aeec0`, 99 testes)
- Feature "Controller & Route Registration" (T1-T9) — COMPLETE
  - T1-T5: Fiber dep/`HttpMethod`, `Context` shell, `Pipe`, `defaultCoerce`, `Route` builder — commits `e7c277b`/`c767832`/`3cb48c6`/`47afc26`/`96cbb4c`
  - T6-T9: `Controller` estendido, `internal/fiberapp` adapter real, `NewApp[T]` genérico + Stage 2.5, exemplo e2e `UserController` — commits `4e99255`/`53cd63f`/`129d2da`/`c5b77ee`
  - Ver AD-007 (STATE.md): idiom de 2 type param pra `NewApp[T]` resolver "T por valor, método pointer-receiver"
  - Ver L-009 (STATE.md): bug real achado+corrigido em T9 — `fiber.Ctx.Params()` zero-copy view, precisava `strings.Clone`
- Feature "App Bootstrap & Listen" (T1-T6) — COMPLETE
  - T1 (`AppOptions`/`LogLevel`/`OnListen` types) — DONE, evaluator PASS, commit `691c653`
  - T2 (`NewApp`/`MustNewApp` aceitam `AppOptions`, breaking) — DONE, evaluator PASS, commit `e080552`
  - T3 (`HttpAdapter.Listen` + `onListen` hook, Fiber `Hooks().OnListen`) — DONE, evaluator PASS, commit `997e238`
  - T4 (`App.MustListen`) — DONE, evaluator PASS, commit `28778a9`
  - T5 (re-exports raiz: `AppOptions`/`LogLevel`/`OnListen`, promoção automática de `MustListen` via type alias) — DONE, evaluator PASS, commit `1e1b50f`
  - T6 (teste e2e real via `net/http.Client` contra porta bindada de verdade) — DONE, evaluator PASS, commit `667a328`
- **Milestone 1 (Core DI & Module System) COMPLETE.** Todo o pipeline funciona: DI graph paralelo, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen — provado por teste e2e real (não só `app.Test`).

## In Progress

- Nada em execução agora.

## Pending

- Especificar primeira feature de **Milestone 2: Exceptions & Response Contract** — `HttpException`/`NewHttpException`, built-ins (`NotFoundException`, `BadRequestException`, `ConflictException`, `UnauthorizedException`, `ForbiddenException`), depois Panic Recovery & Default Handler (ver ROADMAP.md)

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez, ver STATE.md.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `httpctx.Context.route any` + type assertion deveria ser interface tipada (`paramHost`) — barato de trocar agora, mais caro depois.
- `gonest.FiberApp` não existe como alias raiz (`internal/fiberapp.FiberApp` só) — gap descoberto em T5 de "App Bootstrap & Listen", registrado em STATE.md Deferred Ideas.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético, sem prioridade.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator, 2 sub-agents por task), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz), L-007 (`git commit -- <arquivos>` sempre com pathspec)
- Pra retomar: ler STATE.md inteiro primeiro (tem todo histórico de decisões/lições), depois ROADMAP.md Milestone 2 pra especificar "HttpException Core" (Specify phase, feature nova).
