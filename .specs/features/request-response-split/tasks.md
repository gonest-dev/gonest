# Request/Response Split — Tasks

**Spec**: `.specs/features/request-response-split/spec.md`
**Context**: `.specs/features/request-response-split/context.md`
**Status**: Draft

---

## Subagent Roles (ver .specs/project/STATE.md's "Subagent workflow convention")

Esta feature é a primeira a seguir o padrão de 3 papéis de subagente, agora
geral pra toda feature Large/Complex do projeto:

- **Planner** — já rodou (esta sessão) pra produzir este `tasks.md`. Não roda
  de novo pra esta feature a menos que o escopo mude.
- **Implementer** — 1 subagente por task abaixo (ou por grupo `[P]` em
  paralelo). Recebe SÓ a definição da task (não as outras tasks, não o
  histórico da conversa, não avaliações de tasks anteriores).
- **Evaluator** — roda depois de CADA Implementer, antes da task virar
  `completed`. Recebe a definição da task + o diff real (nunca só o relatório
  do Implementer), roda o `Gate` de verdade, confere `Done when` item a item.
  Aprova ou devolve com motivo específico — nunca corrige código ele mesmo.

Todo prompt de Implementer deve incluir: a task inteira (What/Where/Depends
on/Reuses/Done when/Tests/Gate/Commit), `.specs/codebase/CONVENTIONS.md` (se
existir) + `.specs/codebase/TESTING.md` (se existir), e o trecho relevante de
`design.md`/`spec.md` referenciado pela task.

---

## Execution Plan

### Phase 1: Fundação — `Request`/`Response` em `internal/execution` (Sequential)

```
T1 -> T2 -> T3 -> T4
```

### Phase 2: Dispatch (Sequential, depende de T3)

```
T3 -> T5 -> T6
```

### Phase 3: Consumidores do pipeline (Parallel, depende de T5+T6)

```
           ┌→ T7  [P] (guard)
           ├→ T8  [P] (middleware)
T5+T6 ─────┼→ T9  [P] (interceptor)
           ├→ T10 [P] (filter)
           ├→ T11 [P] (route)
           └→ T12 [P] (controller + openapi)
```

### Phase 4: Consolidação de Response (Sequential, depende só de T2 — pode rodar em paralelo com Phase 2/3 se quiser, mas sequenciado aqui por simplicidade)

```
T2 -> T13 -> T14
```

### Phase 5: API pública (Sequential, depende de T7..T12 + T13+T14)

```
T7..T14 -> T15
```

### Phase 6: Migração de testes + exemplos (Sequential)

```
T15 -> T16 -> T17
```

### Phase 7: Gate final

```
T17 -> T18
```

---

## Task Breakdown

### T1: Criar `internal/execution/request.go` com `Request`

**What**: Extrair de `Context` (hoje em `context.go`) tudo que é LEITURA de request pra um novo struct `Request`: `res Responder`, `route any`, `paramsSource/querySource/headersSource Parseable`, `bodySource BodySource`. Métodos: `Method()`, `Path()`, `Header(name)`, `Param(name)`, `Params()`, `Query()`, `Headers()`, `Body() BodySource`, `Route()`, `WithRoute(route any) *Request`, `WithSources(...) *Request`, `FormStream()`. NÃO copiar `Json`/`Status`/`SetHeader`/`HTML`/`SendString` (isso é T2/Response). NÃO apagar `context.go` ainda (T3 faz isso).
**Where**: `internal/execution/request.go` (novo arquivo)
**Depends on**: None
**Reuses**: Todo o código de `Context` já existente em `internal/execution/context.go` (copy+trim, não reescrever do zero) + `Parseable`/`BodySource`/`FormFile` da feature `unified-parse-api` (AD-029), inalterados
**Requirement**: REQRES-01, REQRES-02

**Done when**:
- [ ] `type Request struct` definido com os campos acima
- [ ] Todos os métodos de leitura listados existem em `Request` com o MESMO comportamento observável de `Context` hoje
- [ ] `go build ./internal/execution/...` passa (mesmo com `context.go` antigo ainda presente e agora duplicando símbolos — se colidir nome de método entre `Context`/`Request`, é esperado nesta task; T3 remove `Context`)

**Tests**: nenhum novo ainda (T3 migra os testes de `context_test.go`)
**Gate**: build (`go build ./internal/execution/...`)
**Commit**: `refactor(execution): extract Request from Context (read-side)`

