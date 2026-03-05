package main

import (
	"fmt"

	"github.com/gonest-dev/gonest/controller"
	"github.com/gonest-dev/gonest/core"
	"github.com/gonest-dev/gonest/exceptions"
	"github.com/gonest-dev/gonest/validator"
	"github.com/gonest-dev/gonest/validator/rules"
)

// ========================================
// Exception Examples
// ========================================

func NewExceptionController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/api"),
	)

	// Route 1: BadRequest (400)
	ctrl.Get("/bad-request", func(ctx *core.Context) error {
		return exceptions.BadRequestException("Invalid request parameters").
			WithDetail("param", "id").
			WithDetail("reason", "must be a positive integer")
	})

	// Route 2: Unauthorized (401)
	ctrl.Get("/unauthorized", func(ctx *core.Context) error {
		return exceptions.UnauthorizedException("Authentication required")
	})

	// Route 3: Forbidden (403)
	ctrl.Get("/forbidden", func(ctx *core.Context) error {
		return exceptions.ForbiddenException("Insufficient permissions").
			WithDetail("required", "admin").
			WithDetail("actual", "user")
	})

	// Route 4: NotFound (404)
	ctrl.Get("/not-found", func(ctx *core.Context) error {
		return exceptions.NotFoundException("User not found").
			WithDetail("userId", 123)
	})

	// Route 5: Conflict (409)
	ctrl.Post("/conflict", func(ctx *core.Context) error {
		return exceptions.ConflictException("Email already exists").
			WithDetail("email", "user@example.com")
	})

	// Route 6: Validation Error (422)
	ctrl.Post("/validation", func(ctx *core.Context) error {
		// Simulate validation failure
		result := validator.NewValidationResult()
		result.AddError(validator.NewFieldError("email", "invalid", "Invalid email format"))
		result.AddError(validator.NewFieldError("age", "min", "Age must be at least 18"))

		return exceptions.NewValidationException(result)
	})

	// Route 7: Internal Server Error (500)
	ctrl.Get("/internal-error", func(ctx *core.Context) error {
		return exceptions.InternalServerErrorException("Database connection failed")
	})

	// Route 8: Service Unavailable (503)
	ctrl.Get("/service-unavailable", func(ctx *core.Context) error {
		return exceptions.ServiceUnavailableException("Service is under maintenance")
	})

	// Route 9: Custom Exception
	ctrl.Get("/custom", func(ctx *core.Context) error {
		return exceptions.NewHttpException(418, "I'm a teapot").
			WithDetail("info", "This is a custom status code")
	})

	// Route 10: Success (no exception)
	ctrl.Get("/success", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "Request successful",
		})
	})

	return ctrl
}

// ========================================
// Validation Exception Example
// ========================================

type CreateUserDto struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Age      int    `json:"age"`
}

func (dto *CreateUserDto) Validate() *validator.ValidationResult {
	var temp CreateUserDto
	builder := validator.NewSchema(&temp)

	builder.Field(&temp.Email,
		rules.Required[string](),
		rules.Email(),
	)

	builder.Field(&temp.Password,
		rules.Required[string](),
		rules.MinLength(8),
	)

	builder.Field(&temp.Age,
		rules.Min(18),
	)

	schema := builder.Build()
	return schema.Validate(dto)
}

func NewValidationController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/users"),
	)

	ctrl.Post("", func(ctx *core.Context) error {
		var dto CreateUserDto
		if err := ctx.BindJSON(&dto); err != nil {
			return exceptions.BadRequestException("Invalid JSON").
				WithDetail("error", err.Error())
		}

		// Validate
		result := dto.Validate()
		if result.Invalid() {
			return exceptions.NewValidationException(result)
		}

		return ctx.JSON(201, map[string]any{
			"message": "User created",
			"data":    dto,
		})
	})

	return ctrl
}

// ========================================
// Custom Exception Filter
// ========================================

