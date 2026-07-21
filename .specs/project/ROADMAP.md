# Roadmap

**Current Milestone:** 21 (Enum Branches) -- COMPLETE
**Status:** Milestones 1-21 COMPLETE

---

## Milestone 1: Core DI & Module System

**Goal:** app sobe, resolve grafo de DI em paralelo (errgroup), registra module/controller/route e responde um GET simples — sem middleware/validação ainda.
**Target:** primeiro `go run` funcional com `UserModule` de exemplo do INSIGHT.md respondendo `/user/:id`.

### Features

**Provider & DI Graph** - COMPLETE
- `NewProvider`, `Scope` (Singleton), `Constructor`
- `MustResolve[T]` com placeholder + copy-in-place
- Resolução paralela via `errgroup` para providers sem dependência entre si; espera (`group.Wait()`) para dependentes

**Module Composition** - COMPLETE
- `NewModule`, `Imports`, `Providers`, `Controllers`
- Ordem topológica do import graph

**Controller & Route Registration** - COMPLETE
- `NewController`, `Path`, `Route`, `Handler`
- `HttpCode`, `Context.Json`, `MustParam[T]`
- Adapter Fiber v3 real (`internal/fiberapp`), `NewApp[T HttpAdapter]` genérico + Stage 2.5 (registro/colisão de rota)

**App Bootstrap & Listen** - COMPLETE
- `NewApp`/`MustNewApp` com `AppOptions{BufferLogs, LogLevels}` (config capturada, inerte — sem Logger real ainda)
- `MustListen`/`OnListen` real via `HttpAdapter.Listen` + Fiber `Hooks().OnListen` (callback roda no bind, antes do bloqueio)
- Milestone 1 **COMPLETE** — primeiro `go run` funcional provado via teste e2e com `net/http.Client` real

---

## Milestone 2: Exceptions & Response Contract

**Goal:** contrato de erro consistente `{name, message, details}`; panic não-Exception nunca vaza detalhe interno (500 genérico).

### Features

**HttpException Core** - COMPLETE
- `HttpException`, `NewHttpException` (`internal/exception`, re-exportado na raiz)
- Built-ins: `NotFoundException`, `BadRequestException`, `ConflictException`, `UnauthorizedException`, `ForbiddenException`
- `Exception` interface (satisfação estrutural via embedding) — base pra "Panic Recovery & Default Handler" detectar exceptions

**Panic Recovery & Default Handler** - COMPLETE
- Recover em `internal/fiberapp.RegisterRoute` detecta `exception.Exception` via type-assertion na interface (não type-switch fechado) — `Exception` (built-in ou custom via embedding) → status/body `{name,message,details}`; panic não-Exception → 500 genérico sem leak (comportamento T7 preservado, não regressão)
- Milestone 2 **COMPLETE**

---

## Milestone 3: Request Pipeline

**Goal:** pipeline completo equivalente Nest (Middleware → Guard → Interceptor → Pipe → Handler), aplicável por controller ou globalmente.

### Features

**Middleware** - COMPLETE
- `NewMiddleware`, `Handler(ctx, next)` (`internal/middleware`, re-exportado na raiz)
- `Controller.Use()` (real, era stub desde T6) + `Module.Use()` (novo, global — só o módulo raiz é consultado)
- Composição da chain em Stage 2.5 (`internal/app`): global (root) sempre outermost, depois controller, depois Handler; panic de middleware cai no mesmo recover de "Panic Recovery & Default Handler"

**Guard** - COMPLETE
- `NewGuard`, `Handler(ctx) bool` (`internal/guard`, re-exportado na raiz) — execução imediata, sem MustInject (decisão explícita: Guard pode ser anexado a múltiplos controllers/módulos, sem owner único claro)
- `Controller.Guards()` (real, era stub) — retorno `false` → 403 automático (`ForbiddenException`); panic com `Exception` custom → resposta dessa exception; múltiplos guards avaliados em ordem, short-circuit no primeiro `false`
- Composição em Stage 2.5: Middleware → Guard → Handler (guard fica dentro do wrap de middleware, sem alterar a lógica de composição de middleware já existente)

**Interceptor** - COMPLETE
- `NewInterceptor`, `Handler(ctx, next)` (`internal/interceptor`, re-exportado na raiz) — mesmo padrão AD-008 (sem MustInject, execução imediata), tipo `Next` próprio (não reusa `middleware.Next`)
- `Controller.Interceptors()` (real, era stub) — envolve o Handler puro com before/after; Guard fica MAIS EXTERNO que Interceptor na composição (ordem Middleware → Guard → Interceptor → Handler, corrigida durante revisão — ver L-011 em STATE.md)

