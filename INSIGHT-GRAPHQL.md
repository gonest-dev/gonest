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
})
```

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

### Conclusão dessa reflexão

Ao invés de criarmos tipos isolados, structs repetidas, e usar strings reflexivas apenas para o GraphQL (como é comum em bibliotecas genéricas de Go), a infraestrutura do Gonest transforma o `gonest.Schema` e os construtores de `Module/Provider/Resolver` em **Uma Única Fonte de Verdade**. 

Você declararia a regra, a dependência e o modelo de dados uma única vez, e o Gonest orquestraria isso para servir 4 propósitos simultaneamente:

1. Injeção de dependência universal (Providers independentes de HTTP/GQL)
2. Validação unificada em runtime -- a MESMA dupla `gonest.Parse[T]`/`gonest.MustParse[T]` que já existe para REST hoje (unified-parse-api feature), agora também alimentada por `ctx.Args()` no lugar de `req.Params()`/`req.Query()`/`req.Body().Json()`
3. Geração Automática de Documentação OpenAPI (REST)
4. Geração Automática de Schema SDL (GraphQL)

Dessa forma, Controllers REST e Resolvers GraphQL seriam apenas "transportes/adaptadores", permitindo que uma aplicação escale para os dois mundos utilizando rigorosamente a mesma base de código de negócio e validação.
