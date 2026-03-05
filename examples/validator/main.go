package main

import (
	"context"
	"fmt"
	"time"

	"github.com/leandroluk/gonest/controller"
	"github.com/leandroluk/gonest/core"
	"github.com/leandroluk/gonest/pipes"
	"github.com/leandroluk/gonest/validator"
	"github.com/leandroluk/gonest/validator/rules"
)

// ========================================
// PART 1: Basic Validation Examples
// ========================================

func BasicValidationExamples() {
	fmt.Println("========================================")
	fmt.Println("PART 1: Basic Validation")
	fmt.Println("========================================\n")

	// Example 1: Email validation
	fmt.Println("1. Email Validation:")
	emailValidator := validator.Field[string]("email").
		Is(rules.Required[string]()).
		Is(rules.Email())

	emails := []string{"test@example.com", "invalid-email", ""}
	for _, email := range emails {
		if err := emailValidator.Check(email); err == nil {
			fmt.Printf("   ✓ '%s': Valid\n", email)
		} else {
			fmt.Printf("   ✗ '%s': %s\n", email, err.Message())
		}
	}
	fmt.Println()

	// Example 2: Age validation
	fmt.Println("2. Age Validation:")
	ageValidator := validator.Field[int]("age").
		Is(rules.Min(18)).
		Is(rules.Max(120))

	ages := []int{17, 25, 121}
	for _, age := range ages {
		if err := ageValidator.Check(age); err == nil {
			fmt.Printf("   ✓ %d: Valid\n", age)
		} else {
			fmt.Printf("   ✗ %d: %s\n", age, err.Message())
		}
	}
	fmt.Println()

	// Example 3: Strong password
	fmt.Println("3. Strong Password:")
	passwordValidator := validator.Field[string]("password").
		Is(rules.StrongPassword())

	passwords := []string{"weak", "Strong123!", "NoDigits!"}
	for _, pwd := range passwords {
		if err := passwordValidator.Check(pwd); err == nil {
			fmt.Printf("   ✓ '%s': Strong\n", pwd)
		} else {
			fmt.Printf("   ✗ '%s': %s\n", pwd, err.Message())
		}
	}
	fmt.Println()

	// Example 4: Boolean validation
	fmt.Println("4. Boolean Validation:")
	termsValidator := validator.Field[bool]("acceptTerms").
		Is(rules.MustAccept())

	if err := termsValidator.Check(true); err == nil {
		fmt.Println("   ✓ Terms accepted")
	}
	if err := termsValidator.Check(false); err != nil {
		fmt.Printf("   ✗ Terms not accepted: %s\n", err.Message())
	}
	fmt.Println()
}

// ========================================
// PART 2: Advanced Validation Features
// ========================================

func AdvancedValidationExamples() {
	fmt.Println("========================================")
	fmt.Println("PART 2: Advanced Validation")
	fmt.Println("========================================\n")

	// Example 1: Conditional validation
	fmt.Println("1. Conditional Validation:")
	stockValidator := validator.Field[int]("stock").
		Is(rules.When(
			func(val int) bool { return val > 0 },
			rules.Min[int](1),
		))

	if err := stockValidator.Check(5); err == nil {
		fmt.Println("   ✓ Stock 5: Valid")
	}
	if err := stockValidator.Check(0); err == nil {
		fmt.Println("   ✓ Stock 0: Valid (condition not met)")
	}
	fmt.Println()

	// Example 2: Array validation
	fmt.Println("2. Array Validation:")
	tagsValidator := validator.Field[[]string]("tags").
		Is(rules.ArrayMinSize[string](1)).
		Is(rules.ArrayMaxSize[string](5)).
		Is(rules.ArrayUnique[string]())

	validTags := []string{"tag1", "tag2"}
	invalidTags := []string{"tag1", "tag1"}

	if err := tagsValidator.Check(validTags); err == nil {
		fmt.Printf("   ✓ %v: Valid\n", validTags)
	}
	if err := tagsValidator.Check(invalidTags); err != nil {
		fmt.Printf("   ✗ %v: %s\n", invalidTags, err.Message())
	}
	fmt.Println()

	// Example 3: Date validation
	fmt.Println("3. Date Validation:")
	dateValidator := validator.Field[time.Time]("releaseDate").
		Is(rules.DateFuture())

	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	if err := dateValidator.Check(future); err == nil {
		fmt.Println("   ✓ Future date: Valid")
	}
	if err := dateValidator.Check(past); err != nil {
		fmt.Printf("   ✗ Past date: %s\n", err.Message())
	}
	fmt.Println()

	// Example 4: Async validation
	fmt.Println("4. Async Validation:")
	usernameAsyncValidator := validator.Field[string]("username").
		Is(rules.Required[string]()).
		IsAsync(rules.AsyncCustom(
			func(ctx context.Context, val string) (bool, error) {
				time.Sleep(50 * time.Millisecond)
				if val == "taken" {
					return false, nil // Invalid
				}
				return true, nil // Valid
			},
			"username_taken",
			"Username is already taken",
		))

	ctx := context.Background()
	if err := usernameAsyncValidator.CheckAsync(ctx, "available"); err == nil {
		fmt.Println("   ✓ 'available': Valid")
	}
	if err := usernameAsyncValidator.CheckAsync(ctx, "taken"); err != nil {
		fmt.Printf("   ✗ 'taken': %s\n", err.Message())
	}
	fmt.Println()
}

// ========================================
// PART 3: Schema-Based Validation
// ========================================

type UpdateUserDto struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

