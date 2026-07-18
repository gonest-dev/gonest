# Suporte a GraphQL -- implementado

O sistema de builders e o uso de `gonest.Schema` (que já consolida validação em runtime e geração de OpenAPI) criaram um caminho natural para suportar **GraphQL**, reaproveitando quase toda a infraestrutura base do framework -- só a ponta de exposição muda, seguindo o mesmo modelo mental que o NestJS adotou: **Resolvers ao invés de Controllers**.

## 1. Resolvers

Um `GraphqlResolver` funciona de forma análoga a um `Controller`: pertence a um Módulo, consome dependências via `MustInject`, e é registrado via `module.Resolvers(...)`.

```go
var UserResolver = gonest.NewGraphqlResolver(func(resolver *gonest.GraphqlResolver) {
  userService := gonest.MustInject[*UserService](resolver)
  // MustInject's owner must be resolver itself (or another Resolver/
  // Controller) -- Query/Mutation/Subscription builders below have no DI
  // resolution capability of their own, only Resolver does.

  // Equivalente ao @Query() do Nest
  resolver.Query("getUser", func(query *gonest.GraphqlQuery) {
    // Reutiliza a MESMA gonest.Schema do OpenAPI/HTTP para validar argumentos
    query.Args(userIdParamsSchema)

    // E para tipar o retorno no SDL do GraphQL
    query.Returns(userEntitySchema)

    query.Handler(func(ctx *gonest.GraphqlContext) any {
      // Reaproveita a MESMA API unificada do REST (gonest.MustParse[T] +
      // Parseable, unified-parse-api feature) -- ctx.Args() é um
      // execution.Parseable, igual req.Params()/Query()/Body().Json() já
      // são para REST.
      args := gonest.MustParse[UserIdParams](ctx.Args(), userIdParamsSchema)

      // O valor retornado (any) já É o data do resolver -- igual NestJS,
      // sem um Response/write-side separado (GraphQL não tem status/
      // headers para justificar um).
      return userService.Get(args.UserId)
    })
  })

  // Equivalente ao @Mutation() do Nest -- gonest.GraphqlMutation é um
  // ALIAS de gonest.GraphqlQuery (mesmo shape hoje, só o root type que
  // acaba usando na SDL muda -- Query vs Mutation -- não o Go type).
  resolver.Mutation("createUser", func(mutation *gonest.GraphqlMutation) {
    mutation.Args(userEntitySchema)
    mutation.Returns(userEntitySchema)

    mutation.Handler(func(ctx *gonest.GraphqlContext) any {
      input := gonest.MustParse[UserEntity](ctx.Args(), userEntitySchema)
      return userService.Create(input)
    })
  })

  // Equivalente ao @Subscription() do Nest -- um STREAM, não request-
  // response: o resolver fica vivo emitindo N valores enquanto o client
  // segue conectado. Handler tem assinatura DIFERENTE (ctx, emit), não
  // reusa Handler(ctx) any de Query/Mutation.
  resolver.Subscription("orderCreated", func(subscription *gonest.GraphqlSubscription) {
    subscription.Args(orderFilterSchema)
    subscription.Returns(orderEntitySchema)

    subscription.Handler(func(ctx *gonest.GraphqlContext, emit func(any)) {
      args := gonest.MustParse[OrderFilter](ctx.Args(), orderFilterSchema)
      emitter := gonest.MustInject[*gonest.Emitter](resolver) // NOT subscription -- see note above

      // gonest.Subscribe[T] -- canal dinâmico, vivo só enquanto ESTA
      // conexão durar, fechado quando ctx.Done() dispara (SSE: falha de
      // escrita/heartbeat detecta desconexão; WebSocket: o próprio loop
      // bloqueante de leitura detecta). Complementar ao par estático
      // Emit/MustOn (Event Emitter feature) -- mesmo Emitter, uso diferente.
      events := gonest.Subscribe[OrderCreatedEvent](emitter, ctx.Done())
      for event := range events {
        if event.CustomerId == args.CustomerId {
          emit(event)
        }
      }
    })
  })
})
```

