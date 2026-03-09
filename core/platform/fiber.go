// gonest/platform/fiber.go
package platform

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gonest-dev/gonest/core/common"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// FiberAdapter implements PlatformAdapter for Fiber framework
type FiberAdapter struct {
	config *AdapterConfig
	app    *fiber.App
}

// NewFiberAdapter creates a Fiber adapter
func NewFiberAdapter(config ...*AdapterConfig) *FiberAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	return &FiberAdapter{
		config: cfg,
		app:    app,
	}
}

// Name returns the adapter name
func (a *FiberAdapter) Name() string {
	return "fiber"
}

// RegisterRoute registers a route with the platform
func (a *FiberAdapter) RegisterRoute(route common.RouteDefinition) error {
	// Convert GoNest handler to Fiber handler
	handler := a.wrapHandler(route)

	// Register with Fiber
	a.app.Add(route.Method, route.Path, handler)

	return nil
}

// Handler returns the http.Handler for the platform
// Fiber uses fasthttp, so we need to adapt it to net/http
func (a *FiberAdapter) Handler() http.Handler {
	return adaptor.FiberApp(a.app)
}

// Use registers global middleware
func (a *FiberAdapter) Use(middleware common.MiddlewareFunc) {
	// Convert GoNest middleware to Fiber middleware
	fiberMiddleware := func(c *fiber.Ctx) error {
		var req *http.Request
		var w http.ResponseWriter

		// Convert fasthttp to net/http
		fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			req = r
			w = rw
		}))(c.Context())

		// Create GoNest context
		ctx := common.NewContext(w, req)
		ctx.Set("adapter", "fiber")
		ctx.Set("fiber_context", c)

		// Wrap next handler
		wrapped := middleware(func(_ *common.Context) error {
			return c.Next()
		})

		// Execute
		return wrapped(ctx)
	}

	a.app.Use(fiberMiddleware)
}

// wrapHandler wraps a GoNest route to Fiber handler
func (a *FiberAdapter) wrapHandler(route common.RouteDefinition) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req *http.Request
		var w http.ResponseWriter

		// Convert fasthttp to net/http
		fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			req = r
			w = rw
		}))(c.Context())

		// Create GoNest context
		ctx := common.NewContext(w, req)
		ctx.Set("adapter", "fiber")
		ctx.Set("fiber_context", c)

		// Apply route middlewares
		handler := route.Handler
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		// Execute handler
		return handler(ctx)
	}
}

// GetApp returns the underlying Fiber app for advanced configuration
func (a *FiberAdapter) GetApp() *fiber.App {
	return a.app
}

// Compile-time check
var _ PlatformAdapter = (*FiberAdapter)(nil)


