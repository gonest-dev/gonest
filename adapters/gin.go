package adapters

import (
	"github.com/gonest-dev/gonest/core"
)

// GinAdapter adapts GoNest to Gin framework
type GinAdapter struct {
	config *AdapterConfig
}

// NewGinAdapter creates a Gin adapter
func NewGinAdapter(config ...*AdapterConfig) *GinAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return &GinAdapter{
		config: cfg,
	}
}

// Name returns adapter name
func (a *GinAdapter) Name() string {
	return "gin"
}

// WrapHandler wraps GoNest handler for Gin
// Returns: gin.HandlerFunc signature: func(*gin.Context)
func (a *GinAdapter) WrapHandler(handler core.HandlerFunc) any {
	return func(ginCtx any) {
		ctx := a.CreateContext(ginCtx)

		if err := handler(ctx); err != nil {
			a.handleError(ginCtx, err)
		}
	}
}

// WrapMiddleware wraps GoNest middleware for Gin
func (a *GinAdapter) WrapMiddleware(middleware core.MiddlewareFunc) any {
	return func(ginCtx any) {
		ctx := a.CreateContext(ginCtx)

		wrapped := middleware(func(c *core.Context) error {
			// Continue to next handler
			return nil
		})

		if err := wrapped(ctx); err != nil {
			a.handleError(ginCtx, err)
			// Abort Gin context
			if gc, ok := ginCtx.(interface{ Abort() }); ok {
				gc.Abort()
			}
		}
	}
}

// ExtractContext extracts context from gin.Context
func (a *GinAdapter) ExtractContext(platformCtx any) *core.Context {
	return a.CreateContext(platformCtx)
}

// CreateContext creates GoNest context from gin.Context
// Note: This is a simplified implementation
// In production, would extract Request/ResponseWriter from gin.Context
func (a *GinAdapter) CreateContext(ginCtx any) *core.Context {
	// Create empty context (Gin handles request/response internally)
	ctx := &core.Context{}
	ctx.Set("gin_context", ginCtx)
	ctx.Set("adapter", "gin")

	return ctx
}

// handleError handles errors in Gin context
func (a *GinAdapter) handleError(ginCtx any, err error) {
	if a.config.ErrorHandler != nil {
		_ = a.config.ErrorHandler(err, ginCtx)
		return
	}

	// Default error handling
	// In real implementation: ginCtx.(*gin.Context).JSON(500, gin.H{"error": err.Error()})
}

// ToGinHandler converts GoNest handler to Gin handler
// Usage: router.GET("/path", adapters.ToGinHandler(gonestHandler))
func ToGinHandler(handler core.HandlerFunc) any {
	adapter := NewGinAdapter()
	return adapter.WrapHandler(handler)
}

// ToGinMiddleware converts GoNest middleware to Gin middleware
func ToGinMiddleware(middleware core.MiddlewareFunc) any {
	adapter := NewGinAdapter()
	return adapter.WrapMiddleware(middleware)
}
