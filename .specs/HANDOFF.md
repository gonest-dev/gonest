# Handoff

**Date:** 2026-07-14
**Feature:** Date/Time Branches (Milestone 4) — ✅ COMPLETE (T1). Milestone 4 inteiro fechado.
**Task:** Nenhuma em progresso. Próxima: especificar Milestone 5 (Array & Object).

## Completed ✓

- **Milestones 1-3 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, exceções+recovery, pipeline completo. Ver STATE.md pra histórico completo.
- **Milestone 4 (Metadata Builder — Primitivos) COMPLETE:**
  - **"Metadata Registration Core" (T1-T2) — COMPLETE.** `internal/metadata` novo pacote. `Property(&t.X)` identifica campo via offset de ponteiro (`unsafe.Pointer`+`reflect.VisibleFields`), confirmado empiricamente sem ajuste. Base comum `Required`/`Nullable`/`Description`/`Examples` pronta.
  - **"String-family Branches" (T1-T2) — COMPLETE**
    - `PropertyBuilder` estendido + `StringMetadata` novo, 10 branches: `String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`; re-export raiz. Commits `604dbc2`/`be6d062`.
    - **Padrão arquitetural resolvido nesta feature** (repetido mecanicamente nas próximas): campo `format` fica armazenado no `PropertyBuilder` COMPARTILHADO (não no wrapper descartável). Os 4 métodos comuns REDECLARADOS manualmente em cada wrapper — NÃO se pode confiar na promoção automática de método via embedding.
  - **"Numeric & Boolean Branches" (T1-T2) — COMPLETE**
    - `NumericMetadata` novo, `Integer`/`Int32`/`Float`/`Double` + `Min`/`Max`, mesmo padrão embed+redeclare; `Boolean()` novo. Commits `45e5d22`/`3b32728`.
    - **Ponto novo**: `Boolean()` é o PRIMEIRO branch sem wrapper próprio — devolve `*PropertyBuilder` base direto, identidade de ponteiro confirmada por teste.
  - **"Date/Time Branches" (T1) — COMPLETE**
    - `DateTime()` (`format = "date-time"`) e `Date()` (`format = "date"`) em `internal/metadata/metadata.go`, mesmo padrão sem-wrapper do `Boolean()` — nenhum validador extra pra família date/time. Commit `558e587`.
    - Reproduz `UserEntity.CreatedAt`/`UpdatedAt` (`DateTime().Required()...`) e `DeletedAt` (`DateTime().Nullable()...`, `Examples(nil, time.Now())`) do INSIGHT.md verbatim, mais 1 caso `Date()` extra.
    - **Última feature de Milestone 4** — todo o "Metadata Builder — Primitivos" está fechado.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Milestone 5: Metadata Builder — Array & Object** (`Array()`/`Object()`/`Items()`, ver ROADMAP.md). AD-002 (STATE.md) já resolveu o formato: builder linear/encadeável, `Items()` variádico (zero-arg = branch primitivo, um-arg = `*MetadataDefinition` reusado tipo `$ref`).

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
- Pra retomar: ler STATE.md inteiro primeiro, depois ROADMAP.md Milestone 5 pra especificar "Array & Object" (Specify phase, feature nova) — releia AD-002 em STATE.md ANTES de especificar, decisão de builder linear/`Items()` variádico já está tomada, só precisa virar spec+design+tasks.