---

### T2: Criar `internal/execution/response.go` com `Response`

**What**: Extrair de `Context` tudo que é ESCRITA de response pra um novo struct `Response`: `req *Request` (referência interna, D2 do spec.md), `res Responder`. Métodos: `Status(code int) *Response` (escreve, chaining), `StatusCode() int` (lê — era `ResponseStatus()`), `SetHeader(name, value string)`, `Json(v any) error`, `Request() *Response` → `*Request` (getter da referência interna). NÃO incluir `Html`/`Text`/remoção de `SendString` ainda — isso é T13/T14 (fica como `HTML(s) error` e `SendString(s) error` nesta task, rename vem depois).
**Where**: `internal/execution/response.go` (novo arquivo)
**Depends on**: T1 (precisa do tipo `Request` já existir pra referenciar)
**Reuses**: Código de `Context` já existente (`Status`, `ResponseStatus`, `SetHeader`, `Json`, `HTML`, `SendString`)
**Requirement**: REQRES-01, REQRES-04, REQRES-08

**Done when**:
- [ ] `type Response struct { req *Request; res execution.Responder }` definido
- [ ] `Status`/`StatusCode`/`SetHeader`/`Json`/`HTML`/`SendString` existem em `Response` com comportamento idêntico ao `Context` de hoje
- [ ] `Response.Request() *Request` retorna a referência guardada
- [ ] `go build ./internal/execution/...` passa

**Tests**: nenhum novo ainda
**Gate**: build
**Commit**: `refactor(execution): extract Response from Context (write-side), holds *Request internally`

---

### T3: `New(res Responder) (*Request, *Response)`; remover `Context`

**What**: Substituir `func New(res Responder) *Context` por `func New(res Responder) (*Request, *Response)` — constrói o `Request` primeiro, depois o `Response` já com esse `Request`. Deletar `internal/execution/context.go` por inteiro (tudo que tinha foi migrado pra T1/T2). Migrar `context_test.go` → `request_test.go` + `response_test.go` (split dos testes na mesma linha do split de tipo).
**Where**: `internal/execution/context.go` (deletar), `internal/execution/request_test.go`/`response_test.go` (novos, a partir de `context_test.go`)
**Depends on**: T1, T2
**Reuses**: Testes existentes de `context_test.go`, só reparticionados por qual tipo (`Request`/`Response`) cada um exercita
**Requirement**: REQRES-01

**Done when**:
- [ ] `context.go` não existe mais
- [ ] `New(res Responder) (*Request, *Response)` é o único construtor
- [ ] Todo teste de `context_test.go` foi migrado (não deletado) pro arquivo correspondente
- [ ] `go test ./internal/execution/...` passa

**Tests**: unit — `request_test.go`/`response_test.go` (migrados de `context_test.go`)
**Gate**: quick (`go test ./internal/execution/...`)
**Commit**: `refactor(execution)!: New returns (*Request, *Response), remove Context`

---

### T4: `BodySource` ganha `Raw()`/`Text()`

**What**: Adicionar `Raw() []byte` e `Text() string` em `BodySource` (`internal/execution`), retornando o corpo cru/como string — NÃO um `Parseable` (bytes crus não têm schema pra validar). Remover `Request.RawBody()` (o método avulso que sobrou da feature `unified-parse-api`) — todo call site de `ctx.RawBody()`/`req.RawBody()` no repo migra pra `req.Body().Raw()`.
**Where**: `internal/execution/request.go` (remove `RawBody`), onde `BodySource` está definido (adiciona `Raw`/`Text`)
**Depends on**: T1
**Reuses**: `BodySource`/`jsonFn`/`formFn` da feature `unified-parse-api` (AD-029), inalterados — só adiciona 2 métodos novos
**Requirement**: REQRES-05

**Done when**:
- [ ] `BodySource.Raw() []byte` e `BodySource.Text() string` existem
- [ ] `Request.RawBody()` não existe mais
- [ ] Todo call site interno de `.RawBody()` (grep no repo) migrado pra `.Body().Raw()`
- [ ] `go build ./...` passa

**Tests**: unit — `TestBodySource_Raw_ReturnsRawBytes`, `TestBodySource_Text_ReturnsStringOfRaw` (novos)
**Gate**: quick (`go test ./internal/execution/...`)
**Commit**: `feat(execution): BodySource.Raw()/.Text(), remove Request.RawBody()`

