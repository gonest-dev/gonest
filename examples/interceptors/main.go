package main

import (
	"fmt"
	"time"

	"github.com/leandroluk/gonest/controller"
	"github.com/leandroluk/gonest/core"
	"github.com/leandroluk/gonest/interceptors"
)

// ========================================
// Controllers with Interceptors
// ========================================

func NewLoggingController() controller.Controller {
	loggingInterceptor := interceptors.SimpleLoggingInterceptor()

	ctrl := controller.NewController(
		controller.WithPrefix("/api").
			WithMiddleware(
				interceptors.UseInterceptors(loggingInterceptor),
			),
	)

	ctrl.Get("/users", func(ctx *core.Context) error {
		// Simulate some work
		time.Sleep(100 * time.Millisecond)

		return ctx.JSON(200, map[string]any{
			"users": []string{"user1", "user2"},
		})
	})

	return ctrl
}

func NewTimeoutController() controller.Controller {
	timeoutInterceptor := interceptors.NewTimeoutInterceptor(2 * time.Second)

	ctrl := controller.NewController(
		controller.WithPrefix("/timeout"),
	)

	// Fast endpoint
	ctrl.Get("/fast", func(ctx *core.Context) error {
		time.Sleep(500 * time.Millisecond)
		return ctx.JSON(200, map[string]any{"message": "fast"})
	}).Use(interceptors.UseInterceptors(timeoutInterceptor))

	// Slow endpoint (will timeout)
	ctrl.Get("/slow", func(ctx *core.Context) error {
		time.Sleep(3 * time.Second)
		return ctx.JSON(200, map[string]any{"message": "slow"})
	}).Use(interceptors.UseInterceptors(timeoutInterceptor))

	return ctrl
}

func NewCacheController() controller.Controller {
	cacheInterceptor := interceptors.SimpleCacheInterceptor(1 * time.Minute)

	ctrl := controller.NewController(
		controller.WithPrefix("/cache").
			WithMiddleware(
				interceptors.UseInterceptors(cacheInterceptor),
			),
	)

	ctrl.Get("/data", func(ctx *core.Context) error {
		// Check if cache hit
		if hit, _ := ctx.Get("cache:hit").(bool); hit {
			return ctx.JSON(200, map[string]any{
				"cached": true,
				"data":   "from cache",
			})
		}

		// Simulate expensive operation
		time.Sleep(500 * time.Millisecond)

		return ctx.JSON(200, map[string]any{
			"cached": false,
			"data":   "fresh data",
			"time":   time.Now().Unix(),
		})
	})

	return ctrl
}

func NewTransformController() controller.Controller {
	wrapInterceptor := interceptors.WrapResponse()
	metadataInterceptor := interceptors.AddMetadata(map[string]any{
		"version": "1.0.0",
		"server":  "gonest",
	})

	ctrl := controller.NewController(
		controller.WithPrefix("/transform").
			WithMiddleware(
				interceptors.UseInterceptors(wrapInterceptor, metadataInterceptor),
			),
	)

	ctrl.Get("/users", func(ctx *core.Context) error {
		users := []map[string]any{
			{"id": 1, "name": "User 1"},
			{"id": 2, "name": "User 2"},
		}

		// Response will be wrapped and have metadata added
		ctx.Set("response", users)

		return ctx.JSON(200, users)
	})

	return ctrl
}

func NewErrorController() controller.Controller {
	errorInterceptor := interceptors.SimpleErrorInterceptor()

	ctrl := controller.NewController(
		controller.WithPrefix("/error").
			WithMiddleware(
				interceptors.UseInterceptors(errorInterceptor),
			),
	)

	ctrl.Get("/fail", func(ctx *core.Context) error {
		// This error will be logged by interceptor
		return fmt.Errorf("intentional error for demonstration")
	})

	ctrl.Get("/success", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{"message": "success"})
	})

	return ctrl
}

// ========================================
// Per-Route Interceptors Examples
// ========================================

func NewPerRouteController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/routes"),
	)

	// Route 1: No interceptors
	ctrl.Get("/public", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "Public endpoint - no interceptors",
		})
	})

	// Route 2: Only cache THIS route
	ctrl.Get("/cached", func(ctx *core.Context) error {
		if hit, _ := ctx.Get("cache:hit").(bool); hit {
			return ctx.JSON(200, map[string]any{
				"message": "From cache",
				"cached":  true,
			})
		}

		time.Sleep(300 * time.Millisecond)
		return ctx.JSON(200, map[string]any{
			"message": "Fresh data",
			"cached":  false,
			"time":    time.Now().Unix(),
		})
	}).Use(interceptors.UseInterceptors(
		interceptors.SimpleCacheInterceptor(1 * time.Minute),
	))

	// Route 3: Only logging THIS route
	ctrl.Get("/logged", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "This route is logged",
		})
	}).Use(interceptors.UseInterceptors(
		interceptors.SimpleLoggingInterceptor(),
	))

	// Route 4: Multiple interceptors on THIS route only
	ctrl.Get("/multi", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "Multiple interceptors on this route",
			"time":    time.Now().Unix(),
		})
	}).Use(interceptors.UseInterceptors(
		interceptors.SimpleLoggingInterceptor(),
		interceptors.SimpleCacheInterceptor(30*time.Second),
		interceptors.WrapResponse(),
	))

	return ctrl
}