**Pipe** - ~~COMPLETE~~ REMOVIDO (2026-07-14, ver "Param/Query Validation" em Milestone 6 / AD-013 em STATE.md)
- `NewPipe`, transforma/valida param antes do handler, panic `BadRequestException` se inválido — implementação já existia desde "Controller & Route Registration" T3 (`internal/pipe`), só faltava re-export raiz
- Corrigidos 2 bugs reais de integração achados ao adicionar o primeiro teste end-to-end via dispatch real: `Route.Param` não chamava `Pipe.Declare()`, e `ctx.WithRoute()` nunca era chamado em produção — Pipe customizado nunca funcionava fora de teste isolado. Ver L-012 em STATE.md.
- **REMOVIDO por inteiro** (`internal/pipe`, `Route.Param`/`PipeFor`, `gonest.Pipe`/`NewPipe`) quando "Param/Query Validation" (Milestone 6) generalizou `MustParam[T](ctx,name)` avulso pra `MustParams[T]`/`MustQuery[T]` whole-object -- intenção original de Pipe (transform customizado) absorvida por `PropertyBuilder.Custom(fn)`, dentro da própria declaração de Metadata. Registro histórico mantido aqui (mesmo tratamento de rename anterior, AD-006) -- código de fato não existe mais no repo a partir do commit `db19cfc`.

**Filter** - COMPLETE
- `NewFilter`, `Catch(exemplar, handler)` (`internal/filter`, re-exportado na raiz, reflect-validado como Pipe) — execução imediata, sem MustInject (AD-008)
- `Controller.Filters()` (real, era o último stub) + `Module.Filters()` (novo, global só-root) — captura seletiva de `exception.Exception` por tipo concreto, controller sobrepõe global, não-capturado cai no default `{name,message,details}` já existente
- `filteredHandler` em Stage 2.5: camada mais externa de toda a chain (envolve middleware→guard→interceptor→handler), recover próprio que re-panica se nenhum Filter capturar

**Pipeline Ordering** - COMPLETE
- Teste de integração único reproduz o `UserController` completo do INSIGHT.md (Middleware global+controller, Guard, Interceptor, Pipe customizado, Filter, todos na mesma rota) — ordem observada bate exatamente com o documentado: `global-middleware → controller-middleware → guard → interceptor-before → handler (roda Pipe via MustParam) → interceptor-after`
- Nenhum bug de composição encontrado — cada peça já garantia sua própria ordem corretamente desde a feature que a construiu; esta feature só provou o cenário combinado nunca antes testado

**Milestone 3 (Request Pipeline) COMPLETE.**

---

## Milestone 4: Metadata Builder — Primitivos

**Goal:** `NewMetadata`/`Property` cobre todos os branches flat tipo+format do OpenAPI 3.1 com validadores comuns.

### Features

**Metadata Registration Core** - COMPLETE
- `NewMetadata[T]`, `Property(&t.X)`, base comum: `Description`/`Required`/`Nullable`/`Examples` (`internal/metadata`, re-exportado na raiz)
- Identificação de campo via offset de ponteiro (`unsafe.Pointer`/`reflect.VisibleFields`) — confirmada empiricamente pros 7 tipos de campo do exemplo `UserEntity` do INSIGHT.md, incluindo `time.Time`/`*time.Time`, sem ajuste necessário
- Branches de tipo+format (`String`/`Integer`/`Boolean`/etc) ficam pra próximas features desta milestone — `Property(&t.X)` hoje só devolve o builder base comum

**String-family Branches** - COMPLETE
- `String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary` + `Min`/`Max`/`Pattern` (`internal/metadata/string.go`, re-exportado na raiz)
- Padrão de builder específico-por-branch resolvido: `format` fica no `PropertyBuilder` COMPARTILHADO (não no wrapper `StringMetadata` descartável), `Required`/`Nullable`/`Description`/`Examples` REDECLARADOS manualmente em `StringMetadata` (não confiar em promoção automática de embedding, que quebraria a chain devolvendo o tipo base) — padrão mecânico que as próximas features (Numeric/Boolean, Date/Time) vão repetir

**Numeric & Boolean Branches** - COMPLETE
- `Integer`/`Int32`/`Float`/`Double` + `Min`/`Max` (`NumericMetadata`, mesmo padrão embed+redeclare de `StringMetadata`), `Boolean` (`internal/metadata/numeric.go`, re-exportado na raiz)
- `Boolean()` é o primeiro branch sem wrapper próprio — devolve o `*PropertyBuilder` base direto (identidade de ponteiro confirmada por teste), já que não tem format nem validador extra

**Date/Time Branches** - COMPLETE
- `DateTime`/`Date`, ambos sem wrapper próprio (mesmo padrão de `Boolean()` — devolvem `*PropertyBuilder` base direto, `internal/metadata/metadata.go`, re-exportado na raiz)

---

## Milestone 5: Metadata Builder — Array & Object

**Goal:** estruturas aninhadas reusam metadata já registrada sem duplicar `Property`; builder linear/encadeável (decisão registrada no INSIGHT.md).

### Features

**Array Builder** - COMPLETE
- `Array()` (`internal/metadata/array.go`, re-exportado na raiz) devolve `*ArrayMetadata`, builder DUAL-STATE: `Items(fn func(m *ArrayMetadata))` (callback, não variádico -- revisão de INSIGHT.md nesta sessão) roteia `String`/`Integer`/etc pro ITEM sintético (reusa `StringMetadata`/`NumericMetadata` de graça) e `Required`/`Nullable`/`Description`/`Examples` sempre pro CAMPO container
- `Min`/`Max` do `ArrayMetadata` direto (fora do callback, ex: `Items(fn).Min(1)`) = quantidade do array; `Min`/`Max` do item vive no wrapper devolvido dentro do callback (`m.String().Min(1).Max(50)`) -- duas storages separadas, sem colisão

