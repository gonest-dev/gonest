// gonest/platform/echo.go
package platform

import (
	"github.com/gonest-dev/gonest/core"
	"github.com/labstack/echo/v4"
)

// EchoAdapter adapts GoNest to Echo framework
type EchoAdapter struct {
	config *AdapterConfig
}

// NewEchoAdapter creates an Echo adapter
func NewEchoAdapter(config ...*AdapterConfig) *EchoAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return &EchoAdapter{
		config: cfg,
	}
}

// Name returns adapter name
func (a *EchoAdapter) Name() string {
	return "echo"
}

// WrapHandler wraps GoNest handler for Echo
func (a *EchoAdapter) WrapHandler(handler core.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Echo exposes Request() and Response().Writer
		request := c.Request()
		response := c.Response().Writer

		// Create GoNest context with full access
		ctx := core.NewContext(response, request)
		ctx.Set("adapter", "echo")
		ctx.Set("echo_context", c) // Store for Echo-specific features

		return handler(ctx)
	}
}

// WrapMiddleware wraps GoNest middleware for Echo
func (a *EchoAdapter) WrapMiddleware(middleware core.MiddlewareFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			response := c.Response().Writer

			ctx := core.NewContext(response, request)
			ctx.Set("adapter", "echo")
			ctx.Set("echo_context", c)

			wrapped := middleware(func(_ *core.Context) error { return next(c) })
			return wrapped(ctx)
		}
	}
}

// ExtractContext extracts context from echo.Context
func (a *EchoAdapter) ExtractContext(platformCtx any) *core.Context {
	c, ok := platformCtx.(echo.Context)
	if !ok {
		return &core.Context{}
	}

	return core.NewContext(c.Response().Writer, c.Request())
}

// CreateContext creates GoNest context from echo.Context
func (a *EchoAdapter) CreateContext(c echo.Context) *core.Context {
	ctx := core.NewContext(c.Response().Writer, c.Request())
	ctx.Set("adapter", "echo")
	ctx.Set("echo_context", c)
	return ctx
}

// ToEchoHandler converts GoNest handler to Echo handler
// Usage: e.GET("/path", adapters.ToEchoHandler(gonestHandler))
func ToEchoHandler(handler core.HandlerFunc) echo.HandlerFunc {
	adapter := NewEchoAdapter()
	return adapter.WrapHandler(handler)
}

// ToEchoMiddleware converts GoNest middleware to Echo middleware
func ToEchoMiddleware(middleware core.MiddlewareFunc) echo.MiddlewareFunc {
	adapter := NewEchoAdapter()
	return adapter.WrapMiddleware(middleware)
}