---

### T5: `internal/app`'s dispatch migra pra `(req, res)`

**What**: `registerRoutes`/`composeHandler`/`gatedHandler`/`interceptedHandler`/`filteredHandler`/`withRoute` (`internal/app/app.go`) trocam `func(ctx *execution.Context)` por `func(req *execution.Request, res *execution.Response)` em toda a cadeia de composição. `ctx.WithRoute(currentRoute)` vira `req.WithRoute(currentRoute)`. `ctx.WithSources(...)` (AD-029) vira `req.WithSources(...)`. Mudança MECÂNICA de assinatura — a LÓGICA de composição (ordem middleware→guard→interceptor→handler, filtro mais externo) não muda.
**Where**: `internal/app/app.go`
**Depends on**: T3
**Reuses**: Toda a lógica de composição já existente, só assinatura muda
**Requirement**: REQRES-01, REQRES-02

**Done when**:
- [ ] Nenhuma função em `app.go` recebe `*execution.Context` — todas recebem `(*execution.Request, *execution.Response)`
- [ ] `go build ./internal/app/...` passa (vai falhar até T7-T12 migrarem os tipos que `app.go` compõe — OK, esta task só cobre `app.go`, os erros de tipo esperados em `guard.Handler`/etc são resolvidos nas tasks seguintes)

**Tests**: nenhum ainda (testes de `internal/app` migram em T16)
**Gate**: nenhum (build vai falhar até T7-T12 — esperado, não é regressão desta task)
**Commit**: `refactor(app)!: dispatch pipeline uses (req, res) instead of ctx`

---

### T6: `internal/adapter/fiber`'s `RegisterRoute` migra pra `(req, res)`

**What**: `RegisterRoute(method, path, h func(ctx *execution.Context))` vira `RegisterRoute(method, path, h func(req *execution.Request, res *execution.Response))`. O wrapper interno (`wrapped := func(c fiber.Ctx) error {...}`) constrói `req, res := execution.New(&fiberResponder{c: c})` em vez de `ctx := execution.New(...)`. O `recover()` que hoje escreve erro via `ctx.Status(...).Json(...)` passa a usar `res.Status(...).Json(...)`.
**Where**: `internal/adapter/fiber/fiber.go`
**Depends on**: T3
**Reuses**: Lógica de recover/dispatch já existente
**Requirement**: REQRES-01

**Done when**:
- [ ] `RegisterRoute`'s assinatura de `h` é `func(req *execution.Request, res *execution.Response)`
- [ ] `go build ./internal/adapter/fiber/...` passa

**Tests**: integration — `fiber_test.go` migrado (rename `ctx` → `req`/`res` nos handlers de teste)
**Gate**: quick (`go test ./internal/adapter/fiber/...`)
**Commit**: `refactor(adapter/fiber)!: RegisterRoute handler receives (req, res)`

---

### T7: `internal/guard` migra pra `(req, res)` [P]

**What**: `type GuardHandler func(ctx *execution.Context) bool` vira `func(req *execution.Request, res *execution.Response) bool`. `Guard.Handler`/`Declare` atualizados.
**Where**: `internal/guard/guard.go`
**Depends on**: T5, T6
**Reuses**: Lógica de avaliação de guard já existente
**Requirement**: REQRES-02

**Done when**:
- [ ] Nenhum símbolo em `internal/guard` referencia `*execution.Context`
- [ ] `go test ./internal/guard/...` passa (testes migrados no mesmo commit — arquivo pequeno o bastante pra não precisar task separada)

**Tests**: unit — `guard_test.go` migrado
**Gate**: quick (`go test ./internal/guard/...`)
**Commit**: `refactor(guard)!: Guard.Handler receives (req, res)`

---

### T8: `internal/middleware` migra pra `(req, res, next)` [P]

**What**: `type Next func(ctx *execution.Context)` vira `func(req *execution.Request, res *execution.Response)`. `type MiddlewareHandler func(ctx *execution.Context, next Next)` vira `func(req *execution.Request, res *execution.Response, next Next)`.
**Where**: `internal/middleware/middleware.go`
**Depends on**: T5, T6
**Reuses**: Lógica de composição de middleware já existente
**Requirement**: REQRES-02, REQRES-03

