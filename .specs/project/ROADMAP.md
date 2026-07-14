# Roadmap

**Current Milestone:** OpenAPI Generation (Milestone 7)
**Status:** Milestones 1-6 COMPLETE — starting Milestone 7

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

**Goal:** `NewOpenApiDocument` + `SetupSwagger` geram doc OpenAPI 3.1 a partir da mesma metadata, servido em path configurável.

### Features

**OpenAPI Document Builder** - COMPLETE
- `NewOpenApiDocument`/`OpenApiDocument` (`internal/openapi`, re-exportado na raiz), `Title`/`Description`/`Version`/`Contact`/`License`/`BearerAuth` -- builder mecânico, sem dependência de `internal/metadata`/`internal/route`

**Schema Generation from Metadata** - PLANNED
- Branches de metadata → schema type+format; `$ref` reuso pra Object/Array-de-Object

**Swagger UI Setup** - PLANNED
- `SetupSwagger`, `SwaggerOptions{JsonDocumentUrl, PersistAuth, DocExpansion}`

---

## Milestone 8: Testing Helpers

**Goal:** DX equivalente `@nestjs/testing` — override de provider por interface, teste via request HTTP ou unitário direto.

### Features

**Test App Bootstrap** - PLANNED
- `MustNewTestApp`, `TestBuilder`, `MustOverride[Interface]`

**HTTP Test Client** - PLANNED
- `MustRequest`, `AssertStatus`, `AssertJsonPath`

---

## Future Considerations

- Abstração multi-adapter HTTP (net/http, Echo, Gin) — v1 é só Fiber
- Emitter (event-emitter, equivalente `@nestjs/event-emitter`)
- Scheduler (Cron/Interval/Timeout, equivalente `@nestjs/schedule`)
- Terminus/health checks (equivalente `@nestjs/terminus`)
- CLI de scaffolding (equivalente `nest new`/`nest generate`)
- Microservices/transport layer (equivalente `@nestjs/microservices`)
