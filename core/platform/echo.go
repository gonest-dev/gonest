// gonest/platform/echo.go
package platform

import (
	"net/http"

	"github.com/gonest-dev/gonest/core/common"
	"github.com/labstack/echo/v4"
)

// EchoAdapter implements PlatformAdapter for Echo framework
type EchoAdapter struct {
	config *AdapterConfig
	echo   *echo.Echo
}

// NewEchoAdapter creates an Echo adapter
func NewEchoAdapter(config ...*AdapterConfig) *EchoAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	// Create Echo instance
	e := echo.New()

	// Disable Echo's default logger
	e.HideBanner = true
	e.HidePort = true

	return &EchoAdapter{
		config: cfg,
		echo:   e,
	}
}

// Name returns the adapter name
func (a *EchoAdapter) Name() string {
	return "echo"
}

// RegisterRoute registers a route with the platform
func (a *EchoAdapter) RegisterRoute(route common.RouteDefinition) error {
	// Convert GoNest handler to Echo handler
	handler := a.wrapHandler(route)

	// Register with Echo
	a.echo.Add(route.Method, route.Path, handler)

	return nil
}

// Handler returns the http.Handler for the platform
func (a *EchoAdapter) Handler() http.Handler {
	return a.echo
}

// Use registers global middleware
func (a *EchoAdapter) Use(middleware common.MiddlewareFunc) {
	// Convert GoNest middleware to Echo middleware
	echoMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Create GoNest context
			ctx := common.NewContext(c.Response().Writer, c.Request())
			ctx.Set("adapter", "echo")
			ctx.Set("echo_context", c)

			// Wrap next handler
			wrapped := middleware(func(_ *common.Context) error {
				return next(c)
			})

			// Execute
			return wrapped(ctx)
		}
	}

	a.echo.Use(echoMiddleware)
}

// wrapHandler wraps a GoNest route to Echo handler
func (a *EchoAdapter) wrapHandler(route common.RouteDefinition) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Create GoNest context
		ctx := common.NewContext(c.Response().Writer, c.Request())
		ctx.Set("adapter", "echo")
		ctx.Set("echo_context", c)

		// Apply route middlewares
		handler := route.Handler
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		// Execute handler
		return handler(ctx)
	}
}

// GetEcho returns the underlying Echo instance for advanced configuration
func (a *EchoAdapter) GetEcho() *echo.Echo {
	return a.echo
}

// Compile-time check
var _ PlatformAdapter = (*EchoAdapter)(nil)


