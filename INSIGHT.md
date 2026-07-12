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
      userId := gonest.MustParam[int64](ctx, "user_id")
      ctx.Json(userService.Get(userId))
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
      userId := gonest.MustParam[int64](ctx, "user_id")
      properties := gonest.MustJsonBody[*UserProperties](ctx)
      ctx.Json(userService.Update(userId, properties))
    })
  })
  controller.Route(gonest.HttpDelete, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.Context) {
      userId := gonest.MustParam[int64](ctx, "user_id")
      ctx.Json(userService.Delete(userId))
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

# exemplo de Middleware, Guard, Interceptor, Filter e Pipe

```go
package ex

import (
  "strconv"
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

// ---------- Pipe ----------
// transforma/valida um valor de entrada (param, query, body) antes do handler rodar.
// panica com BadRequestException se a entrada for inválida.
var ParseIntPipe = gonest.NewPipe(func (pipe *gonest.Pipe) {
  pipe.Handler(func(ctx *gonest.Context, raw string) int64 {
    value, err := strconv.ParseInt(raw, 10, 64)
    if err != nil {
      panic(gonest.NewBadRequestException(map[string]any{"raw": raw}))
    }
    return value
  })
})

// ---------- aplicando tudo no controller ----------
var UserController = gonest.NewController(func (controller *gonest.Controller) {
  controller.Path("/user")
  controller.Use(RequestIdMiddleware)
  controller.Guards(AuthGuard)
  controller.Interceptors(TimingInterceptor)
  controller.Filters(FooExampleFilter)

  userService := gonest.MustInject[*UserService](controller)

  controller.Route(gonest.HttpGet, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Param("user_id", ParseIntPipe)
    route.Handler(func(ctx *gonest.Context) {
      userId := gonest.MustParam[int64](ctx, "user_id")
      ctx.Json(userService.Get(userId))
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

  doc := gonest.NewOpenApiDocument("3.1.0", func (b *gonest.OpenApiDocument) {
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
- `gonest.NewOpenApiDocument` + `gonest.SetupSwagger` — equivalente `DocumentBuilder`+`SwaggerModule`.
- `app.MustListen` bloqueia (Go idiomático, tipo `http.ListenAndServe`) — diferente do
  `await app.listen()` do Nest que retorna assim que o bind funciona. `gonest.OnListen(fn)`
  resolve isso: callback roda assim que o bind der certo, antes do bloqueio definitivo.

# exemplo para definição de metadados em estruturas

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
var _ = gonest.NewMetadata[UserEntity](func (t *UserEntity, m *gonest.Metadata) {
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
// Array(func(m *gonest.ArrayMetadata)...) -> Min/Max(items), Unique
// Object(func(m *gonest.ObjectMetadata)...) -> aninhado, reusa NewMetadata do tipo aninhado se já existir
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
  Metadata  map[string]any `json:"metadata"`
}

func init() {
  gonest.NewMetadata[UserEntity](func (t *UserEntity, m *gonest.Metadata) {
    // metadata aninhada declarada dentro do mesmo closure e capturada em variável —
    // sem reflect, sem [T] em método (Go não permite parâmetro de tipo em método).
    // reusada abaixo tanto em Array (Items) quanto em Object direto.
    addressMetadata := gonest.NewMetadata[AddressEntity](func (t *AddressEntity, m *gonest.Metadata) {
      m.Description("Endereço")
      m.Property(&t.Street).String().Required().Description("Logradouro").Examples("Rua A, 123")
      m.Property(&t.City).String().Required().Description("Cidade").Examples("São Paulo")
      m.Property(&t.Zip).String().Required().Pattern(`^\d{5}-?\d{3}$`).Description("CEP").Examples("01310-100")
    })

    m.Description("Entidade de usuário com campos aninhados")
    m.Property(&t.Id).Integer().Required().Description("ID do usuário").Examples(int64(1))

    // Array() de tipo primitivo — Items() sem arg encadeia o branch igual Property faria.
    m.Property(&t.Tags).Array().Items().String().Min(1).Max(50).
      Required().Description("Tags do usuário").Examples("admin", "beta")

    // Array() de número — Min/Max aqui são do ITEM (0 a 100); array em si não tem Min/Max
    // de quantidade nesse caso (poderia mesclar com .Array(1, 10) se quantidade importasse).
    m.Property(&t.Scores).Array().Items().Integer().Min(0).Max(100).
      Required().Description("Notas do usuário").Examples(80, 95)

    // Array() de Object() — Items(addressMetadata) reusa a metadata já registrada acima
    // (mesmo objeto, sem duplicar Property; equivalente a $ref no OpenAPI).
    m.Property(&t.Addresses).Array().Items(addressMetadata).Min(1).
      Required().Description("Endereços do usuário")

    // Object() direto (não-array) — mesma reutilização via valor, sem reflect.
    m.Property(&t.Address).Object(addressMetadata).
      Required().Description("Endereço principal")

    // Object() livre (schema aberto, tipo map[string]any) — sem struct Go aninhada pra
    // reusar, por isso recebe callback em vez de metadata já registrada.
    m.Property(&t.Metadata).Object(func (om *gonest.ObjectMetadata) {
      om.AdditionalProperties()
    }).Nullable().Description("Metadados abertos do usuário")
  })
}

// Items(ref ...*gonest.MetadataDefinition) é variádico — mesmo método resolve os dois casos
// acima: Items() sem arg (item primitivo, encadeia branch tipo .String()/.Integer()) e
// Items(addressMetadata) com arg (item referencia metadata já registrada). Sem overload —
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

// emissão: resolve o Emitter global (singleton do framework) e emite valor tipado.
var UserService_Create_Emits = gonest.NewProvider(func (provider *gonest.Provider) {
  provider.Constructor(func(emitter *gonest.Emitter) *UserService {
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

# exemplo de Terminus/health (equivalente @nestjs/terminus)

```go
package ex

import (
  "context"

  "github.com/gonest-dev/gonest"
)

var AppHealth = gonest.NewHealthCheck(func (health *gonest.HealthCheck) {
  db, redis := gonest.MustInject[*Db](health), gonest.MustInject[*Redis](health)
  health.Check("database", func (ctx context.Context) error { return db.Ping(ctx) })
  health.Check("redis", func (ctx context.Context) error { return redis.Ping(ctx) })
})

var AppModule = gonest.NewModule(func (module *gonest.Module) {
  module.HealthChecks(AppHealth)
})

func main() {
  app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})
  defer app.Close()

  // monta GET /health automaticamente a partir dos HealthChecks registrados no módulo.
  // status 200 + { "status": "ok", "checks": {"database":"ok","redis":"ok"} } se tudo passar.
  // 503 + detalhe do check que falhou (nome + erro) se algum falhar.
  app.UseHealthCheck("/health")

  app.MustListen(":3000", nil)
}
```
