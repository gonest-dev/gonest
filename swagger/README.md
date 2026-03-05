# GoNest Swagger/OpenAPI Integration

Automatic API documentation generation using OpenAPI 3.0 specification with **type-safe descriptors** (no struct tags!).

## Features

- ✅ **OpenAPI 3.0.3** - Full specification support
- ✅ **Type-Safe Descriptors** - No struct tags, compile-time safety
- ✅ **Pointer-Based API** - Consistent with validator module
- ✅ **Swagger UI** - Interactive documentation
- ✅ **Multiple auth schemes** - Bearer, API Key, OAuth2
- ✅ **IDE Support** - Full autocomplete and refactoring

## Why Descriptors Instead of Tags?

### ❌ The Problem with Tags
```go
type CreateUserDto struct {
    Name  string `json:"name" required:"true" description:"User name" minLength:"2"`
    Email string `json:"email" required:"true" format:"email"`
}
```

**Problems:**
- Not type-safe (strings, not code)
- No compile-time checking
- No IDE autocomplete
- Easy to make typos
- Hard to refactor

### ✅ The Descriptor Solution
```go
type CreateUserDto struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func DescribeCreateUserDto() *swagger.Schema {
    var dto CreateUserDto
    descriptor := swagger.NewDescriptor(&dto)
    
    descriptor.Field(&dto.Name).
        Description("User's full name").
        Required().
        MinLength(2)
    
    descriptor.Field(&dto.Email).
        Description("User's email address").
        Required().
        Format("email")
    
    return descriptor.Build()
}
```

**Benefits:**
- ✅ Type-safe (compile-time errors)
- ✅ IDE autocomplete
- ✅ Refactoring safe
- ✅ No magic strings
- ✅ Same pattern as validator module

## Quick Start

### Define Your DTOs

```go
type CreateUserDto struct {
    Name     string `json:"name"`
    Email    string `json:"email"`
    Age      int    `json:"age"`
    Password string `json:"password"`
}
```

### Describe with Type-Safe API (Recommended)

**Clean Callback Style:**
```go
func DescribeCreateUserDto() *swagger.Schema {
    return swagger.Descriptor(func(dto *CreateUserDto, d *swagger.DescriptorBuilder[CreateUserDto]) {
        d.Field(&dto.Name).
            Description("User's full name").
            Required().
            MinLength(2).
            Example("John Doe")
        
        d.Field(&dto.Email).
            Description("User's email address").
            Required().
            Format("email").
            Example("john@example.com")
        
        d.Field(&dto.Password).
            Description("User's password").
            Required().
            MinLength(8).
            WriteOnly()
    })
}
```

**Alternative Style (also type-safe):**
```go
func DescribeCreateUserDto() *swagger.Schema {
    var dto CreateUserDto
    descriptor := swagger.NewDescriptor(&dto)
    
    descriptor.Field(&dto.Name).
        Description("User's full name").
        Required().
        MinLength(2)
    
    descriptor.Field(&dto.Email).
        Description("User's email address").
        Required().
        Format("email")
    
    return descriptor.Build()
}
```

Both styles are type-safe and provide the same functionality!

### Build OpenAPI Document

```go
builder := swagger.NewDocumentBuilder()

// Set API info
builder.SetInfo("My API", "API description", "1.0.0")

// Add schemas
builder.AddSchema("CreateUserDto", DescribeCreateUserDto())

// Add endpoint
builder.AddPath("/users", "POST", swagger.NewOperation(
    "Create user",
    "Creates a new user",
).
    WithRequestBody("User data", true, &swagger.Schema{
        Ref: "#/components/schemas/CreateUserDto",
    }).
    WithResponse("201", "Created", &swagger.Schema{
        Ref: "#/components/schemas/UserResponseDto",
    }))

doc := builder.Build()
```

## Descriptor API Reference

### Create Descriptor

```go
var dto CreateUserDto
descriptor := swagger.NewDescriptor(&dto)
```

### Field Methods

```go
descriptor.Field(&dto.Name).
    Description("Field description").
    Required().
    Example("example value").
    MinLength(2).
    MaxLength(100).
    Pattern("^[a-zA-Z]+$").
    Format("email").
    Default("default value").
    Enum("value1", "value2").
    WriteOnly().
    ReadOnly().
    Deprecated()
```

### Numeric Fields

```go
descriptor.Field(&dto.Age).
    Description("User age").
    Minimum(0).
    Maximum(120).
    Example(25)

descriptor.Field(&dto.Price).
    Minimum(0.01).
    Maximum(9999.99).
    Example(19.99)
```

### String Fields

```go
descriptor.Field(&dto.Email).
    Description("Email address").
    Format("email").
    MinLength(5).
    MaxLength(255).
    Pattern("^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$").
    Example("user@example.com")
```

