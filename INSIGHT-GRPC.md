# reflexão: Suporte a gRPC (Futuro)

## Abstração de Contexto Universal (REST, GraphQL e gRPC)

Para suportar múltiplos transportes de forma segura, o Gonest abandonaria o `*gonest.Context` monolítico em favor de contextos tipados (`*gonest.HttpContext`, `*gonest.GraphContext`, `*gonest.GrpcContext`) que satisfazem **Interfaces de Transporte** específicas.

Dessa forma, o `gonest.Schema` provê os métodos de Parse exigindo apenas a interface correta. O compilador do Go passa a ser o seu guardião: é impossível tentar ler uma "Query String" de um contexto gRPC.

```go
package gonest

// Interfaces de Capacidade (Traits do Transporte)
type HeaderCarrier interface { Header(key string) string }
type QueryCarrier  interface { QueryParam(key string) string }
type ParamCarrier  interface { Param(key string) string }
type ArgsCarrier   interface { Args() map[string]any } // Específico GraphQL
type PayloadCarrier interface { Bind(target any) error } // JSON (REST) ou Protobuf (gRPC)

// gonest expõe funções Must* que exigem a interface (carrier) correta do Contexto.
// O Schema é passado como 2º argumento (decisão arquitetural AD-019).
func MustHeaders[T any](ctx HeaderCarrier, schema *Schema[T]) *T { /* ... */ }
func MustQuery[T any](ctx QueryCarrier, schema *Schema[T]) *T { /* ... */ }
func MustParams[T any](ctx ParamCarrier, schema *Schema[T]) *T { /* ... */ }
func MustArgs[T any](ctx ArgsCarrier, schema *Schema[T]) *T { /* ... */ }
func MustPayload[T any](ctx PayloadCarrier, schema *Schema[T]) *T { /* ... */ }
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
// 1. Uso em um Controller REST (HTTP)
// ==========================================
var UserController = gonest.NewController(func(c *gonest.Controller) {
  c.Route(gonest.HttpPost, "/user", func(r *gonest.Route) {
    // Alimenta o Swagger...
    r.Headers(AuthHeaders)
    r.RequestBody(CreateUserPayload)
    
    r.Handler(func(ctx *gonest.HttpContext) {
      // HttpContext implementa HeaderCarrier e PayloadCarrier
      headers := gonest.MustHeaders[*authHeaders](ctx, AuthHeaders)
      payload := gonest.MustPayload[*createUserPayload](ctx, CreateUserPayload)
      
      // ... executa negócio
    })
  })
})

// ==========================================
// 2. Uso em um Serviço gRPC (Protobuf/HTTP2)
// ==========================================
var UserGrpcService = gonest.NewGrpcService("UserService", func(s *gonest.GrpcService) {
  s.Method("CreateUser", func(m *gonest.GrpcMethod) {
    // Aqui não tem Swagger, mas podemos usar o Schema para injetar 
    // validações na geração do .proto (no futuro!)
    
    m.Handler(func(ctx *gonest.GrpcContext) {
      // GrpcContext implementa HeaderCarrier (lê do metadata.MD do gRPC)
      // e PayloadCarrier (faz unmarshal do Protobuf).
      // A VALIDAÇÃO É EXATAMENTE A MESMA.
      headers := gonest.MustHeaders[*authHeaders](ctx, AuthHeaders)
      payload := gonest.MustPayload[*createUserPayload](ctx, CreateUserPayload)
      
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
      // Cannot use 'ctx' (type *gonest.GrpcContext) as the type gonest.QueryCarrier 
      // Type does not implement 'gonest.QueryCarrier' as it lacks the 'QueryParam' method.
      query := gonest.MustQuery[*someQuery](ctx, SomeQuerySchema) 
    })
  })
})
```

A combinação de **Schemas que detém a inteligência de Extração/Validação** com **Contextos baseados em Interfaces (Traits)** resolve o problema de roteamento e validação universal de ponta a ponta no Go de uma maneira absurdamente Type-Safe.
