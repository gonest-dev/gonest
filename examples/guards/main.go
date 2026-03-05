package main

import (
	"fmt"
	"time"

	"github.com/leandroluk/gonest/controller"
	"github.com/leandroluk/gonest/core"
	"github.com/leandroluk/gonest/guards"
)

// ========================================
// Mock Auth System
// ========================================

var validTokens = map[string]map[string]any{
	"token-admin": {
		"userId": "1",
		"roles":  []string{"admin", "user"},
	},
	"token-user": {
		"userId": "2",
		"roles":  []string{"user"},
	},
	"token-guest": {
		"userId": "3",
		"roles":  []string{"guest"},
	},
}

func validateToken(token string) (bool, error) {
	_, exists := validTokens[token]
	return exists, nil
}

func extractUserFromToken(token string) map[string]any {
	return validTokens[token]
}

// ========================================
// Auth Middleware
// ========================================

func AuthMiddleware() core.MiddlewareFunc {
	return func(next core.HandlerFunc) core.HandlerFunc {
		return func(ctx *core.Context) error {
			// Extract token from auth guard (already validated)
			token, _ := ctx.Get("auth:token").(string)

			if token != "" {
				// Get user data
				userData := extractUserFromToken(token)
				if userData != nil {
					ctx.Set("user:id", userData["userId"])
					ctx.Set("user:roles", userData["roles"])
				}
			}

			return next(ctx)
		}
	}
}

// ========================================
// Public Controller (No Guards)
// ========================================

func NewPublicController() controller.Controller {
	ctrl := controller.NewController(
		controller.WithPrefix("/public"),
	)

	// GET /public/health - No authentication required
	ctrl.Get("/health", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"status": "ok",
			"time":   time.Now().Unix(),
		})
	})

	// GET /public/info
	ctrl.Get("/info", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"name":    "GoNest API",
			"version": "1.0.0",
		})
	})

	return ctrl
}

// ========================================
// Protected Controller (With Guards)
// ========================================

func NewProtectedController() controller.Controller {
	// Create guards
	authGuard := guards.NewAuthGuard(&guards.AuthGuardOptions{
		TokenValidator: validateToken,
	})

	ctrl := controller.NewController(
		controller.WithPrefix("/protected").
			WithMiddleware(
				guards.UseGuards(authGuard),
				AuthMiddleware(),
			),
	)

	// GET /protected/profile - Requires authentication
	ctrl.Get("/profile", func(ctx *core.Context) error {
		userId := ctx.Get("user:id")
		roles := ctx.Get("user:roles")

		return ctx.JSON(200, map[string]any{
			"userId": userId,
			"roles":  roles,
		})
	})

	// GET /protected/data
	ctrl.Get("/data", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "This is protected data",
			"data":    []string{"item1", "item2", "item3"},
		})
	})

	return ctrl
}

// ========================================
// Admin Controller (Roles Guard)
// ========================================

func NewAdminController() controller.Controller {
	authGuard := guards.NewAuthGuard(&guards.AuthGuardOptions{
		TokenValidator: validateToken,
	})

	rolesGuard := guards.RequireRoles("admin")

	ctrl := controller.NewController(
		controller.WithPrefix("/admin").
			WithMiddleware(
				guards.UseGuards(authGuard),
				AuthMiddleware(),
				guards.UseGuards(rolesGuard),
			),
	)

	// GET /admin/users - Requires admin role
	ctrl.Get("/users", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"users": []map[string]any{
				{"id": 1, "name": "User 1"},
				{"id": 2, "name": "User 2"},
			},
		})
	})

	// DELETE /admin/users/:id - Requires admin role
	ctrl.Delete("/users/:id", func(ctx *core.Context) error {
		id := ctx.Param("id")

		return ctx.JSON(200, map[string]any{
			"message": fmt.Sprintf("User %s deleted", id),
		})
	})

	return ctrl
}

// ========================================
// Rate Limited Controller (Throttler)
// ========================================

func NewRateLimitedController() controller.Controller {
	// 5 requests per minute
	throttler := guards.SimpleThrottler(5, time.Minute)

	ctrl := controller.NewController(
		controller.WithPrefix("/api").
			WithMiddleware(guards.UseGuards(throttler)),
	)

	// GET /api/data - Rate limited
	ctrl.Get("/data", func(ctx *core.Context) error {
		return ctx.JSON(200, map[string]any{
			"message": "Rate limited endpoint",
			"data":    time.Now().Unix(),
		})
	})

	return ctrl
}

// ========================================
// Main
// ========================================

func main() {
	fmt.Println("========================================")
	fmt.Println("GoNest Guards & Security Example")
	fmt.Println("========================================")
	fmt.Println()

	// Create controllers
	publicCtrl := NewPublicController()
	protectedCtrl := NewProtectedController()
	adminCtrl := NewAdminController()
	rateLimitedCtrl := NewRateLimitedController()

	// Display routes
	fmt.Println("Public Routes (No Guards):")
	for _, route := range publicCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Protected Routes (AuthGuard):")
	for _, route := range protectedCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Admin Routes (AuthGuard + RolesGuard):")
	for _, route := range adminCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	fmt.Println("Rate Limited Routes (ThrottlerGuard):")
	for _, route := range rateLimitedCtrl.GetRoutes() {
		fmt.Printf("  %s %s\n", route.Method, route.Path)
	}
	fmt.Println()

	// Demonstrate guards
	fmt.Println("========================================")
	fmt.Println("Guard Examples:")
	fmt.Println("========================================")
	fmt.Println()

	// Example 1: AuthGuard
	fmt.Println("1. AuthGuard:")
	fmt.Println("   Valid tokens:")
	fmt.Println("   - token-admin (roles: admin, user)")
	fmt.Println("   - token-user (roles: user)")
	fmt.Println("   - token-guest (roles: guest)")
	fmt.Println()

	// Example 2: RolesGuard
	fmt.Println("2. RolesGuard:")
	fmt.Println("   Admin endpoints require 'admin' role")
	fmt.Println("   Only token-admin can access")
	fmt.Println()

	// Example 3: ThrottlerGuard
	fmt.Println("3. ThrottlerGuard:")
	fmt.Println("   Limit: 5 requests per minute")
	fmt.Println("   Returns 429 (Too Many Requests) when exceeded")
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("Summary:")
	fmt.Println("✓ AuthGuard - JWT/Bearer token validation")
	fmt.Println("✓ RolesGuard - Role-based access control")
	fmt.Println("✓ ThrottlerGuard - Rate limiting")
	fmt.Println("✓ Composable guards")
	fmt.Println("✓ Custom error responses")
	fmt.Println("========================================")
}
