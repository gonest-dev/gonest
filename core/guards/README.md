# GoNest Guards System

Route guards for authentication, authorization, and request validation, inspired by NestJS guards.

## Features

- ✅ **AuthGuard** - JWT/Bearer token authentication
- ✅ **RolesGuard** - Role-based access control (RBAC)
- ✅ **ThrottlerGuard** - Rate limiting / throttling
- ✅ **Custom Guards** - Easy to create custom guards
- ✅ **Composable** - Chain multiple guards
- ✅ **Type-Safe** - Full compile-time type checking

## What are Guards?

Guards are functions that:
1. Execute before the route handler
2. Determine if a request can proceed
3. Return `true` to allow or `false` to deny
4. Can throw errors with custom status codes

## Guard Interface

```go
type Guard interface {
    CanActivate(ctx *ExecutionContext) (bool, error)
}
```

## Built-in Guards

### AuthGuard

Validates authentication tokens (JWT, Bearer, API Keys).

```go
// Simple token validation
authGuard := guards.SimpleAuthGuard("valid-token-1", "valid-token-2")

// Custom validation
authGuard := guards.NewAuthGuard(&guards.AuthGuardOptions{
    TokenValidator: func(token string) (bool, error) {
        // Your validation logic
        return validateJWT(token)
    },
    HeaderName: "Authorization",  // Default
    Prefix:     "Bearer",          // Default
})

// Apply to controller
ctrl := controller.NewController(
    controller.WithPrefix("/protected").
    WithMiddleware(guards.UseGuards(authGuard)),
)
```

### RolesGuard

Validates user roles for authorization.

```go
// Require at least one role
rolesGuard := guards.RequireRoles("admin", "moderator")

// Require ALL roles
rolesGuard := guards.RequireAllRoles("admin", "superuser")

// Custom role extraction
rolesGuard := guards.NewRolesGuard(&guards.RolesGuardOptions{
    RequiredRoles: []string{"admin"},
    UserExtractor: func(ctx *ExecutionContext) ([]string, error) {
        // Extract roles from JWT, database, etc.
        return []string{"user", "admin"}, nil
    },
})
```

### ThrottlerGuard

Rate limiting to prevent abuse.

```go
// Simple: 10 requests per minute
throttler := guards.SimpleThrottler(10, time.Minute)

// IP-based throttling
throttler := guards.IPThrottler(100, time.Hour)

// User-based throttling
throttler := guards.UserThrottler(50, time.Minute)

// Custom key generation
throttler := guards.NewThrottlerGuard(&guards.ThrottlerGuardOptions{
    Limit: 10,
    TTL:   time.Minute,
    KeyGen: func(ctx *ExecutionContext) string {
        // Custom key (e.g., API key, user ID, IP)
        return ctx.Context.Get("api-key")
    },
})
```

## Usage Examples

### Protected Routes

```go
func NewProtectedController() controller.Controller {
    authGuard := guards.SimpleAuthGuard("secret-token")
    
    ctrl := controller.NewController(
        controller.WithPrefix("/protected").
        WithMiddleware(guards.UseGuards(authGuard)),
    )
    
    ctrl.Get("/data", func(ctx *common.Context) error {
        // Only accessible with valid token
        return ctx.JSON(200, map[string]any{"data": "secret"})
    })
    
    return ctrl
}
```

### Admin-Only Routes

```go
func NewAdminController() controller.Controller {
    authGuard := guards.SimpleAuthGuard("admin-token")
    rolesGuard := guards.RequireRoles("admin")
    
    ctrl := controller.NewController(
        controller.WithPrefix("/admin").
        WithMiddleware(
            guards.UseGuards(authGuard, rolesGuard),
        ),
    )
    
    ctrl.Delete("/users/:id", func(ctx *common.Context) error {
        // Only admins can delete users
        id := ctx.Param("id")
        return ctx.JSON(200, map[string]any{"deleted": id})
    })
    
    return ctrl
}
```

### Rate Limited API

```go
func NewAPIController() controller.Controller {
    throttler := guards.IPThrottler(100, time.Hour)
    
    ctrl := controller.NewController(
        controller.WithPrefix("/api").
        WithMiddleware(guards.UseGuards(throttler)),
    )
    
    ctrl.Get("/data", func(ctx *common.Context) error {
        // Limited to 100 requests per hour per IP
        return ctx.JSON(200, map[string]any{"data": "..."})
    })
    
    return ctrl
}
```

## Chaining Multiple Guards

```go
ctrl := controller.NewController(
    controller.WithPrefix("/secure").
    WithMiddleware(
        guards.UseGuards(
            authGuard,      // First: Check authentication
            rolesGuard,     // Second: Check authorization
            throttler,      // Third: Check rate limit
        ),
    ),
)
```

## Custom Guards

```go
type CustomGuard struct{}

func (g *CustomGuard) CanActivate(ctx *guards.ExecutionContext) (bool, error) {
    // Your custom logic
    apiKey := ctx.Context.Get("X-API-Key")
    
    if apiKey != "valid-key" {
        return false, guards.NewGuardError("Invalid API key", 401)
    }
    
    return true, nil
}

// Use it
customGuard := &CustomGuard{}
ctrl.WithMiddleware(guards.UseGuards(customGuard))
```

## Guard Errors

```go
// Create custom error with status code
return false, guards.NewGuardError("Unauthorized", 401)

// Add details
return false, guards.NewGuardError("Forbidden", 403).
    WithDetail("required", "admin role").
    WithDetail("user", "guest role")
```

## Error Responses

Guards return structured JSON errors:

```json
{
  "statusCode": 403,
  "message": "Insufficient permissions",
  "details": {
    "required": ["admin"],
    "user": ["guest"]
  }
}
```

## Execution Context

Guards receive an `ExecutionContext` with:

```go
type ExecutionContext struct {
    Context  *common.Context          // HTTP context
    Handler  common.HandlerFunc        // Route handler
    Metadata map[string]any          // Custom metadata
}
```

## Best Practices

1. **Order Matters** - Guards execute in order (auth → roles → throttle)
2. **Fail Fast** - Put authentication guards first
3. **Store User Data** - Extract and store user info in context
4. **Custom Errors** - Provide clear error messages
5. **Rate Limit Smartly** - Use appropriate limits per endpoint

## Integration with Controllers

### Controller-Level Guards

```go
ctrl := controller.NewController(
    controller.WithPrefix("/api").
    WithMiddleware(guards.UseGuards(authGuard)),
)
// All routes in this controller are protected
```

### Route-Level Guards

```go
ctrl.Get("/admin", adminHandler).
    Use(guards.UseGuards(adminGuard))
// Only this specific route requires admin
```

## Complete Example

```go
func NewSecureAPI() controller.Controller {
    // Setup guards
    authGuard := guards.NewAuthGuard(&guards.AuthGuardOptions{
        TokenValidator: validateJWT,
    })
    
    adminGuard := guards.RequireRoles("admin")
    throttler := guards.IPThrottler(100, time.Hour)
    
    // Create controller with guards
    ctrl := controller.NewController(
        controller.WithPrefix("/api").
        WithMiddleware(
            guards.UseGuards(authGuard, throttler),
        ),
    )
    
    // Public endpoint (bypasses controller guards)
    ctrl.Get("/public", publicHandler)
    
    // Protected endpoint
    ctrl.Get("/data", dataHandler)
    
    // Admin-only endpoint
    ctrl.Delete("/users/:id", deleteHandler).
        Use(guards.UseGuards(adminGuard))
    
    return ctrl
}
```

## License

MIT