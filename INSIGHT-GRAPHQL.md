# reflexão: Suporte a GraphQL (Futuro)

A adoção do sistema de builders e o uso de `gonest.Schema` (que hoje já consolida validação em runtime e geração de OpenAPI) criam um caminho extremamente natural e elegante para suportar **GraphQL** no futuro. Podemos reaproveitar quase toda a infraestrutura base do framework, mudando apenas a ponta de exposição e aproveitando o modelo mental que o próprio NestJS adotou para GraphQL: **Resolvers ao invés de Controllers**.

No NestJS, o GraphQL é construído usando os decorators `@Resolver`, `@Query`, `@Mutation` e `@Args`. No Gonest, adotaríamos a mesma filosofia baseada em closures (builders) semânticos, mantendo a consistência do framework sem recorrer à pesada reflexão de anotações.

## 1. Resolvers como Porteiros do GraphQL

Um `Resolver` atuaria de forma análoga a um `Controller`. Ele pertenceria a um Módulo e consumiria suas dependências normalmente (via `MustInject`), sendo registrado via `module.Resolvers(...)`. 

```go
var UserResolver = gonest.NewResolver(func (r *gonest.Resolver) {
  // Injeção de dependência funciona de forma idêntica a um Controller
  userService := gonest.MustInject[*UserService](r)

  // Equivalente ao @Query() do Nest
  r.Query("getUser", func (q *gonest.Query) {
    // Reutilizamos a MESMA gonest.Schema do OpenAPI/HTTP para validar argumentos!
    q.Args(userIdParamsSchema) 
    
    // E reutilizamos para tipar o retorno no Schema do GraphQL
    q.Returns(userEntitySchema) 

    q.Resolve(func(ctx *gonest.GraphContext) {
      // O mesmo schema do REST extrai os dados do GraphContext, agora passando o schema como argumento
      args := gonest.MustArgs[*UserIdParams](ctx, userIdParamsSchema)
      
      // ctx.Data sinaliza o retorno final do resolver (sem response HTTP)
      ctx.Data(userService.Get(args.UserId))
    })
  })

  // Equivalente ao @Mutation() do Nest
  r.Mutation("createUser", func (m *gonest.Mutation) {
    // O mesmo schema do REST pode ser consumido como Input no GraphQL
    m.Args(userEntitySchema) 
    m.Returns(userEntitySchema)
    
    m.Resolve(func(ctx *gonest.GraphContext) {
      input := gonest.MustArgs[*UserEntity](ctx, userEntitySchema)
      ctx.Data(userService.Create(input))
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
2. Validação unificada em runtime (`MustJsonBody`, `MustParams`, `MustArgs`)
3. Geração Automática de Documentação OpenAPI (REST)
4. Geração Automática de Schema SDL (GraphQL)

Dessa forma, Controllers REST e Resolvers GraphQL seriam apenas "transportes/adaptadores", permitindo que uma aplicação escale para os dois mundos utilizando rigorosamente a mesma base de código de negócio e validação.