// Using clean callback API
var updateUserSchema = validator.Schema(func(
	dto *UpdateUserDto,
	builder *validator.SchemaBuilder[UpdateUserDto],
) {
	builder.Field(&dto.Name,
		rules.Required[string](),
		rules.MinLength(2),
	)

	builder.Field(&dto.Email,
		rules.Required[string](),
		rules.Email(),
	)

	builder.Field(&dto.Age,
		rules.Min(18),
		rules.Max(120),
	)
})

func SchemaValidationExamples() {
	fmt.Println("========================================")
	fmt.Println("PART 3: Schema Validation")
	fmt.Println("========================================")
	fmt.Println()

	fmt.Println("1. Valid DTO:")
	validDto := &UpdateUserDto{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	result := updateUserSchema.Validate(validDto)
	if result.Valid() {
		fmt.Println("   ✓ All fields valid")
	}
	fmt.Println()

	fmt.Println("2. Invalid DTO:")
	invalidDto := &UpdateUserDto{
		Name:  "J",
		Email: "invalid",
		Age:   15,
	}

	result = updateUserSchema.Validate(invalidDto)
	if result.Invalid() {
		fmt.Println("   ✗ Validation errors:")
		for _, err := range result.Errors() {
			fmt.Printf("     - %s: %s\n", err.Field(), err.Message())
		}
	}
	fmt.Println()
}

// ========================================
// PART 4: Pipes Integration
// ========================================

type RegisterDto struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Age      int    `json:"age"`
}

func (dto *RegisterDto) Validate() *validator.ValidationResult {
	schema := validator.Schema(func(d *RegisterDto, builder *validator.SchemaBuilder[RegisterDto]) {
		builder.Field(&d.Email,
			rules.Required[string](),
			rules.Email(),
		)

		builder.Field(&d.Password,
			rules.Required[string](),
			rules.StrongPassword(),
		)

		builder.Field(&d.Age,
			rules.Min(18),
		)
	})

	return schema.Validate(dto)
}

func PipesIntegrationExamples() {
	fmt.Println("========================================")
	fmt.Println("PART 4: Pipes Integration")
	fmt.Println("========================================")
	fmt.Println()

	fmt.Println("1. ParseIntPipe:")
	intPipe := pipes.NewParseIntPipe()
	if result, err := intPipe.Transform("123", nil); err == nil {
		fmt.Printf("   '123' → %d\n", result)
	}
	fmt.Println()

	fmt.Println("2. ParseEnumPipe:")
	enumPipe := pipes.NewParseEnumPipe("active", "inactive")
	if result, err := enumPipe.Transform("active", nil); err == nil {
		fmt.Printf("   'active' → %s ✓\n", result)
	}
	if _, err := enumPipe.Transform("invalid", nil); err != nil {
		fmt.Printf("   'invalid' → ✗ %s\n", err.Error())
	}
	fmt.Println()

	fmt.Println("3. ValidationPipe with DTO:")
	validDto := &RegisterDto{
		Email:    "user@example.com",
		Password: "Strong123!",
		Age:      25,
	}

	if result := validDto.Validate(); result.Valid() {
		fmt.Println("   ✓ DTO validation passed")
	}

	invalidDto := &RegisterDto{
		Email:    "invalid",
		Password: "weak",
		Age:      15,
	}

	if result := invalidDto.Validate(); result.Invalid() {
		fmt.Println("   ✗ DTO validation failed:")
		for _, err := range result.Errors() {
			fmt.Printf("     - %s\n", err.Message())
		}
	}
	fmt.Println()
}

// ========================================
// PART 5: Controller Integration
// ========================================

func NewValidationController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/api"),
	)

	ctrl.Post("/register", func(ctx *core.Context) error {
		dto, err := pipes.ValidateBody[RegisterDto](ctx)
		if err != nil {
			if validationErr, ok := err.(*pipes.ValidationError); ok {
				return ctx.JSON(400, validationErr.ToJSON())
			}
			return ctx.JSON(400, map[string]any{"error": err.Error()})
		}

		return ctx.JSON(201, map[string]any{"message": "Registered", "data": dto})
	})

	ctrl.Get("/users/:id", func(ctx *core.Context) error {
		intPipe := pipes.NewParseIntPipe()
		id, err := intPipe.Transform(ctx.Param("id"), ctx)
		if err != nil {
			return ctx.JSON(400, map[string]any{"error": "Invalid ID"})
		}

		return ctx.JSON(200, map[string]any{"id": id})
	})

	return ctrl
}

func ControllerIntegrationExample() {
	fmt.Println("========================================")
	fmt.Println("PART 5: Controller Integration")
	fmt.Println("========================================")
	fmt.Println()

	ctrl := NewValidationController()

	fmt.Println("Routes:")
	for _, route := range ctrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println("\nFeatures:")
	fmt.Println("  ✓ Automatic DTO validation")
	fmt.Println("  ✓ Parse pipes for parameters")
	fmt.Println("  ✓ Structured error responses")
	fmt.Println()
}

// ========================================
// Main
// ========================================

func main() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║  GoNest Complete Validation Examples  ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	BasicValidationExamples()
	AdvancedValidationExamples()
	SchemaValidationExamples()
	PipesIntegrationExamples()
	ControllerIntegrationExample()

	fmt.Println("========================================")
	fmt.Println("Complete Summary:")
	fmt.Println("========================================")
	fmt.Println("✓ 86+ validation rules")
	fmt.Println("✓ Basic validators (email, password, age)")
	fmt.Println("✓ Advanced features (conditional, async)")
	fmt.Println("✓ Schema-based validation")
	fmt.Println("✓ Array & date validation")
	fmt.Println("✓ Boolean validation")
	fmt.Println("✓ Parse pipes (int, float, bool, enum)")
	fmt.Println("✓ ValidationPipe integration")
	fmt.Println("✓ Controller integration")
	fmt.Println("✓ Type-safe & composable")
	fmt.Println("========================================")
}
