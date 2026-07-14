# Handoff

**Date:** 2026-07-14
**Feature:** JSON Body Validation (Milestone 6) — ✅ COMPLETE (T0-T4)
**Task:** Nenhuma em progresso. Próxima: especificar "Param/Query Validation" (última feature de Milestone 6, ver ROADMAP.md).

## Completed ✓

- **Milestones 1-5 COMPLETE** — DI graph, módulos, controllers/rotas, adapter Fiber real, bootstrap+listen, exceções+recovery, pipeline completo, Metadata Builder (Primitivos + Array & Object). Ver STATE.md pra histórico completo.
- **Milestone 6 (Runtime Validation) em progresso:**
  - **"JSON Body Validation" (T0-T4) — COMPLETE.** Primeira feature que LÊ metadata de verdade, não só constrói.
    - **T0 (commit `d012c7e`) — prerequisito bloqueante descoberto durante o design**: `Min`/`Max`/`Pattern`/`item`/`ref`/etc relocados dos wrappers descartáveis (`StringMetadata`/`NumericMetadata`/`ArrayMetadata`/`ObjectMetadata`) pro `PropertyBuilder` compartilhado, + campo `kind` novo corrigindo colisão `String()`/`Boolean()` (ambos `format==""`). Zero mudança de API pública, 4 features já fechadas continuam passando SEM tocar teste nenhum. SPEC_DEVIATION verificado são: `ArrayMetadata.item` manteve campo próprio (snapshot) em vez de deletar, necessário pra "Array() chamado 2x produz item independente" continuar funcionando -- evaluator confirmou com teste próprio que o path de leitura real (sem wrapper nenhum) funciona. Ver AD-012 em STATE.md.
    - **T1 (commits `74c31bc`/`ebe3f2e`)**: registro global `internal/metadata/registry.go`, auto-populado por `NewMetadata[T]`, panic em declaração duplicada. SPEC_DEVIATION: precisou de `Deregister` test-only (7 arquivos de teste existentes reusam o mesmo tipo Go entre `Test*` functions, colidiam com o panic novo) -- escopo confirmado limpo pelo evaluator.
    - **T2 (commit `5207af2`)**: `Context.Body() []byte` (raw request body), sem cópia defensiva (diferente do fix de L-009 -- `json.Unmarshal` consome sincronamente, não retém o slice).
    - **T3 (commits `25ab1e3`/`36924bf`) — núcleo da feature**: `internal/validate.MustJsonBody[T]`, validador recursivo de verdade (Array valida item+quantidade, Object recursa via `ref`), COLETA todas violações numa request (não fail-fast, decisão explícita do usuário -- ver `context.md` da feature). Evaluator provou rigorosamente o collect-all (não só aceitou a alegação). Um gap achado (código de "rejeita fractional em campo Integer" sem teste) fechado em follow-up.
    - **T4 (commit `a9bbda9`)**: `gonest.MustJsonBody[T]` re-exportado, reprodução completa do `UserEntity` do INSIGHT.md via dispatch HTTP real (caso feliz + caso com violação de array E de object aninhado na MESMA request).

## In Progress

- Nada em execução agora.

## Pending

- Especificar **"Param/Query Validation"** (última feature de Milestone 6, ver ROADMAP.md): `MustParam[T]` integra `Pipe` + coerção via metadata. Reusa toda a leitura de `PropertyBuilder` já resolvida em AD-012 (`KindValue`/`MinValue`/`MaxValue`/`PatternValue` etc) -- provavelmente NÃO precisa de outro prerequisito de storage, já que isso foi resolvido de vez nesta feature.
- Depois: Milestone 7 (OpenAPI Generation) -- primeiro consumidor que vai ler `KindValue()`/`ItemBuilder()`/`MetadataRef()` pra fim diferente de validação (gerar schema), vale revisar se a superfície de leitura do `PropertyBuilder` já é suficiente antes de especificar.

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `execution.Context.route any` + type assertion deveria ser interface tipada (`paramHost`).
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response).
- L-011: cuidado ao desenhar composição de pipeline — traçar ordem de EXECUÇÃO resultante, não só ordem sintática.
- L-012: tipos com `New(fn)` deferido precisam de chamada real de `Declare()` em produção, não só em teste (reforçado nesta sessão: T2/T3 de JSON Body Validation usaram dispatch HTTP real por causa dessa lição).
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes. `MustJsonBody[T]` (T4) confirmou de novo que `gonest.Context` falta -- usa `*execution.Context` direto, mesmo padrão de `MustParam`.
- `HttpStatus` enum completo não existe ainda.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético.
- `internal/metadata.Deregister` (T1) é test-only, sem teste unitário DIRETO próprio (só testado indiretamente via 7 call-sites de cleanup) — baixa prioridade, considerar se mexer em `registry.go` de novo.
- `internal/app/pipeline_ordering_test.go` tem uma mudança local não-commitada (rename `c`→`ctrl`) que apareceu em VÁRIOS relatórios de sub-agent nesta sessão (todos corretamente deixaram intocada, fora de escopo) -- não é minha, provavelmente de uma sessão anterior. Vale decidir: commitar, descartar, ou investigar de onde veio, antes que continue aparecendo em todo relatório de sub-agent futuro como ruído.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito, `-m` ANTES de `--` depois que descobrimos que a ordem inversa quebra neste shell). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/`), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (renomes `httpctx`/`fiberapp`), AD-011 (Array/Object usam callback com escopo), AD-012 (storage de branch-wrapper relocado pro `PropertyBuilder` compartilhado + campo `kind`), L-007 (`git commit -m "..." -- <arquivos>`, `-m` SEMPRE antes de `--` neste shell)
- Pra retomar: ler STATE.md inteiro primeiro (AD-012 é a decisão mais importante pra entender o estado atual de `internal/metadata`), depois ROADMAP.md Milestone 6 pra especificar "Param/Query Validation" (Specify phase, feature nova, escopo bem menor que JSON Body Validation já que o prerequisito de storage já está resolvido).
