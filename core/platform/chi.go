// gonest/platform/chi.go
package platform

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gonest-dev/gonest/core/common"
)

// ChiAdapter implements Adapter for Chi router
type ChiAdapter struct {
	config *AdapterConfig
	router *chi.Mux
}

// NewChiAdapter creates a Chi adapter
func NewChiAdapter(config ...*AdapterConfig) *ChiAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return &ChiAdapter{
		config: cfg,
		router: chi.NewRouter(),
	}
}

// Name returns the adapter name
func (a *ChiAdapter) Name() string {
	return "chi"
}

// RegisterRoute registers a route with the platform
func (a *ChiAdapter) RegisterRoute(route common.RouteDefinition) error {
	// Convert GoNest handler to Chi handler
	handler := a.wrapHandler(route)

	// Register with Chi
	a.router.Method(route.Method, route.Path, handler)

	return nil
}

// Handler returns the http.Handler for the platform
func (a *ChiAdapter) Handler() http.Handler {
	return a.router
}

// Use registers global middleware
func (a *ChiAdapter) Use(middleware common.MiddlewareFunc) {
	// Convert GoNest middleware to Chi middleware
	chiMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create GoNest context
			ctx := common.NewContext(w, r)
			ctx.Set("adapter", "chi")

			// Wrap next handler
			wrapped := middleware(func(_ *common.Context) error {
				next.ServeHTTP(w, r)
				return nil
			})

			// Execute
			if err := wrapped(ctx); err != nil {
				a.handleError(w, err)
			}
		})
	}

	a.router.Use(chiMiddleware)
}

// wrapHandler wraps a GoNest route to Chi handler
func (a *ChiAdapter) wrapHandler(route common.RouteDefinition) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create GoNest context
		ctx := common.NewContext(w, r)
		ctx.Set("adapter", "chi")

		// Extract Chi URL params
		chiCtx := chi.RouteContext(r.Context())
		if chiCtx != nil {
			for i, key := range chiCtx.URLParams.Keys {
				if i < len(chiCtx.URLParams.Values) {
					ctx.SetParam(key, chiCtx.URLParams.Values[i])
				}
			}
		}

		// Apply route middlewares
		handler := route.Handler
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		// Execute handler
		if err := handler(ctx); err != nil {
			a.handleError(w, err)
		}
	}
}

// handleError handles errors
func (a *ChiAdapter) handleError(w http.ResponseWriter, err error) {
	if a.config.ErrorHandler != nil {
		_ = a.config.ErrorHandler(err, w)
		return
	}

	// Default error handling
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": err.Error(),
	})
}

// GetRouter returns the underlying Chi router for advanced configuration
func (a *ChiAdapter) GetRouter() *chi.Mux {
	return a.router
}

// Compile-time check
var _ Adapter = (*ChiAdapter)(nil)



