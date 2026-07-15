# exemplo mais simples

```go
package ex

import (
  "github.com/gonest-dev/gonest"
  "time"
)

type UserProperties struct {
  Name gonest.Value[string] `json:"name"`
  Age  gonest.Value[int]    `json:"age"`
}

type UserEntity struct {
  UserProperties
  ID        gonest.Value[int64]      `json:"id"`
  CreatedAt gonest.Value[time.Time]  `json:"created_at"`
  UpdatedAt gonest.Value[time.Time]  `json:"updated_at"`
  DeletedAt gonest.Value[*time.Time] `json:"deleted_at"`
}

// exemplo mínimo de serviço crud
type UserService struct {
  index int
  list  []*UserEntity
}
func (t *UserService) List() []*UserEntity { /* ... */ }
func (t *UserService) Get(userId int64) *UserEntity {
  user := /* busca... */
  if user == nil {
    panic(gonest.NewNotFoundException(map[string]any{"userId": userId}))
  }
  return user
}
func (t *UserService) Create(properties UserProperties) *UserEntity { /* ...; panica se der erro */ }
func (t *UserService) Update(userId int64, properties UserProperties) *UserEntity { /* ...; panica se der erro */ }
func (t *UserService) Delete(userId int64) *UserEntity { /* ...; panica se der erro */ }

var UserProvider = gonest.NewProvider(func (provider *gonest.Provider) {
  provider.Scope(gonest.ScopeSingleton)
  provider.Constructor(func() *UserService {
    return &UserService{index: 0, list: make([]*UserEntity, 0)}
  })
})

// UserIdParam é o "whole-object" de um único path param -- mesmo mecanismo
// de MustJsonBody, só que alimentado pelo segmento ":user_id" da rota em vez
// do corpo JSON (ver "exemplo de Param/Query Validation" mais abaixo pro
// detalhe completo, incluindo query string e Custom(fn)).
type UserIdParam struct {
  UserId int64 `param:"user_id"`
}

var _ = gonest.NewSchema[UserIdParam](func (t *UserIdParam, m *gonest.Schema) {
  m.Property(&t.UserId).Integer().Min(1).Required()
})

var UserController = gonest.NewController(func (controller *gonest.Controller) {
  // descritivos relacionados ao controller
  controller.Path("/user")

  // resolvendo dependências (placeholder resolvido de verdade no bootstrap do NewApp)
  userService := gonest.MustInject[*UserService](controller)

  // rotas do controller
  controller.Route(gonest.HttpQuery, "/", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.Context) {
      ctx.Json(userService.List())
    })
  })
  controller.Route(gonest.HttpGet, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.Context) {
      params := gonest.MustParams[*UserIdParam](ctx)
      ctx.Json(userService.Get(params.UserId))
    })
  })
  controller.Route(gonest.HttpPost, "/", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusCreated)
    route.Handler(func(ctx *gonest.Context) {
      properties := gonest.MustJsonBody[*UserProperties](ctx)
      ctx.Json(userService.Create(properties))
    })
  })
  controller.Route(gonest.HttpPut, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.Context) {
      params := gonest.MustParams[*UserIdParam](ctx)
      properties := gonest.MustJsonBody[*UserProperties](ctx)
      ctx.Json(userService.Update(params.UserId, properties))
    })
  })
  controller.Route(gonest.HttpDelete, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.Context) {
      params := gonest.MustParams[*UserIdParam](ctx)
      ctx.Json(userService.Delete(params.UserId))
    })
  })
})

var UserModule = gonest.NewModule(func (module *gonest.Module) {
  module.Providers(UserProvider)
  module.Controllers(UserController)
})

var AppModule = gonest.NewModule(func (module *gonest.Module) {
  module.Imports(UserModule)
})

func main() {
  app, err := gonest.NewApp[gonest.FiberApp](AppModule)
  if err != nil {
    panic(err)
  }
  defer app.Close()

  app.Listen(":3000")
}
```

# exemplo de exceptions (erro no body: `{ name, message, details }`)

```go
package ex

import (
  "github.com/gonest-dev/gonest"
)

// exceptions built-in do framework (todas embedam HttpException)
// gonest.NotFoundException     -> 404
// gonest.BadRequestException   -> 400
// gonest.ConflictException     -> 409
// gonest.UnauthorizedException -> 401
// gonest.ForbiddenException    -> 403

// exception de domínio, criada pelo dev do mesmo jeito que as built-in
type FooExampleError struct { gonest.HttpException }
func NewFooExampleError(details any) *FooExampleError {
  return &FooExampleError{
    HttpException: gonest.NewHttpException(gonest.HttpStatusBadRequest, "FooExampleError", "lorem ipsum dolor met", details),
  }
}

type FooService struct{}
func (t *FooService) ShouldThrow() { panic(NewFooExampleError(map[string]any{"input": input}))  }

// resposta HTTP quando panica com Exception (HttpException ou qualquer struct que a embeda):
// status: definido pela exception (ex: 400)
// body:
// {
//   "name": "FooExampleError",
//   "message": "lorem ipsum dolor met",
//   "details": { "input": "" }
// }
//
// panic que NÃO for Exception (nil pointer, index out of range etc) continua
// como 500 genérico, sem vazar detalhe interno no body.
```

# exemplo de Middleware, Guard, Interceptor e Filter

