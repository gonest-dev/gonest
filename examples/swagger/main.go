package main

import (
	"encoding/json"
	"fmt"

	"github.com/leandroluk/gonest/swagger"
)

// ========================================
// DTOs for Swagger Examples
// ========================================

type CreateUserDto struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Age      int    `json:"age"`
	Password string `json:"password"`
}

type UpdateUserDto struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

type UserResponseDto struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

type ErrorResponseDto struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

// ========================================
// Descriptors (Type-Safe, No Tags!)
// Clean Callback API
// ========================================

func DescribeCreateUserDto() *swagger.Schema {
	return swagger.Descriptor(func(dto *CreateUserDto, d *swagger.DescriptorBuilder[CreateUserDto]) {
		d.Field(&dto.Name).
			Description("User's full name").
			Required().
			MinLength(2).
			MaxLength(100).
			Example("John Doe")

		d.Field(&dto.Email).
			Description("User's email address").
			Required().
			Format("email").
			Example("john@example.com")

		d.Field(&dto.Age).
			Description("User's age in years").
			Minimum(0).
			Maximum(120).
			Example(25)

		d.Field(&dto.Password).
			Description("User's password (minimum 8 characters)").
			Required().
			MinLength(8).
			Format("password").
			WriteOnly() // Not returned in responses
	})
}

func DescribeUpdateUserDto() *swagger.Schema {
	return swagger.Descriptor(func(dto *UpdateUserDto, d *swagger.DescriptorBuilder[UpdateUserDto]) {
		d.Field(&dto.Name).
			Description("User's full name").
			MinLength(2).
			MaxLength(100)

		d.Field(&dto.Email).
			Description("User's email address").
			Format("email")

		d.Field(&dto.Age).
			Description("User's age in years").
			Minimum(0).
			Maximum(120)
	})
}

func DescribeUserResponseDto() *swagger.Schema {
	return swagger.Descriptor(func(dto *UserResponseDto, d *swagger.DescriptorBuilder[UserResponseDto]) {
		d.Field(&dto.ID).
			Description("Unique user identifier").
			ReadOnly().
			Example(123)

		d.Field(&dto.Name).
			Description("User's full name").
			Example("John Doe")

		d.Field(&dto.Email).
			Description("User's email address").
			Format("email").
			Example("john@example.com")

		d.Field(&dto.Age).
			Description("User's age in years").
			Example(25)
	})
}

func DescribeErrorResponseDto() *swagger.Schema {
	return swagger.Descriptor(func(dto *ErrorResponseDto, d *swagger.DescriptorBuilder[ErrorResponseDto]) {
		d.Field(&dto.StatusCode).
			Description("HTTP status code").
			Example(400)

		d.Field(&dto.Message).
			Description("Error message").
			Example("Bad Request")
	})
}

// ========================================
// Build OpenAPI Document
// ========================================

func BuildSwaggerDocument() *swagger.OpenAPIDocument {
	builder := swagger.NewDocumentBuilder()

	// Set API Info
	builder.SetInfo(
		"GoNest API",
		"A NestJS-inspired API framework for Go with complete Swagger documentation",
		"1.0.0",
	)

	// Set Contact
	builder.SetContact(
		"API Support",
		"https://github.com/leandroluk/gonest",
		"support@example.com",
	)

	// Set License
	builder.SetLicense("MIT", "https://opensource.org/licenses/MIT")

	// Add Servers
	builder.AddServer("http://localhost:3000", "Development server")
	builder.AddServer("https://api.example.com", "Production server")

	// Add Tags
	builder.AddTag("users", "User management endpoints")
	builder.AddTag("products", "Product management endpoints")
	builder.AddTag("auth", "Authentication endpoints")

	// Add Security Schemes
	builder.AddBearerAuth()
	builder.AddAPIKeyAuth("X-API-Key", "header")

	// Add Schemas using Descriptors (Type-Safe!)
	builder.AddSchema("CreateUserDto", DescribeCreateUserDto())
	builder.AddSchema("UpdateUserDto", DescribeUpdateUserDto())
	builder.AddSchema("UserResponseDto", DescribeUserResponseDto())
	builder.AddSchema("ErrorResponseDto", DescribeErrorResponseDto())

	// Add Paths
	addUserPaths(builder)
	addProductPaths(builder)

	return builder.Build()
}

