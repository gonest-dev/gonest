# reflexão: Suporte a GraphQL (Futuro)

A adoção do sistema de builders e o uso de `gonest.Schema` (que hoje já consolida validação em runtime e geração de OpenAPI) criam um caminho extremamente natural e elegante para suportar **GraphQL** no futuro. Podemos reaproveitar quase toda a infraestrutura base do framework, mudando apenas a ponta de exposição e aproveitando o modelo mental que o próprio NestJS adotou para GraphQL: **Resolvers ao invés de Controllers**.

No NestJS, o GraphQL é construído usando os decorators `@Resolver`, `@Query`, `@Mutation` e `@Args`. No Gonest, adotaríamos a mesma filosofia baseada em closures (builders) semânticos, mantendo a consistência do framework sem recorrer à pesada reflexão de anotações.

## 1. Resolvers como Porteiros do GraphQL

Um `Resolver` atuaria de forma análoga a um `Controller`. Ele pertenceria a um Módulo e consumiria suas dependências normalmente (via `MustInject`), sendo registrado via `module.Resolvers(...)`. 

```go
var UserResolver = gonest.NewResolver(func (resolver *gonest.Resolver) {
  // Injeção de dependência funciona de forma idêntica a um Controller
  userService := gonest.MustInject[*UserService](resolver)

  // Equivalente ao @Query() do Nest
  resolver.Query("getUser", func (query *gonest.Query) {
    // Reutilizamos a MESMA gonest.Schema do OpenAPI/HTTP para validar argumentos!
    query.Args(userIdParamsSchema) 
    
    // E reutilizamos para tipar o retorno no Schema do GraphQL
    query.Returns(userEntitySchema) 

    query.Handler(func(ctx *gonest.GraphqlContext) any {
      // Reaproveita a MESMA API unificada do REST (gonest.MustParse[T] +
      // Parseable, unified-parse-api feature) -- GraphqlContext só precisa
      // expor Args() Parseable, igual gonest.Request (REST) expõe
      // Params()/Query()/Headers()/Body() hoje. É um tipo DIFERENTE
      // (GraphQL não é HTTP), mas satisfaz o mesmo contrato Parseable.
      args := gonest.MustParse[UserIdParams](ctx.Args(), userIdParamsSchema)

      // O valor retornado (any) já É o data do resolver -- igual ao NestJS,
      // sem precisar de um res.Data()/Response separado (GraphQL não tem
      // status/headers para justificar um write-side rico como o REST tem).
      return userService.Get(args.UserId)
    })
  })

  // Equivalente ao @Mutation() do Nest
  resolver.Mutation("createUser", func (mutation *gonest.Mutation) {
    // O mesmo schema do REST pode ser consumido como Input no GraphQL
    mutation.Args(userEntitySchema) 
    mutation.Returns(userEntitySchema)
    
    mutation.Handler(func(ctx *gonest.GraphqlContext) any {
      input := gonest.MustParse[UserEntity](ctx.Args(), userEntitySchema)
      return userService.Create(input)
    })
  })

  // Equivalente ao @Subscription() do Nest -- FUNDAMENTALMENTE diferente de
  // Query/Mutation: não é request-response (1 valor, retorna e acabou), é um
  // STREAM (o resolver fica vivo emitindo N valores enquanto o client
  // continua conectado via websocket). Por isso NÃO reusa Handler(ctx) any
  // -- não tem um "retorno" único pra devolver.
  resolver.Subscription("orderCreated", func(subscription *gonest.Subscription) {
    subscription.Args(orderFilterSchema)
    subscription.Returns(orderEntitySchema)

    // emit(value) publica um evento pro client a cada chamada; ctx.Done()
    // fecha quando o client desconecta (mesmo padrão de context.Context do
    // Go stdlib) -- o Handler só retorna quando a subscription termina.
    //
    // REAPROVEITA o gonest.Emitter que já existe (Event Emitter feature) em
    // vez de inventar um callback ad-hoc do serviço de domínio (a versão
    // anterior desta reflexão usava um `orderService.OnCreated(...)`
    // fictício, o que quebrava a promessa de "Uma Única Fonte de Verdade" da
    // conclusão -- o app já tem UM barramento de eventos, ele deveria ser o
    // mesmo consumido aqui). Emitter.Subscribe é um método NOVO proposto
    // (Emit/MustOn já existem e continuam servindo o caso "fire-and-forget,
    // handler estático registrado no bootstrap via module.Listeners" --
    // Subscribe cobre o caso complementar: "canal dinâmico, vivo só enquanto
    // ESTA conexão GraphQL específica durar", cancelado automaticamente
    // quando o ctx passado morre).
    subscription.Handler(func(ctx *gonest.GraphqlContext, emit func(any)) {
      args := gonest.MustParse[OrderFilter](ctx.Args(), orderFilterSchema)
      emitter := gonest.MustInject[*gonest.Emitter](subscription)

      events := emitter.Subscribe[OrderCreatedEvent](ctx.Done())
      for event := range events {
        if event.CustomerId == args.CustomerId {
          emit(event)
        }
      }
    })
  })
})
```

