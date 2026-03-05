package main

import (
	"context"
	"log"

	"github.com/leandroluk/gonest/core"
)

// ========================================
// Interfaces (contracts)
// ========================================

// IAppService defines the app service contract
type IAppService interface {
	GetHello() string
	GetVersion() string
}

// ========================================
// Implementations
// ========================================

// AppService implements IAppService
type AppService struct {
	version string
}

// Compile-time interface compliance check
var _ IAppService = (*AppService)(nil)
var _ core.OnModuleInit = (*AppService)(nil)
var _ core.OnApplicationBootstrap = (*AppService)(nil)

func (s *AppService) GetHello() string {
	return "Hello from GoNest!"
}

func (s *AppService) GetVersion() string {
	return s.version
}

// Lifecycle hooks
func (s *AppService) OnModuleInit(ctx context.Context) error {
	log.Println("✅ AppService initialized")
	s.version = "1.0.0"
	return nil
}

func (s *AppService) OnApplicationBootstrap(ctx context.Context) error {
	log.Println("🚀 AppService bootstrapped")
	return nil
}

// ========================================
// Controllers
// ========================================

// AppController handles HTTP requests
type AppController struct {
	appService IAppService
}

// Compile-time interface compliance checks
var _ core.Controller = (*AppController)(nil)
var _ core.OnModuleInit = (*AppController)(nil)

// Routes implements Controller interface
func (c *AppController) Routes() []core.RouteDefinition {
	return []core.RouteDefinition{
		{
			Method:  "GET",
			Path:    "/",
			Handler: c.GetHello,
		},
		{
			Method:  "GET",
			Path:    "/health",
			Handler: c.GetHealth,
		},
		{
			Method:  "GET",
			Path:    "/user/:id",
			Handler: c.GetUser,
		},
		{
			Method:  "POST",
			Path:    "/user",
			Handler: c.CreateUser,
		},
	}
}

func (c *AppController) GetHello(ctx *core.Context) error {
	message := c.appService.GetHello()
	version := c.appService.GetVersion()

	return ctx.JSON(200, map[string]any{
		"message": message,
		"version": version,
	})
}

func (c *AppController) GetHealth(ctx *core.Context) error {
	return ctx.JSON(200, map[string]any{
		"status":  "ok",
		"version": c.appService.GetVersion(),
	})
}

func (c *AppController) GetUser(ctx *core.Context) error {
	id := ctx.Param("id")

	return ctx.JSON(200, map[string]any{
		"id":   id,
		"name": "User " + id,
		"type": "example",
	})
}

// DTOs
type CreateUserDto struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (c *AppController) CreateUser(ctx *core.Context) error {
	var dto CreateUserDto
	if err := ctx.BindJSON(&dto); err != nil {
		return ctx.JSON(400, map[string]any{
			"error": "Invalid request body",
		})
	}

	return ctx.JSON(201, map[string]any{
		"id":    "123",
		"name":  dto.Name,
		"email": dto.Email,
	})
}

// Lifecycle hook
func (c *AppController) OnModuleInit(ctx context.Context) error {
	log.Println("✅ AppController initialized")
	return nil
}

// ========================================
// Module
// ========================================

// AppModule is the root module
type AppModule struct{}

// Compile-time interface compliance check
var _ core.Module = (*AppModule)(nil)

func (m *AppModule) Configure(b *core.ModuleBuilder) {
	b.Controllers(&AppController{appService: &AppService{}}).Providers(&AppService{})
}

// ========================================
// Application Bootstrap
// ========================================

func main() {
	log.Println("🚀 Starting GoNest application...")

	// Create application
	app := core.NestFactory{}.Create(
		&AppModule{},
		core.WithShutdownTimeout(10),
	)

	log.Println("📦 Application created successfully")
	log.Println("🌐 Server starting on http://localhost:3000")
	log.Println("")
	log.Println("Available endpoints:")
	log.Println("  GET  http://localhost:3000/")
	log.Println("  GET  http://localhost:3000/health")
	log.Println("  GET  http://localhost:3000/user/:id")
	log.Println("  POST http://localhost:3000/user")
	log.Println("")

	// Start server
	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("❌ Error: %v\n", err)
	}
}
