// gonest/platform/mux.go
package platform

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/gonest-dev/gonest/core"
)

// MuxAdapter implements PlatformAdapter for standard net/http
type MuxAdapter struct {
	config      *AdapterConfig
	mux         *http.ServeMux
	routes      map[string]map[string]core.HandlerFunc // method -> path -> handler
	middlewares []core.MiddlewareFunc
	mu          sync.RWMutex
}

// NewMuxAdapter creates a standard net/http adapter
func NewMuxAdapter(config ...*AdapterConfig) *MuxAdapter {
	cfg := &AdapterConfig{
		Logger: &DefaultLogger{},
	}

	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return &MuxAdapter{
		config:      cfg,
		mux:         http.NewServeMux(),
		routes:      make(map[string]map[string]core.HandlerFunc),
		middlewares: make([]core.MiddlewareFunc, 0),
	}
}

// Name returns the adapter name
func (a *MuxAdapter) Name() string {
	return "standard"
}

// RegisterRoute registers a route with the platform
func (a *MuxAdapter) RegisterRoute(route core.RouteDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	method := strings.ToUpper(route.Method)

	// Initialize method map if needed
	if _, exists := a.routes[method]; !exists {
		a.routes[method] = make(map[string]core.HandlerFunc)
	}

	// Apply route-specific middlewares
	handler := route.Handler
	for i := len(route.Middlewares) - 1; i >= 0; i-- {
		handler = route.Middlewares[i](handler)
	}

	// Apply global middlewares
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		handler = a.middlewares[i](handler)
	}

	a.routes[method][route.Path] = handler

	return nil
}

// Handler returns the http.Handler for the platform
func (a *MuxAdapter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.RLock()
		defer a.mu.RUnlock()

		method := r.Method
		path := r.URL.Path

		// Find handler
		if methodRoutes, exists := a.routes[method]; exists {
			// Try exact match first
			if handler, exists := methodRoutes[path]; exists {
				ctx := core.NewContext(w, r)
				ctx.Set("adapter", "standard")

				if err := handler(ctx); err != nil {
					a.handleError(w, err)
				}
				return
			}

			// Try pattern matching
			if handler := a.matchWithParams(methodRoutes, path, r); handler != nil {
				ctx := core.NewContext(w, r)
				ctx.Set("adapter", "standard")

				if err := handler(ctx); err != nil {
					a.handleError(w, err)
				}
				return
			}
		}

		// Not found
		http.NotFound(w, r)
	})
}

// Use registers global middleware
func (a *MuxAdapter) Use(middleware core.MiddlewareFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.middlewares = append(a.middlewares, middleware)
}

// matchWithParams tries to match a path with parameter placeholders
func (a *MuxAdapter) matchWithParams(routes map[string]core.HandlerFunc, path string, r *http.Request) core.HandlerFunc {
	pathSegments := strings.Split(strings.Trim(path, "/"), "/")

	for routePath, handler := range routes {
		routeSegments := strings.Split(strings.Trim(routePath, "/"), "/")

		if len(pathSegments) != len(routeSegments) {
			continue
		}

		match := true
		params := make(map[string]string)

		for i, segment := range routeSegments {
			if strings.HasPrefix(segment, ":") {
				// Parameter placeholder
				paramName := segment[1:]
				params[paramName] = pathSegments[i]
			} else if segment != pathSegments[i] {
				match = false
				break
			}
		}

		if match {
			// Wrap handler to inject params
			return func(ctx *core.Context) error {
				for key, value := range params {
					ctx.SetParam(key, value)
				}
				return handler(ctx)
			}
		}
	}

	return nil
}

// handleError handles errors
func (a *MuxAdapter) handleError(w http.ResponseWriter, err error) {
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

// Compile-time check
var _ PlatformAdapter = (*MuxAdapter)(nil)
