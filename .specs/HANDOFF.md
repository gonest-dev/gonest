# Handoff

**Date:** 2026-07-13
**Feature:** String-family Branches (Milestone 4) — ✅ COMPLETE (T1-T2)
**Task:** Nenhuma em progresso. Próxima: especificar "Numeric & Boolean Branches" (3ª feature de Milestone 4, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-3 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, exceções+recovery, pipeline completo. Ver STATE.md pra histórico completo.
- **Milestone 4 (Metadata Builder — Primitivos) em progresso:**
  - **"Metadata Registration Core" (T1-T2) — COMPLETE.** `internal/metadata` novo pacote. `Property(&t.X)` identifica campo via offset de ponteiro (`unsafe.Pointer`+`reflect.VisibleFields`), confirmado empiricamente sem ajuste. Base comum `Required`/`Nullable`/`Description`/`Examples` pronta.
  - **"String-family Branches" (T1-T2) — COMPLETE**
    - T1 (`PropertyBuilder` estendido + `StringMetadata` novo, 10 branches: `String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`) — DONE, evaluator PASS (com ênfase máxima no teste crítico de encadeamento), commit `604dbc2`
    - T2 (re-export raiz `StringMetadata`) — DONE, evaluator PASS, commit `be6d062`
    - **Padrão arquitetural resolvido nesta feature** (vai se repetir mecanicamente nas próximas 2): campo `format` fica armazenado no `PropertyBuilder` COMPARTILHADO (não no wrapper `StringMetadata` descartável cada chamada de branch cria) — isso é o que garante que a escolha de branch sobrevive mesmo que o dev descarte o wrapper específico sem usar. Os 4 métodos comuns (`Required`/`Nullable`/`Description`/`Examples`) são REDECLARADOS manualmente em `StringMetadata` (delegam pro objeto embutido, devolvem `*StringMetadata`) — NÃO se pode confiar na promoção automática de método via embedding, que devolveria o tipo BASE (`*PropertyBuilder`) e quebraria a chain fluente.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Numeric & Boolean Branches** (`Integer`/`Int32`/`Float`/`Double` + `Min`/`Max`, `Boolean`, ver ROADMAP.md Milestone 4). **Reusar o padrão já resolvido**: provavelmente `NumericMetadata` (mesmo pacote `internal/metadata`, novo arquivo `numeric.go`) servindo `Integer`/`Int32`/`Float`/`Double` (todos compartilham `Min`/`Max` numérico, igual String-family compartilhou `Min`/`Max`/`Pattern`), e talvez um tipo separado ou nenhum extra pra `Boolean` (sem validador extra documentado no INSIGHT.md além dos 4 comuns — se não precisar de validador próprio, `Boolean()` pode simplesmente devolver o `*PropertyBuilder` base direto, sem precisar de wrapper novo — avaliar isso no design.md dessa próxima feature).
- Depois: Date/Time Branches (`DateTime`/`Date`) fecha Milestone 4, depois Milestone 5 (Array & Object — AD-002 já resolveu o formato de builder linear)

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
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/` — mas note que `internal/metadata` quebra essa regra deliberadamente pra `StringMetadata`, que fica no MESMO pacote que `PropertyBuilder` por precisar de acesso a campos privados; ver design.md da feature String-family Branches), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (`internal/execution` era `httpctx`, `internal/adapter/fiber` era `fiberapp`), L-007 (`git commit -- <arquivos>` sempre com pathspec)
- Pra retomar: ler STATE.md inteiro primeiro, depois ROADMAP.md Milestone 4 pra especificar "Numeric & Boolean Branches" (Specify phase, feature nova) — releia `.specs/features/string-family-branches/design.md`'s Tech Decisions ANTES de especificar, o padrão de "format no objeto compartilhado + redeclaração manual dos 4 comuns" já está resolvido e só precisa ser reaplicado, não re-derivado.
