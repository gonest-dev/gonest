package main

import (
	"fmt"

	"github.com/gonest-dev/gonest/controller"
	"github.com/gonest-dev/gonest/core"
	"github.com/gonest-dev/gonest/pipes"
	"github.com/gonest-dev/gonest/validator"
	"github.com/gonest-dev/gonest/validator/rules"
)

// ========================================
// DTOs with Validation
// ========================================

type CreateUserDto struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func (dto *CreateUserDto) Validate() *validator.ValidationResult {
	var temp CreateUserDto
	builder := validator.NewSchema(&temp)

	builder.Field(&temp.Name, rules.Required[string](), rules.MinLength(2), rules.MaxLength(50))

	builder.Field(&temp.Email, rules.Required[string](), rules.Email())

	builder.Field(&temp.Age, rules.Min(18), rules.Max(120))

	schema := builder.Build()
	return schema.Validate(dto)
}

type QueryDto struct {
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
	Sort  string `json:"sort"`
}

// ========================================
// Controllers with Pipes
// ========================================

func NewUserController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/users"),
	)

	// POST /users - Create with automatic validation
	ctrl.Post("", func(ctx *core.Context) error {
		// Using ValidateBody helper
		dto, err := pipes.ValidateBody[CreateUserDto](ctx)
		if err != nil {
			// Check if validation error
			if validationErr, ok := err.(*pipes.ValidationError); ok {
				return ctx.JSON(400, validationErr.ToJSON())
			}
			return ctx.JSON(400, map[string]any{
				"error": err.Error(),
			})
		}

		return ctx.JSON(201, map[string]any{
			"message": "User created successfully",
			"data":    dto,
		})
	})

	// GET /users/:id - With ParseIntPipe
	ctrl.Get("/:id", func(ctx *core.Context) error {
		// Manual pipe application
		intPipe := pipes.NewParseIntPipe()
		idStr := ctx.Param("id")

		id, err := intPipe.Transform(idStr, ctx)
		if err != nil {
			return ctx.JSON(400, map[string]any{
				"error": fmt.Sprintf("Invalid ID: %s", err.Error()),
			})
		}

		return ctx.JSON(200, map[string]any{
			"data": map[string]any{
				"id":    id,
				"name":  "John Doe",
				"email": "john@example.com",
			},
		})
	})

	// GET /users/uuid/:uuid - With ParseUUIDPipe
	ctrl.Get("/uuid/:uuid", func(ctx *core.Context) error {
		uuidPipe := pipes.NewParseUUIDPipe()
		uuidStr := ctx.Param("uuid")

		uuid, err := uuidPipe.Transform(uuidStr, ctx)
		if err != nil {
			return ctx.JSON(400, map[string]any{
				"error": fmt.Sprintf("Invalid UUID: %s", err.Error()),
			})
		}

		return ctx.JSON(200, map[string]any{
			"uuid": uuid,
		})
	})

	// GET /users?page=1&limit=10&sort=name
	ctrl.Get("", func(ctx *core.Context) error {
		intPipe := pipes.NewParseIntPipe()
		defaultPipe := pipes.NewDefaultValuePipe("10")

		// Parse page
		pageStr := ctx.Query("page")
		if pageStr == "" {
			pageStr = "1"
		}
		page, err := intPipe.Transform(pageStr, ctx)
		if err != nil {
			return ctx.JSON(400, map[string]any{
				"error": "Invalid page number",
			})
		}

		// Parse limit with default
		limitStr, _ := defaultPipe.Transform(ctx.Query("limit"), ctx)
		limit, err := intPipe.Transform(limitStr, ctx)
		if err != nil {
			return ctx.JSON(400, map[string]any{
				"error": "Invalid limit",
			})
		}

		sort := ctx.Query("sort")
		if sort == "" {
			sort = "id"
		}

		return ctx.JSON(200, map[string]any{
			"page":  page,
			"limit": limit,
			"sort":  sort,
			"data":  []map[string]any{},
		})
	})

	return ctrl
}

