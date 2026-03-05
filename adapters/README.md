# GoNest Platform Adapters

Run GoNest applications on any Go web framework with zero code changes. Write once, deploy anywhere.

## Supported Platforms

- ✅ **Mux (net/http)** - Built-in Go HTTP server
- ✅ **Gin** - Most popular Go web framework
- ✅ **Fiber** - Express-inspired, high performance
- ✅ **Echo** - Minimalist, extensible framework
- ✅ **Chi** - Lightweight, composable router

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
    router.GET("/hello", adapters.ToGinHandler(HelloHandler))
    router.Run(":3000")
}
```

### Fiber

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/leandroluk/gonest/adapters"
)

func main() {
    app := fiber.New()
    app.Get("/hello", adapters.ToFiberHandler(HelloHandler))
    app.Listen(":3000")
}
```

### Echo

```go
import (
    "github.com/labstack/echo/v4"
    "github.com/leandroluk/gonest/adapters"
)

func main() {
    e := echo.New()
    e.GET("/hello", adapters.ToEchoHandler(HelloHandler))
    e.Start(":3000")
}
```

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