package controller

import (
	"github.com/gonest-dev/gonest/core"
)

// Builder helps build controllers with a fluent API
type Builder struct {
	routes  []*Route
	options *Options
}

// NewController creates a new controller builder
func NewController(opts ...*Options) *Builder {
	options := &Options{
		Prefix:      "",
		Middlewares: make([]core.MiddlewareFunc, 0),
		Metadata:    make(map[string]any),
	}

	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	return &Builder{
		routes:  make([]*Route, 0),
		options: options,
	}
}

// RouteBuilder helps build individual routes
type RouteBuilder struct {
	controller *Builder
	route      *Route
}

// Get adds a GET route
func (cb *Builder) Get(path string, handler HandlerFunc) *RouteBuilder {
	return cb.addRoute(HTTPMethodGET, path, handler)
}

// Post adds a POST route
func (cb *Builder) Post(path string, handler HandlerFunc) *RouteBuilder {
	return cb.addRoute(HTTPMethodPOST, path, handler)
}

// Put adds a PUT route
func (cb *Builder) Put(path string, handler HandlerFunc) *RouteBuilder {
	return cb.addRoute(HTTPMethodPUT, path, handler)
}

// Patch adds a PATCH route
func (cb *Builder) Patch(path string, handler HandlerFunc) *RouteBuilder {
	return cb.addRoute(HTTPMethodPATCH, path, handler)
}

// Delete adds a DELETE route
func (cb *Builder) Delete(path string, handler HandlerFunc) *RouteBuilder {
	return cb.addRoute(HTTPMethodDELETE, path, handler)
}

// Options adds an OPTIONS route
func (cb *Builder) Options(path string, handler HandlerFunc) *RouteBuilder {
	return cb.addRoute(HTTPMethodOPTIONS, path, handler)
}

// Head adds a HEAD route
func (cb *Builder) Head(path string, handler HandlerFunc) *RouteBuilder {
	return cb.addRoute(HTTPMethodHEAD, path, handler)
}

// addRoute is a helper to add routes
func (cb *Builder) addRoute(method HTTPMethod, path string, handler HandlerFunc) *RouteBuilder {
	// Prepend controller prefix to path
	fullPath := cb.options.Prefix + path

	route := &Route{
		Method:      method,
		Path:        fullPath,
		Handler:     handler,
		Params:      make([]*ParamConfig, 0),
		Middlewares: make([]core.MiddlewareFunc, 0),
		Metadata:    make(map[string]any),
	}

	// Add controller-level middlewares
	route.Middlewares = append(route.Middlewares, cb.options.Middlewares...)

	cb.routes = append(cb.routes, route)

	return &RouteBuilder{
		controller: cb,
		route:      route,
	}
}

// Use adds middleware to the route
func (rb *RouteBuilder) Use(middleware ...core.MiddlewareFunc) *RouteBuilder {
	rb.route.Middlewares = append(rb.route.Middlewares, middleware...)
	return rb
}

// Body adds a body parameter
func (rb *RouteBuilder) Body(name string) *RouteBuilder {
	rb.route.Params = append(rb.route.Params, &ParamConfig{
		Type:     ParamTypeBODY,
		Name:     name,
		Required: true,
	})
	return rb
}

// Query adds a query parameter
func (rb *RouteBuilder) Query(name string, required bool) *RouteBuilder {
	rb.route.Params = append(rb.route.Params, &ParamConfig{
		Type:     ParamTypeQUERY,
		Name:     name,
		Required: required,
	})
	return rb
}

// Param adds a path parameter
func (rb *RouteBuilder) Param(name string) *RouteBuilder {
	rb.route.Params = append(rb.route.Params, &ParamConfig{
		Type:     ParamTypePARAM,
		Name:     name,
		Required: true,
	})
	return rb
}

// Header adds a header parameter
func (rb *RouteBuilder) Header(name string, required bool) *RouteBuilder {
	rb.route.Params = append(rb.route.Params, &ParamConfig{
		Type:     ParamTypeHEADER,
		Name:     name,
		Required: required,
	})
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

// GetRoutes implements the Controller interface
func (cb *Builder) GetRoutes() []*Route {
	return cb.routes
}

// WithPrefix sets the controller prefix
func WithPrefix(prefix string) *Options {
	return &Options{
		Prefix:      prefix,
		Middlewares: make([]core.MiddlewareFunc, 0),
		Metadata:    make(map[string]any),
	}
}

// WithMiddleware adds middleware to controller options
func (opts *Options) WithMiddleware(middleware ...core.MiddlewareFunc) *Options {
	opts.Middlewares = append(opts.Middlewares, middleware...)
	return opts
}
