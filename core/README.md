# GoNest Core Module

The core module provides the fundamental architecture for building NestJS-style applications in Go.

## Features

- ✅ **Module System**: Organize your application into modules
- ✅ **Dependency Injection**: Automatic dependency resolution
- ✅ **Lifecycle Hooks**: Control initialization and shutdown
- ✅ **Request Context**: Type-safe request handling
- ✅ **Routing**: Simple and intuitive route registration
- ✅ **Metadata Storage**: Attach metadata to types

## Installation

```bash
go get github.com/leandroluk/gonest
```

## Quick Start

### 1. Create a Module

```go
package main

import "github.com/leandroluk/gonest/core"

type AppModule struct{}

func (m *AppModule) Configure(b *core.ModuleBuilder) {
  b.Controllers(&AppController{}).
    Providers(&AppService{})
}
```

### 2. Create a Service

```go
type AppService struct{}

func (s *AppService) GetHello() string {
  return "Hello from GoNest!"
}

// Lifecycle hook (optional)
func (s *AppService) OnModuleInit(ctx context.Context) error {
  log.Println("Service initialized")
  return nil
}
```

### 3. Create a Controller

```go
type AppController struct {
  appService *AppService
}

func (c *AppController) Routes() []core.RouteDefinition {
  return []core.RouteDefinition{
    { Method:  "GET", Path:  "/", Handler: c.GetHello },
    { Method:  "GET", Path:  "/user/:id", Handler: c.GetUser },
    { Method:  "POST", Path:  "/user", Handler: c.CreateUser },
  }
}

func (c *AppController) GetHello(ctx *core.Context) error {
  message := c.appService.GetHello()
  return ctx.JSON(200, map[string]string{"message": message})
}

func (c *AppController) GetUser(ctx *core.Context) error {
  id := ctx.Param("id")
  return ctx.JSON(200, map[string]string{"id": id, "name": "User " + id})
}

func (c *AppController) CreateUser(ctx *core.Context) error {
  var dto CreateUserDto
  if err := ctx.BindJSON(&dto); err != nil {
    return ctx.JSON(400, map[string]string{"error": "Invalid body"})
  }
  
  return ctx.JSON(201, dto)
}
```

### 4. Bootstrap Application

```go
func main() {
  app := core.NestFactory{}.Create(&AppModule{})
  
  if err := app.Listen(":3000"); err != nil {
    log.Fatal(err)
  }
}
```

## Module System

### Basic Module

```go
type UserModule struct{}

func (m *UserModule) Configure(b *core.ModuleBuilder) {
  b.Controllers(&UserController{}).
    Providers(&UserService{}, &UserRepository{})
}
```

### Module with Imports

```go
type AppModule struct{}

func (m *AppModule) Configure(b *core.ModuleBuilder) {
  b.Imports(&UserModule{}, &AuthModule{}).
    Controllers(&AppController{}).
    Providers(&AppService{})
}
```

### Module with Exports

```go
type SharedModule struct{}

func (m *SharedModule) Configure(b *core.ModuleBuilder) {
  b.Providers(&SharedService{}).
    Exports(&SharedService{})  // Make available to importing modules
}
```

## Lifecycle Hooks

### OnModuleInit

Called when the module is initialized, after dependencies are resolved.

```go
func (s *DatabaseService) OnModuleInit(ctx context.Context) error {
  return s.Connect()
}
```

### OnApplicationBootstrap

Called after all modules are initialized.

```go
func (s *CacheService) OnApplicationBootstrap(ctx context.Context) error {
  return s.WarmupCache()
}
```

### OnModuleDestroy

Called when the module is being destroyed.

```go
func (s *DatabaseService) OnModuleDestroy(ctx context.Context) error {
  return s.Disconnect()
}
```

### OnApplicationShutdown

Called before the application shuts down.

```go
func (s *QueueService) OnApplicationShutdown(ctx context.Context) error {
  return s.FlushQueue()
}
```

## Request Context

The `Context` provides a unified interface for handling HTTP requests:

### Reading Request Data