```go
package ex

import (
  "fmt"
  "strconv"
  "strings"
  "time"

  "github.com/gonest-dev/gonest"
  "github.com/google/uuid"
)

// ---------- Middleware ----------
// roda antes do roteamento (raw request/response), tipo express middleware.
// não decide autorização, só observa/mutação de contexto (log, request-id etc).
var RequestIdMiddleware = gonest.NewMiddleware(func (middleware *gonest.Middleware) {
  middleware.Handler(func(ctx *gonest.Context, next gonest.Next) {
    requestId, _ := uuid.NewV7()
    ctx.SetHeader("X-Request-Id", requestId.String())
    next(ctx)
  })
})

// ---------- Guard ----------
// decide se request pode prosseguir. retorna bool; false = 403 Forbidden automático.
// pra mensagem custom, panica com Exception própria em vez de retornar false.
var AuthGuard = gonest.NewGuard(func (guard *gonest.Guard) {
  authService := gonest.MustInject[*AuthService](guard)

  guard.Handler(func(ctx *gonest.Context) bool {
    token := ctx.Header("Authorization")
    if token == "" {
      panic(gonest.NewUnauthorizedException(nil))
    }
    return authService.Validate(token)
  })
})

// ---------- Interceptor ----------
// envolve a execução do handler (antes/depois), tipo AOP. pode medir tempo,
// mutar resposta, fazer cache etc. roda depois dos guards, antes/depois do handler.
var TimingInterceptor = gonest.NewInterceptor(func (interceptor *gonest.Interceptor) {
  logger := gonest.MustInject[*LoggerService](interceptor)

  interceptor.Handler(func(ctx *gonest.Context, next gonest.Next) {
    start := time.Now()
    next(ctx)
    logger.Log("request took", time.Since(start))
  })
})

// ---------- Filter ----------
// captura exceptions específicas e customiza status/body da resposta.
// exceptions não capturadas por nenhum filter caem no handler default (name/message/details).
var FooExampleFilter = gonest.NewFilter(func (filter *gonest.Filter) {
  filter.Catch(&FooExampleError{}, func(ctx *gonest.Context, exc *FooExampleError) {
    ctx.Status(gonest.HttpStatusTeapot).Json(map[string]any{
      "custom": true,
      "name":   exc.Name(),
    })
  })
})

// ---------- PrefixedUserIdParam (Custom(fn)) ----------
// Schema's fixed vocabulary (Integer/String/Min/Max/Pattern etc) não
// alcança formatos com prefixo próprio de domínio, tipo um ID exposto como
// "usr_42" em vez de "42" cru -- Custom(fn) é a válvula de escape: recebe o
// valor cru (aqui, a STRING do path param, sem nenhuma coerção prévia) e
// devolve o valor Go já no formato final, ou um error que vira violation.
type PrefixedUserIdParam struct {
  UserId int64 `param:"user_id"`
}

var _ = gonest.NewSchema[PrefixedUserIdParam](func (t *PrefixedUserIdParam, m *gonest.Schema) {
  m.Property(&t.UserId).Custom(func(raw any) (any, error) {
    s, _ := raw.(string)
    rest, ok := strings.CutPrefix(s, "usr_")
    if !ok {
      return nil, fmt.Errorf("expected a %q-prefixed id, got %q", "usr_", s)
    }
    id, err := strconv.ParseInt(rest, 10, 64)
    if err != nil {
      return nil, fmt.Errorf("expected a numeric suffix after %q, got %q", "usr_", s)
    }
    return id, nil
  }).Required()
})

// ---------- aplicando tudo no controller ----------
var UserController = gonest.NewController(func (controller *gonest.Controller) {
  controller.Path("/user")
  controller.Use(RequestIdMiddleware)
  controller.Guards(AuthGuard)
  controller.Interceptors(TimingInterceptor)
  controller.Filters(FooExampleFilter)

  userService := gonest.MustInject[*UserService](controller)

  // rota recebe o ID no formato prefixado "usr_42" (Custom(fn) acima
  // decodifica pro int64 cru antes do handler rodar; violation -- prefixo
  // errado ou sufixo não-numérico -- vira 400 automático igual qualquer
  // outro campo de MustParams).
  controller.Route(gonest.HttpGet, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.Context) {
      params := gonest.MustParams[*PrefixedUserIdParam](ctx)
      ctx.Json(userService.Get(params.UserId))
    })
  })
})

// ---------- aplicando globalmente (todos os controllers/módulos) ----------
var AppModule = gonest.NewModule(func (module *gonest.Module) {
  module.Imports(UserModule)
  module.Use(RequestIdMiddleware)      // global middleware
  module.Filters(FooExampleFilter)     // global exception filter
})
```

# bootstrap: resolução paralela (equivalente ao Promise.all do Nest)

No Nest, providers com `useFactory: async () => ...` resolvem em paralelo via Promise;
`Promise.all` junta os independentes, dependentes esperam quem eles precisam.

Em Go não tem Promise, mas o mesmo modelo mental existe via goroutine + `errgroup`:

- `NewApp` monta a árvore de módulos (import graph) e calcula ordem topológica.
- Providers/Controllers **sem dependência entre si** (mesmo nível/ramos irmãos) resolvem
  em paralelo, cada um numa goroutine, sincronizados por `errgroup.Group`.
- Providers que dependem de outro **esperam** (`group.Wait()`) o dependido terminar antes
  de rodar seu próprio `Constructor`.
- Cada resolução final faz o placeholder+copy-in-place (`*placeholder = *real`) no ponteiro
  que já foi devolvido antecipadamente por `MustInject` na fase de declaração.
- Se qualquer branch falhar (`Constructor` retorna `error` ou panica), `errgroup` cancela
  o resto via `context.Context` e `NewApp` retorna erro sem subir o servidor.

Constructor que precisa fazer I/O (conectar DB, etc — equivalente ao `async factory` do Nest)
pode receber `context.Context` opcionalmente pra suportar timeout/cancelamento:

```go
var DbProvider = gonest.NewProvider(func (provider *gonest.Provider) {
  provider.Scope(gonest.ScopeSingleton)
  provider.Constructor(func(ctx context.Context) (*Db, error) {
    // ctx vem com timeout configurado no bootstrap (equivalente ao "async factory")
    return connectDb(ctx)
  })
})
```

Channel some do lado público (API não expõe canal nenhum pro dev) — vira só o mecanismo
interno (`errgroup`) que orquestra o paralelismo do bootstrap, mesma ideia do `Promise.all`
do Nest só que resolvido de forma síncrona/bloqueante do ponto de vista de quem chama `NewApp`.

# exemplo de bootstrap completo (main.go)

