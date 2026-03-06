// gonest/platform/gin.go
package platform

import (
	"github.com/gin-gonic/gin"
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
func (a *GinAdapter) WrapHandler(handler core.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create GoNest context with full access to Request/Response
		ctx := core.NewContext(c.Writer, c.Request)
		ctx.Set("adapter", "gin")
		ctx.Set("gin_context", c) // Store for Gin-specific features

		if err := handler(ctx); err != nil {
			a.handleError(c, err)
		}
	}
}

// WrapMiddleware wraps GoNest middleware for Gin
func (a *GinAdapter) WrapMiddleware(middleware core.MiddlewareFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := core.NewContext(c.Writer, c.Request)
		ctx.Set("adapter", "gin")
		ctx.Set("gin_context", c)

		wrapped := middleware(func(_ *core.Context) error {
			c.Next()
			return nil
		})

		if err := wrapped(ctx); err != nil {
			a.handleError(c, err)
			c.Abort()
		}
	}
}

// ExtractContext extracts context from gin.Context
func (a *GinAdapter) ExtractContext(platformCtx any) *core.Context {
	c, ok := platformCtx.(*gin.Context)
	if !ok {
		return &core.Context{}
	}

	return core.NewContext(c.Writer, c.Request)
}

// CreateContext creates GoNest context from gin.Context
func (a *GinAdapter) CreateContext(c *gin.Context) *core.Context {
	ctx := core.NewContext(c.Writer, c.Request)
	ctx.Set("adapter", "gin")
	ctx.Set("gin_context", c)
	return ctx
}

// handleError handles errors using Gin's JSON response
func (a *GinAdapter) handleError(c *gin.Context, err error) {
	if a.config.ErrorHandler != nil {
		_ = a.config.ErrorHandler(err, c)
		return
	}

	// Default error handling using Gin
	c.JSON(500, gin.H{
		"error": err.Error(),
	})
}

// ToGinHandler converts GoNest handler to Gin handler
// Usage: router.GET("/path", adapters.ToGinHandler(gonestHandler))
func ToGinHandler(handler core.HandlerFunc) gin.HandlerFunc {
	adapter := NewGinAdapter()
	return adapter.WrapHandler(handler)
}

// ToGinMiddleware converts GoNest middleware to Gin middleware
func ToGinMiddleware(middleware core.MiddlewareFunc) gin.HandlerFunc {
	adapter := NewGinAdapter()
	return adapter.WrapMiddleware(middleware)
}