### Special Modifiers

```go
// Write-only (passwords, not returned in responses)
descriptor.Field(&dto.Password).
    WriteOnly().
    MinLength(8)

// Read-only (IDs, timestamps, not accepted in requests)
descriptor.Field(&dto.ID).
    ReadOnly().
    Example(123)

// Deprecated (old fields)
descriptor.Field(&dto.OldField).
    Deprecated().
    Description("Use newField instead")
```

### Enums

```go
descriptor.Field(&dto.Status).
    Description("User status").
    Enum("active", "inactive", "pending")
```

### Default Values

```go
descriptor.Field(&dto.Role).
    Description("User role").
    Default("user").
    Enum("admin", "user", "guest")
```

## Complete Example

```go
type CreateUserDto struct {
    Name     string `json:"name"`
    Email    string `json:"email"`
    Age      int    `json:"age"`
    Password string `json:"password"`
    Role     string `json:"role"`
    Bio      string `json:"bio"`
}

func DescribeCreateUserDto() *swagger.Schema {
    var dto CreateUserDto
    descriptor := swagger.NewDescriptor(&dto)
    
    descriptor.Field(&dto.Name).
        Description("User's full name").
        Required().
        MinLength(2).
        MaxLength(100).
        Example("John Doe")
    
    descriptor.Field(&dto.Email).
        Description("User's email address").
        Required().
        Format("email").
        Example("john@example.com")
    
    descriptor.Field(&dto.Age).
        Description("User's age in years").
        Minimum(18).
        Maximum(120).
        Example(25)
    
    descriptor.Field(&dto.Password).
        Description("User's password (min 8 characters)").
        Required().
        MinLength(8).
        Format("password").
        WriteOnly()
    
    descriptor.Field(&dto.Role).
        Description("User role in the system").
        Default("user").
        Enum("admin", "user", "guest")
    
    descriptor.Field(&dto.Bio).
        Description("User biography").
        MaxLength(500).
        Example("I'm a software developer")
    
    return descriptor.Build()
}
```

## Build API Documentation

```go
func BuildAPIDocumentation() *swagger.OpenAPIDocument {
    builder := swagger.NewDocumentBuilder()
    
    // API Info
    builder.SetInfo("My API", "API Documentation", "1.0.0")
    builder.SetContact("Support", "https://api.example.com", "support@example.com")
    builder.SetLicense("MIT", "https://opensource.org/licenses/MIT")
    
    // Servers
    builder.AddServer("http://localhost:3000", "Development")
    builder.AddServer("https://api.example.com", "Production")
    
    // Tags
    builder.AddTag("users", "User management")
    builder.AddTag("products", "Product catalog")
    
    // Security
    builder.AddBearerAuth()
    
    // Schemas (using descriptors!)
    builder.AddSchema("CreateUserDto", DescribeCreateUserDto())
    builder.AddSchema("UserResponseDto", DescribeUserResponseDto())
    
    // Endpoints
    builder.AddPath("/users", "POST", swagger.NewOperation(
        "Create user",
        "Create a new user",
    ).
        WithTag("users").
        WithRequestBody("User data", true, &swagger.Schema{
            Ref: "#/components/schemas/CreateUserDto",
        }).
        WithResponse("201", "Created", &swagger.Schema{
            Ref: "#/components/schemas/UserResponseDto",
        }))
    
    return builder.Build()
}
```

## Operations

### Create Operation

```go
operation := swagger.NewOperation(
    "Summary text",
    "Detailed description",
)
```

### Add Parameters

```go
// Path parameter
operation.WithParameter("id", "path", "User ID", true, &swagger.Schema{
    Type: "integer",
})

// Query parameter
operation.WithParameter("page", "query", "Page number", false, &swagger.Schema{
    Type: "integer",
    Default: 1,
})

// Header parameter
operation.WithParameter("X-API-Key", "header", "API Key", true, &swagger.Schema{
    Type: "string",
})
```

### Add Request Body

```go
operation.WithRequestBody("Request description", true, &swagger.Schema{
    Ref: "#/components/schemas/CreateUserDto",
})
```

### Add Responses

```go
// Success response
operation.WithResponse("200", "Successful operation", &swagger.Schema{
    Ref: "#/components/schemas/UserResponseDto",
})

// Error response
operation.WithResponse("404", "Not found", &swagger.Schema{
    Type: "object",
    Properties: map[string]*swagger.Schema{
        "message": {Type: "string"},
    },
})

// No content
operation.WithResponse("204", "No content", nil)
```

### Add Security

```go
// Require bearer token
operation.WithSecurity("bearer")

// Require API key
operation.WithSecurity("apiKey")

// Require specific OAuth2 scopes
operation.WithSecurity("oauth2", "read:users", "write:users")
```