`orderService.Create(...)` (chamado pela Mutation acima) continua disparando `emitter.Emit(OrderCreatedEvent{...})` normalmente -- a Subscription não sabe nada sobre quem publica o evento, só assina o TIPO, exatamente como um `Listener` via `gonest.MustOn` já faz.

Um panic dentro de uma Subscription é recuperado (não derruba o processo), mas não tem canal de erro GraphQL-shaped próprio ainda -- gap reconhecido, aceito, ver "O que fica em aberto" abaixo.

### Transporte: `POST /graphql`, `GET /graphql/stream/:name` (SSE), `GET /graphql/ws/:name` (WebSocket)

Query/Mutation dispatcham através de UM endpoint fixo, `POST /graphql`, seguindo o formato padrão GraphQL-over-HTTP (`{query, variables, operationName}` → `{data, errors}`). Subscription usa dois endpoints à parte (SSE e WebSocket, ambos registrados automaticamente quando pelo menos uma `Subscription` existe) -- `:name` seleciona qual Subscription conectar, args chegam via `?args=<JSON>` na query string (não há corpo numa conexão SSE/WS de longa duração):

```
GET /graphql/stream/onOrderCreated?args={"customerId":42}   -- SSE
GET /graphql/ws/onOrderCreated?args={"customerId":42}       -- WebSocket
```

Ambos os transportes chamam `Subscription.Handler` diretamente -- nenhum passa pelo motor de execução do `graphql-go/graphql` (ele nunca trouxe um pronto, ver decisão de motor abaixo).

### Motor GraphQL: `graphql-go/graphql`

Pesquisado (web + `gh issue view`, 2026-07): `99designs/gqlgen` é **schema-first** (SDL escrito à mão + `go generate`) -- colide com a filosofia runtime-only do resto do gonest. `graphql-go/graphql` é **code-first** (`graphql.NewSchema(...)` direto em Go) -- alinhado, escolhido.

A issue histórica sobre Subscription (github.com/graphql-go/graphql/issues/49, fechada em 2016) confirma que a lib só resolveu o suporte SINTÁTICO -- nunca veio com motor de execução/streaming pronto. Isso não pesou contra a escolha: o gonest já ia construir sua própria camada de execução via `gonest.Subscribe`/SSE/WebSocket de qualquer forma.

**Consequência aceita**: o gonest escreveu sua PRÓPRIA solução de transporte de Subscription, SSE e WebSocket (via `github.com/gofiber/contrib/v3/websocket`), sem seguir `graphql-ws`/`graphql-transport-ws`/`graphql-sse` byte-a-byte (só compatibilidade conceitual).

## 2. Geração Code-First do SDL

`gonest.Schema` já retém os tipos fundamentais (`String`, `Integer`, `Array`, `Object`) e suas restrições (`Required()`, etc) -- o framework traduz isso 1:1 para o SDL GraphQL equivalente, a partir do MESMO `userEntitySchema` que já valida requests REST.

### Branches de formato viram Custom Scalars

Todo branch de formato (`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`/`DateTime`/`Date`) já mapeia 1:1 pro `format` OpenAPI correspondente -- o GraphQL nativo só tem 5 scalars embutidos (`Int`/`Float`/`String`/`Boolean`/`ID`), nenhum cobre "isso é um e-mail". Cada formato usado vira um Custom Scalar declarado uma vez no SDL:

```go
type UserEntity struct {
  Id        int64     `json:"id"`
  Email     string    `json:"email"`
  Website   string    `json:"website"`
  CreatedAt time.Time `json:"createdAt"`
}

var userEntitySchema = gonest.NewSchema(func(t *UserEntity, m *gonest.Schema) {
  m.Property(&t.Id).Integer().Required()
  m.Property(&t.Email).Email().Required()
  m.Property(&t.Website).Uri()
  m.Property(&t.CreatedAt).DateTime().Required()
})
```