**Object Builder** - COMPLETE
- `Object(fn func(om *ObjectMetadata))` (`internal/metadata/object.go`, re-exportado na raiz) -- SEMPRE callback, mas SINGLE-STATE (sem synthetic separado como `ArrayMetadata` -- campo INTEIRO é o objeto, sem conceito de item)
- `om.Metadata(ref)` reusa `*Metadata` já registrada (equivalente `$ref`); `om.AdditionalProperties()` marca schema aberto/livre
- `Required`/`Nullable`/`Description`/`Examples` funcionam idênticos chamados dentro OU fora do callback (mesmo `*PropertyBuilder`, sem ambiguidade de escopo)

---

## Milestone 6: Runtime Validation

**Goal:** mesma declaração de metadata valida request em runtime, rejeitando payload inválido com detalhe de campo.

### Features

**JSON Body Validation** - COMPLETE
- `MustJsonBody[T]` (`internal/validate`, re-exportado na raiz) valida contra `NewMetadata[T]` (via registro global auto-populado), panic `BadRequestException` com lista de `{field, message}` -- COLETA todas violações (não fail-fast), recursivo (Array valida item+quantidade, Object recursa via `ref`)
- Prerequisito descoberto e resolvido nesta feature: storage de `Min`/`Max`/`Pattern`/`item`/`ref`/etc relocado dos wrappers descartáveis (`StringMetadata` etc) pro `PropertyBuilder` compartilhado, + campo `kind` novo (corrige colisão pré-existente `String()`/`Boolean()` ambos com `format==""`) -- ver AD-012 em STATE.md

**Param/Query Validation** - COMPLETE
- `MustParams[T]`/`MustQuery[T]` (`internal/validate`, re-exportados na raiz) -- whole-object, mesma forma de `MustJsonBody`, validam path/query params contra `NewMetadata[T]`, coletam todas violações
- `PropertyBuilder.Custom(fn func(raw any) (any, error))` -- escape hatch de transform arbitrário, absorve a intenção original de `Pipe` (removido, ver acima)
- `MustJsonBody` refatorado pra popular `T` via reflect campo-a-campo (mecanismo compartilhado com `MustParams`/`MustQuery`), não mais `json.Unmarshal` direto -- necessário pra `Custom(fn)` funcionar uniforme nos 3 pontos de entrada

---

## Milestone 7: OpenAPI Generation

**Goal:** `NewOpenAPI` + `SetupSwagger` geram doc OpenAPI 3.1 a partir da mesma metadata, servido em path configurável.

### Features

**OpenAPI Document Builder** - COMPLETE
- `NewOpenAPI`/`OpenAPI` (`internal/openapi`, re-exportado na raiz), `Title`/`Description`/`Version`/`Contact`/`License`/`BearerAuth` -- builder mecânico, sem dependência de `internal/metadata`/`internal/route`

**Schema Generation from Metadata** - COMPLETE
- `App.Root()` (novo, expõe árvore de módulo pós-bootstrap) + `Metadata.Title` + `Controller.Tags`/`BearerAuth` + `Route`'s 10 métodos de documentação (`Summary`/`Description`/`OperationId`/`Tags`/`BearerAuth`/`RequestBody`/`Response`/`PathParams`/`QueryParams`/`ExcludeFromDocs`/`Deprecated`) -- mapeamento direto de `@nestjs/swagger`'s `@Api*` decorators pra builder methods
- `internal/openapi.Generate(doc, root)` -- walker recursivo (módulos + controllers + rotas), gera `paths`+`components.schemas` a partir de TODA a superfície de leitura de `PropertyBuilder` resolvida em AD-012/AD-013 (`KindValue`/`MinValue`/`MaxValue`/`PatternValue`/`ItemBuilder`/`ItemRef`/`MetadataRef`/`IsAdditionalProperties`), dedup por identidade de ponteiro (`$ref` reuso), nullable `$ref` via `anyOf`
- `gonest.GenerateOpenApiSchema(app, doc)` re-exportado na raiz; fechou débito antigo -- `gonest.Route` alias raiz nunca tinha sido adicionado

**Swagger UI Setup** - COMPLETE
- `Context.HTML(s)` (novo, infra de resposta raw) + `SetupSwagger(app, uiPath, doc, options)` -- registra 2 rotas direto no `app.Adapter()` (sem DI/Module), 1 servindo `doc.Document()` JSON, 1 servindo HTML do Swagger UI (via CDN, sem asset vendored) configurado com `PersistAuth`/`DocExpansion`
- `SwaggerOptions{JsonDocumentUrl, PersistAuth, DocExpansion}` re-exportado na raiz -- fecha Milestone 7 inteiro

---

## Milestone 8: Testing Helpers

**Goal:** DX equivalente `@nestjs/testing` — override de provider por interface, teste via request HTTP ou unitário direto.

### Features

