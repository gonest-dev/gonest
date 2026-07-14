# Handoff

**Date:** 2026-07-14
**Feature:** Array Builder (Milestone 5) — ✅ COMPLETE (T1-T2)
**Task:** Nenhuma em progresso. Próxima: especificar "Object Builder" (última feature de Milestone 5, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-4 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, exceções+recovery, pipeline completo, Metadata Builder Primitivos (String/Numeric/Boolean/DateTime family). Ver STATE.md pra histórico completo.
- **Milestone 5 (Metadata Builder — Array & Object) em progresso:**
  - **"Array Builder" (T1-T2) — COMPLETE**
    - T1 (`ArrayMetadata` novo, `Array()` no `PropertyBuilder`, `internal/metadata/array.go`) — DONE, evaluator PASS, commit `31a76b3`
    - T2 (re-export raiz `ArrayMetadata`) — DONE, evaluator PASS, commit `1c5ea45`
    - **Ponto novo desta feature (AD-011, STATE.md)**: primeiro tipo DUAL-STATE do projeto — `ArrayMetadata` guarda 2 `*PropertyBuilder` reais simultâneos (campo container + item sintético, este último nunca registrado em `Metadata.properties`). `Items(fn func(m *ArrayMetadata))` é CALLBACK (usuário revisou INSIGHT.md nesta sessão pra resolver ambiguidade de escopo Required/Description campo-vs-item) — reverte PARCIALMENTE AD-002 (só Array/Object, branches primitivos continuam builder linear puro). Dentro do callback, `m.Required()`/etc sempre mutam o campo; `m.String()`/`m.Integer()`/etc sempre configuram o item, reusando `StringMetadata`/`NumericMetadata` já existentes de graça (zero validador novo). `Min`/`Max` do array (quantidade) e `Min`/`Max` do item (bounds) são storages genuinamente separadas, sem colisão possível por construção de tipo.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **"Object Builder"** (última feature de Milestone 5, ver ROADMAP.md): `Object(metadataValue)` reusa metadata registrada (equivalente `$ref`), `Object(func(om *gonest.ObjectMetadata){...})` schema livre/aberto (`AdditionalProperties()`). Provavelmente mesmo padrão dual-state/callback de AD-011 se aplica (a decidir no design.md quando especificar).
- Depois: Milestone 6 (Runtime Validation).

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
- T1 de "Array Builder" entregou 12 testes (alvo era "15+" no tasks.md) — evaluator aceitou como suficiente (todo item do Done-when coberto por teste substantivo), mas registrar caso o padrão de "15+ sugerido, menos entregue" se repita em Object Builder — talvez o número sugerido nas tasks esteja sistematicamente alto.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/` — mas note que `internal/metadata` quebra essa regra deliberadamente pra branches de tipo, que ficam no MESMO pacote que `PropertyBuilder` por precisar de acesso a campos privados), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (`internal/execution` era `httpctx`, `internal/adapter/fiber` era `fiberapp`), AD-011 (Array/Object usam callback com escopo, reversão parcial de AD-002), L-007 (`git commit -- <arquivos>` sempre com pathspec)
- Pra retomar: ler STATE.md inteiro primeiro, depois `.specs/features/array-builder/design.md`'s Tech Decisions (o padrão dual-state já está resolvido e documentado) ANTES de especificar "Object Builder" — bem provável que o mesmo padrão (campo container + estado sintético interno, callback com escopo) reaplique quase mecanicamente, só trocando "item" por "propriedades do objeto aninhado".
