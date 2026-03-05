# GoNest Type-Safe Validator

100% type-safe validation system using Go generics. Zero magic strings, zero reflection at validation time.

## Features

- ✅ **Fully Type-Safe**: Compile-time type checking with generics
- ✅ **Zero Tags**: No struct tags, no reflection magic
- ✅ **Composable**: Chain validators easily
- ✅ **Async Support**: Built-in async validation
- ✅ **Custom Validators**: Easy to create custom rules
- ✅ **Cross-Field**: Validate relationships between fields
- ✅ **Detailed Errors**: Structured error messages with codes
- ✅ **Performance**: Compiled validators, minimal overhead

## Quick Start

### Schema Validation (Recommended)

**Clean Callback API:**
```go
type CreateUserDto struct {
    Name     string `json:"name"`
    Email    string `json:"email"`
    Age      int    `json:"age"`
}

// Define schema with callback (clean and concise)
var createUserSchema = validator.Schema(func(
    dto *CreateUserDto,
    builder *validator.SchemaBuilder[CreateUserDto],
) {
    builder.Field(&dto.Name, rules.Required[string](), rules.MinLength(2), rules.MaxLength(100))
    builder.Field(&dto.Email, rules.Required[string](), rules.Email())
    builder.Field(&dto.Age, rules.Min(18), rules.Max(120))
})

// Use it
result := createUserSchema.Validate(userDto)
if result.Invalid() {
    // Handle errors
}
```

**Alternative Builder Style:**
```go
var createUserSchema *validator.SchemaType[CreateUserDto]

func init() {
    var dto CreateUserDto
    builder := validator.NewSchema(&dto)
    builder.Field(&dto.Name, rules.Required[string](), rules.MinLength(2))
    builder.Field(&dto.Email, rules.Required[string](), rules.Email())    
    createUserSchema = builder.Build()
}
```

Both styles are type-safe and provide the same functionality!

### Basic Field Validation

```go
import (
    "github.com/gonest-dev/gonest/validator"
    "github.com/gonest-dev/gonest/validator/rules"
)

// Define validators
emailValidator := validator.Field[string]("email").
    Is(rules.Required[string]()).
    Is(rules.Email()).
    Is(rules.MaxLength(255))

// Validate
if err := emailValidator.Check("user@example.com"); err != nil {
    fmt.Println(err.Message())
}
```

### DTO Validation

```go
type CreateUserDto struct {
    Email    string
    Password string
    Age      int
}

// Pre-compile validators (do this once, reuse many times)
var (
    emailVal = validator.Field[string]("email").
        Is(rules.Required[string]()).
        Is(rules.Email())
    
    passwordVal = validator.Field[string]("password").
        Is(rules.Required[string]()).
        Is(rules.MinLength(8))
    
    ageVal = validator.Field[int]("age").
        Is(rules.Min(18)).
        Is(rules.Max(120))
)

func (dto *CreateUserDto) Validate() *validator.ValidationResult {
    result := validator.NewValidationResult()
    
    if err := emailVal.Check(dto.Email); err != nil {
        result.AddError(err)
    }
    
    if err := passwordVal.Check(dto.Password); err != nil {
        result.AddError(err)
    }
    
    if err := ageVal.Check(dto.Age); err != nil {
        result.AddError(err)
    }
    
    return result
}
```

## Built-in Rules

### Common Rules

```go
rules.Required[string]()              // Not zero/empty
rules.NotEmpty[string]()              // Not empty (for slices/maps/strings)
rules.Optional[string]()              // Always passes
rules.Custom[string](fn, "msg")       // Custom predicate
rules.Must[string](fn, "code", "msg") // Custom with code
rules.Equal("expected", "msg")        // Must equal value
rules.NotEqual("rejected", "msg")     // Must not equal value
rules.OneOf([]string{"a", "b"})       // Must be in list
rules.In([]int{1, 2, 3})              // Alias for OneOf
```

### String Rules