func NewHybridController() controller.Controller {
	// Controller-level: all routes get logging
	ctrl := controller.NewController(
		controller.WithPrefix("/hybrid").
			WithMiddleware(
				interceptors.UseInterceptors(
					interceptors.SimpleLoggingInterceptor(),
				),
			),
	)

	// Route 1: Only controller interceptor (logging)
	ctrl.Get("/basic", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "Has logging from controller",
		})
	})

	// Route 2: Controller + route interceptors (logging + cache)
	ctrl.Get("/cached", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "Has logging + cache",
			"time":    time.Now().Unix(),
		})
	}).Use(interceptors.UseInterceptors(
		interceptors.SimpleCacheInterceptor(1 * time.Minute),
	))

	// Route 3: Controller + multiple route interceptors
	ctrl.Get("/protected", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "Has logging + timeout + transform",
		})
	}).Use(interceptors.UseInterceptors(
		interceptors.NewTimeoutInterceptor(5*time.Second),
		interceptors.WrapResponse(),
	))

	return ctrl
}

// ========================================
// Main
// ========================================

func main() {
	fmt.Println("========================================")
	fmt.Println("GoNest Interceptors Example")
	fmt.Println("========================================\n")

	// Create controllers
	loggingCtrl := NewLoggingController()
	timeoutCtrl := NewTimeoutController()
	cacheCtrl := NewCacheController()
	transformCtrl := NewTransformController()
	errorCtrl := NewErrorController()
	perRouteCtrl := NewPerRouteController()
	hybridCtrl := NewHybridController()

	// Display routes
	fmt.Println("Logging Interceptor Routes:")
	for _, route := range loggingCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Timeout Interceptor Routes:")
	for _, route := range timeoutCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Cache Interceptor Routes:")
	for _, route := range cacheCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Transform Interceptor Routes:")
	for _, route := range transformCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Error Interceptor Routes:")
	for _, route := range errorCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Per-Route Interceptors:")
	for _, route := range perRouteCtrl.GetRoutes() {
		interceptorCount := len(route.Middlewares)
		fmt.Printf("  %s %s - %d interceptor(s)\n", route.Method, route.Path, interceptorCount)
	}
	fmt.Println()

	fmt.Println("Hybrid (Controller + Route) Interceptors:")
	for _, route := range hybridCtrl.GetRoutes() {
		interceptorCount := len(route.Middlewares)
		fmt.Printf("  %s %s - %d interceptor(s)\n", route.Method, route.Path, interceptorCount)
	}
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("Interceptor Examples:")
	fmt.Println("========================================\n")

	fmt.Println("1. LoggingInterceptor:")
	fmt.Println("   - Logs all requests and responses")
	fmt.Println("   - Measures request duration")
	fmt.Println("   - Logs errors")
	fmt.Println()

	fmt.Println("2. TimeoutInterceptor:")
	fmt.Println("   - Sets maximum request duration")
	fmt.Println("   - Returns timeout error if exceeded")
	fmt.Println("   - Prevents long-running requests")
	fmt.Println()

	fmt.Println("3. CacheInterceptor:")
	fmt.Println("   - Caches responses for TTL")
	fmt.Println("   - Reduces expensive operations")
	fmt.Println("   - Automatic cache cleanup")
	fmt.Println()

	fmt.Println("4. TransformInterceptor:")
	fmt.Println("   - Wraps responses in standard format")
	fmt.Println("   - Adds metadata (version, server)")
	fmt.Println("   - Serializes responses")
	fmt.Println()

	fmt.Println("5. ErrorInterceptor:")
	fmt.Println("   - Logs all errors")
	fmt.Println("   - Transforms error format")
	fmt.Println("   - Centralizes error handling")
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("Summary:")
	fmt.Println("✓ LoggingInterceptor - request/response logging")
	fmt.Println("✓ TimeoutInterceptor - request timeout")
	fmt.Println("✓ CacheInterceptor - response caching")
	fmt.Println("✓ TransformInterceptor - response transformation")
	fmt.Println("✓ ErrorInterceptor - error handling")
	fmt.Println("✓ Per-route interceptors - selective application")
	fmt.Println("✓ Controller + route - combined interceptors")
	fmt.Println("✓ Composable interceptors")
	fmt.Println("✓ Before/After handling")
	fmt.Println("========================================")
}
