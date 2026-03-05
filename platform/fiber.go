package platform

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gonest-dev/gonest/core"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// FiberAdapter adapts GoNest to Fiber framework
// Note: Fiber uses fasthttp instead of net/http, so we need adapters
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
func (a *FiberAdapter) WrapHandler(handler core.HandlerFunc) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Fiber uses fasthttp, not net/http
		// We need to convert fasthttp.RequestCtx to net/http Request/ResponseWriter

		var req *http.Request
		var w http.ResponseWriter

		// Convert using fasthttpadaptor
		fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			req = r
			w = rw
		}))(c.Context())

		// Create GoNest context
		ctx := core.NewContext(w, req)
		ctx.Set("adapter", "fiber")
		ctx.Set("fiber_context", c) // Store for Fiber-specific features

		return handler(ctx)
	}
}

// WrapMiddleware wraps GoNest middleware for Fiber
func (a *FiberAdapter) WrapMiddleware(middleware core.MiddlewareFunc) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req *http.Request
		var w http.ResponseWriter

		fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			req = r
			w = rw
		}))(c.Context())

		ctx := core.NewContext(w, req)
		ctx.Set("adapter", "fiber")
		ctx.Set("fiber_context", c)

		wrapped := middleware(func(ctx *core.Context) error {
			return c.Next()
		})

		return wrapped(ctx)
	}
}

// ExtractContext extracts context from fiber.Ctx
func (a *FiberAdapter) ExtractContext(platformCtx any) *core.Context {
	c, ok := platformCtx.(*fiber.Ctx)
	if !ok {
		return &core.Context{}
	}

	var req *http.Request
	var w http.ResponseWriter

	fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		req = r
		w = rw
	}))(c.Context())

	return core.NewContext(w, req)
}

// CreateContext creates GoNest context from fiber.Ctx
func (a *FiberAdapter) CreateContext(c *fiber.Ctx) *core.Context {
	var req *http.Request
	var w http.ResponseWriter

	fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		req = r
		w = rw
	}))(c.Context())

	ctx := core.NewContext(w, req)
	ctx.Set("adapter", "fiber")
	ctx.Set("fiber_context", c)
	return ctx
}

// ToFiberHandler converts GoNest handler to Fiber handler
// Usage: app.Get("/path", adapters.ToFiberHandler(gonestHandler))
func ToFiberHandler(handler core.HandlerFunc) fiber.Handler {
	adapter := NewFiberAdapter()
	return adapter.WrapHandler(handler)
}

// ToFiberMiddleware converts GoNest middleware to Fiber middleware
func ToFiberMiddleware(middleware core.MiddlewareFunc) fiber.Handler {
	adapter := NewFiberAdapter()
	return adapter.WrapMiddleware(middleware)
}