```go
rules.MinLength(8)          // Minimum length
rules.MaxLength(255)        // Maximum length
rules.Length(10)            // Exact length
rules.Email()               // Valid email
rules.URL()                 // Valid URL
rules.UUID()                // Valid UUID v4
rules.AlphaNumeric()        // Only letters and numbers
rules.Alpha()               // Only letters
rules.Numeric()             // Only numbers
rules.Pattern(`^[A-Z]`)     // Regex pattern
rules.Contains("substring") // Contains substring
rules.StartsWith("prefix")  // Starts with
rules.EndsWith("suffix")    // Ends with
```

### Number Rules

```go
rules.Min(18)            // Minimum value
rules.Max(120)           // Maximum value
rules.Range(1, 100)      // Within range [min, max]
rules.Positive[int]()    // > 0
rules.Negative[int]()    // < 0
rules.NonNegative[int]() // >= 0
rules.NonPositive[int]() // <= 0
rules.GreaterThan(0)     // > threshold
rules.LessThan(100)      // < threshold
rules.Between(1, 99)     // Strictly between (exclusive)
rules.MultipleOf(5)      // Must be multiple of
```

## Advanced Usage

### Cross-Field Validation

```go
func (dto *CreateUserDto) Validate() *validator.ValidationResult {
    result := validator.NewValidationResult()
    
    // ... field validations ...
    
    // Cross-field validation
    if dto.Password != dto.PasswordConfirm {
        result.AddError(validator.NewFieldError(
            "passwordConfirm",
            "match",
            "Passwords must match",
        ))
    }
    
    return result
}
```

### Custom Validators

```go
// Simple custom validator
func allowedDomain(domains []string) validator.Validator[string] {
    return func(value string) *validator.FieldError {
        for _, domain := range domains {
            if strings.HasSuffix(value, domain) {
                return nil
            }
        }
        
        err := validator.NewFieldError(
            "",
            "domain",
            "Email domain not allowed",
        )
        err.WithParam("allowed", domains)
        return err
    }
}

// Use it
emailVal := validator.Field[string]("email").
    Is(rules.Required[string]()).
    Is(rules.Email()).
    Is(allowedDomain([]string{"@company.com"}))
```

### Async Validation

```go
// Async validator (e.g., check database)
func uniqueEmail(repo UserRepository) validator.ContextValidator[string] {
    return func(ctx context.Context, value string) *validator.FieldError {
        exists, err := repo.EmailExists(ctx, value)
        if err != nil {
            return validator.NewFieldError("", "db_error", err.Error())
        }
        
        if exists {
            return validator.NewFieldError(
                "",
                "unique",
                "Email already taken",
            )
        }
        
        return nil
    }
}

// Use it
emailVal := validator.Field[string]("email").
    Is(rules.Required[string]()).
    Is(rules.Email()).
    IsAsync(uniqueEmail(userRepo))

// Validate with context
err := emailVal.CheckAsync(ctx, "user@example.com")
```

### Schema-Based Validation

```go
var userSchema = validator.NewSchema[CreateUserDto]().
    Field("email", 
        func(u CreateUserDto) any { return u.Email },
        func(v any) *validator.FieldError {
            email := v.(string)
            if err := rules.Required[string]()(email); err != nil {
                return err
            }
            return rules.Email()(email)
        }).
    Field("age",
        func(u CreateUserDto) any { return u.Age },
        func(v any) *validator.FieldError {
            return rules.Range(18, 120)(v.(int))
        }).
    CrossField(func(dto CreateUserDto) *validator.FieldError {
        if dto.Password != dto.PasswordConfirm {
            return validator.NewFieldError(
                "passwordConfirm",
                "match",
                "Passwords must match",
            )
        }
        return nil
    }).
    Build()

// Validate
result := userSchema.Validate(dto)
```

### Conditional Validation

```go
// Validate field only if condition is true
func (dto *SignupDto) Validate() *validator.ValidationResult {
    result := validator.NewValidationResult()
    
    // Always validate email
    if err := emailVal.Check(dto.Email); err != nil {
        result.AddError(err)
    }
    
    // Only validate password if provider is "local"
    if dto.Provider == "local" {
        if err := passwordVal.Check(dto.Password); err != nil {
            result.AddError(err)
        }
    }
    
    return result
}
```

## Error Handling

