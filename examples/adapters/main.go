package main

import (
	"fmt"
	"net/http"

	"github.com/leandroluk/gonest/adapters"
	"github.com/leandroluk/gonest/core"
)

// ========================================
// Sample GoNest Handlers & Middleware
// ========================================

func HelloHandler(ctx *core.Context) error {
	return ctx.JSON(200, map[string]any{
		"message": "Hello from GoNest!",
		"adapter": ctx.Get("adapter"),
	})
}

func UserHandler(ctx *core.Context) error {
	id := ctx.Param("id")

	return ctx.JSON(200, map[string]any{
		"id":   id,
		"name": "User " + id,
	})
}

func LoggingMiddleware() core.MiddlewareFunc {
	return func(next core.HandlerFunc) core.HandlerFunc {
		return func(ctx *core.Context) error {
			fmt.Printf("[LOG] %s %s\n", ctx.Get("method"), ctx.Get("path"))
			return next(ctx)
		}
	}
}

// ========================================
// Adapter Examples
// ========================================

func StandardHTTPExample() {
	fmt.Println("========================================")
	fmt.Println("1. Standard net/http Adapter")
	fmt.Println("========================================")

	mux := http.NewServeMux()

	// Convert GoNest handlers to http.HandlerFunc
	mux.HandleFunc("/hello", adapters.ToMuxHandlerFunc(HelloHandler))
	mux.HandleFunc("/users/", adapters.ToMuxHandlerFunc(UserHandler))

	fmt.Println("Routes:")
	fmt.Println("  GET /hello")
	fmt.Println("  GET /users/:id")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  http.ListenAndServe(\":3000\", mux)")
	fmt.Println()
}

func GinExample() {
	fmt.Println("========================================")
	fmt.Println("2. Gin Framework Adapter")
	fmt.Println("========================================")

	fmt.Println("Code:")
	fmt.Println("  router := gin.Default()")
	fmt.Println("  router.GET(\"/hello\", adapters.ToGinHandler(HelloHandler))")
	fmt.Println("  router.GET(\"/users/:id\", adapters.ToGinHandler(UserHandler))")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  ✓ Automatic context conversion")
	fmt.Println("  ✓ Middleware support")
	fmt.Println("  ✓ Error handling")
	fmt.Println()
}

func FiberExample() {
	fmt.Println("========================================")
	fmt.Println("3. Fiber Framework Adapter")
	fmt.Println("========================================")

	fmt.Println("Code:")
	fmt.Println("  app := fiber.New()")
	fmt.Println("  app.Get(\"/hello\", adapters.ToFiberHandler(HelloHandler))")
	fmt.Println("  app.Get(\"/users/:id\", adapters.ToFiberHandler(UserHandler))")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  ✓ High performance")
	fmt.Println("  ✓ Express-like API")
	fmt.Println("  ✓ Native error handling")
	fmt.Println()
}

func EchoExample() {
	fmt.Println("========================================")
	fmt.Println("4. Echo Framework Adapter")
	fmt.Println("========================================")

	fmt.Println("Code:")
	fmt.Println("  e := echo.New()")
	fmt.Println("  e.GET(\"/hello\", adapters.ToEchoHandler(HelloHandler))")
	fmt.Println("  e.GET(\"/users/:id\", adapters.ToEchoHandler(UserHandler))")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  ✓ Minimalist design")
	fmt.Println("  ✓ Extensible middleware")
	fmt.Println("  ✓ Built-in utilities")
	fmt.Println()
}

func ChiExample() {
	fmt.Println("========================================")
	fmt.Println("5. Chi Router Adapter")
	fmt.Println("========================================")

	fmt.Println("Code:")
	fmt.Println("  r := chi.NewRouter()")
	fmt.Println("  r.Get(\"/hello\", adapters.ToChiHandler(HelloHandler))")
	fmt.Println("  r.Get(\"/users/{id}\", adapters.ToChiHandler(UserHandler))")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  ✓ Standard net/http compatible")
	fmt.Println("  ✓ Composable middleware")
	fmt.Println("  ✓ Context-based routing")
	fmt.Println()
}

// ========================================
// Advanced Features
// ========================================

func AdvancedFeaturesExample() {
	fmt.Println("========================================")
	fmt.Println("Advanced Features with Adapters")
	fmt.Println("========================================")
	fmt.Println()

	fmt.Println("1. Guards with Adapters:")
	fmt.Println("   authGuard := guards.SimpleAuthGuard(\"token\")")
	fmt.Println("   handler := guards.ApplyGuards(HelloHandler, authGuard)")
	fmt.Println("   router.GET(\"/protected\", adapters.ToGinHandler(handler))")
	fmt.Println()

	fmt.Println("2. Interceptors with Adapters:")
	fmt.Println("   logging := interceptors.SimpleLoggingInterceptor()")
	fmt.Println("   handler := interceptors.ApplyInterceptors(HelloHandler, logging)")
	fmt.Println("   app.Get(\"/logged\", adapters.ToFiberHandler(handler))")
	fmt.Println()

	fmt.Println("3. Middleware with Adapters:")
	fmt.Println("   // Standard")
	fmt.Println("   mux.Use(adapters.ToChiMiddleware(LoggingMiddleware()))")
	fmt.Println()
	fmt.Println("   // Gin")
	fmt.Println("   router.Use(adapters.ToGinMiddleware(LoggingMiddleware()))")
	fmt.Println()

	fmt.Println("4. Complete Stack:")
	fmt.Println("   handler := HelloHandler")
	fmt.Println("   handler = guards.ApplyGuards(handler, authGuard)")
	fmt.Println("   handler = interceptors.ApplyInterceptors(handler, logging)")
	fmt.Println("   router.GET(\"/full\", adapters.ToGinHandler(handler))")
	fmt.Println()
}

// ========================================
// Main
// ========================================

func main() {
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║   GoNest Platform Adapters Example    ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	StandardHTTPExample()
	GinExample()
	FiberExample()
	EchoExample()
	ChiExample()
	AdvancedFeaturesExample()

	fmt.Println("========================================")
	fmt.Println("Summary:")
	fmt.Println("========================================")
	fmt.Println("✓ 5 Platform Adapters")
	fmt.Println("  • Standard net/http")
	fmt.Println("  • Gin")
	fmt.Println("  • Fiber")
	fmt.Println("  • Echo")
	fmt.Println("  • Chi")
	fmt.Println()
	fmt.Println("✓ Unified GoNest API")
	fmt.Println("  • Write once, run anywhere")
	fmt.Println("  • Same handlers across platforms")
	fmt.Println("  • Full GoNest features")
	fmt.Println()
	fmt.Println("✓ Full Feature Support")
	fmt.Println("  • Guards")
	fmt.Println("  • Interceptors")
	fmt.Println("  • Middleware")
	fmt.Println("  • Exception filters")
	fmt.Println("========================================")
}
