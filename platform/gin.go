// gonest/platform/gin.go
package platform

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gonest-dev/gonest/core"
)

// GinAdapter implements PlatformAdapter for Gin framework
type GinAdapter struct {
	config *AdapterConfig
	engine *gin.Engine
}

// NewGinAdapter creates a Gin adapter
func NewGinAdapter(config ...*AdapterConfig) *GinAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	// Create Gin engine
	engine := gin.New()

	// Disable Gin's default logger (we use our own)
	gin.SetMode(gin.ReleaseMode)

	return &GinAdapter{
		config: cfg,
		engine: engine,
	}
}

// Name returns the adapter name
func (a *GinAdapter) Name() string {
	return "gin"
}

// RegisterRoute registers a route with the platform
func (a *GinAdapter) RegisterRoute(route core.RouteDefinition) error {
	// Convert GoNest handler to Gin handler
	handler := a.wrapHandler(route)

	// Register with Gin
	a.engine.Handle(route.Method, route.Path, handler)

	return nil
}

// Handler returns the http.Handler for the platform
func (a *GinAdapter) Handler() http.Handler {
	return a.engine
}

// Use registers global middleware
func (a *GinAdapter) Use(middleware core.MiddlewareFunc) {
	// Convert GoNest middleware to Gin middleware
	ginMiddleware := func(c *gin.Context) {
		// Create GoNest context
		ctx := core.NewContext(c.Writer, c.Request)
		ctx.Set("adapter", "gin")
		ctx.Set("gin_context", c)

		// Wrap next handler
		wrapped := middleware(func(_ *core.Context) error {
			c.Next()
			return nil
		})

		// Execute
		if err := wrapped(ctx); err != nil {
			a.handleError(c, err)
			c.Abort()
		}
	}

	a.engine.Use(ginMiddleware)
}

// wrapHandler wraps a GoNest route to Gin handler
func (a *GinAdapter) wrapHandler(route core.RouteDefinition) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create GoNest context
		ctx := core.NewContext(c.Writer, c.Request)
		ctx.Set("adapter", "gin")
		ctx.Set("gin_context", c)

		// Apply route middlewares
		handler := route.Handler
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		// Execute handler
		if err := handler(ctx); err != nil {
			a.handleError(c, err)
		}
	}
}

// handleError handles errors using Gin's JSON response
func (a *GinAdapter) handleError(c *gin.Context, err error) {
	if a.config.ErrorHandler != nil {
		_ = a.config.ErrorHandler(err, c)
		return
	}

	// Default error handling
	c.JSON(500, gin.H{
		"error": err.Error(),
	})
}

// GetEngine returns the underlying Gin engine for advanced configuration
func (a *GinAdapter) GetEngine() *gin.Engine {
	return a.engine
}

// Compile-time check
var _ PlatformAdapter = (*GinAdapter)(nil)