**Done when**:
- [ ] `Next`/`MiddlewareHandler` usam `(req, res)`/`(req, res, next)`
- [ ] `go test ./internal/middleware/...` passa (testes migrados no mesmo commit)

**Tests**: unit — `middleware_test.go` migrado
**Gate**: quick (`go test ./internal/middleware/...`)
**Commit**: `refactor(middleware)!: Handler/Next receive (req, res)`

---

### T9: `internal/interceptor` migra pra `(req, res, next)` [P]

**What**: Mesma mudança de T8, aplicada ao `Next`/`InterceptorHandler` próprios de `internal/interceptor` (tipo `Next` separado do de middleware, não reusa — AD-009 já documenta isso, mantém assim).
**Where**: `internal/interceptor/interceptor.go`
**Depends on**: T5, T6
**Reuses**: Lógica de before/after já existente
**Requirement**: REQRES-02, REQRES-03

**Done when**:
- [ ] `Next`/`InterceptorHandler` usam `(req, res)`/`(req, res, next)`
- [ ] `go test ./internal/interceptor/...` passa (testes migrados no mesmo commit)

**Tests**: unit — `interceptor_test.go` migrado
**Gate**: quick (`go test ./internal/interceptor/...`)
**Commit**: `refactor(interceptor)!: Handler/Next receive (req, res)`

---

### T10: `internal/filter` migra pra `(req, res, err)` [P]

**What**: `type FilterHandler func(ctx *execution.Context, err any) bool` (ou assinatura equivalente hoje) vira `func(req *execution.Request, res *execution.Response, err any) bool`.
**Where**: `internal/filter/filter.go`
**Depends on**: T5, T6
**Reuses**: Lógica de `Catch`/matching por tipo já existente
**Requirement**: REQRES-02

**Done when**:
- [ ] `FilterHandler` usa `(req, res, err)`
- [ ] `go test ./internal/filter/...` passa (testes migrados no mesmo commit)

**Tests**: unit — `filter_test.go` migrado
**Gate**: quick (`go test ./internal/filter/...`)
**Commit**: `refactor(filter)!: Catch handler receives (req, res, err)`

---

### T11: `internal/route` migra pra `(req, res)` [P]

**What**: `type Handler func(ctx *execution.Context)` vira `func(req *execution.Request, res *execution.Response)`. `Route.Handler`/`HandlerFunc()` atualizados.
**Where**: `internal/route/route.go`
**Depends on**: T5, T6
**Reuses**: Lógica de registro de rota já existente
**Requirement**: REQRES-01

**Done when**:
- [ ] `Route.Handler`'s assinatura é `func(req *execution.Request, res *execution.Response)`
- [ ] `go test ./internal/route/...` passa (testes migrados no mesmo commit)

**Tests**: unit — `route_test.go` migrado
**Gate**: quick (`go test ./internal/route/...`)
**Commit**: `refactor(route)!: Handler receives (req, res)`

---

### T12: `internal/controller` + `internal/openapi` migram pra `(req, res)` [P]

**What**: `internal/controller` — qualquer referência a `*execution.Context` em assinatura pública (se houver — controller majoritariamente só delega pra `route`). `internal/openapi/swagger.go` — `SetupSwagger`'s handler interno (`ctx.HTML(...)`) migra pra `res.Html(...)` (nomenclatura final de T14, ou `res.HTML(...)` se T14 ainda não rodou — usar o nome vigente no momento desta task, checar T2's estado atual).
**Where**: `internal/controller/controller.go`, `internal/openapi/swagger.go`
**Depends on**: T5, T6
**Reuses**: Lógica de registro/serving já existente
**Requirement**: REQRES-01

**Done when**:
- [ ] Nenhum símbolo em `internal/controller`/`internal/openapi` referencia `*execution.Context`
- [ ] `go test ./internal/controller/... ./internal/openapi/...` passa (testes migrados no mesmo commit)

**Tests**: unit — `controller_test.go` migrado
**Gate**: quick (`go test ./internal/controller/... ./internal/openapi/...`)
**Commit**: `refactor(controller,openapi)!: internal handlers receive (req, res)`

---

### T13: `Response.Html`/`.Text` com Content-Type forçado; remove `SendString`

