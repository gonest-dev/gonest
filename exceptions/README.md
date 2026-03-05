# GoNest Exception Handling

Structured exception handling system with built-in HTTP exceptions and custom filters, inspired by NestJS.

## Features

- ✅ **HTTP Exceptions** - 8 built-in exception types
- ✅ **ValidationException** - Special handling for validation errors
- ✅ **Exception Filters** - Custom error handling
- ✅ **Global Exception Filter** - Catch-all error handler
- ✅ **Exception Chaining** - Multiple filters
- ✅ **Structured Responses** - Consistent JSON format
- ✅ **Type-Safe** - Full compile-time type checking

## Built-in HTTP Exceptions

### BadRequestException (400)
```go
return exceptions.BadRequestException("Invalid parameters").
    WithDetail("param", "id").
    WithDetail("reason", "must be positive")
```

### UnauthorizedException (401)
```go
return exceptions.UnauthorizedException("Authentication required")
```

### ForbiddenException (403)
```go
return exceptions.ForbiddenException("Insufficient permissions").
    WithDetail("required", "admin")
```

### NotFoundException (404)
```go
return exceptions.NotFoundException("User not found").
    WithDetail("userId", 123)
```

### ConflictException (409)
```go
return exceptions.ConflictException("Email already exists").
    WithDetail("email", "user@example.com")
```

### UnprocessableEntityException (422)
```go
return exceptions.UnprocessableEntityException("Invalid data format")
```

### InternalServerErrorException (500)
```go
return exceptions.InternalServerErrorException("Database error")
```

### ServiceUnavailableException (503)
```go
return exceptions.ServiceUnavailableException("Under maintenance")
```

### Custom Exception
```go
return exceptions.NewHttpException(418, "I'm a teapot").
    WithDetail("info", "Custom status code")
```

## ValidationException

Special exception for validation errors:

```go
result := dto.Validate()
if result.Invalid() {
    return exceptions.NewValidationException(result)
}

// Returns:
// {
//   "statusCode": 400,
//   "message": "Validation failed",
//   "errors": [...]
// }
```

## Exception Filters

### Global Exception Filter

Catches all unhandled exceptions:

```go
globalFilter := exceptions.NewGlobalExceptionFilter(&exceptions.GlobalExceptionFilterOptions{
    Logger:       log.Default(),
    IncludeStack: false,
    ShowDetails:  true,
})

// Apply to controller
ctrl.WithMiddleware(
    exceptions.UseExceptionFilter(globalFilter),
)
```

### Custom Exception Filters

```go
type CustomFilter struct{}

func (f *CustomFilter) Catch(err error, ctx *core.Context) error {
    // Handle specific errors
    if httpErr, ok := err.(*exceptions.HttpException); ok {
        if httpErr.StatusCode == 404 {
            return ctx.JSON(404, map[string]any{
                "error": "Not found",
                "path":  ctx.Get("path"),
            })
        }
    }
    
    // Pass through other errors
    return err
}
```

### Chain Multiple Filters

```go
chain := exceptions.ChainExceptionFilters(
    exceptions.NewNotFoundExceptionFilter(),
    exceptions.NewValidationExceptionFilter(),
    exceptions.NewGlobalExceptionFilter(),
)

ctrl.WithMiddleware(
    exceptions.UseExceptionFilter(chain),
)
```

## Usage in Controllers

```go
func NewUserController() controller.Controller {
    // Apply global filter
    globalFilter := exceptions.NewGlobalExceptionFilter()
    
    ctrl := controller.NewController(
        controller.WithPrefix("/users").
        WithMiddleware(
            exceptions.UseExceptionFilter(globalFilter),
        ),
    )
    
    // Route that throws exception
    ctrl.Get("/:id", func(ctx *core.Context) error {
        id := ctx.Param("id")
        
        user := findUser(id)
        if user == nil {
            return exceptions.NotFoundException("User not found").
                WithDetail("id", id)
        }
        
        return ctx.JSON(200, user)
    })
    
    // Route with validation
    ctrl.Post("", func(ctx *core.Context) error {
        var dto CreateUserDto
        if err := ctx.BindJSON(&dto); err != nil {
            return exceptions.BadRequestException("Invalid JSON")
        }
        
        result := dto.Validate()
        if result.Invalid() {
            return exceptions.NewValidationException(result)
        }
        
        return ctx.JSON(201, map[string]any{"data": dto})
    })
    
    return ctrl
}
```

## Exception Response Format

All exceptions return structured JSON:

```json
{
  "statusCode": 404,
  "message": "User not found",
  "details": {
    "userId": 123
  }
}
```

### With Cause

```go
err := fmt.Errorf("database error")
return exceptions.InternalServerErrorException("Failed to fetch user").
    WithDetail("cause", err)

// Returns:
// {
//   "statusCode": 500,
//   "message": "Failed to fetch user",
//   "cause": "database error",
//   "details": {...}
// }
```

## Best Practices

1. **Use Specific Exceptions** - Choose the right HTTP status
2. **Add Details** - Use `WithDetail()` for debugging
3. **Global Filter** - Always have a catch-all filter
4. **Log Errors** - Enable logging in production
5. **Hide Details** - Don't expose sensitive info in production

## Exception Filter Chain

Filters execute in order:

```go
chain := ChainExceptionFilters(
    NotFoundFilter,      // 1. Check for 404
    ValidationFilter,    // 2. Check for validation errors
    GlobalFilter,        // 3. Catch everything else
)

// If NotFoundFilter handles it, chain stops
// Otherwise, passes to ValidationFilter, etc.
```

## Production Configuration

```go
globalFilter := exceptions.NewGlobalExceptionFilter(&exceptions.GlobalExceptionFilterOptions{
    Logger:       productionLogger,
    IncludeStack: false,           // Don't expose stack traces
    ShowDetails:  false,           // Hide sensitive details
})
```

## Complete Example

```go
func main() {
    // Setup filters
    notFoundFilter := exceptions.NewNotFoundExceptionFilter()
    validationFilter := exceptions.NewValidationExceptionFilter()
    globalFilter := exceptions.NewGlobalExceptionFilter()
    
    chain := exceptions.ChainExceptionFilters(
        notFoundFilter,
        validationFilter,
        globalFilter,
    )
    
    // Create controller
    ctrl := controller.NewController(
        controller.WithPrefix("/api").
        WithMiddleware(
            exceptions.UseExceptionFilter(chain),
        ),
    )
    
    // Routes automatically handle exceptions
    ctrl.Get("/users/:id", func(ctx *core.Context) error {
        // Throws NotFoundException
        return exceptions.NotFoundException("User not found")
    })
    
    ctrl.Post("/users", func(ctx *core.Context) error {
        // Throws ValidationException
        result := dto.Validate()
        if result.Invalid() {
            return exceptions.NewValidationException(result)
        }
        
        return ctx.JSON(201, dto)
    })
}
```

## License

MIT