## Security Schemes

### Bearer Authentication (JWT)

```go
builder.AddBearerAuth()
```

### API Key

```go
builder.AddAPIKeyAuth("X-API-Key", "header")
```

### Basic Auth

```go
builder.doc.Components.SecuritySchemes["basic"] = swagger.SecurityScheme{
    Type:   "http",
    Scheme: "basic",
}
```

## Schemas

### From Struct

```go
type User struct {
    ID    int    `json:"id" description:"User ID"`
    Name  string `json:"name" required:"true" description:"User name"`
    Email string `json:"email" required:"true" description:"Email address"`
}

schema := swagger.SchemaFromStruct(User{})
```

### Manual Schema

```go
schema := &swagger.Schema{
    Type: "object",
    Properties: map[string]*swagger.Schema{
        "name": {
            Type:        "string",
            Description: "User's name",
        },
        "age": {
            Type:    "integer",
            Minimum: 0,
            Maximum: 120,
        },
    },
    Required: []string{"name"},
}
```

### Array Schema

```go
schema := &swagger.Schema{
    Type: "array",
    Items: &swagger.Schema{
        Ref: "#/components/schemas/UserResponseDto",
    },
}
```

## Swagger UI Integration

### Generate UI HTML

```go
html := swagger.GenerateSwaggerUI(&swagger.SwaggerUIConfig{
    Title:   "My API Docs",
    SpecURL: "/api-docs/swagger.json",
})

// Serve HTML
ctx.HTML(200, html)
```

### Serve JSON Spec

```go
doc := BuildAPIDocumentation()
jsonData, _ := swagger.ServeSwaggerJSON(doc)

// Serve JSON
ctx.JSON(200, json.RawMessage(jsonData))
```

## Integration with Controllers

```go
func SetupSwagger(app *core.Application) {
    doc := BuildAPIDocumentation()
    
    // Serve Swagger JSON
    app.Get("/api-docs/swagger.json", func(ctx *core.Context) error {
        jsonData, _ := swagger.ServeSwaggerJSON(doc)
        return ctx.JSON(200, json.RawMessage(jsonData))
    })
    
    // Serve Swagger UI
    app.Get("/api-docs", func(ctx *core.Context) error {
        html := swagger.GenerateSwaggerUI(&swagger.SwaggerUIConfig{
            Title:   "API Documentation",
            SpecURL: "/api-docs/swagger.json",
        })
        return ctx.HTML(200, html)
    })
}
```

## Best Practices

1. **Use Tags** - Organize endpoints by resource
2. **Add Descriptions** - Document everything clearly
3. **Define Schemas** - Reuse schemas in components
4. **Security First** - Document auth requirements
5. **Error Responses** - Document all error cases
6. **Examples** - Add example values to schemas

## Complete CRUD Example

```go
// List
builder.AddPath("/users", "GET", swagger.NewOperation(
    "List users",
    "Get all users with pagination",
).
    WithTag("users").
    WithParameter("page", "query", "Page number", false, intSchema).
    WithParameter("limit", "query", "Page size", false, intSchema).
    WithResponse("200", "Success", arrayOfUsers).
    WithSecurity("bearer"))

// Get
builder.AddPath("/users/{id}", "GET", swagger.NewOperation(
    "Get user",
    "Get user by ID",
).
    WithTag("users").
    WithParameter("id", "path", "User ID", true, intSchema).
    WithResponse("200", "Success", userSchema).
    WithResponse("404", "Not found", errorSchema).
    WithSecurity("bearer"))

// Create
builder.AddPath("/users", "POST", swagger.NewOperation(
    "Create user",
    "Create a new user",
).
    WithTag("users").
    WithRequestBody("User data", true, createUserSchema).
    WithResponse("201", "Created", userSchema).
    WithResponse("400", "Bad request", errorSchema).
    WithSecurity("bearer"))

// Update
builder.AddPath("/users/{id}", "PUT", swagger.NewOperation(
    "Update user",
    "Update an existing user",
).
    WithTag("users").
    WithParameter("id", "path", "User ID", true, intSchema).
    WithRequestBody("User data", true, updateUserSchema).
    WithResponse("200", "Success", userSchema).
    WithResponse("404", "Not found", errorSchema).
    WithSecurity("bearer"))

// Delete
builder.AddPath("/users/{id}", "DELETE", swagger.NewOperation(
    "Delete user",
    "Delete a user",
).
    WithTag("users").
    WithParameter("id", "path", "User ID", true, intSchema).
    WithResponse("204", "No content", nil).
    WithResponse("404", "Not found", errorSchema).
    WithSecurity("bearer"))
```

## License

MIT