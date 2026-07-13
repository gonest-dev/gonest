# Roadmap

**Current Milestone:** Request Pipeline (Milestone 3)
**Status:** Milestones 1-2 COMPLETE — starting Milestone 3

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

**Interceptor** - PLANNED
- `NewInterceptor`, envolve execução do handler (antes/depois)

**Pipe** - PLANNED
- `NewPipe`, transforma/valida param antes do handler, panic `BadRequestException` se inválido

**Filter** - PLANNED
- `NewFilter`, `Catch(exceptionType, handler)`, registro por controller/módulo/global

**Pipeline Ordering** - PLANNED
- Valida ordem Middleware → Guard → Interceptor → Pipe → Handler em cenário com todos combinados

---

## Milestone 4: Metadata Builder — Primitivos

**Goal:** `NewMetadata`/`Property` cobre todos os branches flat tipo+format do OpenAPI 3.1 com validadores comuns.

### Features

**Metadata Registration Core** - PLANNED
- `NewMetadata[T]`, `Property(&t.X)`, base comum: `Description`/`Required`/`Nullable`/`Examples`

**String-family Branches** - PLANNED
- `String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary` + `Min`/`Max`/`Pattern`

**Numeric & Boolean Branches** - PLANNED
- `Integer`/`Int32`/`Float`/`Double` + `Min`/`Max`, `Boolean`

**Date/Time Branches** - PLANNED
- `DateTime`/`Date`

---

## Milestone 5: Metadata Builder — Array & Object

**Goal:** estruturas aninhadas reusam metadata já registrada sem duplicar `Property`; builder linear/encadeável (decisão registrada no INSIGHT.md).

### Features

**Array Builder** - PLANNED
- `Array()`, `Items(...)` variádico (zero-arg encadeia branch primitivo / um-arg recebe metadata pra referência)
- Semântica: `Min`/`Max` logo após `Items()` = item; `Min`/`Max` em `Array()` antes de `Items()` = quantidade

**Object Builder** - PLANNED
- `Object(metadataValue)` reusa metadata registrada (equivalente `$ref`)
- `Object(func(om))` schema livre/aberto (`AdditionalProperties`)

---

## Milestone 6: Runtime Validation

**Goal:** mesma declaração de metadata valida request em runtime, rejeitando payload inválido com detalhe de campo.

### Features

**JSON Body Validation** - PLANNED
- `MustJsonBody[T]` valida contra `NewMetadata[T]`, panic `BadRequestException` com details por campo

**Param/Query Validation** - PLANNED
- `MustParam[T]` integra `Pipe` + coerção via metadata

---

## Milestone 7: OpenAPI Generation

**Goal:** `NewOpenApiDocument` + `SetupSwagger` geram doc OpenAPI 3.1 a partir da mesma metadata, servido em path configurável.

### Features

**OpenAPI Document Builder** - PLANNED
- `NewOpenApiDocument`, `Title`/`Description`/`Version`/`Contact`/`License`/`BearerAuth`

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