**Test App Bootstrap** - COMPLETE
- Bootstrap de `NewApp` reordenado em 3 fases (Provider → Controller → Middleware/Guard/Interceptor/Filter) -- AD-008 revertido, os 4 tipos de pipeline-stage agora deferem `New(fn)` via `Declare(scope)`, igual Provider/Controller já faziam
- `MustInject[T]`/`MustInjectAll[T]` ganham suporte a interface (exact-match + `Implements()` fallback), resolução DIRETA (sem placeholder) a partir de Controller/Middleware/Guard/Interceptor/Filter -- Provider-a-Provider continua via placeholder+PendingEdge, inalterado
- `MustNewTestApp`, `TestBuilder`, `MustOverride[T]` (`internal/app/testapp.go`, reusa o mesmo bootstrap de 3 fases via `resolver.ResolveWithOverrides`) -- reproduz INSIGHT.md's `TestUserController_Get`/`TestUserService_Get_NotFound` verbatim
- Ver `.specs/features/test-app-bootstrap/tasks.md` pra SPEC_DEVIATIONs encontrados na execução (owner de `MustInject`/`MustInjectAll` widened pra `any`; `module.MiddlewareRef`/`FilterRef` novo pra quebrar ciclo de import; `TestApp` mora em `internal/app`, não `internal/testapp`)