type CustomExceptionFilter struct{}

func (f *CustomExceptionFilter) Catch(err error, ctx *core.Context) error {
	// Add custom headers
	ctx.Set("X-Error-Handler", "CustomFilter")

	// Pass to next filter
	return err
}

// ========================================
// Main
// ========================================

func main() {
	fmt.Println("========================================")
	fmt.Println("GoNest Exception Handling Examples")
	fmt.Println("========================================")
	fmt.Println()
	// Create controllers
	exceptionCtrl := NewExceptionController()
	validationCtrl := NewValidationController()

	// Display routes
	fmt.Println("Exception Routes:")
	for _, route := range exceptionCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Validation Routes:")
	for _, route := range validationCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	// Demonstrate exception types
	fmt.Println("========================================")
	fmt.Println("HTTP Exception Types:")
	fmt.Println("========================================")
	fmt.Println()

	fmt.Println("1. BadRequestException (400):")
	badReq := exceptions.BadRequestException("Invalid input").WithDetail("field", "email")
	fmt.Printf("   Status: %d\n", badReq.StatusCode)
	fmt.Printf("   Message: %s\n", badReq.Message)
	fmt.Printf("   JSON: %v\n", badReq.ToJSON())
	fmt.Println()

	fmt.Println("2. UnauthorizedException (401):")
	unauth := exceptions.UnauthorizedException("Token expired")
	fmt.Printf("   Status: %d\n", unauth.StatusCode)
	fmt.Printf("   Message: %s\n", unauth.Message)
	fmt.Println()

	fmt.Println("3. NotFoundException (404):")
	notFound := exceptions.NotFoundException("Resource not found")
	fmt.Printf("   Status: %d\n", notFound.StatusCode)
	fmt.Printf("   Message: %s\n", notFound.Message)
	fmt.Println()

	fmt.Println("4. ValidationException (422):")
	result := validator.NewValidationResult()
	result.AddError(validator.NewFieldError("email", "invalid", "Invalid email"))
	validationEx := exceptions.NewValidationException(result)
	fmt.Printf("   Status: %d\n", validationEx.StatusCode)
	fmt.Printf("   JSON: %v\n", validationEx.ToJSON())
	fmt.Println()

	fmt.Println("5. InternalServerErrorException (500):")
	serverErr := exceptions.InternalServerErrorException("Database error")
	fmt.Printf("   Status: %d\n", serverErr.StatusCode)
	fmt.Printf("   Message: %s\n", serverErr.Message)
	fmt.Println()

	// Exception filters
	fmt.Println("========================================")
	fmt.Println("Exception Filters:")
	fmt.Println("========================================")
	fmt.Println()

	fmt.Println("1. GlobalExceptionFilter:")
	fmt.Println("   - Catches all exceptions")
	fmt.Println("   - Logs errors")
	fmt.Println("   - Returns structured JSON")
	fmt.Println()

	fmt.Println("2. ValidationExceptionFilter:")
	fmt.Println("   - Handles validation errors")
	fmt.Println("   - Formats validation messages")
	fmt.Println()

	fmt.Println("3. NotFoundExceptionFilter:")
	fmt.Println("   - Custom 404 responses")
	fmt.Println("   - Includes request path")
	fmt.Println()

	fmt.Println("4. Chain Multiple Filters:")
	fmt.Println("   - Process exceptions in order")
	fmt.Println("   - Each filter can modify response")
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("Summary:")
	fmt.Println("========================================")
	fmt.Println("✓ 8 HTTP exception types")
	fmt.Println("✓ ValidationException for validation errors")
	fmt.Println("✓ GlobalExceptionFilter")
	fmt.Println("✓ Custom exception filters")
	fmt.Println("✓ Chain multiple filters")
	fmt.Println("✓ Structured JSON responses")
	fmt.Println("✓ Exception details & metadata")
	fmt.Println("========================================")
}
