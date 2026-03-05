# GoNest Platform Adapters

Run GoNest applications on any Go web framework with **full type-safety** and zero code changes. Write once, deploy anywhere.

## Features

- ✅ **Type-Safe** - No `any` casts, full compile-time checking
- ✅ **Native Integration** - Direct access to framework Request/Response
- ✅ **Full Feature Support** - All GoNest features work seamlessly
- ✅ **Zero Overhead** - Minimal wrapper, native performance
- ✅ **IDE Support** - Full autocomplete and type inference

## Supported Platforms

All adapters provide **full support** with complete Request/Response access:

- ✅ **Mux (net/http)** - Direct `http.ResponseWriter` and `*http.Request`
- ✅ **Gin** - Uses `gin.Context` with full access to `Writer` and `Request`
- ✅ **Fiber** - Converts `fiber.Ctx` (fasthttp) to net/http compatible
- ✅ **Echo** - Uses `echo.Context` with direct `Response().Writer` and `Request()`
- ✅ **Chi** - Direct `http.ResponseWriter` and `*http.Request`

### Framework Dependencies

Adapters use the actual framework types for complete functionality:
```go
import "github.com/gin-gonic/gin"          // Gin adapter
import "github.com/gofiber/fiber/v2"       // Fiber adapter
import "github.com/labstack/echo/v4"       // Echo adapter
```

## Why Adapters?

**Problem:** Different frameworks have different APIs
```go
// Gin
router.GET("/user/:id", func(c *gin.Context) { ... })

// Fiber
app.Get("/user/:id", func(c *fiber.Ctx) error { ... })

// Echo
e.GET("/user/:id", func(c echo.Context) error { ... })
```

**Solution:** Write once with GoNest, run anywhere
```go
// Write GoNest handler once
func GetUser(ctx *core.Context) error {
    return ctx.JSON(200, user)
}

// Use on any platform
router.GET("/user/:id", adapters.ToGinHandler(GetUser))
app.Get("/user/:id", adapters.ToFiberHandler(GetUser))
e.GET("/user/:id", adapters.ToEchoHandler(GetUser))
```

## Type-Safety Benefits

### ✅ Before (with types)
```go
// Gin - Full type-safety
func ToGinHandler(handler core.HandlerFunc) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := core.NewContext(c.Writer, c.Request) // ✅ Direct access!
        handler(ctx)
    }
}

// Usage - No casts needed!
router.GET("/hello", adapters.ToGinHandler(HelloHandler))
//                                         ^^^^^^^^^^^^^ Type-safe!
```

### ❌ Alternative (generic)
```go
// Generic - Loses type information
func WrapHandler(handler core.HandlerFunc) any {
    return func(ginCtx any) { // ⚠️ Lost type info
        // Can't access c.Writer or c.Request
    }
}

// Usage - Manual cast required
router.GET("/hello", adapters.ToGinHandler(HelloHandler).(gin.HandlerFunc))
//                                                       ^^^^^^^^^^^^^^^^^ Yuck!
```

## Full Feature Integration

All adapters have **complete access** to Request/Response:

- ✅ **Gin** - Direct access to `c.Writer` and `c.Request`
- ✅ **Echo** - Direct access to `c.Response().Writer` and `c.Request()`
- ✅ **Fiber** - Converts fasthttp to net/http using `fasthttpadaptor`
- ✅ **Mux/Chi** - Native net/http, no conversion needed

This means all GoNest features work perfectly:
- JSON responses
- Request body parsing
- Headers, cookies, params
- File uploads
- Streaming responses

## Quick Start

### Mux (net/http)

```go
import (
    "net/http"
    "github.com/leandroluk/gonest/adapters"
    "github.com/leandroluk/gonest/core"
)

func HelloHandler(ctx *core.Context) error {
    return ctx.JSON(200, map[string]any{"message": "Hello!"})
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/hello", adapters.ToMuxHandlerFunc(HelloHandler))
    http.ListenAndServe(":3000", mux)
}
```

### Gin

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/leandroluk/gonest/adapters"
)

func main() {
    router := gin.Default()
    
    // Full support: path params, query, headers, body
    router.GET("/hello", adapters.ToGinHandler(HelloHandler))
    router.GET("/users/:id", adapters.ToGinHandler(GetUser))
    
    // Middleware support
    router.Use(adapters.ToGinMiddleware(LoggingMiddleware()))
    
    router.Run(":3000")
}
```

**Full Features:**
- ✅ Path parameters (`:id`)
- ✅ Query parameters
- ✅ Request headers
- ✅ Request body
- ✅ Response writing
- ✅ Status codes
- ✅ Middleware chain
- ✅ Access to original `gin.Context` via `ctx.Get("gin_context")`

### Fiber

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/leandroluk/gonest/adapters"
)

func main() {
    app := fiber.New()
    
    // Full support with fasthttp to net/http conversion
    app.Get("/hello", adapters.ToFiberHandler(HelloHandler))
    app.Get("/users/:id", adapters.ToFiberHandler(GetUser))
    
    // Middleware support
    app.Use(adapters.ToFiberMiddleware(LoggingMiddleware()))
    
    app.Listen(":3000")
}
```

**Full Features:**
- ✅ Path parameters (`:id`)
- ✅ Fasthttp to net/http conversion
- ✅ Request/Response adaptation
- ✅ Custom response writer wrapper
- ✅ Middleware chain
- ✅ Access to original `fiber.Ctx` via `ctx.Get("fiber_context")`

### Echo

```go
import (
    "github.com/labstack/echo/v4"
    "github.com/leandroluk/gonest/adapters"
)

func main() {
    e := echo.New()
    
    // Full support with direct Request/Response access
    e.GET("/hello", adapters.ToEchoHandler(HelloHandler))
    e.GET("/users/:id", adapters.ToEchoHandler(GetUser))
    
    // Middleware support
    e.Use(adapters.ToEchoMiddleware(LoggingMiddleware()))
    
    e.Start(":3000")
}
```

