# HttpContext Unification — Tasks

Mesmo padrão de execução do AD-030 (Milestone 14, `request-response-split`): fundação sequencial
(poucas linhas por task não valem overhead de subagente, e T1 é o único ponto onde tudo depende de
tudo -- Response/Reply, RouteResponse/Response e HttpContext precisam existir juntos antes de
QUALQUER consumidor compilar), depois tasks paralelizáveis via subagente (Implementer) uma vez que
a fundação compila, depois sweep final de testes+exemplos+docs.

## T1: Fundação -- `internal/execution`, `internal/route`, `internal/adapter/fiber`, `internal/app` [sequencial, feito direto]

**What:**
1. `internal/execution/response.go`→`reply.go`: `Response`→`Reply` (tipo+métodos, receiver `res`
   inalterado). `response_test.go`→`reply_test.go` (mesma migração nos testes).
2. `internal/execution/httpcontext.go` novo: `HttpContext` struct (`req *Request`/`res *Reply`),
   `NewHttpContext(req, res) *HttpContext`, `Request() *Request`, `Response() *Reply`. Doc
   comments exatamente como em design.md.
3. `internal/route/response.go`: `RouteResponse`→`Response` (tipo+métodos). `internal/route/route.go`:
   `responses map[int]*RouteResponse`→`map[int]*Response`; `Route.Response(status, fn
   ...func(response *RouteResponse))`→`func(response *Response)`; `Responses()
   map[int]*RouteResponse`→`map[int]*Response`; `Route.handler`/`Handler`/`HandlerFunc` migram de
   `func(req *execution.Request, res *execution.Response)` pra `func(c *execution.HttpContext)`.
   `route_test.go` migrado junto.
4. `internal/adapter/fiber/fiber.go`: `App.RegisterRoute`'s `wrapped` closure -- renomeia o
   parâmetro Fiber (`c fiber.Ctx`) pra `fc` (evita colisão de nome com `HttpContext`'s `c`
   convencional), constrói `req, res := execution.New(...)` + `ctx :=
   execution.NewHttpContext(req, res)`, chama `h(ctx)`. Assinatura de `RegisterRoute` migra pro
   novo `h func(c *execution.HttpContext) error`. `fiber_test.go` migrado junto.
5. `internal/app/app.go`: `HttpAdapter.RegisterRoute` interface method assinatura nova;
   `registerRoutes`'s `collected.handler` + toda a cadeia (`withRoute`/`filteredHandler`/
   `gatedHandler`/`interceptedHandler`/`composeHandler`) migrada pra `func(c *execution.HttpContext)`,
   incluindo as 2 chamadas de `h.Call(...)` dentro de `filteredHandler` (2 argumentos agora:
   `reflect.ValueOf(c)`, `reflect.ValueOf(exc)`). `internal/app/graphql.go`'s 3 dispatcher funcs
   migradas junto (mesma assinatura). Todo `_test.go` de `internal/app` migrado.
6. `gonest.go`: bloco de alias exato de design.md (`Reply`, `Response` realocado, `HttpContext`
   novo).

**Where:** `internal/execution/{response.go→reply.go,response_test.go→reply_test.go,httpcontext.go
(novo)}`, `internal/route/{response.go,route.go,route_test.go}`,
`internal/adapter/fiber/{fiber.go,fiber_test.go}`, `internal/app/{app.go,graphql.go}` + todo
`_test.go` de `internal/app`, `gonest.go`.

**Depends on:** nada.

**Done when:** `go build ./internal/execution/... ./internal/route/... ./internal/adapter/fiber/...
./internal/app/... .` limpo (pacotes que ainda não migraram -- guard/middleware/interceptor/
filter/openapi/graphql -- vão quebrar até T2 rodar, esperado).

**Gate:** `go vet ./internal/execution/... ./internal/route/... ./internal/adapter/fiber/...
./internal/app/...` limpo; `go test ./internal/execution/... ./internal/route/...
./internal/adapter/fiber/... -race -count=1` verde (testes de `internal/app` só ficam verdes depois
de T2, já que `internal/app` importa guard/middleware/interceptor/filter).