`orderService.Create(...)` (chamado pela Mutation lá em cima) continuaria
disparando `emitter.Emit(OrderCreatedEvent{...})` normalmente -- a
Subscription não precisa saber NADA sobre quem publica o evento, só assina
o TIPO `OrderCreatedEvent`, exatamente como um `Listener` registrado via
`gonest.MustOn` já faz hoje.

Erro dentro de uma Subscription não pode seguir o mesmo panic/recover de
Query/Mutation (e do resto do framework -- Guard/Middleware/Filter): um
panic ali derrubaria a goroutine da conexão inteira sem jeito de avisar só
AQUELE client, já que não existe mais um único "request" para a exception
virar resposta de. Precisaria de um canal de erro próprio (ex: `emitError(err)`
ao lado de `emit(value)`), fechando a subscription daquele client
graciosamente sem afetar as outras conexões ativas -- ainda em aberto, não
resolvido nesta reflexão.

### Qual lib GraphQL Go por baixo? DECIDIDO: `graphql-go/graphql`

Pesquisado (web, 2026-07): `99designs/gqlgen` é **schema-first** (você
escreve o `.graphql`/SDL à mão, roda `go generate`, a lib produz os
bindings Go) -- colide com a filosofia do resto do gonest, onde
`Schema`/`PropertyBuilder` já funcionam 100% em runtime, sem etapa de
build. `graphql-go/graphql` é **code-first** (schema resolvido via
`graphql.NewSchema(...)` direto em Go, em runtime) -- alinhado.

Achado real que confirma a escolha, não só teoria: a issue histórica sobre
Subscription no `graphql-go/graphql` (github.com/graphql-go/graphql/issues/49,
fechada em 2016) mostra que a lib só resolveu o suporte **sintático**
(parsear `subscription Baz { ... }`, declarar um `SubscriptionRoot` no
schema) -- NUNCA veio com um motor de execução/streaming pronto (sem
`Subscribe()` devolvendo canal, sem transporte WebSocket/SSE embutido).
Isso poderia parecer um ponto contra, mas na prática NÃO é: o gonest já
ia construir sua PRÓPRIA camada de execução de Subscription por cima do
`gonest.Emitter` (ver seção 1 acima) de qualquer forma -- o que a lib de
subscription pronta do `gqlgen` daria de graça, o gonest não usaria do
mesmo jeito mesmo se disponível. `graphql-go/graphql` ganha a filosofia
code-first/runtime sem abrir mão de nada que realmente seria aproveitado.
Repo confirmado ativo (não abandonado: push em 2026-06-23, 10k+ stars).

