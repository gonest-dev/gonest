# GoNest Interceptors System

Intercept and transform requests/responses before and after handler execution, inspired by NestJS interceptors.

## Features

- ✅ **LoggingInterceptor** - Request/response logging
- ✅ **TimeoutInterceptor** - Request timeout handling
- ✅ **CacheInterceptor** - Response caching
- ✅ **TransformInterceptor** - Response transformation
- ✅ **ErrorInterceptor** - Error handling and logging
- ✅ **Composable** - Chain multiple interceptors
- ✅ **Type-Safe** - Full compile-time type checking

## What are Interceptors?

Interceptors:
1. Execute before and after the handler
2. Can transform the request
3. Can transform the response
4. Can handle errors
5. Can add cross-cutting concerns (logging, caching, etc.)

## Interceptor Interface

```go
type Interceptor interface {
    Intercept(ctx *ExecutionContext, next func() error) error
}
```

## Built-in Interceptors

### LoggingInterceptor

Logs requests and responses with duration.

```go
loggingInterceptor := interceptors.SimpleLoggingInterceptor()

ctrl := controller.NewController(
    controller.WithPrefix("/api").
    WithMiddleware(
        interceptors.UseInterceptors(loggingInterceptor),
    ),
)

// Output:
// [REQUEST] GET /api/users
// [RESPONSE] GET /api/users - OK [145ms]
```

### TimeoutInterceptor

Sets maximum request duration.

```go
timeoutInterceptor := interceptors.NewTimeoutInterceptor(5 * time.Second)

ctrl.Get("/data", handler).
    Use(interceptors.UseInterceptors(timeoutInterceptor))

// Returns error if handler takes > 5 seconds
```

### CacheInterceptor

Caches responses to reduce expensive operations.

```go
// Cache for 5 minutes
cacheInterceptor := interceptors.SimpleCacheInterceptor(5 * time.Minute)

ctrl.Get("/expensive", func(ctx *common.Context) error {
    // Check if cached
    if hit, _ := ctx.Get("cache:hit").(bool); hit {
        return ctx.JSON(200, map[string]any{"cached": true})
    }
    
    // Expensive operation
    result := expensiveOperation()
    return ctx.JSON(200, result)
}).Use(interceptors.UseInterceptors(cacheInterceptor))
```

### TransformInterceptor

Transforms responses into standard formats.

```go
// Wrap all responses
wrapInterceptor := interceptors.WrapResponse()
// Transforms: {"users": [...]}
// Into: {"success": true, "data": {"users": [...]}}

// Add metadata
metadataInterceptor := interceptors.AddMetadata(map[string]any{
    "version": "1.0.0",
    "timestamp": time.Now().Unix(),
})
```

### ErrorInterceptor

Handles and transforms errors.

```go
errorInterceptor := interceptors.SimpleErrorInterceptor()

// Custom error transformation
customInterceptor := interceptors.NewErrorInterceptor(&interceptors.ErrorInterceptorOptions{
    LogErrors: true,
    TransformFunc: func(err error) error {
        // Transform error to JSON format
        return fmt.Errorf(`{"error": "%s"}`, err.Error())
    },
})
```

## Controller-Level vs Route-Level

### Controller-Level (All Routes)

Apply interceptors to **all routes** in a controller:

```go
ctrl := controller.NewController(
    controller.WithPrefix("/api").
    WithMiddleware(
        interceptors.UseInterceptors(loggingInterceptor),
    ),
)

ctrl.Get("/users", handler)    // ✓ Has logging
ctrl.Get("/products", handler) // ✓ Has logging
ctrl.Post("/orders", handler)  // ✓ Has logging
```

### Route-Level (Specific Routes)

Apply interceptors to **specific routes only**:

```go
ctrl := controller.NewController(
    controller.WithPrefix("/api"),
)

// No interceptors
ctrl.Get("/public", handler)

// Only this route is cached
ctrl.Get("/expensive", handler).
    Use(interceptors.UseInterceptors(cacheInterceptor))

// Only this route has timeout
ctrl.Get("/slow", handler).
    Use(interceptors.UseInterceptors(timeoutInterceptor))

// Multiple interceptors on this route
ctrl.Get("/special", handler).
    Use(interceptors.UseInterceptors(
        loggingInterceptor,
        cacheInterceptor,
        timeoutInterceptor,
    ))
```

### Combined (Controller + Route)

Routes inherit controller interceptors and can add their own:

```go
// All routes get logging
ctrl := controller.NewController(
    controller.WithPrefix("/api").
    WithMiddleware(
        interceptors.UseInterceptors(loggingInterceptor),
    ),
)

// Has: logging (from controller)
ctrl.Get("/basic", handler)

// Has: logging (from controller) + cache (from route)
ctrl.Get("/cached", handler).
    Use(interceptors.UseInterceptors(cacheInterceptor))

// Has: logging (from controller) + timeout + transform (from route)
ctrl.Get("/special", handler).
    Use(interceptors.UseInterceptors(
        timeoutInterceptor,
        transformInterceptor,
    ))
```

