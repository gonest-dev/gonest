# State

Last synced commit: 419e73f (tag v0.37.0)
**Last Updated:** 2026-08-22

## Current Work

**Feature:** Default response description no OpenAPI (`http.StatusText(status)`).

Concluído nesta sessão:
1. Bug real achado pelo usuário via screenshot do Swagger UI (`POST /auth/register`, resposta `201` sem NENHUMA description).
2. Causa raiz: `internal/openapi/generate.go`'s `buildResponses` só usava `http.StatusText(status)` no caminho de ERRO (`defaultErrorResponse`, ≥400 sem schema) — sucesso e o caminho sintetizado (rota sem `Response()`) caíam em `"description": ""` literal.
3. Fix: os 2 pontos que hardcodeavam `""` agora usam `http.StatusText(status)`/`http.StatusText(r.Code())` — `response.Description(...)` explícito continua sobrescrevendo (comportamento inalterado).
4. `.specs/features/response-default-description/{spec,design,tasks}.md` — mesmo pipeline PO/DEV/QA da feature `redirect`.
5. Teste novo `TestGenerate_ResponseDescription_DefaultsToStatusText` (`internal/openapi/generate_test.go`) — 5 casos (sucesso sem schema, sucesso com schema, rota sem `Response()`, erro sem schema, override explícito). Gate: `go test ./...` verde, 25 pacotes.

**Feature anterior (v0.36.0):** Redirect nativo (`Reply.Redirect`/`Route.Redirect`) — ver AD-066.

## Todos

- [x] Bump de versão + tag `v0.37.0` — commit 419e73f, push no gonest
- [x] Atualizar site repo (`C:\dev\github.com\gonest-dev\site`) — `openapi/index.{mdx,pt.mdx,es.mdx}` documentando default description — commit a244787, push
- [ ] Nenhum outro pendente (ver Deferred Ideas para trabalho futuro não priorizado)

## Active Blockers

### B-001: `-race` quebrava por CC=clang injetado no processo shell (2026-07-12) — RESOLVIDO

**Discovered:** T2, no gate check (`clang: error: unsupported option '-mthreads' for target 'x86_64-pc-windows-msvc'`).
**Impact:** bloquearia o Gate de toda task Go daqui em diante.
**Root cause:** processo que spawna cada shell da sessão injeta `CC=clang` (target MSVC).
**Workaround:** MinGW-w64 instalado; Gate command em TESTING.md prefixa `CC=gcc CXX=g++` inline.
**Resolution:** definitivo seria reiniciar a sessão do harness.

## Recent Decisions (Last 15)

### AD-067: OpenAPI — default response description = `http.StatusText(status)` pra todo status (2026-08-22)

**Decision:** `internal/openapi/generate.go`'s `buildResponses` — os 2 pontos que hardcodeavam `"description": ""` (rota sem `Response()` e status sem `Description()` explícito) passam a usar `http.StatusText(status)`/`http.StatusText(r.Code())`. `defaultErrorResponse` (≥400 sem schema) já fazia isso, ficou como referência.
**Reason:** achado real do usuário (screenshot Swagger UI) — resposta `201` sem NENHUMA description. `response.Description(...)` explícito continua sobrescrevendo, sem mudança nesse contrato.
**Trade-off:** nenhum técnico — muda só o VALOR default, não introduz tipo/campo novo.

### AD-066: Redirect nativo — `Reply.Redirect`/`Route.Redirect`, default 302 (2026-08-22)

**Decision:** `Reply.Redirect(url string, status ...int) error` (dinâmico, `internal/execution/reply.go`) e `Route.Redirect(url string, status ...int) *Route` (estático, `internal/route/route.go`, chama `Reply.Redirect` internamente). Default de status `http.StatusFound` (302).
**Reason:** motivado por `.specs/insight/REDIRECT.md` — código consumidor (`erc`) reimplementava manualmente `SetHeader("Location")+Status+Text("")` por rota. Premissa geral do gonest é espelhar ergonomia do NestJS (`@Redirect()` default 302), não o `308` que o código consumidor usava.
**Trade-off:** sem `RedirectException`/redirect disparado de camada interna (usecase/filter) — YAGNI, NestJS também não tem isso.

### AD-065: builtin HttpException — SetMessage padrão + cobertura completa 4xx/5xx (2026-08-22)