Nomes de scalar gerados (`internal/graphql`'s `NativeScalarName`): `Email`, `Uuid`, `Uri`, `Hostname`, `Ipv4`, `Ipv6`, `Password`, `Byte`, `Binary`, `DateTime`, `Date` -- deduplicados por nome (não por campo) através de TODO o app.

### Tipos verdadeiramente customizados (ex: `primitive.ObjectID` do MongoDB)

Um tipo sem `format` OpenAPI equivalente (resolvido hoje via `Custom(fn)`) não tem nome de scalar óbvio. `Custom(fn)` ganhou `.GraphqlScalar(name)`, relevante só quando o `Schema` é usado por um Resolver -- REST/OpenAPI ignoram esse valor:

```go
m.Property(&t.Id).Custom(decodeObjectID).GraphqlScalar("ObjectID").Required()
```

Dois campos com o MESMO `.GraphqlScalar("ObjectID")` deduplicam pra uma única declaração `scalar ObjectID`. `Custom(fn)` sem `.GraphqlScalar(name)` num schema usado por Resolver falha em BUILD TIME da geração de SDL (`internal/graphql.Build` retorna erro), não em tempo de request.

## Como foi implementado

- Todo o código real vive num pacote ÚNICO, `internal/graphql` (não 3 pacotes separados como cogitado durante o design) -- `Resolver`/`Query`/`Mutation`/`Subscription`/`GraphqlContext` (builder), `Build` (Schema→SDL/`graphql-go/graphql`), `SSEHandler`/`WSHandler` (transportes). Toda referência à lib EXTERNA `graphql-go/graphql` dentro desse pacote é import-aliased (`gql "github.com/graphql-go/graphql"`) para não colidir com o nome do próprio pacote.
- `Module.Resolvers`/`OwnResolvers` -- mesma forma de `Controllers`/`OwnControllers`.
- Dispatch real de Query/Mutation: `graphql.Do` (motor real do `graphql-go/graphql`) é quem invoca `Resolve` por campo -- os callbacks `Resolve` são construídos DENTRO de `Build`, chamando `Query.HandlerFunc()`/`Mutation.HandlerFunc()` diretamente e recuperando um panic de `gonest.MustParse` como erro GraphQL-shaped (`{errors: [...]}`).
- `internal/execution.Responder` ganhou `WriteStream` (via `fasthttp.SetBodyStreamWriter` real) -- necessário pra SSE, não existia forma de "manter conexão aberta, escrever aos poucos" antes desta feature.
- `HttpAdapter` ganhou `RegisterWebSocket` -- via `github.com/gofiber/contrib/v3/websocket` real (`IsWebSocketUpgrade` + `websocket.New`).
- API pública em `gonest.go`: `GraphqlResolver`/`NewGraphqlResolver`/`GraphqlQuery`/`GraphqlMutation`/`GraphqlSubscription`/`GraphqlContext`/`Subscribe[T]`.

## O que continua em aberto (não resolvido nesta feature)

- Canal de erro próprio dentro de Subscription (`emitError(err)` ao lado de `emit(value)`) -- panic é recuperado (não derruba o processo), mas sem notificar o client com uma mensagem GraphQL-shaped.
- Endpoint `/graphql` fixo, não configurável via `AppOptions` ainda.
- `Refine`/`Sanitize` (schema-sanitize-refine feature) não conectados a Args de GraphQL.
- Federation/schema stitching entre múltiplos serviços -- fora de escopo, v1 é um único schema por app.

## Conclusão

Ao invés de criar tipos isolados, structs repetidas, e reflexão ad-hoc só para o GraphQL, a infraestrutura do gonest transforma `gonest.Schema` e os construtores de `Module`/`Provider`/`GraphqlResolver` em **Uma Única Fonte de Verdade**: a mesma regra, dependência e modelo de dados servem (1) injeção de dependência universal, (2) validação unificada em runtime (`Parse[T]`/`MustParse[T]`, alimentado por `req.Params()`/`Query()`/`Body().Json()` OU `ctx.Args()`), (3) geração de OpenAPI (REST), (4) geração de SDL (GraphQL) -- Controllers e Resolvers são só "transportes/adaptadores" sobre a mesma base de código.