**Consequência direta**: o gonest terá que escrever sua PRÓPRIA solução de
transporte de Subscription, cobrindo tanto **SSE** (Server-Sent Events)
quanto **WebSocket** -- nenhum dos dois vem pronto da lib escolhida. Os
protocolos de referência (`graphql-ws`, `graphql-transport-ws` para WS;
`graphql-sse` para SSE) existem como especificação pública e podem servir
de guia de compatibilidade com clients existentes (Apollo Client, urql,
etc), mesmo implementando o transporte por conta própria.

## 2. Geração Code-First do Schema GraphQL (SDL)

Como o `gonest.Schema` já retém nativamente os tipos fundamentais (`String`, `Integer`, `Array`, `Object`) e suas restrições lógicas (ex: `Required()`), nós podemos compilar automaticamente um **Schema GraphQL (SDL)** exatamente com a mesma inteligência do motor que usamos para a especificação do Swagger/OpenAPI.

O framework leria o `userEntitySchema` (que validava os requests no REST) e o traduziria de forma 1:1 para os equivalentes em GraphQL:

```graphql
type UserEntity {
  Id: Int!
  Name: String!
  Address: AddressEntity!
  Addresses: [AddressEntity!]!
}

input UserEntityInput {
  Id: Int!
  Name: String!
  # ...
}
```

### Branches de formato (Email/Uuid/DateTime/etc) viram Custom Scalars

