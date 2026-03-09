# GoNest Controller System

NestJS-inspired controller system for Go with type-safe routing and parameter handling.

## Features

- ✅ **Fluent Builder API** - Easy-to-use controller definition
- ✅ **HTTP Method Decorators** - Get, Post, Put, Patch, Delete, Options, Head
- ✅ **Route Parameters** - Path params, query params, headers, body
- ✅ **Automatic Validation** - Integrate with validator module
- ✅ **Middleware Support** - Controller and route-level middleware
- ✅ **Type-Safe** - Full type safety with generics

## Quick Start

### Basic Controller

```go
import (
    "github.com/gonest-dev/gonest/core/controller"
    "github.com/gonest-dev/gonest/core/common"
)

func NewUserController() controller.Controller {
    ctrl := controller.NewController(
        controller.WithPrefix("/users"),
    )
    
    // GET /users
    ctrl.Get("", func(ctx *common.Context) error {
        return ctx.JSON(200, map[string]any{
            "data": []string{"user1", "user2"},
        })
    })
    
    // GET /users/:id
    ctrl.Get("/:id", func(ctx *common.Context) error {
        id := ctx.Param("id")
        return ctx.JSON(200, map[string]any{
            "id": id,
        })
    }).Param("id")
    
    // POST /users
    ctrl.Post("", func(ctx *common.Context) error {
        var dto CreateUserDto
        if err := ctx.BindJSON(&dto); err != nil {
            return ctx.JSON(400, map[string]any{"error": err.Error()})
        }
        
        return ctx.JSON(201, map[string]any{"data": dto})
    }).Body("user")
    
    return ctrl
}
```

## HTTP Methods

### GET
```go
ctrl.Get("/path", handler)
```

### POST
```go
ctrl.Post("/path", handler)
```

### PUT
```go
ctrl.Put("/path", handler)
```

### PATCH
```go
ctrl.Patch("/path", handler)
```

### DELETE
```go
ctrl.Delete("/path", handler)
```

### OPTIONS
```go
ctrl.Options("/path", handler)
```

### HEAD
```go
ctrl.Head("/path", handler)
```

## Parameter Types

### Path Parameters
```go
ctrl.Get("/:id", handler).Param("id")
ctrl.Get("/:category/:id", handler).Param("category").Param("id")
```

### Query Parameters
```go
ctrl.Get("/search", handler).Query("q", true)  // required
ctrl.Get("/filter", handler).Query("category", false)  // optional
```

### Body Parameters
```go
ctrl.Post("/users", handler).Body("user")
```

### Headers
```go
ctrl.Get("/protected", handler).Header("Authorization", true)
```

## With Validation

```go
type CreateUserDto struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func (dto *CreateUserDto) Validate() *validator.ValidationResult {
    // ... validation logic
}

ctrl.Post("/users", func(ctx *common.Context) error {
    var dto CreateUserDto
    
    if err := ctx.BindJSON(&dto); err != nil {
        return ctx.JSON(400, map[string]any{"error": "Invalid JSON"})
    }
    
    // Validate
    result := dto.Validate()
    if result.Invalid() {
        return ctx.JSON(400, result.ToJSON())
    }
    
    // Process...
    return ctx.JSON(201, map[string]any{"data": dto})
})
```

## Middleware

### Controller-Level Middleware
```go
func AuthMiddleware(ctx *common.Context, next common.NextFunc) error {
    // Auth logic
    return next(ctx)
}

ctrl := controller.NewController(
    controller.WithPrefix("/api").
        WithMiddleware(AuthMiddleware),
)
```

### Route-Level Middleware
```go
ctrl.Get("/protected", handler).
    Use(AuthMiddleware).
    Use(LoggingMiddleware)
```

## Controller Options

```go
opts := controller.WithPrefix("/api/v1").
    WithMiddleware(LoggerMiddleware).
    WithMiddleware(AuthMiddleware)

ctrl := controller.NewController(opts)
```

## Route Configuration

### Chaining Configuration
```go
ctrl.Get("/:id", handler).
    Param("id").
    Query("expand", false).
    Header("Authorization", true).
    Use(middleware).
    Meta("roles", []string{"admin", "user"})
```

## Integration with Module System

```go
// In your module
func NewUserModule() *common.Module {
    return common.NewModule().
        Controllers(NewUserController()).
        Build()
}
```

## Advanced Examples

### CRUD Controller
```go
func NewCrudController() controller.Controller {
    ctrl := controller.NewController(
        controller.WithPrefix("/items"),
    )
    
    // List
    ctrl.Get("", listHandler)
    
    // Get by ID
    ctrl.Get("/:id", getHandler).Param("id")
    
    // Create
    ctrl.Post("", createHandler).Body("item")
    
    // Update
    ctrl.Put("/:id", updateHandler).Param("id").Body("item")
    
    // Delete
    ctrl.Delete("/:id", deleteHandler).Param("id")
    
    return ctrl
}
```

### Query Parameters
```go
ctrl.Get("/search", func(ctx *common.Context) error {
    query := ctx.Query("q")
    page := ctx.Query("page")
    limit := ctx.Query("limit")
    
    return ctx.JSON(200, map[string]any{
        "query": query,
        "page":  page,
        "limit": limit,
    })
}).
    Query("q", true).       // required
    Query("page", false).   // optional
    Query("limit", false)   // optional
```

## Best Practices

1. **Group Related Routes** - Use controllers to group related endpoints
2. **Use DTOs** - Define clear data transfer objects
3. **Validate Input** - Always validate incoming data
4. **Handle Errors** - Return appropriate error responses
5. **Use Middleware** - Share common logic across routes

## Error Handling

```go
ctrl.Post("/users", func(ctx *common.Context) error {
    var dto CreateUserDto
    
    // Binding error
    if err := ctx.BindJSON(&dto); err != nil {
        return ctx.JSON(400, map[string]any{
            "error": "Invalid request body",
        })
    }
    
    // Validation error
    if result := dto.Validate(); result.Invalid() {
        return ctx.JSON(400, result.ToJSON())
    }
    
    // Business logic error
    user, err := userService.Create(&dto)
    if err != nil {
        return ctx.JSON(500, map[string]any{
            "error": err.Error(),
        })
    }
    
    return ctx.JSON(201, map[string]any{"data": user})
})
```

## License

MIT