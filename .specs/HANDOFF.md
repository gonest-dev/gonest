# Handoff

**Date:** 2026-07-14
**Feature:** Schema Generation from Metadata (Milestone 7, 2ª feature) — ✅ COMPLETE (T0-T3)
**Task:** Nenhuma em progresso. Próxima: especificar "Swagger UI Setup" (última feature de Milestone 7).

## Completed ✓

- **Milestones 1-6 COMPLETE** — ver STATE.md pra histórico completo (AD-001 até AD-013).
- **Milestone 7 (OpenAPI Generation) em progresso:**
  - **"OpenAPI Document Builder" — COMPLETE** (commit `9b08afd`). Builder mecânico autocontido.
  - **"Schema Generation from Metadata" — COMPLETE** (T0-T3, commits `2ef60d2`→`b27f7b2`). Mesmo padrão de discussão de "Param/Query Validation" (usuário edita/responde direto no INSIGHT.md) -- ver AD-014 em STATE.md.
    - **T0 (`2ef60d2`)**: `App.Root()` -- gap descoberto durante pesquisa (Module/Controller já retinham a árvore rica de objetos pós-bootstrap, mas `App` não expunha ponto de entrada nenhum).
    - **T1 (`66aef7b`)**: `Metadata.Title`, `Controller.Tags`/`BearerAuth`, `Route`'s 10 métodos de documentação -- mapeamento direto de `@nestjs/swagger`'s `@Api*` decorators.
    - **T2 (`325155c`/`7d4f915`) — núcleo real**: `internal/openapi.Generate` walker recursivo + `schemaFor` (cobre TODAS as branch families) + `Document()`. 2 achados do evaluator corrigidos: gap de teste (nullable `$ref`/`anyOf` sem cobertura) fechado em follow-up; uma SPEC_DEVIATION do dev era FALSA (alegou INSIGHT.md desatualizado, evaluator provou que não era, era só escolha estilística do fixture de teste) -- corrigida no registro.
    - **T3 (`0d20a9c`/`b27f7b2`)**: `gonest.GenerateOpenApiSchema(app, doc)` re-exportado + fechou débito ANTIGO -- `gonest.Route` nunca tinha sido re-exportado na raiz (aparecia na lista de débitos do HANDOFF há várias sessões), achado e corrigido de brinde pelo dev sub-agent.

## In Progress

- Nada em execução agora.

## Pending

- **"Swagger UI Setup"** (última feature de Milestone 7, ver ROADMAP.md): `SetupSwagger(app, path, doc, options)` -- serve o documento gerado (`doc.Document()`, já `json.Marshal`-ável) via HTTP + UI do Swagger. Deve ser MENOR/mais mecânica que as 2 anteriores -- toda a geração já existe, só falta servir. Provavelmente: 1 rota GET servindo o JSON, 1 rota (ou mais) servindo HTML/JS estático do Swagger UI (CDN embutido ou vendored), `SwaggerOptions{JsonDocumentUrl, PersistAuth, DocExpansion}` (INSIGHT.md's bootstrap example já mostra o shape completo).
- Depois: Milestone 8 (Testing Helpers).

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- Ver HANDOFFs anteriores (git log, `.specs/features/*/`) pra lista completa.
- `gonest.FiberApp`/`gonest.Context`/`gonest.HttpGet` (resto do enum `HttpMethod`) ainda faltam como aliases raiz -- `gonest.Route` foi fechado nesta sessão (T3), os outros continuam.
- `internal/app/pipeline_ordering_test.go` ainda tem a modificação NÃO-relacionada (`c`→`ctrl`) não commitada, aparecendo como ruído em relatórios de sub-agent há VÁRIAS sessões agora -- ainda não resolvida, decidir o que fazer (commitar/descartar/investigar) antes que continue confundindo relatórios futuros.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado, pathspec explícito, `-m` antes de `--`.
- Fluxo de trabalho: ver STATE.md (AD-001 até AD-014).
- Pra retomar: ler STATE.md inteiro (AD-014 é a decisão mais recente), depois ROADMAP.md Milestone 7 pra especificar "Swagger UI Setup" -- escopo pequeno, provavelmente não precisa de discussão extensa (INSIGHT.md já mostra o call shape completo de `SetupSwagger`/`SwaggerOptions`).