**What**: Renomear `Response.HTML(s) error` → `Response.Html(s) error` (D6). Adicionar `Response.Text(s) error` que seta `Content-Type: text/plain` explicitamente antes de escrever (D7 — comportamento NOVO, não existia em `SendString`). Remover `Response.SendString` por completo.
**Where**: `internal/execution/response.go`
**Depends on**: T2
**Reuses**: `Response.HTML`'s padrão de `Type("html")`+`SendString` já existente, mesma forma pra `Text` com `text/plain`
**Requirement**: REQRES-06, REQRES-07

**Done when**:
- [ ] `Response.Html(s) error` existe (renomeado de `HTML`)
- [ ] `Response.Text(s) error` existe, seta `Content-Type: text/plain`, escreve `s`
- [ ] `Response.SendString` não existe mais
- [ ] `go build ./...` passa (falhas esperadas em call sites de `SendString`/`HTML` até esta task migrar todos internos — ver Done-when seguinte)
- [ ] Todo call site interno de `.SendString(`/`.HTML(` (grep) migrado pra `.Text(`/`.Html(`

**Tests**: unit/integration — `TestResponse_Text_SetsPlainTextContentType` (novo, prova Content-Type via dispatch real); testes existentes de `HTML`/`SendString` migrados
**Gate**: quick (`go test ./internal/execution/...`)
**Commit**: `refactor(execution)!: Response.Html (was HTML), Response.Text (new, forces text/plain), remove SendString`

---

### T14: `Response.Status`/`StatusCode` — confirma nomenclatura final

