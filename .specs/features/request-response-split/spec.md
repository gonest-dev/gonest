# Request/Response Split Specification

## Problem Statement

Hoje toda a superfície pública do gonest (`Handler`, `Guard`, `Middleware`,
`Interceptor`, `Filter`) recebe um único `*RestContext` (`internal/execution.Context`)
que mistura leitura de request (`Params`, `Query`, `Headers`, `Body`) e escrita
de response (`Json`, `Status`, `HTML`, `SendString`) no mesmo objeto. gonest
mira devusers vindos do ecossistema Node (Express/NestJS), onde `(req, res)`
é o padrão universal — a maioria das libs Express-like da comunidade Node
segue essa convenção. Manter um `ctx` único aumenta a fricção de adoção pra
esse público-alvo e cria ambiguidades de nomenclatura reais (ex: `ctx.Json()`
hoje é escrita, mas não há como nomear uma leitura de JSON no mesmo objeto
sem colidir).

Framework está pré-1.0 (nenhum consumidor externo além do próprio autor via
`.examples/*`) — o custo de uma breaking change de arquitetura é baixo agora
e sobe MUITO depois do primeiro release estável.

## Goals

- [ ] Substituir `*RestContext` único por dois tipos concretos: `*Request`
      (leitura) e `*Response` (escrita)
- [ ] `Response` guarda uma referência interna a `*Request` (decisão do
      usuário — sem problema de acoplamento nesse sentido)
- [ ] Migrar TODAS as assinaturas públicas que hoje recebem `ctx`: `Handler`,
      `Guard`, `Middleware`, `Interceptor`, `Filter`, e o `next` de
      Middleware/Interceptor
- [ ] Consolidar `Request.Body()` com 4 sub-acessores simétricos: `Raw()`
      (`[]byte`), `Text()` (`string`), `Json() Parseable`,
      `Form(onFile) Parseable` — substitui o `Request.RawBody()` avulso
- [ ] Consolidar `Response` em torno de content-type forçado por método:
      `Json(v)`, `Html(s)`, `Text(s)` — cada um seta seu próprio Content-Type
      explicitamente
- [ ] Remover `SendString` (absorvido por `Response.Text`, que agora força
      `text/plain`)
- [ ] Renomear `Status`/`ResponseStatus` pra `Status(code) *Response`
      (escrita) / `StatusCode() int` (leitura) — tira a ambiguidade de prefixo
      Get/Set

## Out of Scope

| Feature                                              | Reason                                                                                     |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `gonest.Value[T]` para os pares Get/Set de Request/Response | Analisado durante o brainstorming: só `Status`/`StatusCode` é um par real (Header é request-read vs response-write, dados diferentes, não um par de verdade); `internal/value.Value[T]` tem semântica de dirty-tracking incompatível com refletir o estado real vindo do fasthttp (ex: status já default pra 200 sem `Set` explícito). Usuário optou por manter `Status`/`StatusCode` simples, sem wrapper genérico |
| XML body parsing (`Request.Body().Xml()`)              | Já adiado desde a feature `unified-parse-api` — continua fora de escopo aqui                |
| Abstração multi-adapter HTTP (net/http, Echo, Gin)     | v1 continua só Fiber — fora do roadmap atual                                                |

---

## Design Decisions (tomadas durante o brainstorming)

| #   | Decisão                                                                                                                                                                                  |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| D1  | Split real (não façade fina sobre um `Context` único por baixo) — usuário escolheu explicitamente a Abordagem A sobre a B, mesmo custo de refactor maior, pelo ganho de isolamento/testabilidade genuínos |
| D2  | `Response` guarda `*Request` internamente (não são independentes) — usuário confirmou que não vê problema nisso |
| D3  | TODAS as assinaturas migram (`Handler`, `Guard`, `Middleware`, `Interceptor`, `Filter`) — não só o `Handler` final |
| D4  | `next` de Middleware/Interceptor vira `func(req *Request, res *Response)` — espelha Express de verdade (`middleware(req, res, next)`, `next(req, res)`), não `func()` sem argumentos |
| D5  | `Request.RawBody()` vira `Request.Body().Raw()`, e `Request.Body().Text()` é adicionado — tudo relacionado a body fica atrás de um único `Body()` |
| D6  | `Response.HTML` vira `Response.Html` — consistência com `Json` (já não-all-caps no resto do projeto, ex: `BodyJsonSchema`) |
| D7  | `Response.Text(s)` força `Content-Type: text/plain` explicitamente (mudança de comportamento vs `SendString` de hoje, que não seta nada) — mesma filosofia de `Json`/`Html`, que já forçam seu content-type |
| D8  | `gonest.Value[T]` NÃO é usado aqui — ver Out of Scope |

