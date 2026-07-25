# HttpContext Unification — Spec

## Context

AD-028/AD-030 (Milestone 14, Request/Response Split) replaced a single `*Context` with separate
`*Request`/`*Response` parameters across `Handler`/`Guard`/`Middleware`/`Interceptor`/`Filter`,
reasoned explicitly on Express/NestJS parity (`(req, res)` is universal in that ecosystem). That
reasoning stands for THAT specific shape question (2 params vs 1), but a separate, real pain point
surfaced independently: `gonest.Response` (write-side HTTP type) and `gonest.RouteResponse`
(OpenAPI-per-status documentation builder) read as near-duplicate names and sit side by side in the
same controller file (`Route.Response(status, func(*RouteResponse){...})` next to `func(req
*Request, res *Response) {...}`), confusing exactly where clarity matters most.

Discussed live: rather than only fixing the name collision (`Response`→`Reply` for the write side,
freeing `Response` for the OpenAPI builder), the user opted to also revisit the 2-param shape --
wrap `Request`+the renamed write-side type behind a single `HttpContext` parameter (2 accessor
methods, `Request()`/`Response()`, everything else reached through those), while STILL fixing the
name collision underneath. `GraphqlContext` (existing, `internal/graphql/context.go`) is a loose
precedent for "resolvers/handlers take one context object" -- not identical in shape (GraphQL
resolvers return values, no write-side to expose), but establishes the naming pattern
(`XxxContext`) this reuses.

## User Story

Como usuário do gonest, quero que toda `Handler`/`Guard`/`Middleware`/`Interceptor`/`Filter.Catch`
receba um único `*HttpContext` (com `c.Request()`/`c.Response()` como os 2 únicos métodos diretos,
todo o resto alcançado a partir deles), e que o tipo retornado por `c.Response()` não colida de nome
com o builder de documentação OpenAPI usado em `Route.Response(status, func(*gonest.Response) {...})`.

## Requirements

1. `internal/execution.Response` (write-side: `Status`/`StatusCode`/`SetHeader`/`Json`/`Html`/
   `Text`/`Stream`/`UpgradeWebSocket`/`Request()`) renomeado pra `internal/execution.Reply`.
   Arquivo `response.go`→`reply.go`, `response_test.go`→`reply_test.go`. Nome do RECEIVER (`res`)
   inalterado.
2. `internal/route.RouteResponse` (builder de documentação OpenAPI por status) renomeado pra
   `internal/route.Response` -- nome do MÉTODO `Route.Response(status, fn)` inalterado, só o tipo
   do parâmetro do callback muda.
3. `internal/execution.HttpContext` novo -- struct com `req *Request`/`res *Reply`, exatamente 2
   métodos: `Request() *Request` e `Response() *Reply`. Nenhum outro método promovido -- todo
   acesso de leitura/escrita passa por um desses dois primeiro (`c.Request().Param("id")`,
   `c.Response().Status(200).Json(...)`).
4. Toda assinatura pública migra de `(req *Request, res *Response)` pra `(c *HttpContext)`:
   - `Route.Handler(fn func(c *HttpContext))` / `HandlerFunc() func(*HttpContext)`
   - `Guard.Handler(fn func(c *HttpContext) bool)`
   - `Middleware.Handler(fn func(c *HttpContext, next Next))`, `Next func(c *HttpContext)`
   - `Interceptor.Handler(fn func(c *HttpContext, next Next))`, `Next func(c *HttpContext)`
   - `Filter.Catch[T](fn func(c *HttpContext, exc T))`
   - `HttpAdapter.RegisterRoute(method, path, h func(c *HttpContext)) error` (+ Fiber adapter impl)
   - `internal/app`'s dispatch composition (`filteredHandler`/`gatedHandler`/`interceptedHandler`/
     `composeHandler`/`withRoute` closures)
   - GraphQL realtime handlers (`internal/graphql`'s SSE Distinct/Single + WS protocol dispatch
     funcs) and `internal/openapi/swagger.go`'s 2 `SetupSwagger` route closures
5. `gonest.go`: `Response = execution.Response` alias vira `Reply = execution.Reply`;
   `RouteResponse = route.RouteResponse` vira `Response = route.Response`; novo `HttpContext =
   execution.HttpContext` alias.
6. Todo callsite (`.examples/*`, README.md, doc comments, testes) migrado pro novo shape de 1
   parâmetro + tipos renomeados.
7. Site (`C:\dev\gonest-dev\site`) atualizado em commit separado, mesma convenção de sempre.

## Out of Scope

| Item | Motivo |
| --- | --- |
| Promover métodos de leitura/escrita direto no `HttpContext` (`c.Param(...)` sem `c.Request()`) | Decisão explícita do usuário: só 2 métodos diretos, tudo mais nasce deles -- evita 2 formas de fazer a mesma coisa. |
| `GraphqlContext` | Já é objeto único, formato diferente (sem write-side real), fora do escopo -- não precisa mudar. |
| Renomear `Request` | Sem colisão, sem confusão relatada. |
| Mudar comportamento de qualquer método existente de `Request`/`Reply` | Rename + wrap puro, zero mudança de contrato nos métodos individuais. |

## Requirement Traceability

| ID | Requirement | Status |
| --- | --- | --- |
| HTTPCTX-01 | `execution.Response`→`Reply` | Verified |
| HTTPCTX-02 | `route.RouteResponse`→`Response` | Verified |
| HTTPCTX-03 | `HttpContext` novo (2 métodos: `Request()`/`Response()`) | Verified |
| HTTPCTX-04 | Toda assinatura pública migrada pra `(c *HttpContext)` | Verified |
| HTTPCTX-05 | `gonest.go` alias novo/realocado (`Reply`, `Response`, `HttpContext`) | Verified |
| HTTPCTX-06 | Callsites (`.examples`, README, testes) migrados | Verified |
| HTTPCTX-07 | Site atualizado | Implementing |
