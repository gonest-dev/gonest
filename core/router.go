package core

import (
	"net/http"
	"strings"
	"sync"
)

// Router manages HTTP routes
type Router struct {
	routes map[string]map[string]HandlerFunc // method -> path -> handler
	mu     sync.RWMutex
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]map[string]HandlerFunc),
	}
}

// Register registers a route
func (r *Router) Register(route RouteDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	method := strings.ToUpper(route.Method)

	if _, exists := r.routes[method]; !exists {
		r.routes[method] = make(map[string]HandlerFunc)
	}

	// Apply middlewares to handler
	handler := route.Handler
	for i := len(route.Middlewares) - 1; i >= 0; i-- {
		handler = route.Middlewares[i](handler)
	}

	r.routes[method][route.Path] = handler
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	method := req.Method
	path := req.URL.Path

	// Find handler
	if methodRoutes, exists := r.routes[method]; exists {
		if handler, exists := methodRoutes[path]; exists {
			ctx := NewContext(w, req)

			if err := handler(ctx); err != nil {
				// Default error handling
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Try to match with path parameters
		if handler := r.matchWithParams(methodRoutes, path); handler != nil {
			ctx := NewContext(w, req)

			if err := handler(ctx); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}

	// Not found
	http.NotFound(w, req)
}

// matchWithParams tries to match a path with parameter placeholders
func (r *Router) matchWithParams(routes map[string]HandlerFunc, path string) HandlerFunc {
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
			return func(ctx *Context) error {
				for key, value := range params {
					ctx.SetParam(key, value)
				}
				return handler(ctx)
			}
		}
	}

	return nil
}

// Get registers a GET route
func (r *Router) Get(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.Register(RouteDefinition{
		Method:      http.MethodGet,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
	})
}

// Post registers a POST route
func (r *Router) Post(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.Register(RouteDefinition{
		Method:      http.MethodPost,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
	})
}

// Put registers a PUT route
func (r *Router) Put(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.Register(RouteDefinition{
		Method:      http.MethodPut,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
	})
}

// Patch registers a PATCH route
func (r *Router) Patch(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.Register(RouteDefinition{
		Method:      http.MethodPatch,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
	})
}

// Delete registers a DELETE route
func (r *Router) Delete(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.Register(RouteDefinition{
		Method:      http.MethodDelete,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
	})
}

// Options registers an OPTIONS route
func (r *Router) Options(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.Register(RouteDefinition{
		Method:      http.MethodOptions,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
	})
}

// Head registers a HEAD route
func (r *Router) Head(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	r.Register(RouteDefinition{
		Method:      http.MethodHead,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
	})
}

// Use applies middleware globally
func (r *Router) Use(middleware MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Apply middleware to all existing routes
	for method := range r.routes {
		for path, handler := range r.routes[method] {
			r.routes[method][path] = middleware(handler)
		}
	}
}