Baseado em dois bootstrap reais de Nest (main.ts com Express/helmet/DomainErrorFilter,
bootstrap.ts com Zod+Swagger 3.1). Diferença chave pro gonest: `NewApp` já resolve a
árvore INTEIRA (em paralelo, via errgroup, seção acima) antes de retornar — não existe
`app.resolve()` assíncrono depois do `create()` feito igual Nest, então nada de
`Promise.all([app.resolve(Logger), app.resolve(Config)])`: quando `NewApp` volta, tudo já
tá pronto e `MustInject` só pega o ponteiro já populado.

```go
package main

import (
  "fmt"

  "github.com/gonest-dev/gonest"
)

func main() {
  if err := bootstrap(); err != nil {
    panic(err)
  }
}

func bootstrap() error {
  app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{
    BufferLogs: true,
    LogLevels:  []gonest.LogLevel{gonest.LogLevelError, gonest.LogLevelWarn},
  })
  defer app.Close()

  // grafo já resolvido pelo NewApp acima — MustInject aqui é leitura direta, sem espera.
  config := gonest.MustInject[*AppConfig](app)
  logger := gonest.MustInject[*AppLogger](app)

  app.SetGlobalPrefix(config.Prefix)
  app.UseLogger(logger)
  app.UseGlobalFilters(DomainErrorFilter)
  app.Use(gonest.Helmet(gonest.HelmetOptions{ContentSecurityPolicy: false}))
  app.EnableCors(gonest.CorsOptions{
    Origins: []string{"*"}, // "*" ou lista de origins
  })

  doc := gonest.NewOpenAPI("3.1.0", func (b *gonest.OpenAPI) {
    b.Title(config.OpenApi.Title)
    b.Description(config.OpenApi.Description)
    b.Version(config.OpenApi.Version)
    b.Contact(config.OpenApi.Contact.Name, config.OpenApi.Contact.Url, config.OpenApi.Contact.Email)
    b.License(config.OpenApi.License.Name, config.OpenApi.License.Url)
    b.BearerAuth()
  })
  gonest.SetupSwagger(app, config.OpenApi.UiPath, doc, gonest.SwaggerOptions{
    JsonDocumentUrl:  config.OpenApi.JsonPath,
    PersistAuth:      true,
    DocExpansion:     "none",
  })

  // MustListen bloqueia a goroutine (padrão Go, tipo http.ListenAndServe).
  // OnListen roda assim que o bind der certo, ANTES do bloqueio efetivo —
  // é onde entra o log de "started", nunca depois de MustListen.
  app.MustListen(fmt.Sprintf("0.0.0.0:%d", config.Port), gonest.OnListen(func () {
    logger.Log(fmt.Sprintf("🚀 %s running on http://localhost:%d", config.Name, config.Port))
  }))
  return nil
}
```

Pontos que vieram direto dos exemplos reais e precisam existir na API:
- `AppOptions{BufferLogs, LogLevels}` — mesmo `bufferLogs`/`logger: ['error','warn']` do Nest.
- `app.UseLogger(logger)` — logger customizado plugado depois do bootstrap.
- `app.UseGlobalFilters(...)` — filtro de exception global (equivalente `DomainErrorFilter`),
  além dos filters por controller/módulo já vistos antes.
- `app.SetGlobalPrefix` / `app.EnableCors` / `app.Use(Helmet)` — infra HTTP padrão.
- `gonest.NewOpenAPI` + `gonest.SetupSwagger` — equivalente `DocumentBuilder`+`SwaggerModule`.
- `app.MustListen` bloqueia (Go idiomático, tipo `http.ListenAndServe`) — diferente do
  `await app.listen()` do Nest que retorna assim que o bind funciona. `gonest.OnListen(fn)`
  resolve isso: callback roda assim que o bind der certo, antes do bloqueio definitivo.

# exemplo para definição de schemas em estruturas

```go
package ex

import (
  "github.com/gonest-dev/gonest"
)

type UserEntity struct {
  Id         int64  `json:"id"`
  Name       string `json:"name"`
  Email      string `json:"email"`
  IsActive   bool   `json:"isActive"`
  CreatedAt  time.Time `json:"createdAt"`
  UpdatedAt  time.Time `json:"updatedAt"`
  DeletedAt  *time.Time `json:"deletedAt"`
}

// cada método é uma combinação type+format do OpenAPI 3.1 (achatado, sem tipo pai
// tipo Number().Integer() — direto Integer(), Float(), Double(), Email(), Uuid() etc).
// Required/Nullable/Description/Examples ficam na base, comuns a qualquer branch.
// mesma declaração alimenta: schema OpenAPI (oas) + validação runtime (MustJsonBody/MustInject).
var _ = gonest.NewSchema[UserEntity](func (t *UserEntity, m *gonest.Schema) {
  m.Description("Entidade de usuário")
  m.Property(&t.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))
  m.Property(&t.Name).String().Required().Description("Nome do usuário").Examples("John Doe")
  m.Property(&t.Email).Email().Required().Description("Email do usuário").Examples("[EMAIL_ADDRESS]")
  m.Property(&t.IsActive).Boolean().Required().Description("Status do usuário").Examples(true)
  m.Property(&t.CreatedAt).DateTime().Required().Description("Data de criação do usuário").Examples(time.Now())
  m.Property(&t.UpdatedAt).DateTime().Required().Description("Data de atualização do usuário").Examples(time.Now())
  m.Property(&t.DeletedAt).DateTime().Nullable().Description("Data de exclusão do usuário").Examples(nil, time.Now())
})

// branches previstos (OpenAPI 3.1 type+format), cada um com métodos próprios de validação:
// String()   -> Min/Max(len), Pattern
// Email()    -> (format: email)
// Uuid()     -> (format: uuid)
// Uri()      -> (format: uri)
// Hostname() -> (format: hostname)
// Ipv4() / Ipv6() -> (format: ipv4/ipv6)
// Password() -> (format: password)
// Byte()     -> (format: byte, base64)
// Binary()   -> (format: binary)
// Integer()  -> (format: int64 default), Int32() variante -> Min/Max
// Float()    -> (format: float) / Double() -> (format: double) -> Min/Max
// Boolean()  -> sem format
// DateTime() -> (format: date-time) / Date() -> (format: date)
// Array(func(m *gonest.ArraySchema)...) -> Min/Max(items), Unique
// Object(func(m *gonest.ObjectSchema)...) -> aninhado, reusa NewSchema do tipo aninhado se já existir
```