---

## Architecture Note

`internal/execution/context.go` (um único `Context`) é substituído por dois
arquivos/tipos: `internal/execution/request.go` (`Request`) e
`internal/execution/response.go` (`Response`). `Responder` (a interface
fiber-agnostic que abstrai o adapter HTTP real) continua existindo como hoje,
compartilhada pelos dois — só o `Context` que a envolvia deixa de existir
como tipo único.

`New(res Responder) (*Request, *Response)` substitui `New(res Responder) *Context`.
`Response` recebe o `*Request` já construído e guarda a referência
internamente (`Response.Request() *Request` expõe essa referência de volta).

Toda a cadeia de composição em `internal/app` (`composeHandler`,
`gatedHandler`, `interceptedHandler`, `filteredHandler`, `withRoute`) e o
dispatch em `internal/adapter/fiber` (`RegisterRoute`) mudam de
`func(ctx *execution.Context)` para `func(req *execution.Request, res *execution.Response)`
— mudança mecânica de assinatura, sem mudança de lógica de composição.

---

## API Sketch

```go
package ex

import "gonest.dev/gonest"

var Controller = gonest.NewController(func(controller *gonest.Controller) {
  controller.Path("/ex")
  controller.RoutePost("/", func(req *gonest.Request, res *gonest.Response) {
    body := gonest.MustParse[BodyJson](req.Body().Json(), BodyJsonSchema)
    raw := req.Body().Raw()   // []byte
    text := req.Body().Text() // string
    _ = raw
    _ = text
    res.Status(gonest.HttpStatusCreated).Json(body)
  })

  controller.Route(gonest.HttpGet, "/health", func(req *gonest.Request, res *gonest.Response) {
    res.Text("OK") // Content-Type: text/plain
  })
})

var LoggingMiddleware = gonest.NewMiddleware(func(m *gonest.Middleware) {
  m.Handler(func(req *gonest.Request, res *gonest.Response, next func(*gonest.Request, *gonest.Response)) {
    next(req, res)
    // res.StatusCode() disponível aqui, depois de next rodar
  })
})
```

---

## User Stories

### P1: Split `Request`/`Response` ⭐ MVP

**User Story**: Como desenvolvedor vindo de Express/NestJS, quero receber
`(req, res)` separados em todo Handler/Guard/Middleware/Interceptor/Filter,
pra ter uma API reconhecível e sem ambiguidade entre ler e escrever dados da
requisição.

**Why P1**: É o núcleo da feature — todo o resto (Body consolidado, Response
consolidado) só faz sentido em cima do split já existindo.

**Acceptance Criteria**:

1. WHEN um Handler é registrado via `controller.Route(method, path, func(req *Request, res *Response) {...})` THEN o dispatch real (Fiber) SHALL invocar essa função com `req`/`res` distintos e funcionais
2. WHEN um Guard/Middleware/Interceptor/Filter é declarado THEN sua assinatura SHALL receber `(req *Request, res *Response, ...)` no lugar de `ctx *RestContext`
3. WHEN o `next` de um Middleware/Interceptor é chamado THEN SHALL aceitar `(req *Request, res *Response)` como argumentos
4. WHEN `res` precisa consultar dados do `req` que o originou (ex: logging) THEN `res.Request()` SHALL retornar o `*Request` correspondente

**Independent Test**: `go test ./... -race` passa; um Handler reproduzindo o exemplo do INSIGHT.md via dispatch HTTP real (`app.Test`) funciona com `(req, res)`.

---

### P1: Consolidar `Request.Body()` ⭐ MVP

