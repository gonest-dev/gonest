package adapters

import (
	"github.com/leandroluk/gonest/core"
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
// Returns: echo.HandlerFunc signature: func(echo.Context) error
func (a *EchoAdapter) WrapHandler(handler core.HandlerFunc) any {
	return func(echoCtx any) error {
		ctx := a.CreateContext(echoCtx)
		return handler(ctx)
	}
}

// WrapMiddleware wraps GoNest middleware for Echo
func (a *EchoAdapter) WrapMiddleware(middleware core.MiddlewareFunc) any {
	return func(next any) any {
		return func(echoCtx any) error {
			ctx := a.CreateContext(echoCtx)

			wrapped := middleware(func(c *core.Context) error {
				// Continue to next handler
				// In real implementation: return next.(echo.HandlerFunc)(echoCtx)
				return nil
			})

			return wrapped(ctx)
		}
	}
}

// ExtractContext extracts context from echo.Context
func (a *EchoAdapter) ExtractContext(platformCtx any) *core.Context {
	return a.CreateContext(platformCtx)
}

// CreateContext creates GoNest context from echo.Context
// Note: This is a simplified implementation
// In production, would extract Request/ResponseWriter from echo.Context
func (a *EchoAdapter) CreateContext(echoCtx any) *core.Context {
	// Create empty context (Echo handles request/response internally)
	ctx := &core.Context{}
	ctx.Set("echo_context", echoCtx)
	ctx.Set("adapter", "echo")

	return ctx
}

// ToEchoHandler converts GoNest handler to Echo handler
// Usage: e.GET("/path", adapters.ToEchoHandler(gonestHandler))
func ToEchoHandler(handler core.HandlerFunc) any {
	adapter := NewEchoAdapter()
	return adapter.WrapHandler(handler)
}

// ToEchoMiddleware converts GoNest middleware to Echo middleware
func ToEchoMiddleware(middleware core.MiddlewareFunc) any {
	adapter := NewEchoAdapter()
	return adapter.WrapMiddleware(middleware)
}
