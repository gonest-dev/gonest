# Handoff

**Date:** 2026-07-13
**Feature:** Panic Recovery & Default Handler (Milestone 2) — ✅ COMPLETE (T1, único task). **Milestone 2 inteiro COMPLETE.**
**Task:** Nenhuma em progresso. Próxima: especificar primeira feature de Milestone 3 (Request Pipeline — `Middleware`, ver ROADMAP.md).

## Completed ✓

- **Milestone 1 (Core DI & Module System) COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen. Ver STATE.md pra histórico completo.
- **Milestone 2 (Exceptions & Response Contract) COMPLETE**
  - Feature "HttpException Core" (T1-T3) — `Exception` interface + `HttpException` core (`internal/exception`, novo pacote), 5 built-ins (`NotFoundException`/`BadRequestException`/`ConflictException`/`UnauthorizedException`/`ForbiddenException`), re-exports raiz. Commits `9e096a6`/`619ea7c`/`aa74048`.
  - Feature "Panic Recovery & Default Handler" (T1, único task) — `internal/fiberapp.RegisterRoute`'s recover branch estendido: type-assert contra `exception.Exception` (interface, não type-switch fechado — detecta built-in OU custom exception igual), responde `ctx.Status(exc.Status()).Json({name,message,details})`; panic não-Exception mantém 500 genérico sem leak (T7 preservado, teste de não-regressão prova isso). Commit `bc3941f`.
- **Milestone 2 COMPLETE.** Ciclo `panic(exception) → resposta HTTP estruturada` fechado — Nest-parity pro contrato de erro `{name,message,details}`.

## In Progress

- Nada em execução agora.

## Pending

- Especificar primeira feature de **Milestone 3: Request Pipeline** — `Middleware` (`NewMiddleware`, `Handler(ctx, next)`, `Use()` por controller/módulo), depois `Guard`/`Interceptor`/`Pipe`/`Filter`/Pipeline Ordering (ver ROADMAP.md — pipeline completo equivalente Nest: Middleware → Guard → Interceptor → Pipe → Handler)

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez, ver STATE.md.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `httpctx.Context.route any` + type assertion deveria ser interface tipada (`paramHost`) — barato de trocar agora, mais caro depois.
- `gonest.FiberApp` não existe como alias raiz (`internal/fiberapp.FiberApp` só) — gap pré-existente, registrado em STATE.md Deferred Ideas.
- `HttpStatus` enum completo não existe ainda — `HttpException Core`/`Panic Recovery` usaram `net/http` stdlib direto por decisão de design. INSIGHT.md usa `gonest.HttpStatusXxx` nomeado em vários exemplos (incluindo o de `Filter`/`Route.HttpCode` de Milestone 3) — considerar se alguma feature de Milestone 3 precisa introduzir isso.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético, sem prioridade.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator, 2 sub-agents por task), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz — root files são a ÚNICA porta pública já que Go bloqueia import externo de `internal/*`), L-007 (`git commit -- <arquivos>` sempre com pathspec)
- Pra retomar: ler STATE.md inteiro primeiro (tem todo histórico de decisões/lições), depois ROADMAP.md Milestone 3 pra especificar "Middleware" (Specify phase, feature nova). `Filter`/`Guard` de Milestone 3 vão reusar `internal/exception.Exception` direto (Guard já panica com `UnauthorizedException` no INSIGHT.md, Filter faz `Catch(&FooExampleError{}, ...)`).