**User Story**: Como desenvolvedor, quero todo acesso ao corpo da requisição
(bytes crus, texto, JSON, form) atrás de um único `req.Body()`, com métodos
simétricos por formato.

**Why P1**: Fecha a inconsistência que sobrou da feature `unified-parse-api`
(`RawBody()` avulso fora de `Body()`).

**Acceptance Criteria**:

1. WHEN o desenvolvedor chama `req.Body().Raw()` THEN SHALL retornar `[]byte` (mesmo comportamento do antigo `RawBody()`)
2. WHEN o desenvolvedor chama `req.Body().Text()` THEN SHALL retornar `string(Raw())`
3. WHEN o desenvolvedor chama `req.Body().Json()`/`req.Body().Form(onFile)` THEN SHALL continuar retornando `Parseable`, comportamento inalterado da feature anterior

**Independent Test**: testes existentes de `unified-parse-api` migrados continuam verdes; novos testes cobrem `Raw()`/`Text()`.

---

### P1: Consolidar `Response` (Json/Html/Text) ⭐ MVP

**User Story**: Como desenvolvedor, quero que cada método de escrita de
`Response` force o Content-Type correspondente, e que não exista um método
"neutro" (`SendString`) que escreve sem Content-Type.

**Why P1**: Consistência de API — evita a pergunta "qual método eu uso pra
escrever texto puro" ter uma resposta ambígua (`SendString` vs `HTML`).

**Acceptance Criteria**:

1. WHEN o desenvolvedor chama `res.Html(s)` THEN SHALL setar `Content-Type: text/html` e escrever `s` (mesmo comportamento do antigo `HTML`, só renomeado)
2. WHEN o desenvolvedor chama `res.Text(s)` THEN SHALL setar `Content-Type: text/plain` e escrever `s` (COMPORTAMENTO NOVO — `SendString` não setava Content-Type)
3. WHEN o código busca por `SendString` THEN SHALL não existir mais como método de `Response` (removido)
4. WHEN o desenvolvedor chama `res.Status(code)` THEN SHALL setar o status e retornar `*Response` (chaining); `res.StatusCode()` SHALL ler o status atual

**Independent Test**: `TestResponse_Text_SetsPlainTextContentType` (novo) prova o Content-Type via dispatch real; `grep -r "SendString" --include=*.go` retorna vazio fora de `.specs`/STATE.md.

---

## Edge Cases

- WHEN o fallback genérico de 500 (panic não-`Exception`) roda em `internal/adapter/fiber` THEN SHALL continuar usando `fiber.Ctx` cru diretamente (não `Response.Text()`) — já documentado no código como último recurso, deliberadamente fora do wrapper
- WHEN um Filter customizado (`Module.Filters`) reescreve o status dentro do seu handler THEN `res.StatusCode()` SHALL refletir o valor mais recente (mesmo comportamento observável de hoje, só via `Response` em vez de `Context`)

---

## Requirement Traceability

| Requirement ID | Story                                         | Phase    | Status  |
| -------------- | ---------------------------------------------- | -------- | ------- |
| REQRES-01      | P1: Split Request/Response — Handler            | Execute  | Verified |
| REQRES-02      | P1: Split Request/Response — Guard/Middleware/Interceptor/Filter | Execute  | Verified |
| REQRES-03      | P1: Split Request/Response — next(req,res)      | Execute  | Verified |
| REQRES-04      | P1: Split Request/Response — Response.Request() | Execute  | Verified |
| REQRES-05      | P1: Request.Body().Raw()/.Text()                | Execute  | Verified |
| REQRES-06      | P1: Response.Html/Text content-type forçado     | Execute  | Verified |
| REQRES-07      | P1: Remoção de SendString                       | Execute  | Verified |
| REQRES-08      | P1: Response.Status(code)/StatusCode()          | Execute  | Verified |

---

## Success Criteria

- [ ] `go test ./... -race` passa após migração completa
- [ ] Zero ocorrência de `RestContext`/`execution.Context`/`SendString` fora de `.specs`/STATE.md
- [ ] `.examples/simple-todo` e `.examples/blog-api` migrados e buildando
- [ ] `Request`/`Response` existem e funcionam corretamente via dispatch HTTP real