---

## T2: `internal/guard`, `internal/middleware`, `internal/interceptor`, `internal/filter` [1 Implementer, sequencial após T1]

**What:**
1. `internal/guard/guard.go`: `Guard.handler`/`Handler`/`HandlerFunc` migram pra `func(c
   *execution.HttpContext) bool`. `guard_test.go` migrado.
2. `internal/middleware/middleware.go`: `Next func(c *execution.HttpContext)`,
   `Middleware.handler`/`Handler`/`HandlerFunc` migram pra `func(c *execution.HttpContext, next
   Next)`. `middleware_test.go` migrado.
3. `internal/interceptor/interceptor.go`: mesmo shape de middleware (`Next`, `handler`, `Handler`,
   `HandlerFunc`). `interceptor_test.go` migrado.
4. `internal/filter/filter.go`: `requestType`/`responseType` vars → 1 `contextType =
   reflect.TypeOf((*execution.HttpContext)(nil))`. `isValidCatchSignature` valida `NumIn()==2,
   In(0)==contextType, In(1)==excType` (era 3 args). Mensagem de panic de `Catch` atualizada
   (`"...expected func(c *execution.HttpContext, exc " + excType.String() + ")"`). `filter_test.go`
   migrado -- inclusive o teste que cobre o shape INVÁLIDO (confirma que o 3-arg antigo agora
   panica com a mensagem nova, não só que o 2-arg novo é aceito).

**Where:** `internal/guard/{guard.go,guard_test.go}`,
`internal/middleware/{middleware.go,middleware_test.go}`,
`internal/interceptor/{interceptor.go,interceptor_test.go}`,
`internal/filter/{filter.go,filter_test.go}`.

**Depends on:** T1 (precisa de `execution.HttpContext` existindo).

**Reuses:** nada -- 4 pacotes irmãos, mesmo shape de mudança repetido.

**Done when:** os 4 pacotes compilam com a assinatura nova; `Filter.Catch` rejeita o shape antigo
de 3 args com a mensagem de panic atualizada.

**Tests:** suites existentes migradas, sem caso novo além da correção do teste de shape inválido
do `Filter.Catch`.

**Gate:** `go test ./internal/guard/... ./internal/middleware/... ./internal/interceptor/...
./internal/filter/... -race -count=1` verde.

---

## T3: `internal/openapi`, `internal/graphql` (SSE/WS realtime) [1 Implementer, paralelo a T2 -- ambos só dependem de T1]

**What:**
1. `internal/openapi/swagger.go`: as 2 closures `RegisterRoute(..., func(req *execution.Request,
   res *execution.Response) {...})` dentro de `SetupSwagger` migram pra `func(c
   *execution.HttpContext) {...}` (corpo troca `res.Json(...)`/`res.Html(...)` por
   `c.Response().Json(...)`/`c.Response().Html(...)`).
2. `internal/openapi/generate.go`/`generate_test.go`: onde ler `Route.Responses()` -- tipo do mapa
   virou `map[int]*route.Response` (era `*route.RouteResponse`), ajustar só a referência de tipo,
   lógica de geração de doc OpenAPI inalterada.
3. `internal/graphql/sse_distinct.go`, `sse_single.go`, `ws_protocol.go`: todo dispatcher/handler
   exportado com shape `func(req *execution.Request, res *execution.Response) {...}` migra pra
   `func(c *execution.HttpContext) {...}`, corpo ajustado pra `c.Request()`/`c.Response()` onde
   `req`/`res` eram usados. `internal/graphql`'s `_test.go` correspondentes migrados.

**Where:** `internal/openapi/{swagger.go,generate.go,generate_test.go,swagger_test.go}`,
`internal/graphql/{sse_distinct.go,sse_single.go,ws_protocol.go}` + `_test.go` de cada.

