# Handoff

**Date:** 2026-07-13
**Feature:** Numeric & Boolean Branches (Milestone 4) — ✅ COMPLETE (T1-T2)
**Task:** Nenhuma em progresso. Próxima: especificar "Date/Time Branches" (última feature de Milestone 4, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-3 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, exceções+recovery, pipeline completo. Ver STATE.md pra histórico completo.
- **Milestone 4 (Metadata Builder — Primitivos) em progresso:**
  - **"Metadata Registration Core" (T1-T2) — COMPLETE.** `internal/metadata` novo pacote. `Property(&t.X)` identifica campo via offset de ponteiro (`unsafe.Pointer`+`reflect.VisibleFields`), confirmado empiricamente sem ajuste. Base comum `Required`/`Nullable`/`Description`/`Examples` pronta.
  - **"String-family Branches" (T1-T2) — COMPLETE**
    - T1 (`PropertyBuilder` estendido + `StringMetadata` novo, 10 branches: `String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`) — DONE, evaluator PASS, commit `604dbc2`
    - T2 (re-export raiz `StringMetadata`) — DONE, evaluator PASS, commit `be6d062`
    - **Padrão arquitetural resolvido nesta feature** (repetido mecanicamente nas próximas): campo `format` fica armazenado no `PropertyBuilder` COMPARTILHADO (não no wrapper descartável cada chamada de branch cria). Os 4 métodos comuns (`Required`/`Nullable`/`Description`/`Examples`) são REDECLARADOS manualmente em cada wrapper — NÃO se pode confiar na promoção automática de método via embedding.
  - **"Numeric & Boolean Branches" (T1-T2) — COMPLETE**
    - T1 (`NumericMetadata` novo, `Integer`/`Int32`/`Float`/`Double` + `Min`/`Max`, mesmo padrão embed+redeclare; `Boolean()` novo) — DONE, evaluator PASS, commit `45e5d22`
    - T2 (re-export raiz `NumericMetadata`) — DONE, evaluator PASS, commit `3b32728`
    - **Ponto novo desta feature**: `Boolean()` é o PRIMEIRO branch sem wrapper próprio — devolve o `*PropertyBuilder` base direto (sem `BooleanMetadata`), já que não tem format nem validador extra. Identidade de ponteiro confirmada por teste (`got == p`). Prova que a decisão original de "Metadata Registration Core" (base `PropertyBuilder` usável sozinho) estava certa.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Date/Time Branches** (`DateTime`/`Date`, última feature de Milestone 4, ver ROADMAP.md). Reusar o padrão já resolvido: provavelmente `DateTimeMetadata` (mesmo pacote `internal/metadata`, novo arquivo `datetime.go`), mecanicamente igual a `StringMetadata`/`NumericMetadata` — avaliar no design.md se `Date`/`DateTime` compartilham um tipo só (igual numeric) ou precisam de validadores diferentes.
- Depois: Milestone 5 (Array & Object — AD-002 já resolveu o formato de builder linear/encadeável, `Items()` variádico)

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `execution.Context.route any` + type assertion deveria ser interface tipada (`paramHost`).
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response).
- L-011: cuidado ao desenhar composição de pipeline — traçar ordem de EXECUÇÃO resultante, não só ordem sintática.
- L-012: tipos com `New(fn)` deferido precisam de chamada real de `Declare()` em produção, não só em teste.
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes.
- `HttpStatus` enum completo não existe ainda.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/` — mas note que `internal/metadata` quebra essa regra deliberadamente pra branches de tipo, que ficam no MESMO pacote que `PropertyBuilder` por precisar de acesso a campos privados), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (`internal/execution` era `httpctx`, `internal/adapter/fiber` era `fiberapp`), L-007 (`git commit -- <arquivos>` sempre com pathspec)
- Pra retomar: ler STATE.md inteiro primeiro, depois ROADMAP.md Milestone 4 pra especificar "Date/Time Branches" (Specify phase, feature nova) — releia `.specs/features/numeric-boolean-branches/design.md`'s Tech Decisions ANTES de especificar, o padrão de "format no objeto compartilhado + redeclaração manual dos 4 comuns" já está resolvido e só precisa ser reaplicado, não re-derivado.
