// gonest/core/controller/builder.go
package controller

import (
	"github.com/gonest-dev/gonest/core/common"
)

// Builder helps build controllers with a fluent API
type Builder struct {
	routes  []common.RouteDefinition
	options *Options
}

// NewController creates a new controller builder
func NewController(opts ...*Options) *Builder {
	options := &Options{
		Prefix:      "",
		Middlewares: make([]common.MiddlewareFunc, 0),
		Metadata:    make(map[string]any),
	}

	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	return &Builder{
		routes:  make([]common.RouteDefinition, 0),
		options: options,
	}
}

// RouteBuilder helps build individual routes
type RouteBuilder struct {
	controller *Builder
	route      *common.RouteDefinition
	params     []*ParamConfig
}

// Get adds a GET route
func (cb *Builder) Get(path string, handler common.HandlerFunc) *RouteBuilder {
	return cb.addRoute("GET", path, handler)
}

// Post adds a POST route
func (cb *Builder) Post(path string, handler common.HandlerFunc) *RouteBuilder {
	return cb.addRoute("POST", path, handler)
}

// Put adds a PUT route
func (cb *Builder) Put(path string, handler common.HandlerFunc) *RouteBuilder {
	return cb.addRoute("PUT", path, handler)
}

// Patch adds a PATCH route
func (cb *Builder) Patch(path string, handler common.HandlerFunc) *RouteBuilder {
	return cb.addRoute("PATCH", path, handler)
}

// Delete adds a DELETE route
func (cb *Builder) Delete(path string, handler common.HandlerFunc) *RouteBuilder {
	return cb.addRoute("DELETE", path, handler)
}

// Options adds an OPTIONS route
func (cb *Builder) Options(path string, handler common.HandlerFunc) *RouteBuilder {
	return cb.addRoute("OPTIONS", path, handler)
}

// Head adds a HEAD route
func (cb *Builder) Head(path string, handler common.HandlerFunc) *RouteBuilder {
	return cb.addRoute("HEAD", path, handler)
}

// addRoute is a helper to add routes
func (cb *Builder) addRoute(method string, path string, handler common.HandlerFunc) *RouteBuilder {
	// Prepend controller prefix to path
	fullPath := cb.options.Prefix + path

	route := common.RouteDefinition{
		Method:      method,
		Path:        fullPath,
		Handler:     handler,
		Middlewares: make([]common.MiddlewareFunc, 0),
		Metadata:    make(map[string]any),
	}

	// Add controller-level middlewares
	route.Middlewares = append(route.Middlewares, cb.options.Middlewares...)

	// Copy controller metadata
	for k, v := range cb.options.Metadata {
		route.Metadata[k] = v
	}

	cb.routes = append(cb.routes, route)

	return &RouteBuilder{
		controller: cb,
		route:      &cb.routes[len(cb.routes)-1],
		params:     make([]*ParamConfig, 0),
	}
}

// Use adds middleware to the route
func (rb *RouteBuilder) Use(middleware ...common.MiddlewareFunc) *RouteBuilder {
	rb.route.Middlewares = append(rb.route.Middlewares, middleware...)
	return rb
}

// Body adds a body parameter
func (rb *RouteBuilder) Body(name string) *RouteBuilder {
	rb.params = append(rb.params, &ParamConfig{
		Type:     ParamTypeBODY,
		Name:     name,
		Required: true,
	})
	rb.route.Metadata["params"] = rb.params
	return rb
}

// Query adds a query parameter
func (rb *RouteBuilder) Query(name string, required bool) *RouteBuilder {
	rb.params = append(rb.params, &ParamConfig{
		Type:     ParamTypeQUERY,
		Name:     name,
		Required: required,
	})
	rb.route.Metadata["params"] = rb.params
	return rb
}

// Param adds a path parameter
func (rb *RouteBuilder) Param(name string) *RouteBuilder {
	rb.params = append(rb.params, &ParamConfig{
		Type:     ParamTypePARAM,
		Name:     name,
		Required: true,
	})
	rb.route.Metadata["params"] = rb.params
	return rb
}

// Header adds a header parameter
func (rb *RouteBuilder) Header(name string, required bool) *RouteBuilder {
	rb.params = append(rb.params, &ParamConfig{
		Type:     ParamTypeHEADER,
		Name:     name,
		Required: required,
	})
	rb.route.Metadata["params"] = rb.params
	return rb
}

// Meta adds metadata to the route
func (rb *RouteBuilder) Meta(key string, value any) *RouteBuilder {
	rb.route.Metadata[key] = value
	return rb
}

// Build returns the controller
func (rb *RouteBuilder) Build() *Builder {
	return rb.controller
}

// Routes implements the common.Controller interface
func (cb *Builder) Routes() []common.RouteDefinition {
	return cb.routes
}

// GetRoutes returns the routes as controller.Route slice
func (cb *Builder) GetRoutes() []*Route {
	routes := make([]*Route, len(cb.routes))
	for i, r := range cb.routes {
		var params []*ParamConfig
		if p, ok := r.Metadata["params"].([]*ParamConfig); ok {
			params = p
		}

		routes[i] = &Route{
			Method:      HTTPMethod(r.Method),
			Path:        r.Path,
			Handler:     HandlerFunc(r.Handler),
			Params:      params,
			Middlewares: r.Middlewares,
			Metadata:    r.Metadata,
		}
	}
	return routes
}

// WithPrefix sets the controller prefix
func WithPrefix(prefix string) *Options {
	return &Options{
		Prefix:      prefix,
		Middlewares: make([]common.MiddlewareFunc, 0),
		Metadata:    make(map[string]any),
	}
}

// WithMiddleware adds middleware to controller options
func (opts *Options) WithMiddleware(middleware ...common.MiddlewareFunc) *Options {
	opts.Middlewares = append(opts.Middlewares, middleware...)
	return opts
}


