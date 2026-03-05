package main

import (
	"fmt"

	"github.com/leandroluk/gonest/controller"
	"github.com/leandroluk/gonest/core"
	"github.com/leandroluk/gonest/validator"
	"github.com/leandroluk/gonest/validator/rules"
)

// ========================================
// DTOs
// ========================================

type CreateUserDto struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func (dto *CreateUserDto) Validate() *validator.ValidationResult {
	var temp CreateUserDto
	builder := validator.NewSchema(&temp)

	builder.Field(&temp.Name, rules.Required[string](), rules.MinLength(2))
	builder.Field(&temp.Email, rules.Required[string](), rules.Email())
	builder.Field(&temp.Age, rules.Min(18), rules.Max(120))

	schema := builder.Build()
	return schema.Validate(dto)
}

type UpdateUserDto struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ========================================
// User Controller
// ========================================

func NewUserController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/users"),
	)

	// GET /users - List all users
	ctrl.Get("", func(ctx *core.Context) error {
		users := []map[string]any{
			{"id": 1, "name": "John Doe", "email": "john@example.com"},
			{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
		}

		return ctx.JSON(200, map[string]any{"data": users})
	})

	// GET /users/:id - Get user by ID
	ctrl.Get("/:id", func(ctx *core.Context) error {
		id := ctx.Param("id")

		return ctx.JSON(200, map[string]any{
			"data": map[string]any{
				"id":    id,
				"name":  "John Doe",
				"email": "john@example.com",
			},
		})
	}).Param("id")

	// POST /users - Create user
	ctrl.Post("", func(ctx *core.Context) error {
		var dto CreateUserDto

		if err := ctx.BindJSON(&dto); err != nil {
			return ctx.JSON(400, map[string]any{"error": "Invalid JSON"})
		}

		// Validate
		result := dto.Validate()
		if result.Invalid() {
			return ctx.JSON(400, result.ToJSON())
		}

		// Simulate user creation
		return ctx.JSON(201, map[string]any{
			"data": map[string]any{
				"id":    123,
				"name":  dto.Name,
				"email": dto.Email,
				"age":   dto.Age,
			},
		})
	}).Body("user")

	// PUT /users/:id - Update user
	ctrl.Put("/:id", func(ctx *core.Context) error {
		id := ctx.Param("id")

		var dto UpdateUserDto
		if err := ctx.BindJSON(&dto); err != nil {
			return ctx.JSON(400, map[string]any{"error": "Invalid JSON"})
		}

		return ctx.JSON(200, map[string]any{
			"data": map[string]any{
				"id":    id,
				"name":  dto.Name,
				"email": dto.Email,
			},
		})
	}).Param("id").Body("user")

	// DELETE /users/:id - Delete user
	ctrl.Delete("/:id", func(ctx *core.Context) error {
		id := ctx.Param("id")

		return ctx.JSON(200, map[string]any{"message": fmt.Sprintf("User %s deleted", id)})
	}).Param("id")

	return ctrl
}

// ========================================
// Product Controller
// ========================================

func NewProductController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/products"),
	)

	// GET /products?category=electronics&minPrice=100
	ctrl.Get("", func(ctx *core.Context) error {
		category := ctx.Query("category")
		minPrice := ctx.Query("minPrice")

		return ctx.JSON(200, map[string]any{
			"filters": map[string]any{
				"category": category,
				"minPrice": minPrice,
			},
			"data": []map[string]any{
				{"id": 1, "name": "Product 1", "price": 99.99},
				{"id": 2, "name": "Product 2", "price": 149.99},
			},
		})
	}).Query("category", false).Query("minPrice", false)

	return ctrl
}

// ========================================
// Main
// ========================================

func main() {
	fmt.Println("========================================")
	fmt.Println("GoNest Controller Example")
	fmt.Println("========================================")
	fmt.Println()

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

	fmt.Println("========================================")
	fmt.Println("Summary:")
	fmt.Println("✓ Controller builder pattern")
	fmt.Println("✓ HTTP method decorators (Get, Post, Put, etc)")
	fmt.Println("✓ Route parameters")
	fmt.Println("✓ Query parameters")
	fmt.Println("✓ Body validation")
	fmt.Println("✓ Automatic DTO binding")
	fmt.Println("========================================")
}