**Depends on:** T1 apenas (não depende de T2 -- não usa Guard/Middleware/Interceptor/Filter).

**Reuses:** nada.

**Done when:** os 2 pacotes compilam com a assinatura nova; geração de documento OpenAPI e
handlers de realtime GraphQL (SSE/WS) mantêm o MESMO comportamento observável, só o shape do
parâmetro muda.

**Tests:** suites existentes migradas.

**Gate:** `go test ./internal/openapi/... ./internal/graphql/... -race -count=1` verde.

---

## T4: Root package (`gonest_test.go`) + `.examples/*` + `README.md` [1 Implementer, depende de T1+T2+T3]

**What:**
1. `gonest_test.go`: todo teste que constrói `execution.New(...)` diretamente, ou passa
   `(req, res)` pra um Handler/Guard/Middleware/Interceptor/Filter sob teste, ou referencia
   `gonest.Response`/`gonest.RouteResponse` pelos nomes antigos, migrado pro shape novo
   (`gonest.HttpContext`/`gonest.Reply`/`gonest.Response`).
2. Todo `.examples/*` (`simple-todo`, `blog-api`, `blog-graphql`, `config-dotenv`,
   `full-text-search`, `lifecycle-hooks`, `notification-driver` -- confirmar lista completa via
   `ls .examples/`, não assumir só as 5 do pre-push hook) -- cada `Handler`/`Guard`/`Middleware`/
   `Interceptor`/`Filter.Catch`/`Route.Response(status, func(*gonest.RouteResponse){})` migrado pro
   shape novo. Cada exemplo precisa continuar buildando (`go build ./...` dentro de cada
   `.examples/<nome>`).
3. `README.md`: toda seção/exemplo de código que mostra `func(req *gonest.Request, res
   *gonest.Response) {...}`, `Route.Response(status, func(*gonest.RouteResponse){})`,
   `next(req, res)` migrada pro shape novo (`func(c *gonest.HttpContext) {...}`,
   `c.Response().Status(...).Json(...)`, `next(c)`). Toda seção "Implementation Status"/tabela que
   cita `Response`/`RouteResponse` por nome atualizada.
4. `.specs/insight/*.md` -- se algum ainda citar `Response`/`RouteResponse`/`(req, res)` pelos
   nomes antigos, atualizar (histórico genuinamente superado, não histórico-preservado -- mesmo
   critério que AD-030 já aplicou na sua própria época).

**Where:** `gonest_test.go`, `.examples/*/**/*.go`, `README.md`, `.specs/insight/*.md` (se
aplicável).

**Depends on:** T1, T2, T3 (precisa de TODO o resto compilando pra validar os exemplos).

**Reuses:** nada.

**Done when:** `go build ./...` (raiz) limpo, `go build ./...` dentro de CADA `.examples/<nome>`
limpo, todo snippet de código do README recompilado manualmente (copiar pra arquivo scratch,
`go build`) antes de considerar concluído -- mesmo padrão que AD-030 usou.

**Tests:** `gonest_test.go` migrado, sem caso novo.

**Gate:** `go test ./... -race -count=1` (repo inteiro) verde, 24+ pacotes, zero asserção
pré-existente alterada além do shape de assinatura. `go build ./...` em cada `.examples/*`.

---

## T5 (Evaluator/orquestrador, não subagente): STATE.md/ROADMAP.md + tag + site

Depois de T1-T4 aprovados: nova entrada AD em STATE.md, novo Milestone em ROADMAP.md, commit+tag
(`vX.Y.0`, breaking -- confirmar se o esquema de versionamento deste projeto bump o 2º segmento
pra breaking, mesmo padrão já usado em AD-056/TokenRef), push. Depois, subagente separado pra
atualizar o site (`C:\dev\gonest-dev\site`) -- toda página que mostra `func(req, res)`,
`gonest.Response`/`gonest.RouteResponse` pelos nomes antigos, precisa do mesmo tratamento de
`.examples`/README acima, nas 3 línguas (en/es/pt).
