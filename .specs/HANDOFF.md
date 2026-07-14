# Handoff

**Date:** 2026-07-14
**Feature:** OpenAPI Document Builder (Milestone 7, 1ª feature) — ✅ COMPLETE
**Task:** Nenhuma em progresso. Próxima: DISCUTIR (não especificar direto) "Schema Generation from Metadata".

## Completed ✓

- **Milestones 1-6 COMPLETE** — ver STATE.md pra histórico completo (AD-001 até AD-013).
- **Milestone 7 (OpenAPI Generation) em progresso:**
  - **"OpenAPI Document Builder" — COMPLETE** (commit `9b08afd`). `internal/openapi.OpenApiDocument` (re-exportado na raiz via `type OpenApiDocument = openapi.OpenApiDocument` + `var NewOpenApiDocument = openapi.New`) -- builder mecânico, autocontido, ZERO dependência de `internal/metadata`/`internal/route`. `Title`/`Description`/`Version`/`Contact(name,url,email)`/`License(name,url)`/`BearerAuth()`, todos com getter próprio (`TitleText()` etc, convenção já estabelecida). Escopo Medium (sem ambiguidade, INSIGHT.md's bootstrap example já especificava tudo) -- pulei design.md/tasks.md formais, spec.md só.

## In Progress

- Nada em execução agora.

## Pending

- **"Schema Generation from Metadata"** (2ª feature de Milestone 7, ver ROADMAP.md): como Routes linkam pra request/response schemas no documento OpenAPI. SEM exemplo concreto em INSIGHT.md (mesma classe de ambiguidade que "Param/Query Validation" teve no início da sessão -- ver AD-013, cresceu MUITO além do escopo original do ROADMAP através de discussão). NÃO especificar direto -- discutir primeiro (`AskUserQuestion` e/ou pedir pro usuário editar INSIGHT.md com metacódigo, mesmo padrão que funcionou bem pra "Param/Query Validation"). Pontos prováveis de ambiguidade: como uma `*Route` aponta pro `*Metadata` do request body/response; se existe summary/tags/operationId por rota; como Array/Object aninhado (`ItemRef()`/`MetadataRef()`) vira `$ref` no schema OpenAPI de verdade.
- **"Swagger UI Setup"** (3ª feature de Milestone 7) -- depende de "Schema Generation" existir, mesmo então provavelmente builder simples (`SetupSwagger(app, path, doc, options)`).

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- Ver HANDOFF anterior (git log, `.specs/features/param-query-validation/`) pra lista completa -- nada mudou nesta feature pequena.
- `internal/app/pipeline_ordering_test.go` ainda tem a modificação NÃO-relacionada (`c`→`ctrl`) não commitada, aparecendo como ruído em relatórios de sub-agent há várias sessões -- ainda não resolvida.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado, pathspec explícito, `-m` antes de `--`.
- Fluxo de trabalho: ver STATE.md (AD-001 até AD-013). Nada novo registrado nesta feature pequena (builder mecânico, sem decisão não-óbvia).
- Pra retomar: ler STATE.md, depois **discutir** (não especificar direto) "Schema Generation from Metadata" com o usuário -- via `AskUserQuestion` e/ou pedindo pra editar INSIGHT.md com metacódigo, mesmo padrão que revelou o escopo real de "Param/Query Validation".
