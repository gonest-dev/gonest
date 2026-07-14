# Handoff

**Date:** 2026-07-14
**Feature:** Param, Query & Custom Validation (Milestone 6, 2ª feature) — ✅ COMPLETE (T0-T4). **Milestone 6 inteiro fechado.**
**Task:** Nenhuma em progresso. Próxima: especificar Milestone 7 (OpenAPI Generation).

## Completed ✓

- **Milestones 1-5 COMPLETE** — ver STATE.md pra histórico completo.
- **Milestone 6 (Runtime Validation) COMPLETE:**
  - **"JSON Body Validation" (T0-T4) — COMPLETE.** `MustJsonBody[T]`, primeiro consumidor real da metadata. AD-012: relocou storage de branch-wrapper pro `PropertyBuilder` compartilhado + campo `kind`.
  - **"Param, Query & Custom Validation" (T0-T4) — COMPLETE.** Cresceu MUITO além do escopo vago original do ROADMAP através de 3 rodadas de `AskUserQuestion` + iteração de metacódigo direto no INSIGHT.md (ver `.specs/features/param-query-validation/context.md` pra trilha completa das 5 decisões). Resultado final (AD-013, STATE.md):
    - **T0 (`8d1aa85`)** — `PropertyBuilder.Custom(fn func(raw any) (any, error))`, escape hatch de transform arbitrário. `MustJsonBody` REFATORADO (já fechado nesta mesma sessão) pra popular `T` via reflect compartilhado (`populate`/`setField`) em vez de `json.Unmarshal` opaco -- necessário pra `Custom(fn)` funcionar. Zero regressão confirmada (suite existente sem NENHUMA mudança de asserção).
    - **T1 (`9b0d22d`)** — `MustParams[T]`, path params whole-object, reusa todo o núcleo de T0.
    - **T2 (`00cda54`)** — `MustQuery[T]` + `Context.Queries()`. Dev achou proativamente um bug real (Fiber's `Queries()` reusa buffer, mesma classe do L-009), evaluator confirmou lendo o source do Fiber, fix aceito.
    - **T3 (`db19cfc`)** — `Pipe` REMOVIDO POR INTEIRO (`internal/pipe`, `Route.Param`/`PipeFor`, `gonest.Pipe`/`NewPipe`), `MustParam[T](ctx,name)` avulso removido. Feature de Milestone 3 (já fechada) retroativamente superseded -- registro histórico mantido no ROADMAP.md (mesmo tratamento de AD-006). Todo call-site migrado, cobertura preservada (evaluator confirmou cada migração).
    - **T4 (`cf6fd3c`/`ea9f72a`)** — root re-export + INSIGHT.md reescrito (3 seções: exemplo mais simples, Middleware/Guard/Interceptor/Filter com exemplo de `Custom(fn)`, seção final settled de Param/Query).

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Milestone 7: OpenAPI Generation** (ver ROADMAP.md): `NewOpenApiDocument`/`SetupSwagger`/geração de schema a partir da Metadata. Primeiro consumidor que lê `KindValue()`/`ItemBuilder()`/`ItemRef()`/`MetadataRef()`/`IsAdditionalProperties()`/`CustomFunc()` (toda a superfície resolvida em AD-012/AD-013) pra fim DIFERENTE de validação -- gerar documento, não validar request. Vale revisar se `CustomFunc()` tem algum jeito de expressar schema OpenAPI (provavelmente não -- é função Go arbitrária, sem forma declarativa; documentar como limitação conhecida antes de especificar).

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- L-008: `execution.Context.route any` + type assertion deveria ser interface tipada (`paramHost`).
- L-010: `Context.Header()`/`SetHeader()` são stores diferentes (request/response).
- L-011: cuidado ao desenhar composição de pipeline — traçar ordem de EXECUÇÃO resultante, não só ordem sintática.
- L-012: tipos com `New(fn)` deferido precisam de chamada real de `Declare()` em produção -- este lesson MORREU junto com Pipe (removido), mas o princípio geral (testar wiring via dispatch HTTP real, não só construção manual) continua valendo e foi reforçado por TODA task desta sessão.
- `gonest.FiberApp`/`gonest.Context`/`gonest.Route`/`gonest.HttpGet` não existem como aliases raiz — gaps pré-existentes, ainda não fechados.
- `HttpStatus` enum completo não existe ainda.
- `gofmt` em 2 arquivos pré-existentes (`internal/resolver/stage3_test.go`, `transient_test.go`) — cosmético, nunca tocado.
- `internal/metadata.Deregister` (test-only, de "JSON Body Validation" T1) sem teste unitário direto próprio.
- `Custom(fn)` chamado até 2x por campo por request (validate + populate) -- documentado como aceitável (fn deve ser idempotente), não cacheado deliberadamente. Se um caso de uso real aparecer com `fn` caro/não-idempotente, revisitar essa decisão (AD-013).
- `internal/app/pipeline_ordering_test.go` teve uma modificação própria migrada de Pipe pra `Custom(fn)` nesta sessão (T3) -- outra modificação NÃO-relacionada (rename `c`→`ctrl`) apareceu como "pre-existing uncommitted" em VÁRIOS relatórios de sub-agent ao longo de toda a sessão, sempre corretamente deixada intocada. Ainda não commitada nem investigada -- decidir o que fazer com ela antes que continue confundindo relatórios futuros.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado (código via sub-agents, docs `.specs/*` via orquestrador, sempre com pathspec explícito, `-m` ANTES de `--`). `.vscode/` untracked (não gerado por mim, deixado como está).
- Fluxo de trabalho: AD-001 (planner→developer→evaluator), AD-004 (1 pacote por tipo em `internal/`), AD-008 (pipeline-stage types sem MustInject), AD-009 (raiz consolidada em `gonest.go`/`gonest_test.go` único), AD-010 (renomes `httpctx`/`fiberapp`), AD-011 (Array/Object usam callback com escopo), AD-012 (storage de branch-wrapper relocado pro `PropertyBuilder` compartilhado + campo `kind`), AD-013 (Pipe removido, `Custom(fn)` substitui, `MustJsonBody` unificado com `MustParams`/`MustQuery`), L-007 (`git commit -m "..." -- <arquivos>`, `-m` SEMPRE antes de `--`)
- **Padrão que se repetiu 2x nesta sessão** (AD-012 e AD-013): ao especificar uma feature nova que LÊ metadata já declarada, é comum descobrir que o storage precisa de ajuste retroativo em features já fechadas -- isso é esperado, não um sinal de que o trabalho anterior estava errado (as features anteriores foram corretamente escopadas pro que precisavam NA HORA; só quando um consumidor real aparece é que o gap fica visível). Ao especificar Milestone 7, esperar a MESMA dinâmica pode se repetir (ex: falta algum getter, ou `Custom(fn)` precisa de uma forma declarativa alternativa pra OpenAPI) -- não é motivo pra hesitar, é motivo pra isolar como T0 com evaluator próprio, igual das duas vezes anteriores.
- Pra retomar: ler STATE.md inteiro primeiro (AD-012 e AD-013 são as decisões mais importantes pra entender o estado atual de `internal/metadata`/`internal/validate`), depois ROADMAP.md Milestone 7 pra especificar "OpenAPI Document Builder".
