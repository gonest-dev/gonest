# exemplo mais simples

```go
package ex

import (
  "time"

  "gonest.dev/gonest"
)

type UserProperties struct {
  Name string `json:"name"`
  Age  int    `json:"age"`
}

type UserEntity struct {
  UserProperties
  ID        int64      `json:"id"`
  CreatedAt time.Time  `json:"created_at"`
  UpdatedAt time.Time  `json:"updated_at"`
  DeletedAt *time.Time `json:"deleted_at"`
}

var userPropertiesSchema = gonest.NewSchema[UserProperties](func (t *UserProperties, m *gonest.Schema) {
  m.Property(&t.Name).String().Required()
  m.Property(&t.Age).Integer().Required()
})

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
// de MustParseRestJsonBody, só que alimentado pelo segmento ":user_id" da rota em vez
// do corpo JSON (ver "exemplo de Param/Query Validation" mais abaixo pro
// detalhe completo, incluindo query string e Custom(fn)).
type UserIdParam struct {
  UserId int64 `param:"user_id"`
}

// userIdParamSchema precisa de nome (não `var _ =`) -- todo MustParseRestParams/
// MustParseRestQuery/MustParseRestJsonBody agora recebe o Schema explícito como argumento
// (ver seção "hipótese" mais abaixo, decisão tomada e já executada), não
// existe mais lookup por tipo num registry global.
var userIdParamSchema = gonest.NewSchema[UserIdParam](func (t *UserIdParam, m *gonest.Schema) {
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
    route.Handler(func(ctx *gonest.RestContext) {
      ctx.Json(userService.List())
    })
  })
  controller.Route(gonest.HttpGet, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.RestContext) {
      params := gonest.MustParseRestParams[*UserIdParam](ctx, userIdParamSchema)
      ctx.Json(userService.Get(params.UserId))
    })
  })
  controller.Route(gonest.HttpPost, "/", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusCreated)
    route.Handler(func(ctx *gonest.RestContext) {
      properties := gonest.MustParseRestJsonBody[*UserProperties](ctx, userPropertiesSchema)
      ctx.Json(userService.Create(properties))
    })
  })
  controller.Route(gonest.HttpPut, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.RestContext) {
      params := gonest.MustParseRestParams[*UserIdParam](ctx, userIdParamSchema)
      properties := gonest.MustParseRestJsonBody[*UserProperties](ctx, userPropertiesSchema)
      ctx.Json(userService.Update(params.UserId, properties))
    })
  })
  controller.Route(gonest.HttpDelete, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.RestContext) {
      params := gonest.MustParseRestParams[*UserIdParam](ctx, userIdParamSchema)
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
  app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})
  app.MustListen(":3000")
}
```

# exemplo de Value[T] (PATCH / atualização parcial)

`gonest.Value[T]` é um wrapper que rastreia se um campo foi explicitamente
enviado no payload (dirty-tracking). Campos omitidos ficam `IsDirty() == false`
-- o handler só aplica o que realmente chegou, sem precisar de heurísticas
(`if name != "" { ... }` ou ponteiros opcionais como `*string`).

```go
package ex

import "gonest.dev/gonest"

// UserPatchDTO usa Value[T] nos campos mutáveis.
// Campos omitidos no JSON ficam com IsDirty() == false e são ignorados.
type UserPatchDTO struct {
  Name gonest.Value[string] `json:"name"`
  Age  gonest.Value[int]    `json:"age"`
}

var userPatchSchema = gonest.NewSchema[UserPatchDTO](func(t *UserPatchDTO, m *gonest.Schema) {
  // As constraints só são verificadas se o campo vier no payload (dirty).
  // Quando omitido, não existe violation de Required -- é um PATCH, não um PUT.
  m.Property(&t.Name).String().Min(2)
  m.Property(&t.Age).Integer().Min(0)
})

var _ = gonest.NewController(func(c *gonest.Controller) {
  c.Route(gonest.HttpPatch, "/:user_id", func(r *gonest.Route) {
    r.HttpCode(gonest.HttpStatusOk)
    r.Params(userIdParamSchema)
    r.RequestBody(userPatchSchema)
    r.Handler(func(ctx *gonest.RestContext) {
      params := gonest.MustParseRestParams[*UserIdParam](ctx, userIdParamSchema)
      patch  := gonest.MustParseRestJsonBody[*UserPatchDTO](ctx, userPatchSchema)

      user := userService.Get(params.UserId)

      // Cada campo só é aplicado se tiver sido enviado no payload.
      patch.Name.OnDirty(func(name string) { user.Name = name })
      patch.Age.Apply(&user.Age)

      // Alternativa para ORMs que aceitam map (ex: GORM):
      // db.Model(user).Updates(gonest.ValueToDirtyMap(patch))

      ctx.Json(user)
    })
  })
})
```

# exemplo de exceptions (erro no body: `{ name, message, details }`)

```go
package ex

import (
  "gonest.dev/gonest"
)

// exceptions built-in do framework (todas embedam HttpException)
// gonest.NotFoundException     -> 404
// gonest.BadRequestException   -> 400
// gonest.ConflictException     -> 409
// gonest.UnauthorizedException -> 401
// gonest.ForbiddenException    -> 403

// exception de domínio, criada pelo dev do mesmo jeito que as built-in --
// NewHttpException() é zero-arg (status default 500), cada Set* devolve
// uma CÓPIA (builder imutável, não muta o receiver).
type FooExampleError struct { gonest.HttpException }
func NewFooExampleError(details any) *FooExampleError {
  return &FooExampleError{
    HttpException: gonest.NewHttpException().
      SetStatus(gonest.HttpStatusBadRequest).
      SetName("FooExampleError").
      SetMessage("lorem ipsum dolor met").
      SetDetails(details),
  }
}

type FooService struct{}
func (t *FooService) ShouldThrow(input string) { panic(NewFooExampleError(map[string]any{"input": input})) }

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

  "gonest.dev/gonest"
  "github.com/google/uuid"
)

// ---------- Middleware ----------
// roda antes do roteamento (raw request/response), tipo express middleware.
// não decide autorização, só observa/mutação de contexto (log, request-id etc).
var RequestIdMiddleware = gonest.NewMiddleware(func (middleware *gonest.Middleware) {
  middleware.Handler(func(ctx *gonest.RestContext, next gonest.Next) {
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

  guard.Handler(func(ctx *gonest.RestContext) bool {
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

  interceptor.Handler(func(ctx *gonest.RestContext, next gonest.Next) {
    start := time.Now()
    next(ctx)
    logger.Log("request took", time.Since(start))
  })
})

// ---------- Filter ----------
// captura exceptions específicas e customiza status/body da resposta.
// exceptions não capturadas por nenhum filter caem no handler default (name/message/details).
var FooExampleFilter = gonest.NewFilter(func (filter *gonest.Filter) {
  filter.Catch(&FooExampleError{}, func(ctx *gonest.RestContext, exc *FooExampleError) {
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

var prefixedUserIdParamSchema = gonest.NewSchema[PrefixedUserIdParam](func (t *PrefixedUserIdParam, m *gonest.Schema) {
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
  // outro campo de MustParseRestParams).
  controller.Route(gonest.HttpGet, "/:user_id", func (route *gonest.Route) {
    route.HttpCode(gonest.HttpStatusOk)
    route.Handler(func(ctx *gonest.RestContext) {
      params := gonest.MustParseRestParams[*PrefixedUserIdParam](ctx, prefixedUserIdParamSchema)
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

  "gonest.dev/gonest"
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