## exemplo de Array e Object aninhados

```go
package ex

import (
  "github.com/gonest-dev/gonest"
)

type AddressEntity struct {
  Street string `json:"street"`
  City   string `json:"city"`
  Zip    string `json:"zip"`
}

type UserEntity struct {
  Id        int64          `json:"id"`
  Tags      []string       `json:"tags"`
  Scores    []int          `json:"scores"`
  Addresses []AddressEntity `json:"addresses"`
  Address   AddressEntity  `json:"address"`
  Schema  map[string]any `json:"schema"`
}

func init() {
  gonest.NewSchema[UserEntity](func (t *UserEntity, m *gonest.Schema) {
    // schema aninhada declarada dentro do mesmo closure e capturada em variável —
    // sem reflect, sem [T] em método (Go não permite parâmetro de tipo em método).
    // reusada abaixo tanto em Array (Items) quanto em Object direto.
    addressSchema := gonest.NewSchema[AddressEntity](func (t *AddressEntity, m *gonest.Schema) {
      m.Description("Endereço")
      m.Property(&t.Street).String().Required().Description("Logradouro").Examples("Rua A, 123")
      m.Property(&t.City).String().Required().Description("Cidade").Examples("São Paulo")
      m.Property(&t.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`).Description("CEP").Examples("01310-100")
    })

    m.Description("Entidade de usuário com campos aninhados")
    m.Property(&t.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))

    // Array() de tipo primitivo — Items() sem arg encadeia o branch igual Property faria.
    m.Property(&t.Tags).Array().Items(func(m *gonest.ArraySchema) {
      m.String().Min(1).Max(50)
      m.Required()
      m.Description("Tags do usuário")
      m.Examples("admin", "beta")
    })

    // Array() de número — Min/Max aqui são do ITEM (0 a 100); array em si não tem Min/Max
    // de quantidade nesse caso (poderia mesclar com .Array(1, 10) se quantidade importasse).
    m.Property(&t.Scores).Array().Items(func(m *gonest.ArraySchema) {
      m.Integer().Min(0).Max(100)
      m.Required()
      m.Description("Notas do usuário")
      m.Examples(80, 95)
    })

    // Array() de Object() — Items(addressSchema) reusa a schema já registrada acima
    // (mesmo objeto, sem duplicar Property; equivalente a $ref no OpenAPI).
    m.Property(&t.Addresses).Array().Items(func(m *gonest.ArraySchema) {
      m.Object(addressSchema)
      m.Required()
      m.Min(1)
      m.Description("Endereços do usuário")
      m.Examples("admin", "beta")
    })

    // Object() direto (não-array) — mesma reutilização via valor, sem reflect.
    m.Property(&t.Address).Object(func(om *gonest.ObjectSchema) {
      om.Schema(addressSchema)
      om.Required()
      om.Description("Endereço principal")
    })

    // Object() livre (schema aberto, tipo map[string]any) — sem struct Go aninhada pra
    // reusar, por isso recebe callback em vez de schema já registrada.
    m.Property(&t.Schema).Object(func (om *gonest.ObjectSchema) {
      om.AdditionalProperties()
    }).Nullable().Description("Metadados abertos do usuário")
  })
}

// Items(ref ...*gonest.SchemaDefinition) é variádico — mesmo método resolve os dois casos
// acima: Items() sem arg (item primitivo, encadeia branch tipo .String()/.Integer()) e
// Items(addressSchema) com arg (item referencia schema já registrada). Sem overload —
// Go não permite dois métodos com o mesmo nome, então é 1 método só variádico.
//
// por ser builder linear (não callback com escopo próprio), dá pra mesclar validação de
// array/objeto com Required/Nullable/Description/Examples na mesma chain, na ordem que
// fizer sentido — não existe separação rígida "dentro do callback" vs "fora dele" como
// na versão anterior. Único cuidado: Min/Max logo após Items() valem pro ITEM; Min/Max
// chamado direto em Array() (antes de Items) valeria pra quantidade de items do array.
```

# exemplo de Testing (equivalente @nestjs/testing)

```go
package ex_test

import (
  "testing"

  "github.com/gonest-dev/gonest"
)

// pré-requisito pra mock funcionar: dependência tem que ser injetada por INTERFACE,
// não struct concreta. Go não troca comportamento de um *struct em runtime (sem vtable),
// então UserController precisa depender de IUserService, não *UserService direto.
type IUserService interface {
  Get(userId int64) *UserEntity
}

var _ IUserService = (*UserService)(nil) // garante que UserService implementa a interface

// mock simples, implementa a mesma interface
type UserServiceMock struct {
  GetFn func(userId int64) *UserEntity
}
func (m *UserServiceMock) Get(userId int64) *UserEntity { return m.GetFn(userId) }

func TestUserController_Get(t *testing.T) {
  mock := &UserServiceMock{
    GetFn: func(userId int64) *UserEntity {
      return &UserEntity{ID: gonest.NewValue(userId)}
    },
  }

  tester := gonest.MustNewTestApp(UserModule, func (b *gonest.TestBuilder) {
    gonest.MustOverride[IUserService](b, mock)
  })
  defer tester.Close()

  res := tester.MustRequest(gonest.HttpGet, "/user/42", nil)
  res.AssertStatus(t, gonest.HttpStatusOk)
  res.AssertJsonPath(t, "id", int64(42))
}

