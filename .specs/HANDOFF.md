# Handoff

**Date:** 2026-07-13
**Feature:** HttpException Core (Milestone 2) — ✅ COMPLETE (T1-T3)
**Task:** Nenhuma em progresso. Próxima: especificar "Panic Recovery & Default Handler" (2ª e última feature de Milestone 2, ver ROADMAP.md).

## Completed ✓

- **Milestone 1 (Core DI & Module System) COMPLETE** — "Provider & DI Graph", "Module Composition", "Controller & Route Registration" (T1-T9), "App Bootstrap & Listen" (T1-T6). Pipeline inteiro provado por teste e2e real (`net/http.Client` contra porta bindada). Ver STATE.md pra histórico completo de decisões/lições dessa milestone.
- **Feature "HttpException Core" (Milestone 2, T1-T3) — COMPLETE**
  - T1 (`Exception` interface + `HttpException` core, novo pacote `internal/exception`) — DONE, evaluator PASS, commit `9e096a6`
  - T2 (5 built-ins: `NotFoundException`/`BadRequestException`/`ConflictException`/`UnauthorizedException`/`ForbiddenException`) — DONE, evaluator PASS, commit `619ea7c`
  - T3 (re-exports raiz: `Exception`/`HttpException`/`NewHttpException`/5 built-ins, `var`-alias direto já que nenhum é genérico) — DONE, evaluator PASS, commit `aa74048` (nota do evaluator: pequena imprecisão no self-report do dev sub-agent — "4 tests" vs 3 reais no arquivo, não bloqueou o PASS pois o requisito era 3+)

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Panic Recovery & Default Handler** (última feature de Milestone 2) — recover global no pipeline de request, `Exception` (built-in ou custom) → status/body `{name,message,details}` mapeado corretamente, panic não-Exception → 500 genérico sem leak (comportamento atual do `internal/fiberapp`'s wrapper, T7 da feature anterior, continua valendo pra panics não-Exception — essa feature nova ENSINA o wrapper a checar `recover()` contra `exception.Exception` antes de cair no 500 genérico)

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez, ver STATE.md.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `httpctx.Context.route any` + type assertion deveria ser interface tipada (`paramHost`) — barato de trocar agora, mais caro depois.
- `gonest.FiberApp` não existe como alias raiz (`internal/fiberapp.FiberApp` só) — gap pré-existente, registrado em STATE.md Deferred Ideas.
- `HttpStatus` enum completo (`HttpStatusOk`, `HttpStatusBadRequest`, etc.) não existe ainda — `HttpException Core` usou `net/http` stdlib direto por decisão de design (ver design.md dessa feature); INSIGHT.md usa `gonest.HttpStatusBadRequest` em alguns exemplos, ainda não implementado. Considerar se "Panic Recovery & Default Handler" precisa disso ou se fica pra depois.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético, sem prioridade.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator, 2 sub-agents por task), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz), L-007 (`git commit -- <arquivos>` sempre com pathspec)
- Pra retomar: ler STATE.md inteiro primeiro (tem todo histórico de decisões/lições), depois ROADMAP.md Milestone 2 pra especificar "Panic Recovery & Default Handler" (Specify phase, feature nova).