**Full Features:**
- ✅ Path parameters (`:id`)
- ✅ Direct access to `Request()` and `Response().Writer`
- ✅ Full HTTP support
- ✅ Middleware chain
- ✅ Access to original `echo.Context` via `ctx.Get("echo_context")`

### Chi

```go
import (
    "github.com/go-chi/chi/v5"
    "github.com/leandroluk/gonest/adapters"
)

func main() {
    r := chi.NewRouter()
    r.Get("/hello", adapters.ToChiHandler(HelloHandler))
    http.ListenAndServe(":3000", r)
}
```

## Full GoNest Features

All GoNest features work with adapters:

### Guards

```go
authGuard := guards.SimpleAuthGuard("secret-token")
handler := guards.ApplyGuards(HelloHandler, authGuard)

// Use on any platform
router.GET("/protected", adapters.ToGinHandler(handler))
```

### Interceptors

```go
logging := interceptors.SimpleLoggingInterceptor()
handler := interceptors.ApplyInterceptors(HelloHandler, logging)

app.Get("/logged", adapters.ToFiberHandler(handler))
```

### Exception Filters

```go
globalFilter := exceptions.NewGlobalExceptionFilter()

func ProtectedHandler(ctx *core.Context) error {
    if !authorized {
        return exceptions.UnauthorizedException("Access denied")
    }
    return ctx.JSON(200, data)
}

e.GET("/protected", adapters.ToEchoHandler(ProtectedHandler))
```

### Pipes & Validation

```go
func CreateUser(ctx *core.Context) error {
    dto, err := pipes.ValidateBody[CreateUserDto](ctx)
    if err != nil {
        return err
    }
    
    // Create user...
    return ctx.JSON(201, user)
}

r.Post("/users", adapters.ToChiHandler(CreateUser))
```

## Middleware Conversion

Convert GoNest middleware to platform middleware:

### Mux/Chi

```go
gonestMiddleware := func(next core.HandlerFunc) core.HandlerFunc {
    return func(ctx *core.Context) error {
        // Before
        err := next(ctx)
        // After
        return err
    }
}

mux.Use(adapters.ToChiMiddleware(gonestMiddleware))
```

### Gin

```go
router.Use(adapters.ToGinMiddleware(gonestMiddleware))
```

### Fiber

```go
app.Use(adapters.ToFiberMiddleware(gonestMiddleware))
```

### Echo

```go
e.Use(adapters.ToEchoMiddleware(gonestMiddleware))
```

## Complete Example

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/leandroluk/gonest/adapters"
    "github.com/leandroluk/gonest/core"
    "github.com/leandroluk/gonest/guards"
    "github.com/leandroluk/gonest/interceptors"
    "github.com/leandroluk/gonest/pipes"
)

// GoNest handler (platform-agnostic)
func GetUser(ctx *core.Context) error {
    id := ctx.Param("id")
    return ctx.JSON(200, map[string]any{"id": id})
}

func CreateUser(ctx *core.Context) error {
    dto, err := pipes.ValidateBody[CreateUserDto](ctx)
    if err != nil {
        return err
    }
    return ctx.JSON(201, dto)
}

func main() {
    router := gin.Default()
    
    // Setup guards and interceptors
    authGuard := guards.SimpleAuthGuard("secret")
    logging := interceptors.SimpleLoggingInterceptor()
    
    // Public route
    router.GET("/health", adapters.ToGinHandler(GetUser))
    
    // Protected route with guard
    protected := guards.ApplyGuards(GetUser, authGuard)
    router.GET("/users/:id", adapters.ToGinHandler(protected))
    
    // Route with interceptor
    logged := interceptors.ApplyInterceptors(CreateUser, logging)
    router.POST("/users", adapters.ToGinHandler(logged))
    
    router.Run(":3000")
}
```

## Adapter Configuration

Configure adapters with custom options:

```go
config := &adapters.AdapterConfig{
    ErrorHandler: func(err error, ctx any) error {
        // Custom error handling
        return err
    },
    Logger: customLogger,
}

adapter := adapters.NewGinAdapter(config)
handler := adapter.WrapHandler(HelloHandler)
```

## Best Practices

1. **Write handlers platform-agnostic** - Use `core.Context` only
2. **Compose features** - Guards → Interceptors → Handler
3. **Convert at the edge** - Only use adapters when registering routes
4. **Reuse handlers** - Same handler works across all platforms
5. **Test independently** - Test GoNest logic without platform dependencies

## Migration Guide

### From Gin

```go
// Before (Gin-specific)
func GetUser(c *gin.Context) {
    id := c.Param("id")
    c.JSON(200, gin.H{"id": id})
}

// After (GoNest)
func GetUser(ctx *core.Context) error {
    id := ctx.Param("id")
    return ctx.JSON(200, map[string]any{"id": id})
}

// Use adapter
router.GET("/users/:id", adapters.ToGinHandler(GetUser))
```

### From Fiber

```go
// Before (Fiber-specific)
func GetUser(c *fiber.Ctx) error {
    id := c.Params("id")
    return c.JSON(fiber.Map{"id": id})
}

// After (GoNest)
func GetUser(ctx *core.Context) error {
    id := ctx.Param("id")
    return ctx.JSON(200, map[string]any{"id": id})
}

// Use adapter
app.Get("/users/:id", adapters.ToFiberHandler(GetUser))
```

## Performance

Adapters add minimal overhead:
- Simple wrapper function
- No reflection at runtime
- Zero allocations per request
- Comparable to native performance

## License

MIT