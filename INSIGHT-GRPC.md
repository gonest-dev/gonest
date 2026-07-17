# reflexão: Suporte a gRPC (Futuro)

## Abstração de Contexto Universal (REST, GraphQL e gRPC)

O REST já andou nessa direção: `*gonest.RestContext` (um `ctx` monolítico) foi
substituído por `*gonest.Request`/`*gonest.Response` (request-response-split
feature), e toda leitura de dado passa por um valor `gonest.Parseable`
(`req.Params()`, `req.Query()`, `req.Headers()`, `req.Body().Json()`,
`req.Body().Form(onFile)`) consumido por **dois** entry points genéricos e
já existentes: `gonest.Parse[T](src Parseable, schema *Schema) (T, error)` e
`gonest.MustParse[T](src Parseable, schema *Schema) T`.

Para suportar múltiplos transportes, essa MESMA arquitetura já vale --
`Parseable` não é específico de REST, é só "algo que sabe popular um `*T` a
partir do schema" (`ParseInto(dst any, schema any) error`, ver
`execution.Parseable`). GraphQL e gRPC não precisam de um conjunto de funções
`Must*` novo por transporte -- precisam apenas que seus próprios tipos de
contexto (`*gonest.GraphRequest`, `*gonest.GrpcContext`) exponham métodos que
devolvem um `Parseable`, do mesmo jeito que `Request` já faz. O compilador do
Go continua sendo o guardião: `*gonest.GrpcContext` simplesmente NÃO tem um
método `Query()`, então tentar chamar `gonest.MustParse[T](ctx.Query(), ...)`
nele já não compila -- não é preciso uma interface `Carrier` separada por
capacidade, o próprio método ausente já impede o uso errado.

```go
package gonest

// Parseable já existe (unified-parse-api feature) -- nenhuma interface nova
// aqui, cada transporte só precisa devolver um Parseable dos seus próprios
// métodos de leitura.
type Parseable interface {
  ParseInto(dst any, schema any) error
}

// GrpcContext expõe os mesmos métodos-fábrica de Parseable que Request já
// expõe para REST -- Metadata() no lugar de Headers(), Payload() no lugar
// de Body().Json() (protobuf em vez de JSON por baixo, mas o contrato
// Parseable/Parse[T]/MustParse[T] não muda).
type GrpcContext struct { /* ... */ }
func (ctx *GrpcContext) Metadata() Parseable { /* ... */ }
func (ctx *GrpcContext) Payload() Parseable  { /* ... */ }
```

### Estresse de Hipótese: Compartilhando Validação entre REST e gRPC

Se no futuro o Gonest suportar **gRPC**, o `GrpcContext` possuirá a capacidade de acessar "Metadata" (o equivalente do gRPC aos Headers HTTP) e de extrair o payload Protobuf. 

O mesmo `Schema` que valida um Controller REST pode validar um serviço gRPC sem qualquer alteração:

```go
var AuthHeaders = gonest.NewSchema[authHeaders](func(h *authHeaders, m *gonest.Schema) {
  m.Property(&h.Authorization).String().Required().Pattern("^Bearer ")
})

var CreateUserPayload = gonest.NewSchema[createUserPayload](func(p *createUserPayload, m *gonest.Schema) {
  m.Property(&p.Age).Integer().Min(18).Required()
  m.Property(&p.Email).String().Required() // valida formato
})

// ==========================================
// 1. Uso em um Controller REST (HTTP) -- API real de hoje
// ==========================================
var UserController = gonest.NewController(func(c *gonest.Controller) {
  c.Route(gonest.HttpPost, "/user", func(r *gonest.Route) {
    // Alimenta o Swagger...
    r.Headers(AuthHeaders)
    r.RequestBody(CreateUserPayload)

    r.Handler(func(req *gonest.Request, res *gonest.Response) {
      // Request expõe Headers()/Body().Json() como Parseable
      headers := gonest.MustParse[authHeaders](req.Headers(), AuthHeaders)
      payload := gonest.MustParse[createUserPayload](req.Body().Json(), CreateUserPayload)

      // ... executa negócio
      res.Json(payload)
    })
  })
})

// ==========================================
// 2. Uso em um Serviço gRPC (Protobuf/HTTP2) -- hipotético
// ==========================================
var UserGrpcService = gonest.NewGrpcService("UserService", func(s *gonest.GrpcService) {
  s.Method("CreateUser", func(m *gonest.GrpcMethod) {
    // Aqui não tem Swagger, mas podemos usar o Schema para injetar 
    // validações na geração do .proto (no futuro!)

    m.Handler(func(ctx *gonest.GrpcContext) {
      // GrpcContext.Metadata() lê do metadata.MD do gRPC, GrpcContext.Payload()
      // faz unmarshal do Protobuf -- ambos Parseable, MESMA gonest.MustParse[T]
      // usada acima para REST. A VALIDAÇÃO É EXATAMENTE A MESMA.
      headers := gonest.MustParse[authHeaders](ctx.Metadata(), AuthHeaders)
      payload := gonest.MustParse[createUserPayload](ctx.Payload(), CreateUserPayload)

      // ... executa negócio idêntico
    })
  })
})

// ==========================================
// O que NÃO compila (Segurança Absoluta)
// ==========================================
var BadGrpcService = gonest.NewGrpcService("Bad", func(s *gonest.GrpcService) {
  s.Method("WillFail", func(m *gonest.GrpcMethod) {
    m.Handler(func(ctx *gonest.GrpcContext) {
      // ERRO DE COMPILAÇÃO:
      // ctx.Query undefined (type *gonest.GrpcContext has no field or method Query)
      // -- GrpcContext nunca implementou Query() pra começo de conversa, não
      // existe uma interface "Carrier" separada pra violar, só um método que
      // não existe no tipo.
      query := gonest.MustParse[someQuery](ctx.Query(), SomeQuerySchema)
    })
  })
})
```

A combinação de **Schemas que detém a inteligência de Extração/Validação** com **Contextos que só expõem os métodos que fazem sentido pro seu próprio transporte** (sem interface `Carrier` separada -- o método ausente já é a barreira de compilação) resolve o problema de roteamento e validação universal de ponta a ponta no Go de uma maneira absurdamente Type-Safe.
