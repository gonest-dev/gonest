package adapters

import (
	"github.com/leandroluk/gonest/core"
)

// FiberAdapter adapts GoNest to Fiber framework
type FiberAdapter struct {
	config *AdapterConfig
}

// NewFiberAdapter creates a Fiber adapter
func NewFiberAdapter(config ...*AdapterConfig) *FiberAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return &FiberAdapter{
		config: cfg,
	}
}

// Name returns adapter name
func (a *FiberAdapter) Name() string {
	return "fiber"
}

// WrapHandler wraps GoNest handler for Fiber
// Returns: fiber.Handler signature: func(*fiber.Ctx) error
func (a *FiberAdapter) WrapHandler(handler core.HandlerFunc) any {
	return func(fiberCtx any) error {
		ctx := a.CreateContext(fiberCtx)
		return handler(ctx)
	}
}

// WrapMiddleware wraps GoNest middleware for Fiber
func (a *FiberAdapter) WrapMiddleware(middleware core.MiddlewareFunc) any {
	return func(fiberCtx any) error {
		ctx := a.CreateContext(fiberCtx)

		wrapped := middleware(func(c *core.Context) error {
			// Continue to next handler
			return nil
		})

		return wrapped(ctx)
	}
}

// ExtractContext extracts context from fiber.Ctx
func (a *FiberAdapter) ExtractContext(platformCtx any) *core.Context {
	return a.CreateContext(platformCtx)
}

// CreateContext creates GoNest context from fiber.Ctx
// Note: This is a simplified implementation
// In production, would extract Request/ResponseWriter from fiber.Ctx
func (a *FiberAdapter) CreateContext(fiberCtx any) *core.Context {
	// Create empty context (Fiber handles request/response internally)
	ctx := &core.Context{}
	ctx.Set("fiber_context", fiberCtx)
	ctx.Set("adapter", "fiber")

	return ctx
}

// ToFiberHandler converts GoNest handler to Fiber handler
// Usage: app.Get("/path", adapters.ToFiberHandler(gonestHandler))
func ToFiberHandler(handler core.HandlerFunc) any {
	adapter := NewFiberAdapter()
	return adapter.WrapHandler(handler)
}

// ToFiberMiddleware converts GoNest middleware to Fiber middleware
func ToFiberMiddleware(middleware core.MiddlewareFunc) any {
	adapter := NewFiberAdapter()
	return adapter.WrapMiddleware(middleware)
}