**Decision:** todos os construtores `New*Exception` recebem `(message string, details ...any)` com mensagem padrão por tipo (ex: "Resource not found", "Bad request", "Forbidden"). Adicionados 35 novos tipos cobrindo todos os status 4xx e 5xx do `net/http`. `builtin.go` usa helper genérico `newBuiltin[T]`. Aliases e wrappers em `gonest.go` ordenados por status code.
**Reason:** padronização solicitada pelo usuário após implementar o padrão em `NewNotFoundException`. Expansão para cobertura completa de HTTP solicitada na sequência.
**Trade-off:** call-sites internos passando só `details` (sem message) precisaram de `""` como primeiro argumento — 6 arquivos em `internal/validate/` + `internal/app/app.go` + arquivos de teste atualizados. 1 assertiva de teste que esperava `message == ""` atualizada para o novo default.

### AD-064: guard fail-fast contra `MustInject`/`MustInjectAll` chamado dentro de `Constructor` (2026-08-20)

**Decision:** `internal/inject.resolving` (novo `atomic.Bool`) marcado `true` por `internal/resolver/stage3.go`'s `resolveGraph` durante toda a janela de Stage 3 (`defer` desmarca ao sair). `Must[T]`/`MustAll[T]` checam a flag logo no topo — panic imediato nomeando o tipo e explicando causa + fix.
**Reason:** achado REAL do próprio usuário — chamar `MustInject` de DENTRO do `Constructor` sempre foi erro silencioso.
**Trade-off:** nenhum técnico — aditivo, 1 leitura atomic por chamada.

### AD-063: Provider-side `MustInjectAll` — slice pré-alocado + escrita in-place via reflect (2026-08-20)

**Decision:** `gonest.MustInjectAll[T](p)` passa a funcionar de dentro do builder fn de um `gonest.NewProvider`, reusando indireção reflect generalizada para N slots. Novo: `PendingAllEdge`/`findAllRefs`/`mustAllProvider` em `internal/inject/inject.go`; loops aditivos em `BuildGraph`/`invokeAndCopy`. Só Singleton pode ser membro; ordem não garantida; zero matches = slice vazio sem panic.
**Reason:** motivado por `INSIGHT-MUST-INJECT-ALL.md`.

### AD-062: `gonest.Logger` pluggable + `GetLogger`/`GetLoggerFor` (2026-08-18)

**Decision:** `internal/logger` vira instância trocável; `AppOptions.Logger` troca no factory; `gonest.GetLogger`/`GetLoggerFor[T]` funções de pacote (não via DI). Todos os 10 `recover()` do codebase auditados; 8 silenciosos ganharam log estruturado.
**Reason:** achado dogfooding — `MustInject[port.Logger](p)` dentro de Constructor panicava; gonest não tinha logger trocável.

_(Ver `STATE_ARCHIVE.md` para AD-061 até AD-001 completos.)_

### Índice AD-061..AD-048 (texto completo em `STATE_ARCHIVE.md`):

- AD-061: `HttpContext` unifica `(req, res)`; `Response`→`Reply`, `RouteResponse`→`Response` — Milestone 26 (2026-07-25)
- AD-060: `gonest.MustSetupSwagger` (2026-07-24)
- AD-059: banner startup mostra `localhost:PORT` (2026-07-24)
- AD-058: `HttpException.Error()` cai pra JSON de `Details()` quando `Message()` não setado (2026-07-24)
- AD-057: Mensagem de panic de lifecycle hook nomeia provider + assinatura (2026-07-24)
- AD-056: `TokenRef` unifica TODOS os markers de `Module` — Milestone 25, T1 (2026-07-24)
- AD-055: Workflow Conventions formalizadas em PROJECT.md (2026-07-23)
- AD-054: `Module.Lazy`/`gonest.LazyModule` — Milestone 24 (2026-07-23)
- AD-053: `gonest.ProviderAs[T]` — fallback implícito removido (2026-07-23)
- AD-052: `Module.Exports` unificado (`ExportableRef`) (2026-07-22)
- AD-051: `Module.ExportModules` — reexport transitivo (2026-07-21)
- AD-050: `gonest.ProviderRef`/`ControllerRef`/... — reexport marker interfaces (2026-07-21)
- AD-049: `gonest.SchemaFor[T]()` (2026-07-21)
- AD-048: `findFieldByOffset` — 2 bugs reais embed multi-nível (2026-07-21)

## Recent Progress (Last 10)