// pra teste de unidade sem subir rota HTTP nenhuma, resolve o provider direto:
func TestUserService_Get_NotFound(t *testing.T) {
  tester := gonest.MustNewTestApp(UserModule, nil) // sem overrides
  defer tester.Close()

  service := gonest.MustInject[*UserService](tester)

  defer func() {
    exc, ok := recover().(*gonest.NotFoundException)
    if !ok {
      t.Fatal("esperava NotFoundException")
    }
    _ = exc
  }()
  service.Get(999)
}
```

# exemplo de MustInjectAll (multi-binding por interface)

`MustInject[T]` (T interface) espera EXATAMENTE 1 provider cuja implementação
satisfaça `T` -- panica se 0 ou 2+. `MustInjectAll[T]` (T SEMPRE interface,
nunca ponteiro concreto) é o caso "2+ é esperado": devolve `[]T` com TODA
implementação registrada que satisfaz a interface, sem panicar por
ambiguidade -- útil pra padrão de plugin/estratégia (ex: múltiplos handlers
de notificação, múltiplos validators customizados etc).

```go
package ex

import (
  "github.com/gonest-dev/gonest"
)

// Connectable é a interface -- múltiplos providers podem satisfazer a mesma.
type Connectable interface { Ping() bool }

type Postgres struct{}
var _ Connectable = (*Postgres)(nil)
func (c *Postgres) Ping() bool { return true }

var PostgresProvider = gonest.NewProvider(func (provider *gonest.Provider) {
  provider.Constructor(func() *Postgres { return &Postgres{} })
})

type Redis struct{}
var _ Connectable = (*Redis)(nil)
func (d *Redis) Ping() bool { return true }
var RedisProvider = gonest.NewProvider(func (provider *gonest.Provider) {
  provider.Constructor(func() *Redis { return &Redis{} })
})

// ConnectableService recebe TODOS os Connectable registrados no módulo, sem
// precisar conhecer Postgres/Redis especificamente -- novo Connectable registrado no
// módulo (ex: PostgresDatabase) aparece automaticamente na próxima resolução, sem
// mudar DatabaseQueryService nenhum.
type ConnectableService struct {
  connectables []Connectable
}
func (t *ConnectableService) PingAll() []bool {
  out := make([]bool, 0, len(t.connectables))
  for _, a := range t.connectables {
    out = append(out, a.Ping())
  }
  return out
}

var SystemController = gonest.NewController(func (controller *gonest.Controller) {
  controller.Path("/health")

  // MustInjectAll[Connectable] -- resolvido UMA vez aqui, fora do Handler (o
  // grafo de providers já está totalmente resolvido nesse ponto -- ver
  // "exemplo mais simples" pra ordem de bootstrap). Handler só fecha sobre
  // o slice já pronto, não resolve de novo a cada request.
  connectables := gonest.MustInjectAll[Connectable](controller)
  service := &ConnectableService{connectables: connectables}

  controller.Route(gonest.HttpGet, "/ping", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.Context) {
      ctx.Json(service.PingAll())
    })
  })
})

var SystemModule = gonest.NewModule(func (module *gonest.Module) {
  module.Providers(PostgresProvider, RedisProvider)
  module.Controllers(SystemController)
})
```

# exemplo de Emitter (equivalente @nestjs/event-emitter, casa com cqrs.EventBus)

```go
package ex

import (
  "context"

  "github.com/gonest-dev/gonest"
)

// evento tipado por struct — mesma filosofia do MustCatch (sem string solta tipo "user.created").
type UserCreatedEvent struct {
  UserID int64
}

// listener: função livre MustOn (método não pode ser genérico), registrado num Listener builder.
var UserCreatedListener = gonest.NewListener(func (listener *gonest.Listener) {
  logger := gonest.MustInject[*LoggerService](listener)
  gonest.MustOn[UserCreatedEvent](listener, func(ctx context.Context, event UserCreatedEvent) {
    logger.Log("user created", event.UserID)
  })
})

// emissão: resolve o Emitter global (singleton do framework, SEMPRE disponível
// em qualquer módulo sem registro explícito) via MustInject DENTRO do builder
// do Provider (padrão real de dependência Provider-a-Provider -- Constructor
// em si só aceita func()/func()(T,error)/func(ctx)T/func(ctx)(T,error), NUNCA
// parâmetro de dependência direto; a dependência é capturada por closure ANTES
// de Constructor ser chamado, mesmo padrão de Guard/Scheduler/HealthCheck).
var UserProvider = gonest.NewProvider(func (provider *gonest.Provider) {
  emitter := gonest.MustInject[*gonest.Emitter](provider)
  provider.Constructor(func() *UserService {
    return &UserService{emitter: emitter}
  })
})

func (t *UserService) Create(properties UserProperties) *UserEntity {
  entity := &UserEntity{ /* ... */ }
  t.emitter.Emit(UserCreatedEvent{UserID: entity.ID.Value()})
  return entity
}

var UserModule = gonest.NewModule(func (module *gonest.Module) {
  module.Providers(UserProvider)
  module.Listeners(UserCreatedListener) // registrado no bootstrap junto com providers
})

// Emit é assíncrono: dispara uma goroutine por listener registrado pro tipo do evento
// e retorna na hora (fire-and-forget), não bloqueia quem chamou Emit. Panic/erro dentro
// de um listener nunca propaga pro chamador — cai só no logger interno do framework,
// igual comportamento isolado do Scheduler (recover próprio por execução).
```

# exemplo de Schedule (equivalente @nestjs/schedule: @Cron/@Interval/@Timeout)

```go
package ex

import (
  "context"
  "time"

  "github.com/gonest-dev/gonest"
)

var CleanupScheduler = gonest.NewScheduler(func (scheduler *gonest.Scheduler) {
  userService := gonest.MustInject[*UserService](scheduler)
  scheduler.Cron("cleanup-expired-users", "0 0 * * *", func (ctx context.Context) { userService.PurgeExpired(ctx) })
  scheduler.Interval("healthcheck-ping", time.Minute, func (ctx context.Context) { userService.Ping(ctx) })
  scheduler.Timeout("warmup-cache", 5*time.Second, func (ctx context.Context) { userService.WarmupCache(ctx) })
})