```go
func (c *Controller) Handler(ctx *core.Context) error {
  // Path parameters
  id := ctx.Param("id")
  
  // Query parameters
  page := ctx.Query("page")
  limit := ctx.QueryDefault("limit", "10")
  
  // Headers
  auth := ctx.Header("Authorization")
  
  // JSON body
  var dto CreateDto
  if err := ctx.BindJSON(&dto); err != nil {
    return err
  }
  
  // Raw body
  body, err := ctx.Body()
}
```

### Sending Responses

```go
// JSON response
ctx.JSON(200, map[string]string{"status": "ok"})

// String response
ctx.String(200, "Hello, World!")

// HTML response
ctx.HTML(200, "<h1>Hello</h1>")

// Raw data
ctx.Data(200, "application/pdf", pdfBytes)

// Status only
ctx.Status(204)
```

### Context Metadata

```go
// Store data
ctx.Set("user", user)
ctx.Set("requestId", uuid.New())

// Retrieve data
user := ctx.Get("user")
userId := ctx.GetString("userId")
isAdmin := ctx.GetBool("isAdmin")
count := ctx.GetInt("count")
```

## Routing

### Basic Routes

```go
func (c *Controller) Routes() []core.RouteDefinition {
  return []core.RouteDefinition{
    {Method: "GET", Path: "/users", Handler: c.GetUsers},
    {Method: "POST", Path: "/users", Handler: c.CreateUser},
    {Method: "GET", Path: "/users/:id", Handler: c.GetUser},
    {Method: "PUT", Path: "/users/:id", Handler: c.UpdateUser},
    {Method: "DELETE", Path: "/users/:id", Handler: c.DeleteUser},
  }
}
```

### Path Parameters

```go
// Route: /users/:id/posts/:postId
func (c *Controller) GetUserPost(ctx *core.Context) error {
  userId := ctx.Param("id")
  postId := ctx.Param("postId")
  // ...
}
```

### Middleware

```go
func LoggingMiddleware(next core.HandlerFunc) core.HandlerFunc {
  return func(ctx *core.Context) error {
    start := time.Now()
    err := next(ctx)
    log.Printf("%s %s - %v", ctx.Method(), ctx.Path(), time.Since(start))
    return err
  }
}

// Apply to route
{
  Method: "GET",
  Path: "/protected",
  Handler: c.Protected,
  Middlewares: []core.MiddlewareFunc{LoggingMiddleware, AuthMiddleware},
}
```

## Application Options

```go
app := core.NestFactory{}.Create(
  &AppModule{},
  core.WithShutdownTimeout(30 * time.Second),
  core.WithReadTimeout(10 * time.Second),
  core.WithWriteTimeout(10 * time.Second),
  core.WithIdleTimeout(120 * time.Second),
)
```

## Graceful Shutdown

The application handles graceful shutdown automatically:

```go
app.Listen(":3000")  // Blocks until SIGINT or SIGTERM

// On shutdown:
// 1. Calls OnApplicationShutdown hooks
// 2. Calls OnModuleDestroy hooks
// 3. Stops HTTP server gracefully
```

## Error Handling

```go
func (c *Controller) Handler(ctx *core.Context) error {
  if err := doSomething(); err != nil {
    return ctx.JSON(500, map[string]string{
      "error": err.Error(),
    })
  }
  return ctx.JSON(200, result)
}
```

## Testing

```go
func TestController(t *testing.T) {
  // Create test context
  req := httptest.NewRequest("GET", "/test", nil)
  w := httptest.NewRecorder()
  ctx := core.NewContext(w, req)
  
  // Test handler
  controller := &TestController{}
  err := controller.GetTest(ctx)
  
  assert.NoError(t, err)
  assert.Equal(t, 200, ctx.StatusCode())
}
```

## Best Practices

1. **Single Responsibility**: Each module should have a single, well-defined purpose
2. **Dependency Direction**: Dependencies should flow inward (controllers → services → repositories)
3. **Lifecycle Management**: Use lifecycle hooks for initialization and cleanup
4. **Error Handling**: Always return errors from handlers for proper middleware handling
5. **Metadata Storage**: Use the metadata system for cross-cutting concerns

## Next Steps

- Explore the [DI Module](../di/README.md) for advanced dependency injection
- Check out [Validation](../validator/README.md) for request validation
- Learn about [Guards](../guards/README.md) for route protection
- See [Interceptors](../interceptors/README.md) for request/response transformation

## License

MIT