func addUserPaths(builder *swagger.DocumentBuilder) {
	// GET /users - List all users
	listUsers := swagger.NewOperation(
		"List all users",
		"Retrieve a list of all users in the system",
	).
		WithTag("users").
		WithParameter("page", "query", "Page number", false, &swagger.Schema{Type: "integer"}).
		WithParameter("limit", "query", "Items per page", false, &swagger.Schema{Type: "integer"}).
		WithResponse("200", "Successful response", &swagger.Schema{
			Type: "object",
			Properties: map[string]*swagger.Schema{
				"data": {
					Type:  "array",
					Items: &swagger.Schema{Ref: "#/components/schemas/UserResponseDto"},
				},
			},
		}).
		WithResponse("401", "Unauthorized", &swagger.Schema{
			Ref: "#/components/schemas/ErrorResponseDto",
		}).
		WithSecurity("bearer")

	builder.AddPath("/users", "GET", listUsers)

	// GET /users/:id - Get user by ID
	getUser := swagger.NewOperation(
		"Get user by ID",
		"Retrieve a specific user by their ID",
	).
		WithTag("users").
		WithParameter("id", "path", "User ID", true, &swagger.Schema{Type: "integer"}).
		WithResponse("200", "Successful response", &swagger.Schema{
			Ref: "#/components/schemas/UserResponseDto",
		}).
		WithResponse("404", "User not found", &swagger.Schema{
			Ref: "#/components/schemas/ErrorResponseDto",
		}).
		WithSecurity("bearer")

	builder.AddPath("/users/{id}", "GET", getUser)

	// POST /users - Create user
	createUser := swagger.NewOperation(
		"Create a new user",
		"Create a new user with the provided information",
	).
		WithTag("users").
		WithRequestBody("User data", true, &swagger.Schema{
			Ref: "#/components/schemas/CreateUserDto",
		}).
		WithResponse("201", "User created successfully", &swagger.Schema{
			Ref: "#/components/schemas/UserResponseDto",
		}).
		WithResponse("400", "Invalid request", &swagger.Schema{
			Ref: "#/components/schemas/ErrorResponseDto",
		}).
		WithSecurity("bearer")

	builder.AddPath("/users", "POST", createUser)

	// PUT /users/:id - Update user
	updateUser := swagger.NewOperation(
		"Update user",
		"Update an existing user's information",
	).
		WithTag("users").
		WithParameter("id", "path", "User ID", true, &swagger.Schema{Type: "integer"}).
		WithRequestBody("Updated user data", true, &swagger.Schema{
			Ref: "#/components/schemas/UpdateUserDto",
		}).
		WithResponse("200", "User updated successfully", &swagger.Schema{
			Ref: "#/components/schemas/UserResponseDto",
		}).
		WithResponse("404", "User not found", &swagger.Schema{
			Ref: "#/components/schemas/ErrorResponseDto",
		}).
		WithSecurity("bearer")

	builder.AddPath("/users/{id}", "PUT", updateUser)

	// DELETE /users/:id - Delete user
	deleteUser := swagger.NewOperation(
		"Delete user",
		"Delete a user from the system",
	).
		WithTag("users").
		WithParameter("id", "path", "User ID", true, &swagger.Schema{Type: "integer"}).
		WithResponse("204", "User deleted successfully", nil).
		WithResponse("404", "User not found", &swagger.Schema{
			Ref: "#/components/schemas/ErrorResponseDto",
		}).
		WithSecurity("bearer")

	builder.AddPath("/users/{id}", "DELETE", deleteUser)
}