**What**: Confirmar que `Status(code int) *Response` (escrita, chaining) e `StatusCode() int` (leitura, era `ResponseStatus()`) estão com esses nomes finais em todo o repo (T2 já pode ter feito isso — esta task é o ponto de fechamento/double-check + migração de call sites que ainda usem `ResponseStatus()`).
**Where**: `internal/execution/response.go` + grep de call sites (`internal/app`'s `NewLoggerMiddleware`, se aplicável)
**Depends on**: T2, T13
**Reuses**: Lógica já existente, só nomenclatura
**Requirement**: REQRES-08

**Done when**:
- [ ] `ResponseStatus()` não existe mais como nome — só `StatusCode()`
- [ ] Todo call site interno migrado
- [ ] `go build ./...` passa

**Tests**: nenhum novo — cobertura já existe via T2/T13
**Gate**: build
**Commit**: `refactor(execution): Response.ResponseStatus renamed to StatusCode`

---

### T15: `gonest.go` — API pública final

**What**: `type Request = execution.Request`, `type Response = execution.Response` (novos aliases). `type RestContext = execution.Context` REMOVIDO. `Handler`/`GuardHandler`/`MiddlewareHandler`(`Next`)/`InterceptorHandler`(`Next`)/`FilterHandler` (quaisquer aliases raiz existentes pra esses tipos) atualizados pra `(req *Request, res *Response, ...)`. `RouteXxx`/`Controller.Route`/etc — nenhuma mudança de assinatura própria (já delegam pro tipo `route.Handler`, migrado em T11).
**Where**: `gonest.go`
**Depends on**: T7, T8, T9, T10, T11, T12, T13, T14
**Reuses**: Padrão de type alias já usado pra `Schema`/`Parseable`/etc
**Requirement**: REQRES-01, REQRES-02, REQRES-03, REQRES-04

**Done when**:
- [ ] `gonest.Request`/`gonest.Response` existem como aliases
- [ ] `gonest.RestContext` não existe mais
- [ ] `go build .` passa

**Tests**: nenhum novo — `gonest_test.go` migra em T16
**Gate**: build
**Commit**: `refactor(gonest)!: expose Request/Response, remove RestContext`

---

### T16: Migrar todos os testes internos + `.examples/*`

**What**: Migrar `gonest_test.go` + os 17 arquivos internos já identificados (`internal/app/{app,pipeline_ordering,three_phase_bootstrap}_test.go`, `internal/adapter/fiber/fiber_test.go`, `internal/route/route_test.go`, `internal/guard/guard_test.go`, `internal/interceptor/interceptor_test.go`, `internal/filter/filter_test.go`, `internal/middleware/middleware_test.go`, `internal/controller/controller_test.go`, cada `fakeResponder`/handler de teste) pra `(req, res)`. Migrar `.examples/simple-todo/controller.go` e `.examples/blog-api/module/*/controller.go` (todo `func(ctx *gonest.RestContext)` vira `func(req *gonest.Request, res *gonest.Response)`, `ctx.Json(...)`→`res.Json(...)`, `ctx.Params()`→`req.Params()`, etc).
**Where**: ver lista acima
**Depends on**: T15
**Requirement**: REQRES-01..08

**Done when**:
- [ ] `grep -r "RestContext\|execution\.Context\b" . --include=*.go` retorna vazio (fora de `.specs`/STATE.md)
- [ ] `grep -r "SendString" . --include=*.go` retorna vazio (fora de `.specs`/STATE.md — exceção documentada: fallback genérico de 500 em `fiber.go` usa `fiber.Ctx.SendString` CRU, não `Response.SendString` — não é este símbolo)
- [ ] `go build ./...` passa (repo inteiro, incluindo `.examples/*` via seus próprios `go build ./...`)
- [ ] `go test ./...` passa (23 pacotes, mesma contagem do baseline)

**Tests**: unit + integration (todos os testes existentes)
**Gate**: quick (`go test ./...`)
**Commit**: `refactor: migrate all call sites to (req, res)`

---

### T17: Migrar `INSIGHT-*.md` que referenciam `RestContext`/`ctx`

**What**: Atualizar `INSIGHT-MODULE.md`/`INSIGHT-OPENAPI.md`/`INSIGHT-PARSE.md` (e quaisquer outros `INSIGHT-*.md` vivos) trocando exemplos de `func(ctx *gonest.RestContext)` por `func(req *gonest.Request, res *gonest.Response)`, confirmando que cada snippet ainda compila (mesmo processo de verificação usado em `unified-parse-api`'s T13 — build real num módulo de exemplo, não assumido).
**Where**: `INSIGHT-*.md` na raiz
**Depends on**: T16
**Requirement**: (documentação, sem REQRES próprio)

**Done when**:
- [ ] Todo snippet de `INSIGHT-*.md` usando `ctx`/`RestContext` atualizado
- [ ] Cada snippet confirmado compilando via build real (não assumido)

**Tests**: nenhum (documentação)
**Gate**: manual (build real do snippet)
**Commit**: `docs: update INSIGHT-*.md examples for Request/Response split`

---

### T18: Gate final + STATE.md

**What**: Rodar suite completa, confirmar zero símbolo legado, atualizar `STATE.md` com o AD final desta feature (decisões tomadas DURANTE a execução, se houver SPEC_DEVIATION — mesmo padrão de AD-029 pra `unified-parse-api`). Atualizar `ROADMAP.md`'s Milestone 14 pra COMPLETE. Atualizar `spec.md`'s traceability pra "Verified".
**Where**: raiz, `.specs/project/{STATE,ROADMAP}.md`, `.specs/features/request-response-split/spec.md`
**Depends on**: T17

**Done when**:
- [ ] `go test ./... -race` passa — 23 pacotes, sem falha nova
- [ ] `go build ./...` passa
- [ ] `STATE.md` tem novo AD documentando a execução (+ SPEC_DEVIATIONs, se houver)
- [ ] `ROADMAP.md`'s Milestone 14 → COMPLETE
- [ ] `spec.md`'s traceability table → todo REQRES-0x → Verified

**Tests**: integration (suite completa)
**Gate**: full (`go test ./... -race`)
**Commit**: `chore: finalize request-response-split feature — update STATE, verify gate`

---

## Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Request extraction | 1 tipo + métodos de leitura | ✅ |
| T2: Response extraction | 1 tipo + métodos de escrita | ✅ |
| T3: New() + remove Context | 1 construtor + 1 deleção + split de teste | ✅ |
| T4: BodySource.Raw/Text | 2 métodos + 1 remoção | ✅ |
| T5: internal/app dispatch | 1 arquivo, mudança mecânica | ✅ |
| T6: adapter/fiber dispatch | 1 arquivo, mudança mecânica | ✅ |
| T7-T12: consumidores [P] | 1 arquivo cada, mudança mecânica | ✅ |
| T13: Response.Html/Text/remove SendString | 1 arquivo, 3 mudanças relacionadas | ✅ |
| T14: Status/StatusCode | double-check + call sites | ✅ |
| T15: gonest.go public API | aliases + tipos, 1 arquivo | ✅ |
| T16: migrar testes+exemplos | mecânico, escopo grande mas repetitivo | ✅ |
| T17: INSIGHT-*.md | documentação | ✅ |
| T18: gate final | verificação + docs | ✅ |
