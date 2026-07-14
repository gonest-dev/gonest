# Handoff

**Date:** 2026-07-13
**Feature:** Metadata Registration Core (Milestone 4) — ✅ COMPLETE (T1-T2)
**Task:** Nenhuma em progresso. Próxima: especificar "String-family Branches" (2ª feature de Milestone 4, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-3 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, exceções+recovery, pipeline completo (Middleware/Guard/Interceptor/Pipe/Filter) com ordem combinada provada. Ver STATE.md pra histórico completo.
- **Milestone 4 (Metadata Builder — Primitivos) iniciado:**
  - **"Metadata Registration Core" (T1-T2) — COMPLETE**
    - T1 (`internal/metadata`: `Metadata`/`PropertyBuilder` core, identificação de campo via offset de ponteiro) — DONE, evaluator PASS, commit `f60415a`. Técnica confirmada EMPIRICAMENTE (não só assumida do design) pros 7 tipos de campo do exemplo `UserEntity` do INSIGHT.md, incluindo os casos de risco `time.Time`/`*time.Time` — funcionou exatamente como desenhado, zero ajuste necessário.
    - T2 (re-exports raiz: `NewMetadata[T]`/`Metadata`/`PropertyBuilder`, adicionados ao `gonest.go`/`gonest_test.go` per AD-009) — DONE, evaluator PASS, commit `11ba9d4`
  - Escopo desta feature foi DELIBERADAMENTE estreito: só a base comum (`Description`/`Required`/`Nullable`/`Examples`), NENHUM branch de tipo+format ainda (`.String()`/`.Integer()`/`.Boolean()`/etc — essas vêm nas próximas 3 features de Milestone 4). `Property(&t.X)` hoje devolve só o `PropertyBuilder` base.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **String-family Branches** (`String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary` + `Min`/`Max`/`Pattern`, ver ROADMAP.md Milestone 4). **Decisão de design pendente, documentada em `.specs/features/metadata-registration-core/design.md`'s Tech Decisions**: cada branch precisa devolver um builder MAIS ESPECÍFICO (com métodos próprios tipo `.Pattern()`) enquanto ainda mantém `Required`/`Nullable`/`Description`/`Examples` encadeáveis — Go não tem overload de método nem "return-type polymorphism" limpo. Abordagem mais provável: cada branch-type (`StringMetadata`, depois `IntegerMetadata` etc) EMBUTE `*PropertyBuilder` e REDECLARA os 4 métodos comuns com retorno próprio (duplicação mecânica, mas simples e sem mágica) — essa decisão específica cabe ao design.md da PRÓXIMA feature, não foi resolvida ainda.
- Depois: Numeric & Boolean Branches, Date/Time Branches (fecham Milestone 4), depois Milestone 5 (Array & Object — AD-002 já resolveu o formato de builder linear pra isso)

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
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/`, reexport fino na raiz), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (`internal/execution` era `httpctx`, `internal/adapter/fiber` era `fiberapp`), L-007 (`git commit -- <arquivos>` sempre com pathspec), L-011/L-012 (cuidados de composição de pipeline e de fiação Declare/WithRoute)
- Pra retomar: ler STATE.md inteiro primeiro, depois ROADMAP.md Milestone 4 pra especificar "String-family Branches" (Specify phase, feature nova) — releia `.specs/features/metadata-registration-core/design.md`'s Tech Decisions ANTES de especificar, já que a decisão de "como Go representa retorno tipado por branch" precisa ser resolvida logo no design dessa próxima feature.
