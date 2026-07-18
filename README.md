<div align="center">
  <h1>gonest</h1>
  <p><strong>A NestJS-inspired dependency-injection and HTTP framework for Go.</strong></p>
</div>

<div align="center">
  <a href="https://pkg.go.dev/gonest.dev/gonest">
    <img src="https://pkg.go.dev/badge/gonest.dev/gonest.svg" alt="Go Reference" />
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License" />
  </a>
  <a href="https://gonest.dev">
    <img src="https://img.shields.io/badge/docs-gonest.dev-14A9BE.svg" alt="Documentation" />
  </a>
</div>

<br/>

---

## Contents

- [Contents](#contents)
- [Getting started](#getting-started)
- [Implementation Status](#implementation-status)
- [Next Steps](#next-steps)
- [About the Project](#about-the-project)
- [Documentation](#documentation)
  - [Modules, Providers, Controllers](#modules-providers-controllers)
  - [Exceptions](#exceptions)
  - [Middleware, Guard, Interceptor, Filter](#middleware-guard-interceptor-filter)
  - [Multi-binding (MustInjectAll)](#multi-binding-mustinjectall)
  - [Schema declaration + validation](#schema-declaration--validation)
  - [Path params, query string, JSON body](#path-params-query-string-json-body)
  - [Partial updates (Accessor dirty-tracking)](#partial-updates-accessor-dirty-tracking)
  - [File upload (multipart/form-data streaming)](#file-upload-multipartform-data-streaming)
  - [OpenAPI / Swagger](#openapi--swagger)
  - [GraphQL](#graphql)
  - [Event Emitter](#event-emitter)
  - [Scheduler (Cron/Interval/Timeout)](#scheduler-cronintervaltimeout)
  - [Health checks (Terminus)](#health-checks-terminus)
  - [Testing](#testing)
  - [Development](#development)
- [Contributors](#contributors)
- [License](#license)

---

## Getting started

```go
package main

import "gonest.dev/gonest"

type UserService struct{}

func (s *UserService) List() []string { return []string{"Ada", "Grace"} }

var UserProvider = gonest.NewProvider(func(provider *gonest.Provider) {
  provider.Constructor(func() *UserService { return &UserService{} })
})

var UserController = gonest.NewController(func(controller *gonest.Controller) {
  controller.Path("/users")
  userService := gonest.MustInject[*UserService](controller)

  controller.RouteGet("/", func(route *gonest.Route) {
    route.Handler(func(req *gonest.Request, res *gonest.Response) {
      res.Json(userService.List())
    })
  })
})

var UserModule = gonest.NewModule(func(module *gonest.Module) {
  module.Providers(UserProvider)
  module.Controllers(UserController)
})

func main() {
  app := gonest.MustNewApp[gonest.FiberApp](UserModule) // AppOptions is optional
  app.MustListen(":3000")
}
```

See [`.examples/simple-todo`](.examples/simple-todo) for a minimal MVC example (no external
dependencies), [`.examples/blog-api`](.examples/blog-api) for a denser one (guards,
interceptors, middleware, filters, OpenAPI/Swagger, SQLite persistence, a real 3-module domain),
and [`.examples/blog-graphql`](.examples/blog-graphql) for GraphQL (Query/Mutation/Subscription
over SSE and WebSocket). See [Documentation](#documentation) below for the full API, or
[gonest.dev](https://gonest.dev) for the full docs site.

---

## Implementation Status

- [x] M1 - Core DI & Module System (`Module`, `Provider`, `Controller`, `MustInject`)
- [x] M2 - Exceptions & Response Contract (`HttpException`, built-in exceptions, panic-recovery default handler)
- [x] M3 - Request Pipeline (`Middleware`, `Guard`, `Interceptor`, `Filter`)
- [x] M4 - Schema Builder — Primitives (`NewSchema[T]`, `String`/`Integer`/`Boolean`/etc branches)
- [x] M5 - Schema Builder — Array & Object (nested schemas, `$ref`-style reuse)
- [x] M6 - Runtime Validation (`gonest.Parse[T]`/`gonest.MustParse[T]`, `Custom(fn)`)
- [x] M7 - OpenAPI Generation (`GenerateOpenApiSchema`, `SetupSwagger`)
- [x] M8 - Testing Helpers (`MustNewTestApp`, `MustOverride`, `MustRequest`)
- [x] M9 - Event Emitter (`gonest.Emitter`, `NewListener`, `MustOn`)
- [x] M10 - Scheduler (`Cron`/`Interval`/`Timeout`, `Stop`)
- [x] M11 - Terminus / health checks (`MustInjectAll[Pingable]` pattern, no dedicated bootstrap type needed)
- [x] M12 - Multipart Form Streaming (`req.Body().Form(onFile)`, true streaming file upload -- see below)
- [x] M13 - Unified Parse API (`gonest.Parse[T]`/`MustParse[T]`, `Parseable`, `req.Params()`/`Query()`/`Headers()`/`Body()`)
- [x] M14 - Request/Response Split (`gonest.Request`/`gonest.Response`, Express-style handler signature)
- [x] M15 - Schema Value Support (`gonest.NewValue[T]`/`gonest.Value` for standalone primitive schemas, `gonest.Accessor[T]` dirty-tracking wrapper -- renamed from `Value[T]`)
- [x] M16 - Schema Sanitize/Refine (`PropertyBuilder.Sanitize(fn)` pre-processing, `Schema.Refine(fn)` cross-field post-processing)
- [x] M17 - GraphQL Support (`gonest.NewGraphqlResolver`/`Query`/`Mutation`/`Subscription`, SDL generation, SSE/WebSocket subscriptions -- see below)

See `.specs/project/ROADMAP.md` for the full milestone breakdown and `.specs/project/STATE.md` for
the history of architecture decisions.

---

## Next Steps

Milestones 1-17 are all complete (see git tags for the full release history).
Versioning follows `v0.{major}.{minor}` under a fixed leading `v0` (never incrementing to `v1`/`v2`,
sidestepping Go's `v2+` import-path-suffix requirement while keeping semver's "no stability
guarantee yet" signal for `v0.x`) -- `major` bumps on a breaking change, `minor` on a
backward-compatible feature.

Planned, not yet started (see `.specs/project/ROADMAP.md`'s "Future Considerations"):

- Multi-adapter HTTP abstraction (`net/http`, Echo, Gin) -- v1 is Fiber-only
- CLI scaffolding (equivalent to `nest new`/`nest generate`)
- Microservices/transport layer (equivalent to `@nestjs/microservices`)
- gRPC adapter (early exploratory notes: `INSIGHT-GRPC.md`, `INSIGHT-MICROSERVICE.md`)

---

## About the Project

gonest brings NestJS's dependency-injection/module/decorator-driven ergonomics to Go, using
generics and field-pointer references instead of struct tags or a decorator equivalent (Go has
none) or code generation. A `Module` declares its own `Provider`s and `Controller`s, imports other
modules, and exports a subset of its providers to importers -- `MustInject[T]`/`MustInjectAll[T]`
resolve dependencies by type (pointer exact-match, or interface match for multi-binding).

`NewApp[T HttpAdapter]`/`MustNewApp[T]` bootstrap the whole DI graph in 3 phases (Provider
resolution → Controller declaration → Pipeline-stage-type declaration for
Middleware/Guard/Interceptor/Filter) before ever registering a route, so `MustInject` inside any of
those builders is always a direct, already-resolved lookup -- never a placeholder. v1 ships one
real `HttpAdapter`: `gonest.FiberApp`, backed by [Fiber v3](https://github.com/gofiber/fiber).

Request validation (path params, query string, JSON body, multipart form fields) and OpenAPI schema
generation are driven by the exact same declaration: `NewSchema[T]` builds a `*Schema` once, and
that same value is what both `gonest.Parse[T]`/`Route.RequestBody`/`Route.Response` consume --
no second, parallel declaration to keep in sync.

See `.specs/project/PROJECT.md` for the full vision/scope and `.specs/project/STATE.md` for the
history of design decisions.

---

## Documentation

### Modules, Providers, Controllers

```go
package ex

import "gonest.dev/gonest"

type UserService struct{ db *sql.DB }

var UserProvider = gonest.NewProvider(func(provider *gonest.Provider) {
  provider.Scope(gonest.ScopeSingleton) // ScopeSingleton (default) | ScopeTransient | ScopeRequest
  db := gonest.MustInject[*sql.DB](provider) // Provider-to-Provider dependency
  provider.Constructor(func() *UserService { return &UserService{db: db} })
})

var UserController = gonest.NewController(func(controller *gonest.Controller) {
  controller.Path("/users")
  controller.Tags("users")     // OpenAPI tag, inherited by every route below
  controller.BearerAuth()      // OpenAPI security requirement, inherited too

  userService := gonest.MustInject[*UserService](controller)

  controller.RouteGet("/", func(route *gonest.Route) {
    route.Summary("List users")
    route.Handler(func(req *gonest.Request, res *gonest.Response) { res.Json(userService.List()) })
  })
})

var UserModule = gonest.NewModule(func(module *gonest.Module) {
  module.Providers(UserProvider)
  module.Controllers(UserController)
  module.Exports(UserProvider) // makes UserProvider resolvable by modules that Import this one
})

var AppModule = gonest.NewModule(func(module *gonest.Module) {
  module.Imports(UserModule)
})
```

`NewApp`/`MustNewApp` take `AppOptions` as an optional (variadic, at most one) argument --
`NewApp[gonest.FiberApp](AppModule)` and `NewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})`
are equivalent; passing more than one `AppOptions` panics. It configures bootstrap:
`BufferLogs`/`LogLevels` (real `internal/logger` wiring), `DisableBanner`/`DisableLoaded` (startup
output), `EnableFormStreaming` (see [File upload](#file-upload-multipartform-data-streaming)
below).

`controller.Route(method, path, fn)` (still available, e.g. for a method chosen at runtime) takes
any `gonest.HttpMethod` (`HttpGet`/`HttpPost`/`HttpPut`/`HttpPatch`/`HttpDelete`/`HttpHead`/
`HttpOptions`/`HttpTrace`/`HttpConnect`/`HttpQuery`). Each verb also has a shorthand that skips the
method argument: `controller.RouteGet(path, fn)`, `RoutePost`, `RoutePut`, `RoutePatch`,
`RouteDelete`, `RouteHead`, `RouteOptions`, `RouteTrace`, `RouteConnect`, `RouteQuery` -- same
registration, less repetition.

### Exceptions

```go
package ex

import (
  "net/http"

  "gonest.dev/gonest"
)

// Built-in exceptions (all embed HttpException):
// gonest.NotFoundException(404) / BadRequestException(400) / ConflictException(409) /
// UnauthorizedException(401) / ForbiddenException(403)

// Domain-defined exception, same shape as a built-in:
type FooError struct{ gonest.HttpException }

func NewFooError(details any) *FooError {
  return &FooError{
    HttpException: gonest.NewHttpException().
      SetStatus(http.StatusBadRequest).
      SetName("FooError").
      SetMessage("lorem ipsum").
      SetDetails(details),
  }
}

// A panic with any Exception-satisfying value (built-in or domain-defined) becomes:
// { "name": "FooError", "message": "lorem ipsum", "details": {...} }
// with the exception's own status code. A panic with anything else (nil pointer, index
// out of range, etc) becomes a generic 500, without leaking internal detail.
```

`gonest.HttpStatus*` re-exports all 63 standard `net/http.StatusXxx` constants under the `gonest`
namespace (`gonest.HttpStatusNotFound`, etc), so route/handler code never needs to import `net/http`
just for a status code.

### Middleware, Guard, Interceptor, Filter

```go
package ex

import "gonest.dev/gonest"

// Middleware: runs before routing (raw request/response), like Express middleware.
var RequestIdMiddleware = gonest.NewMiddleware(func(middleware *gonest.Middleware) {
  middleware.Handler(func(req *gonest.Request, res *gonest.Response, next gonest.Next) {
    res.SetHeader("X-Request-Id", "...")
    next(req, res)
  })
})

// Guard: decides whether the request proceeds. false = automatic 403 Forbidden;
// panic with an Exception for a custom message instead.
var AuthGuard = gonest.NewGuard(func(guard *gonest.Guard) {
  guard.Handler(func(req *gonest.Request, res *gonest.Response) bool {
    return req.Header("Authorization") != ""
  })
})

// Interceptor: wraps the handler's execution (before/after), like AOP.
var TimingInterceptor = gonest.NewInterceptor(func(interceptor *gonest.Interceptor) {
  interceptor.Handler(func(req *gonest.Request, res *gonest.Response, next gonest.InterceptorNext) {
    next(req, res)
  })
})

// Filter: catches specific exception types and customizes the response.
var FooFilter = gonest.NewFilter(func(filter *gonest.Filter) {
  filter.Catch(&FooError{}, func(req *gonest.Request, res *gonest.Response, exc *FooError) {
    res.Status(http.StatusTeapot).Json(map[string]any{"custom": true})
  })
})

var UserController = gonest.NewController(func(controller *gonest.Controller) {
  controller.Use(RequestIdMiddleware)
  controller.Guards(AuthGuard)
  controller.Interceptors(TimingInterceptor)
  controller.Filters(FooFilter)
})

// Or applied globally, at the root module:
var AppModule = gonest.NewModule(func(module *gonest.Module) {
  module.Use(RequestIdMiddleware)
  module.Filters(FooFilter)
})
```

### Multi-binding (MustInjectAll)

```go
package ex

import "gonest.dev/gonest"

type Connectable interface{ Ping() bool }

var SystemController = gonest.NewController(func(controller *gonest.Controller) {
  // resolves EVERY provider whose concrete type implements Connectable --
  // MustInject[T] (T interface) requires exactly 1 match; MustInjectAll never panics
  // on ambiguity, useful for plugin/strategy patterns.
  connectables := gonest.MustInjectAll[Connectable](controller)
  _ = connectables
})
```

### Schema declaration + validation

```go
package ex

import "gonest.dev/gonest"

type UserEntity struct {
  Id    int64  `json:"id"`
  Name  string `json:"name"`
  Email string `json:"email"`
}

var userEntitySchema = gonest.NewSchema[UserEntity](func(t *UserEntity, s *gonest.Schema) {
  s.Title("UserEntity") // components.schemas name in OpenAPI; defaults to the Go type name
  s.Property(&t.Id).Integer().Required()
  s.Property(&t.Name).String().Required().Min(1).Max(50)
  s.Property(&t.Email).Email().Required()
})
```

Branches: `String`/`Email`/`Uuid`/`Uri`/`Hostname`/`Ipv4`/`Ipv6`/`Password`/`Byte`/`Binary`,
`Integer`/`Int32`, `Float`/`Double`, `Boolean`, `DateTime`/`Date`, `Array`, `Object`. Each field
also takes `Required()`/`Nullable()`/`Description()`/`Examples()`, plus `Custom(fn)` as an escape
hatch for domain-specific formats the fixed vocabulary doesn't cover.

`Sanitize(fn func(raw any) any)` pre-processes a field's raw value BEFORE any other check runs
(including `Custom`) -- unlike `Custom`, it never replaces `Min`/`Max`/`Pattern`, it only transforms
what they go on to check:

```go
m.Property(&t.Cpf).String().Min(11).Max(11).Pattern(`^\d{11}$`).Sanitize(func(raw any) any {
  s, _ := raw.(string)
  return strings.TrimSpace(s) // "  12345678901  " still passes Min/Max/Pattern after this
}).Required()
```

`Schema.Refine(fn func(dst any) (field string, err error))` registers a cross-field check, run only
after every individual field has validated AND been populated -- for comparisons no single field can
know on its own (e.g. password confirmation):

```go
m.Refine(func(dst any) (field string, err error) {
  d := dst.(*UpdateUserDTO)
  if d.Password != d.ConfirmPassword {
    return "confirmPassword", errors.New("must match password")
  }
  return "", nil
})
```

For a value with no struct around it at all (e.g. a bare CPF string), `gonest.NewValue[T]` builds a
`*Schema` directly, reusing the same branch methods:

```go
var cpfSchema = gonest.NewValue[string](func(m *gonest.Value) {
  m.String().Min(11).Max(11).Pattern(`^\d{11}$`).Required()
})
```

`gonest.Accessor[T]` (renamed from `Value[T]`) is the unrelated dirty-tracking field wrapper for
PATCH-style handlers -- see the JSON body example below.

### Path params, query string, JSON body

```go
package ex

import "gonest.dev/gonest"

type UserIdParams struct {
  UserId int64 `param:"user_id"`
}

var userIdParamsSchema = gonest.NewSchema[UserIdParams](func(t *UserIdParams, s *gonest.Schema) {
  s.Property(&t.UserId).Integer().Min(1).Required()
})

var UserController = gonest.NewController(func(controller *gonest.Controller) {
  controller.RouteGet("/:user_id", func(route *gonest.Route) {
    route.Params(userIdParamsSchema)                     // documents it in OpenAPI
    route.Response(gonest.HttpStatusOk, func(response *gonest.RouteResponse) {
      response.Schema(userEntitySchema)
    })

    route.Handler(func(req *gonest.Request, res *gonest.Response) {
      // gonest.MustParse uses the SAME Schema for runtime validation --
      // the Schema value must be passed explicitly (a compile-time guarantee
      // that it's never forgotten, not a hidden global-registry lookup).
      params := gonest.MustParse[UserIdParams](req.Params(), userIdParamsSchema)
      res.Json(params)
    })
  })
})
```

`param:"..."`/`query:"..."`/`json:"..."` are separate tag families (one per source), each read via
`req.Params()`/`req.Query()`/`req.Body().Json()` (a `Parseable`). Every
`gonest.MustParse[T](src, schema)` has a non-panicking `gonest.Parse[T](src, schema) (T, error)`
twin for callers that want to handle the error themselves.

### Partial updates (Accessor dirty-tracking)

`gonest.Accessor[T]` tracks whether a field was explicitly present in the payload -- including
explicit `null` -- so a PATCH handler applies only what the client actually sent:

```go
type UpdateUserDTO struct {
  Name  gonest.Accessor[string] `json:"name"`
  Email gonest.Accessor[string] `json:"email"`
}

// only fields present in the JSON body are marked dirty
body := gonest.MustParse[*UpdateUserDTO](req.Body().Json(), updateUserSchema)
body.Name.OnDirty(func(name string) { user.Name = name })
body.Email.Apply(&user.Email)

// or build a partial-update map directly, keyed by json tag
changes := gonest.AccessorsToDirtyMap(body)
```

### File upload (multipart/form-data streaming)

Most frameworks (including NestJS's own `multer`/`FileInterceptor`) buffer an uploaded file to
memory or a local temp file before handler code ever runs. gonest streams it instead: `onFile`
fires the instant a file part is seen in the raw multipart stream, so a handler can start
forwarding bytes to S3 (or any `io.Writer`) without ever buffering the whole file locally.

```go
package ex

import "gonest.dev/gonest"

type CreatePostForm struct {
  Title string `form:"title"`
}

var createPostFormSchema = gonest.NewSchema[CreatePostForm](func(t *CreatePostForm, s *gonest.Schema) {
  s.Property(&t.Title).String().Required()
})

var PostController = gonest.NewController(func(controller *gonest.Controller) {
  controller.RoutePost("/", func(route *gonest.Route) {
    // Documents the requestBody as multipart/form-data (not application/json)
    // -- "file" becomes {type: string, format: binary}, so Swagger UI renders
    // a real file-upload widget for this route.
    route.FormBody(createPostFormSchema, "file")
    route.Handler(func(req *gonest.Request, res *gonest.Response) {
      form := gonest.MustParse[CreatePostForm](req.Body().Form(func(f *gonest.FormFile) error {
        // f.Reader() is the still-unconsumed part -- pipe it straight to S3/etc.
        return uploadToS3(f.Filename(), f.ContentType(), f.Reader())
      }), createPostFormSchema)
      res.Json(map[string]string{"title": form.Title})
    })
  })
})

func main() {
  app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{
    EnableFormStreaming: true, // app-wide setting; existing JSON/param/query routes are unaffected
  })
  app.MustListen(":3000")
}
```

### OpenAPI / Swagger

```go
package ex

import "gonest.dev/gonest"

func main() {
  app := gonest.MustNewApp[gonest.FiberApp](AppModule, gonest.AppOptions{})

  doc := gonest.NewOpenAPI("3.1.0", func(b *gonest.OpenAPI) {
    b.Title("Example API")
    b.Version("1.0.0")
    b.BearerAuth()
  })
  gonest.GenerateOpenApiSchema(app, doc) // walks the whole module tree, populates paths/components.schemas

  gonest.SetupSwagger(app, "/docs", doc, gonest.SwaggerOptions{
    JsonDocumentUrl: "/openapi.json",
    PersistAuth:     true,
  })

  app.MustListen(":3000")
}
```

### GraphQL

`GraphqlResolver` is analogous to `Controller` -- registered via `module.Resolvers(...)`, reusing the
same `Schema`/`Parse[T]`/`MustInject` REST already uses:

```go
package ex

import "gonest.dev/gonest"

var UserResolver = gonest.NewGraphqlResolver(func(resolver *gonest.GraphqlResolver) {
  userService := gonest.MustInject[*UserService](resolver)

  resolver.Query("user", func(query *gonest.GraphqlQuery) {
    query.Args(userIdParamsSchema)   // same Schema already used for REST validation
    query.Returns(userEntitySchema)  // same Schema drives the generated SDL too

    query.Handler(func(ctx *gonest.GraphqlContext) any {
      args := gonest.MustParse[UserIdParams](ctx.Args(), userIdParamsSchema)
      return userService.Get(args.UserId) // the returned value IS the data, no Response/write-side
    })
  })

  resolver.Subscription("orderCreated", func(subscription *gonest.GraphqlSubscription) {
    subscription.Returns(orderEntitySchema)
    subscription.Handler(func(ctx *gonest.GraphqlContext, emit func(any)) {
      // gonest.Subscribe[T] is Emitter's dynamic counterpart to MustOn --
      // a channel that lives only as long as ctx.Done() stays open (one
      // connection), closed automatically when the client disconnects.
      emitter := gonest.MustInject[*gonest.Emitter](resolver)
      for event := range gonest.Subscribe[OrderCreatedEvent](emitter, ctx.Done()) {
        emit(event)
      }
    })
  })
})
```

Query/Mutation dispatch through one `POST /graphql` endpoint by default (standard `{query, variables,
operationName}` → `{data, errors}`) -- override via `AppOptions{GraphqlPath: "/api/gql"}`. Subscriptions
get their own `GET <path>/stream/:name` (SSE) and `GET <path>/ws/:name` (WebSocket) endpoints, derived
from the same path, args passed via `?args=<JSON>`. Branches with a format (`Email`/`Uuid`/`DateTime`/etc)
become GraphQL Custom Scalars automatically in the generated SDL; `Custom(fn).GraphqlScalar(name)` names
a scalar for a `Custom(fn)` field with no native format equivalent (e.g. `primitive.ObjectID`).

### Event Emitter

```go
package ex

import (
  "context"

  "gonest.dev/gonest"
)

type UserCreatedEvent struct{ UserID int64 }

var UserCreatedListener = gonest.NewListener(func(listener *gonest.Listener) {
  gonest.MustOn[UserCreatedEvent](listener, func(ctx context.Context, event UserCreatedEvent) {
    // ...
  })
})

var UserModule = gonest.NewModule(func(module *gonest.Module) {
  module.Listeners(UserCreatedListener)
})

// emitting: gonest.Emitter is a framework singleton, resolvable from any module with
// no explicit registration.
var UserProvider = gonest.NewProvider(func(provider *gonest.Provider) {
  emitter := gonest.MustInject[*gonest.Emitter](provider)
  provider.Constructor(func() *UserService { return &UserService{emitter: emitter} })
})
```

`Emit` is fire-and-forget (1 goroutine per registered listener); a listener's panic/error never
propagates to the caller.

### Scheduler (Cron/Interval/Timeout)

```go
package ex

import (
  "context"
  "time"

  "gonest.dev/gonest"
)

var CleanupScheduler = gonest.NewScheduler(func(scheduler *gonest.Scheduler) {
  scheduler.Cron("cleanup", "0 0 * * *", func(ctx context.Context) { /* ... */ })
  scheduler.Interval("ping", time.Minute, func(ctx context.Context) { /* ... */ })
  scheduler.Timeout("warmup", 5*time.Second, func(ctx context.Context) { /* ... */ })
})

var AppModule = gonest.NewModule(func(module *gonest.Module) {
  module.Schedulers(CleanupScheduler)
})
```

Each job runs isolated (its own recover) -- a panic never derails the process or blocks the next
run. `Scheduler.Stop(name)` cancels a running job.

### Health checks (Terminus)

No dedicated bootstrap type: a health check is just a normal `Controller` using
`MustInjectAll[Pingable]`.

```go
package ex

import (
  "context"

  "gonest.dev/gonest"
)

type Pingable interface {
  Name() string
  Ping(ctx context.Context) error
}

var HealthController = gonest.NewController(func(controller *gonest.Controller) {
  controller.Path("/health")
  pingables := gonest.MustInjectAll[Pingable](controller)

  controller.RouteGet("/readyz", func(route *gonest.Route) {
    route.Handler(func(req *gonest.Request, res *gonest.Response) {
      status := gonest.HttpStatusOk
      for _, p := range pingables {
        if p.Ping(context.Background()) != nil {
          status = gonest.HttpStatusServiceUnavailable
        }
      }
      res.Status(status).Json(map[string]string{"status": "ok"})
    })
  })
})
```

### Testing

```go
package ex_test

import (
  "testing"

  "gonest.dev/gonest"
)

func TestUserController_Get(t *testing.T) {
  tester := gonest.MustNewTestApp(UserModule, func(b *gonest.TestBuilder) {
    gonest.MustOverride[IUserService](b, &UserServiceMock{ /* ... */ })
  })
  defer tester.Close()

  res := tester.MustRequest(gonest.HttpGet, "/users/42", nil)
  res.AssertStatus(t, gonest.HttpStatusOk)
  res.AssertJsonPath(t, "id", int64(42))
}
```

`MustOverride` requires the dependency to be injected by INTERFACE (Go can't swap a concrete
`*struct`'s behavior at runtime with no vtable) -- see [`.examples`](.examples) for both a minimal
and a denser full example app.

### Development

This repo uses [`Taskfile.yml`](Taskfile.yml) ([go-task](https://taskfile.dev)) for the common
commands:

```sh
task           # build + vet + test
task test:race # full suite, -race, no cache
task examples  # build both .examples/* modules
task gate      # everything above, the full pre-commit check
task --list    # see all tasks
```

---

## Contributors

Thanks to all the people who contribute!

---

## License

[MIT](LICENSE)