var AppModule = gonest.NewModule(func (module *gonest.Module) {
  module.Imports(UserModule)
  module.Schedulers(CleanupScheduler)
})

// panic dentro do handler de Cron/Interval/Timeout não derruba o processo — cada execução
// roda isolada (recover próprio), erro só vai pro logger, igual comportamento de job.
```

# exemplo de Probes / health (equivalente @nestjs/terminus)

No NestJS, o Terminus não introduz uma estrutura nova de bootstrap; ele é
simplesmente um Controller padrão (anotado com `@Controller`) que expõe as
checagens. No `gonest`, o modelo mental é exatamente o mesmo: não precisamos
de um `NewProbe` dedicado, apenas usamos um `NewController` normal!

```go
package ex

import (
  "context"

  "github.com/gonest-dev/gonest"
)

// Pingable é a interface que toda dependência checável implementa -- nome
// distinto de Connectable (usada no exemplo de MustInjectAll acima, que tem
// assinatura diferente: Ping() bool, sem Name()) para não colidir.
type Pingable interface {
  Name() string
  Ping(ctx context.Context) error
}

// HealthController atua exatamente como o Terminus no NestJS: é um controller
// comum, onde você injeta as dependências que quer checar e as expõe via rotas.
var HealthController = gonest.NewController(func (controller *gonest.Controller) {
  controller.Path("/health")

  // MustInjectAll resolve todas as implementações de Pingable (ex: Db, Redis)
  pingables := gonest.MustInjectAll[Pingable](controller)

  // Readiness Probe (Padrão K8s: /readyz) - "Estou pronto pra receber tráfego?"
  controller.Route(gonest.HttpGet, "/readyz", func (route *gonest.Route) {
    route.Handler(func(ctx *gonest.Context) {
      results, status := make(map[string]string), gonest.HttpStatusOk

      for _, c := range pingables {
        name := c.Name()
        if err := c.Ping(ctx); err != nil {
          results[name], status = "down", gonest.HttpStatusServiceUnavailable
        } else {
          results[name] = "up"
        }
      }

      ctx.Status(status).Json(map[string]any{"status": "ok","checks": results})
    })
  })

  // Liveness Probe (Padrão K8s: /livez) - "Eu travei em deadlock?"
  // Responder 200 OK direto costuma ser o suficiente, já que se o container
  // travar no Go, o servidor HTTP sequer conseguirá responder a esse request.
  controller.Route(gonest.HttpGet, "/livez", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.Context) {
      ctx.Status(gonest.HttpStatusOk).SendString("OK")
    })
  })
})

var AppModule = gonest.NewModule(func (module *gonest.Module) {
  module.Controllers(HealthController) // registrado como um controller normal!
})

func main() {
  app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})
  defer app.Close()

  // Nenhuma configuração especial no main() para Probes/HealthChecks! 
  // O HealthController já registrou as rotas de forma nativa.
  app.MustListen(":3000", nil)
}
```

# exemplo de Param/Query Validation

Path params e query string seguem exatamente o mesmo mecanismo de `MustJsonBody`
(exemplo acima): declara um struct pequeno, registra sua `Schema` via
`NewSchema[T]` (tags `param:"..."`/`query:"..."` em vez de `json:"..."`), e
o handler resolve o struct inteiro já validado de uma vez -- nunca campo a
campo. Não existe (nem nunca existiu como API pública) um `MustParam[T](ctx, name)`
avulso paralelo; `MustParams`/`MustQuery` são o único caminho pra path
param/query string.

```go
package ex

import (
  "github.com/gonest-dev/gonest"
)

// path params: struct + tag `param:"..."`, um campo por segmento ":name" da rota.
type UserIdParams struct {
  UserId int64 `param:"user_id"`
}

var _ = gonest.NewSchema[UserIdParams](func (t *UserIdParams, m *gonest.Schema) {
  m.Property(&t.UserId).Integer().Min(1).Required()
})

// query string: struct + tag `query:"..."`, mesmo mecanismo, um campo por
// parâmetro esperado na query string (`?page=1&limit=20`).
type ListUsersQuery struct {
  Page  int `query:"page"`
  Limit int `query:"limit"`
}

var _ = gonest.NewSchema[ListUsersQuery](func (t *ListUsersQuery, m *gonest.Schema) {
  m.Property(&t.Page).Integer().Min(1).Required()
  m.Property(&t.Limit).Integer().Min(1).Max(100).Required()
})

var UserController = gonest.NewController(func (controller *gonest.Controller) {
  controller.Path("/user")
  userService := gonest.MustInject[*UserService](controller)

  controller.Route(gonest.HttpGet, "/:user_id", func (route *gonest.Route) {
    route.Handler(func(ctx *gonest.Context) {
      // MustParams[T](ctx) -- igual MustJsonBody, mas lê os ":name" da rota
      // atual em vez do corpo JSON. Path param ausente ou fora de Min/Max
      // vira violation, coletada junto com qualquer outra (não fail-fast) --
      // panica *BadRequestException se alguma sobrar.
      params := gonest.MustParams[*UserIdParams](ctx)
      ctx.Json(userService.Get(params.UserId))
    })
  })

  controller.Route(gonest.HttpGet, "/", func (route *gonest.Route) {
    route.Handler(func(ctx *gonest.Context) {
      // MustQuery[T](ctx) -- mesmo mecanismo de MustParams, lendo da query
      // string em vez dos ":name" da rota.
      query := gonest.MustQuery[*ListUsersQuery](ctx)
      ctx.Json(userService.List(query.Page, query.Limit))
    })
  })
})