### Validation Result

```go
result := dto.Validate()

// Check if valid
if result.Valid() {
    // All good
}

// Get errors
for _, err := range result.Errors() {
    fmt.Printf("%s: %s\n", err.Field(), err.Message())
}

// Get first error
firstErr := result.First()

// Check specific field
if result.HasField("email") {
    emailErrors := result.GetFieldErrors("email")
}

// Error count
count := result.Count()
```

### JSON Error Format

```go
result := dto.Validate()

// Convert to JSON-friendly format
jsonErrors := result.ToJSON()

// Output:
// {
//   "valid": false,
//   "errors": {
//     "email": [
//       {
//         "code": "required",
//         "message": "This field is required",
//         "params": {}
//       }
//     ],
//     "age": [
//       {
//         "code": "min",
//         "message": "Value is below minimum",
//         "params": {
//           "min": 18,
//           "actual": 15
//         }
//       }
//     ]
//   }
// }
```

### Custom Error Messages

```go
emailVal := validator.Field[string]("email").
    Is(rules.Required[string]()).
    Is(rules.Email()).
    WithMessage("Please provide a valid email address")

// All errors from this validator will use the custom message
```

## Performance

### Pre-compile Validators

```go
// ✅ GOOD: Compile once, use many times
var emailVal = validator.Field[string]("email").
    Is(rules.Required[string]()).
    Is(rules.Email())

func ValidateEmail(email string) error {
    return emailVal.Check(email)
}
```

```go
// ❌ BAD: Creating validator every time
func ValidateEmail(email string) error {
    validator := validator.Field[string]("email").
        Is(rules.Required[string]()).
        Is(rules.Email())
    return validator.Check(email)
}
```

### Benchmarks

```
BenchmarkEmailValidation-8      5000000    250 ns/op
BenchmarkIntValidation-8       10000000    120 ns/op
BenchmarkComplexDTO-8           1000000   1500 ns/op
```

## Integration with Controllers

```go
type UserController struct {
    userService *UserService
}

func (c *UserController) Create(ctx *core.Context) error {
    var dto CreateUserDto
    
    // Bind JSON
    if err := ctx.BindJSON(&dto); err != nil {
        return ctx.JSON(400, map[string]string{"error": "Invalid JSON"})
    }
    
    // Validate
    result := dto.Validate()
    if result.Invalid() {
        return ctx.JSON(400, result.ToJSON())
    }
    
    // Process
    user, err := c.userService.Create(&dto)
    if err != nil {
        return ctx.JSON(500, map[string]string{"error": err.Error()})
    }
    
    return ctx.JSON(201, user)
}
```

## Best Practices

### 1. Pre-compile Validators

Define validators as package-level variables or struct fields for reuse.

### 2. Use Type Parameters

Leverage generics for type safety:

```go
// ✅ Type-safe
rules.Min[int](18)

// ✅ Compiler catches this
rules.Min[string](18)  // Compile error!
```

### 3. Explicit Field Names

Always provide field names for clear error messages:

```go
validator.Field[string]("email")  // ✅ Clear
validator.Field[string]("")       // ❌ No field name
```

### 4. Compose Validators

Build complex validators from simple ones:

```go
strongPassword := validator.Field[string]("password").
    Is(rules.MinLength(8)).
    Is(rules.Pattern(`[A-Z]`)).
    Is(rules.Pattern(`[a-z]`)).
    Is(rules.Pattern(`[0-9]`)).
    Is(rules.Pattern(`[!@#$%^&*]`))
```

### 5. Use Custom Messages Sparingly

Only override messages when needed for clarity:

```go
// ✅ Good: specific message
passwordVal.WithMessage("Password must contain uppercase, lowercase, number, and symbol")

// ❌ Bad: generic message
ageVal.WithMessage("Invalid")
```

## Testing

```go
func TestEmailValidation(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"invalid email", "not-an-email", true},
        {"empty email", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := emailVal.Check(tt.email)
            if (err != nil) != tt.wantErr {
                t.Errorf("wanted error: %v, got: %v", tt.wantErr, err)
            }
        })
    }
}
```

## License

MIT