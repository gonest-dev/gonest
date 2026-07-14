# Handoff

**Date:** 2026-07-14
**Feature:** Swagger UI Setup (Milestone 7, 3ª e última feature) — ✅ COMPLETE. **Milestone 7 inteiro fechado.**
**Task:** Nenhuma em progresso. Próxima: especificar Milestone 8 (Testing Helpers).

## Completed ✓

- **Milestones 1-6 COMPLETE** — ver STATE.md pra histórico completo (AD-001 até AD-013).
- **Milestone 7 (OpenAPI Generation) COMPLETE:**
  - **"OpenAPI Document Builder"** (commit `9b08afd`) — builder mecânico autocontido.
  - **"Schema Generation from Metadata"** (T0-T3, commits `2ef60d2`→`b27f7b2`) — walker recursivo + geração de schema completa, ver AD-014 em STATE.md.
  - **"Swagger UI Setup"** (commit `22e6c9d`) — `Context.HTML(s)` (infra nova, mesma classe de `Body()`/`Queries()`), `SetupSwagger(app, uiPath, doc, options)` registra 2 rotas DIRETO no `app.Adapter()` (sem passar por Controller/Module/DI -- mecanismo de baixo nível já usado internamente pelo bootstrap), servindo `doc.Document()` (JSON) e uma página HTML do Swagger UI (via CDN `unpkg.com/swagger-ui-dist@5`, sem asset vendored) configurada com `PersistAuth`/`DocExpansion`. `SwaggerOptions` re-exportado na raiz.
    - Nota do dev: sessão teve uma intercorrência de tooling (bloqueio temporário do PATH lookup de `go`), contornada chamando o binário por caminho absoluto -- resolvido, não afetou o resultado final (TDD seguido, gate verde).

## In Progress

- Nada em execução agora.

## Pending

- Especificar **Milestone 8: Testing Helpers** (ver ROADMAP.md, INSIGHT.md's seção "# exemplo de Testing"): `MustNewTestApp(module, overridesFn)`, `TestBuilder`, `MustOverride[Interface](b, mock)` (override de provider por interface -- pré-requisito: dependência tem que ser injetada por interface, não struct concreta, já documentado no próprio INSIGHT.md), `tester.MustRequest`/`AssertStatus`/`AssertJsonPath` (client HTTP de teste). Escopo provavelmente médio -- INSIGHT.md já tem exemplo completo, mas "override de provider" pode exigir mexer no grafo de DI (`internal/resolve`/`internal/module`) pra substituir um provider já resolvido, o que pode ter mais nuance que aparenta.

## Blockers

- Nenhum ativo. B-001 (`-race`/CC=clang) resolvido de vez.

## Débitos leves registrados (não bloqueiam nada)

- Ver HANDOFFs anteriores pra lista completa.
- `gonest.FiberApp`/`gonest.Context`/`gonest.HttpGet` (resto do enum `HttpMethod`) ainda faltam como aliases raiz.
- `internal/app/pipeline_ordering_test.go` ainda tem a modificação NÃO-relacionada (`c`→`ctrl`) não commitada, aparecendo como ruído em relatórios de sub-agent há VÁRIAS sessões -- ainda não resolvida.

## Context

- Branch: `master`
- Todo trabalho desta sessão commitado, pathspec explícito, `-m` antes de `--`.
- Fluxo de trabalho: ver STATE.md (AD-001 até AD-014).
- Pra retomar: ler STATE.md inteiro, depois ROADMAP.md Milestone 8 pra especificar "Test App Bootstrap"/"HTTP Test Client" -- ler a seção "exemplo de Testing" do INSIGHT.md primeiro, já tem bastante coisa concreta especificada.