// path param + query juntos na mesma rota: MustParams e MustQuery são
// independentes entre si, dá pra chamar os dois no mesmo handler.
var OrderController = gonest.NewController(func (controller *gonest.Controller) {
  controller.Path("/user")
  userService := gonest.MustInject[*UserService](controller)

  controller.Route(gonest.HttpGet, "/:user_id/orders", func (route *gonest.Route) {
    route.Handler(func(ctx *gonest.Context) {
      params := gonest.MustParams[*UserIdParams](ctx)
      query := gonest.MustQuery[*ListUsersQuery](ctx)
      ctx.Json(userService.ListOrders(params.UserId, query.Page, query.Limit))
    })
  })
})
```

Quando o vocabulário fixo de `Schema` (`Integer`/`String`/`Min`/`Max`/`Pattern`
etc) não alcança um formato de domínio específico (ex: um ID exposto com
prefixo, tipo `"usr_42"`), `PropertyBuilder.Custom(fn)` é a válvula de escape
-- funciona igual em `MustParams`/`MustQuery`/`MustJsonBody`, sempre recebendo
o valor CRU (string, no caso de param/query) e devolvendo o valor Go final ou
um `error` que vira violation. Ver a seção "exemplo de Middleware, Guard,
Interceptor e Filter" acima (`PrefixedUserIdParam`) pro exemplo completo.

# exemplo de Schema Generation from Schema

Mapeamento NestJS `@nestjs/swagger` -> gonest (decorator -> builder method, já
que Go não tem decorator):

- `@ApiTags` -> `Controller.Tags(...)` (nível controller, herda pra toda rota)
  / `Route.Tags(...)` (override por rota -- SUBSTITUI o valor do controller
  por completo quando chamado, nunca soma, mesma prioridade "rota vence" do
  Nest)
- `@ApiOperation` -> `Route.Summary(s)` / `Route.Description(s)` /
  `Route.OperationId(s)`
- `@ApiBody` -> `Route.RequestBody(schema)`
- `@ApiResponse`/`@ApiOkResponse`/`@ApiCreatedResponse`/etc ->
  `Route.Response(status, func(response *gonest.Response))` -- o callback
  é opcional (variádico): zero args documenta o status sem body, um arg
  permite formatar a resposta (schema, description). Chamar de novo pro MESMO status sobrescreve; pra status
  DIFERENTES acumula.
- `@ApiParam` -> `Route.PathParams(schema)`
- `@ApiQuery` -> `Route.QueryParams(schema)`
- `@ApiBearerAuth`/`@ApiBasicAuth` -> `Controller.BearerAuth()` (herda) /
  `Route.BearerAuth()` (override, mesma prioridade "rota vence" de Tags)
- `@ApiExcludeEndpoint` -> `Route.ExcludeFromDocs()`
- `@ApiDeprecated` -> `Route.Deprecated()`
- `@ApiProperty` -> já coberto, é `Property()`/`Description()`/`Examples()`/
  `Required()` da própria `Schema` (ver "exemplo para definição de
  schemas em estruturas" acima)

```go
package ex

import (
  "github.com/gonest-dev/gonest"
)

type AddressEntity struct {
  Street string `json:"street"`
  City   string `json:"city"`
  Zip    string `json:"zip"`
}

type UserEntity struct {
  Id        int64           `json:"id"`
  Name      string          `json:"name"`
  Address   AddressEntity   `json:"address"`
  Addresses []AddressEntity `json:"addresses"`
}

type UserIdParams struct {
  UserId int64 `param:"user_id"`
}

var addressSchema = gonest.NewSchema[AddressEntity](func (t *AddressEntity, m *gonest.Schema) {
  m.Property(&t.Street).String().Required()
  m.Property(&t.City).String().Required()
  m.Property(&t.Zip).String().Required()
})

// Object(func(om *gonest.ObjectSchema){ om.Schema(ref) }) e
// Array().Items(func(am *gonest.ArraySchema){ am.Object(ref) }) reusam a
// MESMA addressSchema declarada acima -- Schema Generation dedup automático
// por identidade de ponteiro, não por nome (ver abaixo).
var userEntitySchema = gonest.NewSchema[UserEntity](func (t *UserEntity, m *gonest.Schema) {
  m.Title("UserEntity") // nome do schema em components.schemas -- default é
                        // o nome do tipo Go (reflect.Type.Name()), Title()
                        // sobrescreve.
  m.Property(&t.Id).Integer().Required()
  m.Property(&t.Name).String().Required()
  m.Property(&t.Address).Object(func (om *gonest.ObjectSchema) {
    om.Schema(addressSchema)
  }).Required()
  m.Property(&t.Addresses).Array().Items(func (am *gonest.ArraySchema) {
    am.Object(addressSchema)
  })
})

var userIdParamsSchema = gonest.NewSchema[UserIdParams](func (t *UserIdParams, m *gonest.Schema) {
  m.Property(&t.UserId).Integer().Min(1).Required()
})

var UserController = gonest.NewController(func (controller *gonest.Controller) {
  controller.Path("/user")
  controller.Tags("users")      // aplica em TODA rota deste controller
  controller.BearerAuth()       // idem -- toda rota exige bearer, salvo override por rota
  userService := gonest.MustInject[*UserService](controller)

  controller.Route(gonest.HttpPost, "/", func (route *gonest.Route) {
    // descritivos de documentação -- viram summary/operationId no path item
    // OpenAPI. Tags/BearerAuth herdados do controller acima, sem repetir.
    route.Summary("Cria um novo usuário")

    // liga o corpo esperado a um *Schema JÁ registrado (mesmo valor que
    // MustJsonBody[*UserProperties] vai usar dentro do Handler -- reusa a
    // MESMA declaração, não duplica). Schema Generation lê isso pra montar
    // requestBody no OpenAPI; MustJsonBody continua sendo quem VALIDA em
    // runtime -- RequestBody() aqui é só DECLARATIVO/documental.
    route.RequestBody(userEntitySchema)

    // liga status HTTP -> *Schema da resposta. Múltiplas chamadas = múltiplos
    // status documentados (ex: 201 sucesso, 409 conflito reusando outra Schema).
    route.Response(201, func (response *gonest.Response) {
      response.Description("Usuário criado com sucesso")
      response.Schema(userEntitySchema)
    })
    route.Response(409, func (response *gonest.Response) {
      response.Description("Conflito: usuário já existe")
    })

    route.HttpCode(201)
    route.Handler(func(ctx *gonest.Context) {
      properties := gonest.MustJsonBody[*UserEntity](ctx)
      ctx.Json(userService.Create(properties))
    })
  })

  controller.Route(gonest.HttpGet, "/:user_id", func (route *gonest.Route) {
    route.Summary("Busca um usuário por ID")
    // path params TAMBÉM documentados via Schema já registrada (mesma
    // UserIdParams de MustParams) -- Schema Generation vira "parameters"
    // (in: path) no OpenAPI a partir dela, sem redeclarar nada.
    route.PathParams(userIdParamsSchema)
    route.Response(200, func (response *gonest.Response) {
      response.Description("Usuário encontrado")
      response.Schema(userEntitySchema)
    })
    // sem passar a função de construção da response então retorna um modelo padronizado
    // seguindo as mensagens comuns em ingles e o formato do gonest.HttpException básico
    route.Response(404) 

    route.HttpCode(200)
    route.Handler(func(ctx *gonest.Context) {
      params := gonest.MustParams[*UserIdParams](ctx)
      ctx.Json(userService.Get(params.UserId))
    })
  })

  // rota interna, não documentada -- equivalente @ApiExcludeEndpoint.
  controller.Route(gonest.HttpGet, "/_internal/debug", func (route *gonest.Route) {
    route.ExcludeFromDocs()
    route.HttpCode(200)
    route.Handler(func(ctx *gonest.Context) { ctx.Json(map[string]any{"ok": true}) })
  })
})