// Controller with enum validation
func NewProductController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/products"),
	)

	// GET /products?status=active
	ctrl.Get("", func(ctx *core.Context) error {
		enumPipe := pipes.NewParseEnumPipe("active", "inactive", "draft")

		statusStr := ctx.Query("status")
		if statusStr == "" {
			statusStr = "active"
		}

		status, err := enumPipe.Transform(statusStr, ctx)
		if err != nil {
			return ctx.JSON(400, map[string]any{
				"error": err.Error(),
			})
		}

		return ctx.JSON(200, map[string]any{
			"status": status,
			"data":   []map[string]any{},
		})
	})

	// GET /products?tags=electronics,gadgets,new
	ctrl.Get("/search", func(ctx *core.Context) error {
		arrayPipe := pipes.NewParseArrayPipe(",")

		tagsStr := ctx.Query("tags")
		tags, err := arrayPipe.Transform(tagsStr, ctx)
		if err != nil {
			return ctx.JSON(400, map[string]any{
				"error": err.Error(),
			})
		}

		return ctx.JSON(200, map[string]any{
			"tags": tags,
			"data": []map[string]any{},
		})
	})

	return ctrl
}

// ========================================
// Main
// ========================================

func main() {
	fmt.Println("========================================")
	fmt.Println("GoNest Pipes & Validation Example")
	fmt.Println("========================================\n")

	// Create controllers
	userController := NewUserController()
	productController := NewProductController()

	// Display routes
	fmt.Println("User Controller Routes:")
	for _, route := range userController.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Product Controller Routes:")
	for _, route := range productController.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	// Demonstrate pipe usage
	fmt.Println("Pipe Examples:")
	fmt.Println()

	// ParseIntPipe
	fmt.Println("1. ParseIntPipe:")
	intPipe := pipes.NewParseIntPipe()
	if result, err := intPipe.Transform("123", nil); err == nil {
		fmt.Printf("   '123' -> %d (type: %T)\n", result, result)
	}
	fmt.Println()

	// ParseFloatPipe
	fmt.Println("2. ParseFloatPipe:")
	floatPipe := pipes.NewParseFloatPipe()
	if result, err := floatPipe.Transform("123.45", nil); err == nil {
		fmt.Printf("   '123.45' -> %.2f (type: %T)\n", result, result)
	}
	fmt.Println()

	// ParseBoolPipe
	fmt.Println("3. ParseBoolPipe:")
	boolPipe := pipes.NewParseBoolPipe()
	if result, err := boolPipe.Transform("true", nil); err == nil {
		fmt.Printf("   'true' -> %v (type: %T)\n", result, result)
	}
	fmt.Println()

	// ParseEnumPipe
	fmt.Println("4. ParseEnumPipe:")
	enumPipe := pipes.NewParseEnumPipe("active", "inactive")
	if result, err := enumPipe.Transform("active", nil); err == nil {
		fmt.Printf("   'active' -> %s ✓\n", result)
	}
	if _, err := enumPipe.Transform("invalid", nil); err != nil {
		fmt.Printf("   'invalid' -> ✗ %s\n", err.Error())
	}
	fmt.Println()

	// ParseArrayPipe
	fmt.Println("5. ParseArrayPipe:")
	arrayPipe := pipes.NewParseArrayPipe(",")
	if result, err := arrayPipe.Transform("a,b,c", nil); err == nil {
		fmt.Printf("   'a,b,c' -> %v\n", result)
	}
	fmt.Println()

	// DefaultValuePipe
	fmt.Println("6. DefaultValuePipe:")
	defaultPipe := pipes.NewDefaultValuePipe("default")
	if result, err := defaultPipe.Transform("", nil); err == nil {
		fmt.Printf("   '' -> '%s'\n", result)
	}
	if result, err := defaultPipe.Transform("custom", nil); err == nil {
		fmt.Printf("   'custom' -> '%s'\n", result)
	}
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("Summary:")
	fmt.Println("✓ ValidationPipe - automatic DTO validation")
	fmt.Println("✓ ParseIntPipe - string to int")
	fmt.Println("✓ ParseFloatPipe - string to float")
	fmt.Println("✓ ParseBoolPipe - string to bool")
	fmt.Println("✓ ParseUUIDPipe - UUID validation")
	fmt.Println("✓ ParseEnumPipe - enum validation")
	fmt.Println("✓ ParseArrayPipe - comma-separated to array")
	fmt.Println("✓ DefaultValuePipe - provide defaults")
	fmt.Println("========================================")
}