Todo branch de formato que já existe hoje no `PropertyBuilder` (String-family
Branches feature: `Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/
`Byte`/`Binary`, Date/Time Branches feature: `DateTime`/`Date`) já mapeia
1:1 para o OpenAPI `format` correspondente (`email`, `uuid`, `date-time`,
etc). O GraphQL nativo só tem 5 scalars embutidos (`Int`, `Float`, `String`,
`Boolean`, `ID`) -- nenhum deles cobre "isso é um e-mail" ou "isso é uma
data" nativamente, então cada um desses formatos viraria um **Custom
Scalar** declarado uma vez no SDL gerado, reaproveitando exatamente a mesma
declaração de campo que já existe para REST:

```go
type UserEntity struct {
  Id        int64     `json:"id"`
  Email     string    `json:"email"`
  Website   string    `json:"website"`
  CreatedAt time.Time `json:"createdAt"`
  BirthDate time.Time `json:"birthDate"`
}

var userEntitySchema = gonest.NewSchema[UserEntity](func(t *UserEntity, m *gonest.Schema) {
  m.Property(&t.Id).Integer().Required()
  m.Property(&t.Email).Email().Required()       // format: email
  m.Property(&t.Website).Uri()                  // format: uri
  m.Property(&t.CreatedAt).DateTime().Required() // format: date-time
  m.Property(&t.BirthDate).Date()               // format: date (sem hora)
})
```

Gera o SDL abaixo -- cada `format` distinto usado em QUALQUER schema do app
vira uma declaração `scalar` (uma vez só, não repetida por campo), e o
campo aponta pro scalar em vez do tipo primitivo cru:

```graphql
scalar Email
scalar URI
scalar DateTime
scalar Date

type UserEntity {
  Id: Int!
  Email: Email!
  Website: URI
  CreatedAt: DateTime!
  BirthDate: Date
}
```

A validação de `serialize`/`parseValue`/`parseLiteral` de cada scalar (o
que uma lib GraphQL Go como `gqlgen`/`graphql-go` exige para um Custom
Scalar funcionar de verdade) reaproveitaria a MESMA lógica de
`validatePrimitive`/`Pattern` que já valida esses formatos hoje para REST
(`internal/validate`) -- um `Email`/`Uuid`/`DateTime` inválido seria
rejeitado na entrada do resolver, antes mesmo de `gonest.MustParse[T]`
rodar, com a idêntica mensagem de violação que o REST já produz.

### Tipos verdadeiramente customizados (ex: `primitive.ObjectID` do MongoDB)

Os formatos acima (Email/Uuid/DateTime/etc) têm um `format` OpenAPI padrão
conhecido de antemão, então o framework sabe automaticamente que nome de
scalar gerar. Mas tipos de domínio/biblioteca (o exemplo clássico é
`primitive.ObjectID` do driver oficial do MongoDB -- um `[12]byte`
serializado como hex de 24 caracteres, sem nenhum `format` OpenAPI
equivalente) não têm essa correspondência pronta -- já são resolvidos hoje
para REST via `PropertyBuilder.Custom(fn)` (o escape hatch existente,
Param/Query Validation feature), mas `Custom(fn)` sozinho não diz ao gerador
de SDL QUAL NOME de scalar GraphQL usar (`Custom` é genérico por natureza,
o framework não tem como adivinhar "isso representa um ObjectID" só olhando
pra uma func).

Por isso a proposta é `Custom(fn)` ganhar um modificador
`.GraphqlScalar(name)`, só relevante quando o `Schema` for usado através de
um Resolver -- REST/OpenAPI ignoram esse valor por completo:

```go
type PostEntity struct {
  Id       primitive.ObjectID `json:"id" bson:"_id"`
  AuthorId primitive.ObjectID `json:"authorId" bson:"authorId"`
  Title    string             `json:"title"`
}

var postEntitySchema = gonest.NewSchema[PostEntity](func(t *PostEntity, m *gonest.Schema) {
  // decodeObjectID já é o padrão Custom(fn) de hoje: recebe o raw (string
  // hex vindo do REST OU do GraphQL, mesmo formato em ambos os transportes)
  // e devolve o valor tipado já convertido.
  m.Property(&t.Id).Custom(decodeObjectID).GraphqlScalar("ObjectID").Required()
  m.Property(&t.AuthorId).Custom(decodeObjectID).GraphqlScalar("ObjectID").Required()
  m.Property(&t.Title).String().Required()
})

func decodeObjectID(raw any) (any, error) {
  s, ok := raw.(string)
  if !ok {
    return nil, errors.New("expected a 24-char hex string")
  }
  return primitive.ObjectIDFromHex(s)
}
```

Gera:

```graphql
scalar ObjectID

type PostEntity {
  Id: ObjectID!
  AuthorId: ObjectID!
  Title: String!
}
```

Dois `Custom(fn)` diferentes que chamam `.GraphqlScalar("ObjectID")` com o
MESMO nome (`Id` e `AuthorId` acima) gerariam UMA declaração `scalar
ObjectID` só, igual ao dedup por identidade que `internal/openapi.Generate`
já faz hoje pra `$ref`/`components.schemas` reusados -- não uma declaração
repetida por campo. Chamar `.Custom(fn)` sem `.GraphqlScalar(name)` num
schema que acaba sendo usado por um Resolver seria um erro de configuração
detectável em BUILD TIME da geração de SDL (mesma categoria de falha que
`resolveSchema`'s panic de schema mismatch -- um erro de programador, não
uma falha de request), já que o gerador não teria nome nenhum pra dar ao
scalar.

### Conclusão dessa reflexão

Ao invés de criarmos tipos isolados, structs repetidas, e usar strings reflexivas apenas para o GraphQL (como é comum em bibliotecas genéricas de Go), a infraestrutura do Gonest transforma o `gonest.Schema` e os construtores de `Module/Provider/Resolver` em **Uma Única Fonte de Verdade**. 

Você declararia a regra, a dependência e o modelo de dados uma única vez, e o Gonest orquestraria isso para servir 4 propósitos simultaneamente:

1. Injeção de dependência universal (Providers independentes de HTTP/GQL)
2. Validação unificada em runtime -- a MESMA dupla `gonest.Parse[T]`/`gonest.MustParse[T]` que já existe para REST hoje (unified-parse-api feature), agora também alimentada por `ctx.Args()` no lugar de `req.Params()`/`req.Query()`/`req.Body().Json()`
3. Geração Automática de Documentação OpenAPI (REST)
4. Geração Automática de Schema SDL (GraphQL)

Dessa forma, Controllers REST e Resolvers GraphQL seriam apenas "transportes/adaptadores", permitindo que uma aplicação escale para os dois mundos utilizando rigorosamente a mesma base de código de negócio e validação.
