# Handoff

**Date:** 2026-07-14
**Feature:** Object Builder (Milestone 5) — ✅ COMPLETE (T1-T2). Milestone 5 inteiro fechado.
**Task:** Nenhuma em progresso. Próxima: especificar Milestone 6 (Runtime Validation).

## Completed ✓

- **Milestones 1-4 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, exceções+recovery, pipeline completo, Metadata Builder Primitivos. Ver STATE.md pra histórico completo.
- **Milestone 5 (Metadata Builder — Array & Object) COMPLETE:**
  - **"Array Builder" (T1-T2) — COMPLETE**
    - `ArrayMetadata` (`internal/metadata/array.go`, commits `31a76b3`/`1c5ea45`) — builder DUAL-STATE: 2 `*PropertyBuilder` reais (campo container + item sintético, nunca registrado em `Metadata.properties`). `Items(fn func(m *ArrayMetadata))` é CALLBACK (AD-011, STATE.md) -- resolve ambiguidade real: `m.Required()`/`Description()` sempre mutam o campo, `m.String()`/`m.Integer()`/etc sempre configuram o item (reusa `StringMetadata`/`NumericMetadata` já existentes de graça). `Min`/`Max` do array (quantidade) e do item (bounds) são storages separadas, sem colisão por construção de tipo.
  - **"Object Builder" (T1-T2) — COMPLETE**
    - `ObjectMetadata` (`internal/metadata/object.go`, commits `6a602fb`/`1e734ba`) — builder SINGLE-STATE: ao contrário de Array, NÃO tem synthetic separado — o campo inteiro É o objeto, sem conceito de "item" distinto. `Object(fn func(om *ObjectMetadata))` também é callback (consistência de API com `Items`), mas SEM ambiguidade real pra resolver: `Required`/`Nullable`/`Description`/`Examples` chamados dentro OU fora do callback produzem resultado idêntico (mesmo `*PropertyBuilder`, testado explicitamente). `om.Metadata(ref)` reusa `*Metadata` já registrada (`$ref`); `om.AdditionalProperties()` marca schema aberto/livre.
    - **Milestone 5 inteiro fechado** — Metadata Builder (Primitivos + Array & Object) completo.

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Milestone 6: Runtime Validation** (ver ROADMAP.md): "JSON Body Validation" (`MustJsonBody[T]` valida contra `NewMetadata[T]`, panic `BadRequestException` com details por campo) e "Param/Query Validation" (`MustParam[T]` integra `Pipe` + coerção via metadata). Primeira vez que a metadata registrada (Milestones 4-5) é LIDA/USADA de verdade, não só armazenada -- toda a introspecção (`FormatValue()`, `OwnProperties()`, `ItemBuilder()`/`ItemRef()`, `MetadataRef()`/`IsAdditionalProperties()`) construída até aqui vira consumida pela primeira vez.

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
- Sub-agents de Object Builder (T1 e T2) tiveram erro de sintaxe com `git commit -- <paths> -m "..."` (ordem de flags mal-parseada nesse shell) — os dois contornaram corretamente (`git commit -m "..." -- <paths>` ou `git add` explícito + `git status` de confirmação antes do commit), evaluators confirmaram escopo limpo nos dois casos. Vale ajustar o texto padrão do dispatch de sub-agent pra já sugerir `git commit -m "..." -- <paths>` (ordem que funciona) em vez de `git commit -- <paths> -m "..."`.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/` — mas note que `internal/metadata` quebra essa regra deliberadamente pra branches de tipo), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (`internal/execution` era `httpctx`, `internal/adapter/fiber` era `fiberapp`), AD-011 (Array/Object usam callback com escopo; Object mostrou que callback nem sempre resolve ambiguidade real — às vezes é só consistência de API), L-007 (`git commit -- <arquivos>` sempre com pathspec, mas ver nota acima sobre ordem de flags)
- Pra retomar: ler STATE.md inteiro primeiro, depois ROADMAP.md Milestone 6 pra especificar "JSON Body Validation" (Specify phase, feature nova) — esta é a primeira feature que CONSOME a metadata (Milestones 4-5), não só constrói -- vale revisar toda a superfície de leitura já disponível (`OwnProperties`, `FormatValue`, `IsRequired`, `ItemBuilder`/`ItemRef`, `MetadataRef`/`IsAdditionalProperties`) antes de desenhar o validador, pra não duplicar acesso.