## Usage Examples

### Single Interceptor

```go
func NewAPIController() controller.Controller {
    loggingInterceptor := interceptors.SimpleLoggingInterceptor()
    
    ctrl := controller.NewController(
        controller.WithPrefix("/api").
        WithMiddleware(
            interceptors.UseInterceptors(loggingInterceptor),
        ),
    )
    
    ctrl.Get("/users", usersHandler)
    
    return ctrl
}
```

### Multiple Interceptors

```go
func NewAPIController() controller.Controller {
    logging := interceptors.SimpleLoggingInterceptor()
    timeout := interceptors.NewTimeoutInterceptor(5 * time.Second)
    cache := interceptors.SimpleCacheInterceptor(1 * time.Minute)
    
    ctrl := controller.NewController(
        controller.WithPrefix("/api").
        WithMiddleware(
            interceptors.UseInterceptors(
                logging,   // First: Log
                timeout,   // Second: Timeout
                cache,     // Third: Cache
            ),
        ),
    )
    
    return ctrl
}
```

### Route-Specific Interceptor

```go
ctrl.Get("/cached", handler).
    Use(interceptors.UseInterceptors(
        interceptors.SimpleCacheInterceptor(5 * time.Minute),
    ))

ctrl.Get("/slow", handler).
    Use(interceptors.UseInterceptors(
        interceptors.NewTimeoutInterceptor(10 * time.Second),
    ))
```

## Custom Interceptors

```go
type CustomInterceptor struct {
    // Your fields
}

func (i *CustomInterceptor) Intercept(ctx *interceptors.ExecutionContext, next func() error) error {
    // Before handler
    fmt.Println("Before")
    
    // Execute handler
    err := next()
    
    // After handler
    fmt.Println("After")
    
    return err
}

// Use it
customInterceptor := &CustomInterceptor{}
ctrl.WithMiddleware(interceptors.UseInterceptors(customInterceptor))
```

## Execution Order

Interceptors execute in order:

```go
interceptors.UseInterceptors(
    logging,    // 1. Logs "Request"
    timeout,    // 2. Starts timeout
    cache,      // 3. Checks cache
    // → Handler executes here
    // ← Handler returns
    cache,      // 4. Stores in cache
    timeout,    // 5. Cancels timeout
    logging,    // 6. Logs "Response"
)
```

## Execution Context

Interceptors receive an `ExecutionContext`:

```go
type ExecutionContext struct {
    Context   *common.Context      // HTTP context
    Handler   common.HandlerFunc   // Route handler
    Metadata  map[string]any     // Custom metadata
    StartTime time.Time          // Request start time
}
```

## Best Practices

1. **Order Matters** - Place logging first, cache last
2. **Keep Lightweight** - Interceptors run on every request
3. **Handle Errors** - Always handle errors properly
4. **Use Caching Wisely** - Don't cache everything
5. **Log Appropriately** - Don't over-log

## Common Use Cases

### API Response Wrapper

```go
wrapInterceptor := interceptors.WrapResponse()
// All responses: {"success": true, "data": {...}}
```

### Request Logging

```go
loggingInterceptor := interceptors.SimpleLoggingInterceptor()
// Logs: [REQUEST] GET /api/users
//       [RESPONSE] GET /api/users - OK [145ms]
```

### Timeout Protection

```go
timeoutInterceptor := interceptors.NewTimeoutInterceptor(5 * time.Second)
// Prevents long-running requests
```

### Response Caching

```go
cacheInterceptor := interceptors.SimpleCacheInterceptor(5 * time.Minute)
// Caches expensive operations
```

## Integration with Controllers

### Controller-Level

```go
ctrl := controller.NewController(
    controller.WithPrefix("/api").
    WithMiddleware(
        interceptors.UseInterceptors(loggingInterceptor),
    ),
)
// All routes use logging interceptor
```

### Route-Level

```go
ctrl.Get("/cached", handler).
    Use(interceptors.UseInterceptors(cacheInterceptor))
// Only this route uses cache
```

## Complete Example

```go
func NewCompleteAPI() controller.Controller {
    // Setup interceptors
    logging := interceptors.SimpleLoggingInterceptor()
    timeout := interceptors.NewTimeoutInterceptor(10 * time.Second)
    cache := interceptors.SimpleCacheInterceptor(5 * time.Minute)
    wrap := interceptors.WrapResponse()
    errorHandler := interceptors.SimpleErrorInterceptor()
    
    // Create controller
    ctrl := controller.NewController(
        controller.WithPrefix("/api").
        WithMiddleware(
            interceptors.UseInterceptors(
                logging,
                errorHandler,
                timeout,
                wrap,
            ),
        ),
    )
    
    // Cached endpoint
    ctrl.Get("/expensive", expensiveHandler).
        Use(interceptors.UseInterceptors(cache))
    
    // Fast endpoint (no cache)
    ctrl.Get("/fast", fastHandler)
    
    return ctrl
}
```

## License

MIT