// rota SEM nenhuma chamada de documentação (Summary/RequestBody/Response/
// PathParams) ainda aparece em paths -- Schema Generation infere o que já dá
// pra inferir do que a Route/Controller já sabem (path/método/HttpCode), sem
// exigir documentação explícita como pré-requisito pra aparecer.
```

`gonest.GenerateOpenApiSchema(app *gonest.App, doc *gonest.OpenAPI)`
percorre `app`'s árvore de módulos inteira (root + `ImportedModules()`
recursivo, cycle-safe) já montada pelo `NewApp` anterior, e popula
`doc`'s `paths`/`components.schemas` a partir de TODO Controller/Route
registrado (chamado depois de `NewApp`, antes de servir o documento):

```go
// (continuação do "exemplo de bootstrap completo" acima)
doc := gonest.NewOpenAPI("3.1.0", func (b *gonest.OpenAPI) {
  b.Title("Example API")
  b.Version("1.0.0")
  b.BearerAuth()
})

gonest.GenerateOpenApiSchema(app, doc)

// doc.Document() monta a estrutura OpenAPI 3.1 completa (openapi/info/paths/
// components/security) já pronta pra json.Marshal -- é isso que uma futura
// feature "Swagger UI Setup" (SetupSwagger, fora de escopo aqui, ver
// ROADMAP.md) vai servir.
json := doc.Document()
```

Array/Object aninhado (Milestone 5): ao gerar schema de um campo `Object(ref)`
ou `Array().Items(ref)`, Schema Generation usa `ItemRef()`/`SchemaRef()`
(já existem, AD-012) pra emitir `"$ref": "#/components/schemas/AddressEntity"`
em vez de inline -- MESMA `*Schema` nomeada uma vez em `components.schemas`
(dedup por identidade de ponteiro, não por nome), reusada por todo campo/rota
que apontar pra ela.

Quando o vocabulário fixo de `Schema` (`Integer`/`String`/`Min`/`Max`/`Pattern`
etc) não alcança um formato de domínio específico, `PropertyBuilder.Custom(fn)`
ainda funciona em Schema Generation: o campo aparece no schema SEM
type/format (só `description`/`examples`/`nullable`/`required` se setados) --
limitação documentada, não erro.

# reflexão: Suporte a GraphQL (Futuro)

A adoção do sistema de builders e o uso de `gonest.Schema` (que hoje já consolida validação em runtime e geração de OpenAPI) criam um caminho extremamente natural e elegante para suportar **GraphQL** no futuro. Podemos reaproveitar quase toda a infraestrutura base do framework, mudando apenas a ponta de exposição e aproveitando o modelo mental que o próprio NestJS adotou para GraphQL: **Resolvers ao invés de Controllers**.

No NestJS, o GraphQL é construído usando os decorators `@Resolver`, `@Query`, `@Mutation` e `@Args`. No Gonest, adotaríamos a mesma filosofia baseada em closures (builders) semânticos, mantendo a consistência do framework sem recorrer à pesada reflexão de anotações.

## 1. Resolvers como Porteiros do GraphQL

Um `Resolver` atuaria de forma análoga a um `Controller`. Ele pertenceria a um Módulo e consumiria suas dependências normalmente (via `MustInject`), sendo registrado via `module.Resolvers(...)`. 

```go
var UserResolver = gonest.NewResolver("User", func (r *gonest.Resolver) {
  // Injeção de dependência funciona de forma idêntica a um Controller
  userService := gonest.MustInject[*UserService](r)

  // Equivalente ao @Query() do Nest
  r.Query("getUser", func (q *gonest.Query) {
    // Reutilizamos a MESMA gonest.Schema do OpenAPI/HTTP para validar argumentos!
    q.Args(userIdParamsSchema) 
    
    // E reutilizamos para tipar o retorno no Schema do GraphQL
    q.Returns(userEntitySchema) 

    q.Resolve(func(ctx *gonest.Context) {
      // MustArgs seria o equivalente GraphQL para MustParams / MustJsonBody
      args := gonest.MustArgs[*UserIdParams](ctx)
      
      // ctx.Data ou ctx.GraphQL sinaliza o retorno final do resolver (sem response HTTP)
      ctx.Data(userService.Get(args.UserId))
    })
  })

  // Equivalente ao @Mutation() do Nest
  r.Mutation("createUser", func (m *gonest.Mutation) {
    // O mesmo schema do REST pode ser consumido como Input no GraphQL
    m.Args(userEntitySchema) 
    m.Returns(userEntitySchema)
    
    m.Resolve(func(ctx *gonest.Context) {
      input := gonest.MustArgs[*UserEntity](ctx)
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
