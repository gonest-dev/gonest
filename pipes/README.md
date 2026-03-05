# GoNest Pipes System

Transformation and validation pipes for request data, inspired by NestJS pipes.

## Features

- ✅ **ValidationPipe** - Automatic DTO validation
- ✅ **ParseIntPipe** - String to int conversion
- ✅ **ParseFloatPipe** - String to float conversion
- ✅ **ParseBoolPipe** - String to bool conversion
- ✅ **ParseUUIDPipe** - UUID validation
- ✅ **ParseEnumPipe** - Enum validation
- ✅ **ParseArrayPipe** - Comma-separated string to array
- ✅ **DefaultValuePipe** - Provide default values
- ✅ **Type-Safe** - Full compile-time type checking

## What are Pipes?

Pipes are functions that:
1. Transform input data (e.g., string to int)
2. Validate input data (e.g., check if UUID is valid)
3. Can throw errors if validation fails

## ValidationPipe

Automatically validates DTOs that implement the `Validatable` interface.

```go
type CreateUserDto struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func (dto *CreateUserDto) Validate() *validator.ValidationResult {
    // ... validation logic
}

// In handler
func createUser(ctx *core.Context) error {
    dto, err := pipes.ValidateBody[CreateUserDto](ctx)
    if err != nil {
        if validationErr, ok := err.(*pipes.ValidationError); ok {
            return ctx.JSON(400, validationErr.ToJSON())
        }
        return ctx.JSON(400, map[string]any{"error": err.Error()})
    }
    
    // dto is validated and ready to use
    return ctx.JSON(201, map[string]any{"data": dto})
}
```

## Parse Pipes

### ParseIntPipe

```go
intPipe := pipes.NewParseIntPipe()
id, err := intPipe.Transform(ctx.Param("id"), ctx)
// "123" -> 123
```

### ParseFloatPipe

```go
floatPipe := pipes.NewParseFloatPipe()
price, err := floatPipe.Transform(ctx.Query("price"), ctx)
// "19.99" -> 19.99
```

### ParseBoolPipe

```go
boolPipe := pipes.NewParseBoolPipe()
active, err := boolPipe.Transform(ctx.Query("active"), ctx)
// "true" -> true
```

### ParseUUIDPipe

```go
uuidPipe := pipes.NewParseUUIDPipe()
uuid, err := uuidPipe.Transform(ctx.Param("uuid"), ctx)
// Validates UUID format
```

### ParseEnumPipe

```go
enumPipe := pipes.NewParseEnumPipe("active", "inactive", "draft")
status, err := enumPipe.Transform(ctx.Query("status"), ctx)
// Only allows: "active", "inactive", or "draft"
```

### ParseArrayPipe

```go
arrayPipe := pipes.NewParseArrayPipe(",")
tags, err := arrayPipe.Transform(ctx.Query("tags"), ctx)
// "tag1,tag2,tag3" -> []string{"tag1", "tag2", "tag3"}
```

### DefaultValuePipe

```go
defaultPipe := pipes.NewDefaultValuePipe("10")
limit, err := defaultPipe.Transform(ctx.Query("limit"), ctx)
// Empty string -> "10"
// "20" -> "20"
```

## Complete Example

```go
func NewUserController() controller.Controller {
    ctrl := controller.NewController(
        controller.WithPrefix("/users"),
    )
    
    // POST /users - with automatic validation
    ctrl.Post("", func(ctx *core.Context) error {
        dto, err := pipes.ValidateBody[CreateUserDto](ctx)
        if err != nil {
            if validationErr, ok := err.(*pipes.ValidationError); ok {
                return ctx.JSON(400, validationErr.ToJSON())
            }
            return ctx.JSON(400, map[string]any{"error": err.Error()})
        }
        
        return ctx.JSON(201, map[string]any{"data": dto})
    })
    
    // GET /users/:id - with ParseIntPipe
    ctrl.Get("/:id", func(ctx *core.Context) error {
        intPipe := pipes.NewParseIntPipe()
        id, err := intPipe.Transform(ctx.Param("id"), ctx)
        if err != nil {
            return ctx.JSON(400, map[string]any{
                "error": "Invalid ID format",
            })
        }
        
        return ctx.JSON(200, map[string]any{"id": id})
    })
    
    // GET /users?page=1&limit=10
    ctrl.Get("", func(ctx *core.Context) error {
        intPipe := pipes.NewParseIntPipe()
        defaultPipe := pipes.NewDefaultValuePipe("10")
        
        pageStr := ctx.Query("page")
        if pageStr == "" {
            pageStr = "1"
        }
        page, _ := intPipe.Transform(pageStr, ctx)
        
        limitStr, _ := defaultPipe.Transform(ctx.Query("limit"), ctx)
        limit, _ := intPipe.Transform(limitStr, ctx)
        
        return ctx.JSON(200, map[string]any{
            "page":  page,
            "limit": limit,
        })
    })
    
    return ctrl
}
```

## Custom Pipes

Create your own pipes by implementing the `PipeTransform` interface:

```go
type CustomPipe struct{}

func (p *CustomPipe) Transform(value any, ctx *core.Context) (any, error) {
    // Your transformation logic
    return transformedValue, nil
}
```

## Validation Options

```go
opts := &pipes.ValidationPipeOptions{
    Transform:             true,   // Enable transformation
    Whitelist:             false,  // Strip non-decorated properties
    ForbidNonWhitelisted:  false,  // Throw if extra properties
    SkipMissingProperties: false,  // Skip validation of missing props
    DisableErrorMessages:  false,  // Disable detailed errors
}

validationPipe := pipes.NewValidationPipe(opts)
```

## Error Handling

### Validation Errors

```go
dto, err := pipes.ValidateBody[CreateUserDto](ctx)
if err != nil {
    if validationErr, ok := err.(*pipes.ValidationError); ok {
        // Return structured validation errors
        return ctx.JSON(400, validationErr.ToJSON())
    }
    // Other errors
    return ctx.JSON(400, map[string]any{"error": err.Error()})
}
```

### Parse Errors

```go
id, err := intPipe.Transform(value, ctx)
if err != nil {
    return ctx.JSON(400, map[string]any{
        "error": fmt.Sprintf("Invalid ID: %s", err.Error()),
    })
}
```

## Best Practices

1. **Always Validate User Input** - Use ValidationPipe for DTOs
2. **Transform Early** - Convert types as soon as possible
3. **Provide Clear Errors** - Return detailed validation errors
4. **Use Type-Safe Pipes** - Leverage generics for type safety
5. **Chain Pipes** - Combine multiple pipes when needed

## Integration with Controllers

```go
ctrl.Get("/:id", func(ctx *core.Context) error {
    // 1. Parse parameter
    intPipe := pipes.NewParseIntPipe()
    id, err := intPipe.Transform(ctx.Param("id"), ctx)
    if err != nil {
        return ctx.JSON(400, map[string]any{"error": "Invalid ID"})
    }
    
    // 2. Use parsed value
    user := userService.FindByID(id)
    
    return ctx.JSON(200, map[string]any{"data": user})
})
```

## License

MIT