- [2026-08-22] OpenAPI default response description = http.StatusText(status). Gate: `go test ./...` verde, 25 pacotes. Ver AD-067.
- [2026-08-22] Redirect nativo (Reply.Redirect + Route.Redirect, default 302). Gate: `go test ./...` verde, 25 pacotes. Ver AD-066.
- [2026-08-22] Fix builtin exception panic on missing details (v0.35.1). Gate: `go test ./...` verde, 25 pacotes.
- [2026-08-22] Built-in HTTP exceptions: SetMessage padrão + 35 novos tipos 4xx/5xx (v0.35.0). Gate: `go test ./...` verde, 25 pacotes.
- [2026-08-20] MustInject/MustInjectAll guard fail-fast dentro de Constructor. Gate: `go test ./... -race -count=1` verde, 25 pacotes. Ver AD-064.
- [2026-08-20] Provider-side MustInjectAll (Milestone 27, T1-T10). Gate: `go test ./... -race -count=1` verde, 25 pacotes. Ver AD-063.
- [2026-08-18] Duration Branch feature complete. Gate: `go test ./... -count=1` verde, 25 pacotes.
- [2026-07-23] Milestone 24 (Module Lazy Loading) T1-T8 complete. Gate: `go test ./... -race -count=1` verde, 24 pacotes. Ver AD-054.
- [2026-07-23] Milestone 22 (Provider Interface Export) + Milestone 23 (Thing_ Naming) complete. Gate: `go test ./... -race -count=1` verde. Ver AD-053.
- [2026-07-21] Pós-Milestone 21: `.examples/full-text-search` + Swagger + 2 bugs fix. Gate: verde, 24 pacotes. Ver AD-048.

## Lessons Learned (Last 5)

### L-008: `Context.route any` + assertion — deveria ter sido interface tipada (2026-07-13)

**Prevents:** quando precisar ligar 2 pacotes que já têm import na direção oposta, preferir interface pequena definida no pacote "de baixo" em vez de `any`+assertion.

### L-009: Fiber v3 `Ctx.Params()` devolve view zero-copy sobre buffer reusado — precisa `strings.Clone` (2026-07-13)

**Prevents:** qualquer código em `internal/fiberapp` que leia string de `fiber.Ctx` e repasse pra fora do escopo da request precisa copiar explicitamente.

### L-010: `httpctx.Context.Header()`/`SetHeader()` são stores diferentes (request vs response) (2026-07-13)

**Prevents:** "ler o header que acabei de setar" não é possível hoje via `Context` público.

### L-011: design.md descreveu ordem Guard/Interceptor invertida — achado só na revisão pós-implementação (2026-07-13)

**Prevents:** ao desenhar composição de múltiplos estágios de pipeline, sempre traçar a ordem de EXECUÇÃO resultante e comparar contra a ordem documentada ANTES de mandar pro developer.

### L-012: `Pipe.Declare()` e `ctx.WithRoute()` nunca eram chamados em produção (2026-07-13)

**Prevents:** qualquer tipo com padrão `New(fn)` deferido precisa ter, no fluxo de PRODUÇÃO, uma chamada real a `Declare()`.

## Quick Tasks Completed

_Nenhuma ainda._

## Deferred Ideas

- [ ] Abstração multi-adapter HTTP (net/http, Echo, Gin) — Captured during: definição de escopo v1
- [ ] Emitter/Scheduler/Terminus — Captured during: definição de escopo v1
- [ ] `gonest.FiberApp` como alias raiz — Captured during: T5 de "App Bootstrap & Listen" (2026-07-13)
- [ ] `gonest.Context`/`gonest.Route`/`gonest.HttpGet` e resto como aliases raiz — Captured during: T5 de "Middleware" (2026-07-13)

## Preferences

**Model Guidance Shown:** never

**Commit convention (2026-07-15):** every commit MUST be en-US, Conventional Commits format (`type(scope): summary`). Body explains WHY. Types: `feat`, `fix`, `refactor` (with `!` for breaking), `docs`.

**Subagent workflow convention (2026-07-17):** toda `tasks.md` usa 3 papéis — Planner, Implementer (1 por task), Evaluator (após cada Implementer). Cada papel roda com contexto MÍNIMO.

**Site sync convention (2026-08-22, OBRIGATÓRIO):** toda mudança de API pública dispara atualização do repo irmão `C:\dev\github.com\gonest-dev\site` no MESMO milestone — mínimo `content/docs/api-reference/*.mdx` + `content/docs/core-concepts/*.mdx` relevantes, nos 3 idiomas (en=`.mdx`, pt=`.pt.mdx`, es=`.es.mdx`).

**Versioning convention (2026-07-15/16):** `{major}.{minor}.{release}` sob `v0` fixo. Tag semver 3-segmentos (`v0.6.0`). `major` em BREAKING, `minor` em FEATURE, `release` em FIX/patch.