**HTTP Test Client** - COMPLETE
- `HttpAdapter` ganha `Test(req *http.Request) (*http.Response, error)` (Fiber delega pra `*fiber.App.Test`); `TestApp.MustRequest`/`TestResponse.AssertStatus`/`AssertJsonPath` (`internal/app/testresponse.go`, re-exportados na raiz)
- Root ganhou `HttpMethod`/`HttpGet`/`HttpPost`/`HttpPut`/`HttpDelete`/`HttpQuery` (débito antigo fechado -- INSIGHT.md's exemplos já assumiam essas aliases)
- Fecha Milestone 8 inteiro -- reproduz `TestUserController_Get` do INSIGHT.md verbatim

---

## Milestone 9: Emitter (event-emitter)

**Goal:** equivalente `@nestjs/event-emitter` -- evento tipado por struct, emissão assíncrona fire-and-forget, listener registrado via `Module.Listeners()`.
**Status:** COMPLETE (2026-07-15, commit `1e08298`)

### Features

**Emitter & Listener** - COMPLETE
- `gonest.Emitter` -- singleton do framework, SEMPRE disponível via `MustInject[*gonest.Emitter]` em qualquer módulo, sem registro explícito -- novo mecanismo genérico `internal/inject.RegisterGlobalSingleton`/`GlobalSingletonFor`, checado ANTES de `directResolver`/placeholder em `MustInject`
- `NewListener`, `MustOn[EventType](listener, handler)` (função livre, não método -- Go não permite parâmetro de tipo em método, L-001 em STATE.md) -- `Listener` segue o padrão de ownership de módulo único do `Controller` (registrado via `Module.Listeners`, declarado na fase 2)
- `Module.Listeners(...)` -- registro no bootstrap junto com providers (`module.ListenerRef` marker interface novo, mesmo padrão de `ProviderRef`/`ControllerRef`/`MiddlewareRef`/`FilterRef`)
- `Emitter.Emit(event)` -- assíncrono (1 goroutine por listener registrado pro tipo), fire-and-forget, panic/erro de listener nunca propaga pro chamador (cai só no logger interno, hoje só `recover()` silencioso -- sem Logger real ainda)
- Reproduz o exemplo `UserCreatedEvent`/`UserCreatedListener`/`UserProvider` do INSIGHT.md verbatim

---

## Milestone 10: Scheduler (Cron/Interval/Timeout)

**Goal:** equivalente `@nestjs/schedule` -- jobs agendados, cada execução isolada (recover próprio, não derruba o processo).
**Status:** COMPLETE (2026-07-15, commit `34cb536`)

### Features

**Scheduler** - COMPLETE
- `NewScheduler`, `Module.Schedulers(...)` -- mesmo padrão de ownership de módulo único do Controller/Listener
- `scheduler.Cron(name, expr, fn)` (via novo dependency `github.com/robfig/cron/v3` pro parsing de expressão padrão de 5 campos) / `scheduler.Interval(name, dur, fn)` / `scheduler.Timeout(name, dur, fn)`
- Cada execução roda isolada (recover próprio, goroutine independente), panic/erro nunca derruba o processo nem impede execução seguinte
- Reproduz `CleanupScheduler` do INSIGHT.md verbatim (durações em milissegundos no teste)

---

## Milestone 11: Terminus/health checks

**Goal:** equivalente `@nestjs/terminus` -- probes `/readyz` (readiness) e `/livez` (liveness), estilo Kubernetes.
**Status:** COMPLETE (2026-07-15, commit `5c2fde4`)

### Features

**Health Check** - COMPLETE
- `HealthController` é um `gonest.NewController` comum, SEM tipo novo de bootstrap -- `MustInjectAll[Connectable](controller)` resolve todo `Connectable` registrado (ex: Db, Redis), `/readyz` pinga todos e agrega status, `/livez` responde 200 estático via `Context.SendString`
- `Context.SendString` (novo, `internal/execution`, `Responder` ganhou o método) + `gonest.HttpStatusOk`/`HttpStatusServiceUnavailable` (constantes raiz -- INSIGHT.md já assumia que existiam em mais de um exemplo) -- ambos de uso geral, não específicos de Terminus
- Reproduz `HealthController`/`AppModule` do INSIGHT.md verbatim via dispatch HTTP real

---

## Milestone 12: Multipart Form Streaming

**Goal:** `gonest.ParseRestFormBody`/`MustParseRestFormBody` -- multipart/form-data com upload de arquivo, streaming de verdade (sem bufferizar o arquivo inteiro localmente antes de repassar pra storage tipo S3), confirmado tecnicamente viável com as dependências já vendorizadas (`gofiber/fiber/v3@v3.4.0`+`valyala/fasthttp@v1.72.0`, ver spec.md's Problem Statement).
**Status:** COMPLETE (2026-07-15, T1-T6 -- ver `.specs/features/multipart-form-streaming/tasks.md`)

### Features

**Multipart Form Streaming** - COMPLETE
- `AppOptions.EnableFormStreaming` -- liga `fiber.Config.StreamRequestBody`+`DisablePreParseMultipartForm` juntos (as 2 sempre, nunca uma sem a outra) na construção do adapter -- exigiu `HttpAdapter.Init()` virar `Init(opts AppOptions)` (AD-022) e `AppOptions` ser extraído pro pacote-folha novo `internal/appoptions` (resolve ciclo de import com `internal/adapter/fiber`)
- `Responder.BodyStream()`/`Context.FormStream()` (novo) -- acesso ao corpo cru da request como stream + boundary multipart
- `internal/validate.FormFile`/`ParseFormBody`/`MustFormBody` (novo, `form.go`) -- caminha o stream multipart uma única vez, despachando cada parte-arquivo pro callback `onFile` SINCRONAMENTE (streaming de verdade) e cada campo normal pra validação via nova tag `form:"..."` (mesmo mecanismo de `param`/`query`)
- `gonest.ParseRestFormBody`/`MustParseRestFormBody`/`FormFile` (root, par Parse/Must igual AD-021)
- Prova real de streaming: `TestParseRestFormBody_RealHTTPDispatch_StreamsFileWithoutFullBuffering` (`gonest_test.go`) -- dial TCP real (não `app.Test`, que bufferiza a request inteira antes do fasthttp rodar, descoberto durante a execução) com arquivo dividido num `io.Pipe` gated, provando que `onFile` dispara ANTES da segunda metade (ainda represada) chegar
- `Route.FormBody` (T7, pós-dogfooding) -- documenta `multipart/form-data` no OpenAPI (schema inline com `file` como `type: string, format: binary`), revertendo o "Out of Scope" original assim que o usuário reparou que a rota de upload aparecia no Swagger sem jeito de anexar arquivo (AD-023)

---

## Milestone 13: Unified Parse API

**Goal:** Substituir os 8 pares `MustParseRest*`/`ParseRest*` por dois entry points genéricos unificados (`Parse[T]`/`MustParse[T]`) que recebem um `Parseable` — opaque value retornado por `ctx.Params()`, `ctx.Query()`, `ctx.Headers()`, `ctx.Body().Json()`, `ctx.Body().Form(onFile)`. Breaking change aceita: remoção imediata dos legados.
**Status:** COMPLETE (2026-07-17, T1-T13 -- suite inteira `go test ./... -race` verde, 23 pacotes, `.examples/*` migrados)

### Features

**Unified Parse API** - COMPLETE
- `gonest.Parse[T](src Parseable, schema *Schema) (T, error)` + `gonest.MustParse[T](src Parseable, schema *Schema) T`
- `ctx.Body() BodySource` (novo) / `ctx.RawBody() []byte` (era `ctx.Body() []byte`) / `ctx.Body().Json() Parseable` / `ctx.Body().Form(onFile) Parseable`
- `ctx.Params() Parseable` / `ctx.Query() Parseable` / `ctx.Headers() Parseable` (headers parsing é novo — sem equivalente today)
- Remoção de: `MustParseRestJsonBody`, `ParseRestJsonBody`, `MustParseRestParams`, `ParseRestParams`, `MustParseRestQuery`, `ParseRestQuery`, `MustParseRestFormBody`, `ParseRestFormBody`
- Ver AD-025 (STATE.md) para decisões arquiteturais (Parseable em `execution`, wiring em `adapter/fiber`)
- Ver `.specs/features/unified-parse-api/tasks.md` para breakdown das 13 tasks

---

## Milestone 14: Request/Response Split

**Goal:** Substituir `*RestContext` único por dois tipos concretos `*Request`/`*Response`, espelhando o padrão `(req, res)` de Express/NestJS — motivo de produto (adoção de devusers vindos do Node), não só técnico. Framework pré-1.0, breaking change aceita.
**Status:** COMPLETE (2026-07-17, T1-T18 -- suite inteira `go test ./... -race` verde, 23 pacotes, `.examples/*` migrados, README.md + INSIGHT-*.md atualizados, todos via subagentes Planner/Implementer/Evaluator)

### Features

**Request/Response Split** - COMPLETE
- `internal/execution.Context` → `Request`/`Response`, `Response` guarda `*Request` internamente
- Migra `Handler`/`Guard`/`Middleware`/`Interceptor`/`Filter` (17 arquivos internos identificados) + `next(req, res)`
- `Request.Body()` ganha `.Raw()`/`.Text()` (consolida o `RawBody()` avulso da feature anterior)
- `Response.Html`/`.Text`/`.Json` cada um força seu Content-Type; `SendString` removido
- `Response.Status(code)`/`StatusCode()` substitui `Status`/`ResponseStatus`
- Ver `.specs/features/request-response-split/{spec,context,tasks}.md` — decidido via `superpowers:brainstorming` conduzido só na conversa (usuário pediu sem gerar `docs/superpowers/specs/*.md`); tasks.md é a primeira feature a seguir o padrão de 3 papéis de subagente (Planner/Implementer/Evaluator, ver STATE.md's "Subagent workflow convention")

---

## Milestone 15: Schema Value Support

**Goal:** Permitir `gonest.NewSchema`-like para um valor primitivo isolado (sem struct em volta -- ex: um CPF `string` solto), e renomear `gonest.Value[T]` (dirty-tracking existente) para `gonest.Accessor[T]`, liberando o nome `Value` para o conceito novo.
**Status:** COMPLETE (T1-T7 executados, `go test ./... -race` verde, 23 pacotes)

### Features

**Schema Value Support** - COMPLETE
- `gonest.Value[T]` (dirty-tracking, código real hoje) → `gonest.Accessor[T]` -- mesma API, só o nome muda
- `gonest.NewValue[T](func(m *gonest.Value) {...})` novo -- constrói um `*Schema` pra valor único, reaproveitando 100% do `PropertyBuilder`/`validateValue` já existentes
- `gonest.Property(&t.X)` (dentro de `NewSchema[T]`, struct) permanece inalterado -- decisão explícita de não reformar por causa de um caso secundário
- Ver `.specs/features/schema-value-support/{spec,context,design,tasks}.md` — brainstorm evoluindo `INSIGHT-SCHEMA.md`, mesmo padrão de 3 papéis de subagente (Planner/Implementer/Evaluator) da feature anterior

---

## Milestone 16: Schema Sanitize/Refine

**Goal:** `PropertyBuilder.Sanitize(fn)` -- pré-processamento por campo (composto com Min/Max/Pattern, ao contrário de `Custom`); `Schema.Refine(fn)` -- pós-processamento cross-field (ex: `password == confirmPassword`), rodado depois de toda validação individual + população terem sucesso.
**Status:** COMPLETE (T1-T5 executados, `go test ./... -race` verde, 23 pacotes)

### Features

**Schema Sanitize/Refine** - COMPLETE
- `PropertyBuilder.Sanitize(fn func(raw any) any)` -- transforma `raw` ANTES de qualquer check (inclusive antes de `Custom`), sem substituir Min/Max/Pattern
- `Schema.Refine(fn func(dst any) (field string, err error))` -- cross-field, roda só depois de `validateStruct`+`populate` terem sucesso; múltiplos `Refine` acumulam (collect-all)
- V1 escopo: JSON body apenas (`params`/`query`/`form`/`headers` e `Value`-schemas ficam de fora, ver spec.md's Out of Scope)
- Ver `.specs/features/schema-sanitize-refine/{spec,design,tasks}.md` — brainstorm evoluindo `INSIGHT-SCHEMA.md`'s seção "Pré/pós-processamento"

---

## Milestone 17: GraphQL Support

**Goal:** `Resolver`/`Query`/`Mutation`/`Subscription` como nova ponta de exposição GraphQL, reaproveitando 100% de `Schema`/`Parse[T]`/`MustParse[T]`/`MustInject`/`Emitter` já existentes -- sem duplicar validação/DI entre REST e GraphQL.
**Status:** COMPLETE (T1-T11 executados, `go test ./... -race` verde, 24 pacotes)

### Features

**GraphQL Support** - COMPLETE
- `gonest.NewGraphqlResolver`/`resolver.Query`/`Mutation` -- `Handler(func(ctx *GraphqlContext) any)`, retorno direto vira o `data` (igual NestJS, sem `Response` separado)
- `resolver.Subscription` -- `Handler(func(ctx, emit func(any)))`, reaproveita `gonest.Emitter` via novo `gonest.Subscribe[T](emitter, done) <-chan T`
- Geração automática de SDL a partir do MESMO `gonest.Schema` usado em REST/OpenAPI; branches de formato (Email/Uuid/DateTime/etc) viram Custom Scalars
- `Custom(fn).GraphqlScalar(name)` -- nomeia scalar pra tipos sem `format` OpenAPI nativo (ex: `primitive.ObjectID`)
- Motor: `graphql-go/graphql` (code-first, runtime, sem `go generate`) -- decidido após pesquisa real (web + `gh issue view`), não assumido
- Transporte de Subscription (SSE + WebSocket) escrito por conta própria -- nenhum dos dois vem pronto do motor escolhido; WebSocket via `github.com/gofiber/contrib/v3/websocket` (nova `HttpAdapter.RegisterWebSocket` capability)
- Todo o código real vive em UM único pacote `internal/graphql` (builder + gerador SDL + transportes) -- não 3 pacotes separados como o design original previa, reorganizado durante a execução
- Ver `.specs/features/graphql-support/{spec,context,design,tasks}.md` — evolui `INSIGHT-GRAPHQL.md`

---

## Milestone 18: GraphQL Realtime Protocols

**Goal:** Substituir os transportes de Subscription ad-hoc (SSE/WS próprios, Milestone 17/T9-T10) pelos protocolos REAIS e amplamente adotados -- `graphql-transport-ws` (WebSocket) e `graphql-sse` (SSE, os dois modos: Distinct connections e Single connection), ambos direto no `/graphql` já existente. Motivado por um bug real: uma IDE GraphQL de verdade tentou WS em `/graphql` esperando o protocolo padrão e falhou.
**Status:** COMPLETE (T1-T18 executados via subagentes Planner/Implementer/Evaluator, `go test ./... -race` verde, 25 pacotes, `.examples/*` buildam)

### Features

**GraphQL Realtime Protocols** - COMPLETE
- WebSocket: `graphql-transport-ws` real (`ConnectionInit`/`Ack`, `Ping`/`Pong`, `Subscribe`/`Next`/`Error`/`Complete`, multiplexação por `id`, fechamentos `4408`/`4429`/`4409`/`4401`/`4400`) -- só o subprotocolo moderno, sem o legado `graphql-ws`; servidor NEGOCIA o subprotocolo de verdade (`Sec-WebSocket-Protocol` ecoado no handshake, achado real rodando `.examples/blog-graphql` -- sem isso, IDEs reais recusariam a conexão, o MESMO tipo de falha que motivou a feature inteira)
- SSE Distinct connections mode: `GET /graphql` com `Accept: text/event-stream`, 1 conexão por operação, evento `complete` inclui `data: ` vazio (exigido pelo PROTOCOL.md real pro listener do `EventSource` disparar -- confirmado via `curl` direto no PROTOCOL.md, não assumido)
- SSE Single connection mode: `PUT` (reserva+token) → `GET` (conexão única) → `POST`/`DELETE` (executa/encerra operação, multiplexado por `operationId`)
- Removidos por inteiro os 2 endpoints ad-hoc (`/graphql/stream/:name`, `/graphql/ws/:name`, `internal/graphql/{ws,sse}.go`) -- nenhum teve consumidor real
- Decisão central de arquitetura: `HttpAdapter.RegisterWebSocket` removido, substituído por `execution.Response.UpgradeWebSocket`/`execution.Request.IsWebSocketUpgrade` (novo capability em `Responder`) -- permite `POST`/`PUT`/`GET`/`DELETE` coexistirem no MESMO `/graphql` sem o `app.Use(path, ...)` antigo interceptando todo método (o Edge Case que o spec.md deixou em aberto pra Design)
- `.examples/blog-graphql` demonstra os 3 transportes reais, com evidência real de dial WS/SSE colada nos relatórios de execução
- Ver `.specs/features/graphql-realtime-protocols/{spec,context,design,tasks}.md`, AD-040 em STATE.md

---

## Milestone 19: Config Loading

**Goal:** Equivalente ao `ConfigModule` do NestJS -- carregar `.env` pro processo (paridade completa
com o formato real do [`dotenvx`](https://dotenvx.com)) e validar/popular structs de config tipadas a
partir de variáveis de ambiente, reaproveitando 100% do `Schema`/`Parse[T]`/`MustParse[T]`/`Provider`
que REST já usa. Motivado pelo `ConfigModule` do NestJS e por um modelo próprio do usuário
(`gox/env`), que por sua vez foi construído mirando o comportamento do `dotenvx`.
**Status:** COMPLETE (T1-T12 executados via subagentes, `go test ./... -race` verde, 25 pacotes, `.examples/config-dotenv` demonstra fluxo real)

### Features

**Dotenv Loading** - COMPLETE
- `gonest.Dotenv()` -- singleton SEM DI (funciona em `main()` antes de qualquer bootstrap), `Load`/
  `MustLoad(paths ...string)`
- Sintaxe `.env` com paridade completa com o `dotenvx` real (`https://dotenvx.com/docs/env-file`,
  pesquisado nesta sessão): comentários (linha inteira + inline), aspas simples (literal) vs duplas
  (interpoladas), interpolação `${VAR}`/`$VAR`, os 4 operadores de default/alternate
  (`${VAR:-x}`/`${VAR-x}`/`${VAR:+x}`/`${VAR+x}`), multiline via backtick, escapes `\n`/`\r`/`\t`/`\\`
- Ver `.specs/features/dotenv-loading/{spec,context}.md`

**Env → Schema Binding** - COMPLETE
- `*Dotenv` (mesma instância da feature acima) ganha `ParseInto`, satisfazendo `execution.Parseable`
  -- `gonest.MustParse[DatabaseConfig](gonest.Dotenv(), schema)` funciona igual a qualquer `Parse[T]`
  já existente (REST)
- Tag `env:"NOME_DA_VAR"` nova (mapeia campo → env var, mesmo padrão de `param`/`query`)
- `PropertyBuilder.Default(value)` novo -- campo ausente da fonte usa o default em vez de disparar
  `Required` (decisão: escopado só pra `env` nesta feature, generalizar é trabalho futuro)
- `Schema.Validate(instance)`/`MustValidate(instance)` do rascunho original REJEITADOS -- criaria
  segundo caminho de validação; `envSource` novo em `internal/validate` (mesmo nível de
  `paramsSource`/`querySource`) resolve reusando `validateStruct`/`populate` sem tocar em nenhum
- Ver `.specs/features/env-schema-binding/{spec,context}.md`

---

## Milestone 20: Lifecycle Hooks

**Goal:** Equivalente aos lifecycle hooks do NestJS (`OnModuleInit`/`OnApplicationBootstrap`/
`OnModuleDestroy`/`BeforeApplicationShutdown`/`OnApplicationShutdown`) -- deixar um `Provider` rodar
código próprio em pontos bem definidos do ciclo de vida da app (setup após dependências prontas,
barreira global de bootstrap, e os 3 estágios de shutdown gracioso), fechando um gap que
`INSIGHT-ON.md` (rascunho do usuário) já vinha apontando desde antes do Milestone 19.
**Status:** COMPLETE (T1-T7 executados via subagentes Planner/Implementer/Evaluator, `go test ./... -race`
verde, 25 pacotes, `.examples/lifecycle-hooks` demonstra o fluxo real)

### Features

**Lifecycle Hooks** - COMPLETE
- `Provider` ganha 5 métodos novos (`OnModuleInit`/`OnApplicationBootstrap`/`OnModuleDestroy`/
  `BeforeApplicationShutdown`/`OnApplicationShutdown`), 1:1 com o hook set REAL do NestJS (confirmado via
  Context7 contra `nestjs/docs.nestjs.com` -- `INSIGHT-ON.md`'s rascunho original só tinha 4, faltava
  `BeforeApplicationShutdown`)
- `OnModuleInit`/`OnApplicationBootstrap` rodam automaticamente durante `NewApp`, sequencial, ordem de
  módulo leaf-first (reverso do BFS root-first que `Module.Assemble()` já produzia) -- DEPOIS do Stage 3
  concorrente já ter resolvido todo o grafo, sem tocar nessa concorrência
- `App.EnableShutdownHooks()` (opt-in, igual Nest) + `App.Close(ctx)` -- disparam
  `OnModuleDestroy → BeforeApplicationShutdown → OnApplicationShutdown` sequencial, ordem ROOT-first
  (reversa do init), com o signal (`"SIGINT"`/`"SIGTERM"`/`""` pra `Close` manual) passado aos 2 últimos
- Hooks só disparam pra Provider `scope.Singleton` (Request/Transient excluídos, mesma exclusão real
  documentada pelo Nest)
- Erro em qualquer hook aborta o restante da MESMA fase (bootstrap) ou da sequência INTEIRA de shutdown
  restante (destroy), sem swallow -- mesmo comportamento sequencial-await do Nest real
- Ver `.specs/features/lifecycle-hooks/{spec,context,design,tasks}.md`, AD-044 em STATE.md

---

## Milestone 21: Enum Branches

**Goal:** `StringSchema`/`NumericSchema` ganham `Enum(items ...T)`, chainable como `Min`/`Max`/`Pattern` --
fecha um gap real encontrado dogfooding `.examples/full-text-search` (`search.FieldsSchemaFor[T]`
precisou de um `Custom(fn)` pra validar "valor precisa ser um dos nomes de campo válidos" porque não
existia mecanismo declarativo de enum).
**Status:** COMPLETE (T1-T4 executados via subagentes Implementer/Evaluator (Planner = eu, tasks.md
já granular o bastante), `go test ./... -race -count=1` verde, 24 pacotes core; `.examples/full-text-search`
migrado e verificado ao vivo via curl -- ver AD-047 em STATE.md)

### Features

**Enum Branches** - COMPLETE
- `StringSchema.Enum(items ...string)`, `NumericSchema.Enum(items ...int64)` -- por branch, não
  `Enum(items ...any)` (perderia inferência de tipo no call site, rejeitado em brainstorming)
- `internal/validate` rejeita valor fora da lista quando Enum setado, mesma postura "coleta toda
  violação" do resto de `validatePrimitive`
- `internal/openapi` emite `"enum": [...]` no schema gerado quando setado
- `.examples/full-text-search`'s `search.FieldsSchemaFor` migra de `Custom(fn)` pro `Enum(...)` real
- Ver `.specs/features/enum-branches/spec.md`

---

## Future Considerations

- Abstração multi-adapter HTTP (net/http, Echo, Gin) — v1 é só Fiber
- CLI de scaffolding (equivalente `nest new`/`nest generate`)
- Microservices/transport layer (equivalente `@nestjs/microservices`)
- XML body parsing via `ctx.Body().Xml()` — arquitetura `Parseable` suporta como drop-in, sem tocar `Parse[T]`/`MustParse[T]`