func addProductPaths(builder *swagger.DocumentBuilder) {
	// GET /products - List products
	listProducts := swagger.NewOperation(
		"List all products",
		"Retrieve a list of all products",
	).
		WithTag("products").
		WithParameter("category", "query", "Filter by category", false, &swagger.Schema{Type: "string"}).
		WithResponse("200", "Successful response", &swagger.Schema{
			Type: "object",
			Properties: map[string]*swagger.Schema{
				"data": {Type: "array", Items: &swagger.Schema{Type: "object"}},
			},
		})

	builder.AddPath("/products", "GET", listProducts)
}

// ========================================
// Main
// ========================================

func main() {
	fmt.Println("========================================")
	fmt.Println("GoNest Swagger/OpenAPI Example")
	fmt.Println("========================================")
	fmt.Println()

	// Build OpenAPI document
	doc := BuildSwaggerDocument()

	// Display Info
	fmt.Println("API Information:")
	fmt.Printf("  Title: %s\n", doc.Info.Title)
	fmt.Printf("  Version: %s\n", doc.Info.Version)
	fmt.Printf("  Description: %s\n", doc.Info.Description)
	fmt.Println()

	// Display Servers
	fmt.Println("Servers:")
	for _, server := range doc.Servers {
		fmt.Printf("  - %s (%s)\n", server.URL, server.Description)
	}
	fmt.Println()

	// Display Tags
	fmt.Println("Tags:")
	for _, tag := range doc.Tags {
		fmt.Printf("  - %s: %s\n", tag.Name, tag.Description)
	}
	fmt.Println()

	// Display Security Schemes
	fmt.Println("Security Schemes:")
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		for name, scheme := range doc.Components.SecuritySchemes {
			fmt.Printf("  - %s (%s)\n", name, scheme.Type)
		}
	}
	fmt.Println()

	// Display Paths
	fmt.Println("Endpoints:")
	for path, pathItem := range doc.Paths {
		if pathItem.Get != nil {
			fmt.Printf("  GET    %s - %s\n", path, pathItem.Get.Summary)
		}
		if pathItem.Post != nil {
			fmt.Printf("  POST   %s - %s\n", path, pathItem.Post.Summary)
		}
		if pathItem.Put != nil {
			fmt.Printf("  PUT    %s - %s\n", path, pathItem.Put.Summary)
		}
		if pathItem.Delete != nil {
			fmt.Printf("  DELETE %s - %s\n", path, pathItem.Delete.Summary)
		}
	}
	fmt.Println()

	// Display Schemas
	fmt.Println("Schemas:")
	if doc.Components != nil && doc.Components.Schemas != nil {
		for name := range doc.Components.Schemas {
			fmt.Printf("  - %s\n", name)
		}
	}
	fmt.Println()

	// Generate JSON
	fmt.Println("========================================")
	fmt.Println("JSON Output (preview):")
	fmt.Println("========================================")

	jsonData, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print first 50 lines
	lines := 0
	for i, b := range jsonData {
		if b == '\n' {
			lines++
			if lines >= 30 {
				fmt.Println("... (truncated)")
				break
			}
		}
		if lines < 30 {
			fmt.Print(string(b))
		}
		_ = i
	}

	fmt.Println("\n========================================")
	fmt.Println("Summary:")
	fmt.Println("========================================")
	fmt.Println("✓ OpenAPI 3.0.3 specification")
	fmt.Println("✓ Type-safe descriptors (NO TAGS!)")
	fmt.Println("✓ Pointer-based field selection")
	fmt.Println("✓ Complete CRUD documentation")
	fmt.Println("✓ Security schemes (Bearer, API Key)")
	fmt.Println("✓ Request/Response schemas")
	fmt.Println("✓ Parameter documentation")
	fmt.Println("✓ Multiple servers")
	fmt.Println("✓ Tags for organization")
	fmt.Println("✓ Ready for Swagger UI")
	fmt.Println()
	fmt.Println("Descriptor Pattern Benefits:")
	fmt.Println("✓ Compile-time type checking")
	fmt.Println("✓ IDE autocomplete support")
	fmt.Println("✓ Refactoring safe")
	fmt.Println("✓ No magic strings")
	fmt.Println("✓ Consistent with validator module")
	fmt.Println("========================================")
}
