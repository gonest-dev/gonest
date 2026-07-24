// Package controller implements the declarative Controller API of the DI
// graph. Controller.New(fn) defers fn until bootstrap runs it; it exists at
// this stage only as a minimal shell that satisfies module.Owner and the
// module package's unexported controllerRef marker interface, so it can be
// registered via Module.Controllers and later act as a MustInject
// consumer. Route/Path/Handler registration is out of scope here -- see the
// "Controller & Route Registration" feature.
package controller

import (
	"reflect"

	"gonest.dev/gonest/internal/filter"
	"gonest.dev/gonest/internal/guard"
	"gonest.dev/gonest/internal/interceptor"
	"gonest.dev/gonest/internal/middleware"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
	"gonest.dev/gonest/internal/route"
)

// Controller represents a declarative unit that consumes providers via
// MustInject. It does not participate in the provider resolution graph --
// it only consumes placeholders it requests.
//
// New(fn) does not execute fn at call time. fn is deferred until bootstrap
// (Stage 2, builder execution) runs it, since that is the point at which
// MustInject calls inside fn need a known owner module to resolve scope
// against.
type Controller struct {
	fn func(*Controller)

	owner    *module.Module
	declared bool

	pathPrefix string
	routes     []*route.Route

	middleware   []*middleware.Middleware
	guards       []*guard.Guard
	interceptors []*interceptor.Interceptor
	filters      []*filter.Filter

	tags       []string
	bearerAuth bool
}

// New creates a Controller that defers fn until bootstrap runs it. fn is
// expected to declare routes/handlers and call MustInject for its
// dependencies -- neither is implemented yet at this shell stage.
func New(fn func(*Controller)) *Controller {
	return &Controller{fn: fn}
}

// Declare runs this controller's deferred fn exactly once. It is a no-op on
// any call after the first (including when fn is nil), mirroring
// Provider.Declare -- callers that walk the assembled module tree (Stage 2
// of bootstrap) can call Declare on every registered controller without
// tracking which ones they already visited.
func (c *Controller) Declare() {
	if c.declared {
		return
	}
	c.declared = true
	if c.fn != nil {
		c.fn(c)
	}
}

// IsController is the marker method that satisfies module.ControllerRef, so
// *Controller can be passed to (*module.Module).Controllers. Exported: Go
// ties unexported interface methods to the declaring package, so an
// unexported marker here could never satisfy module's interface across
// packages.
func (c *Controller) IsController() {}

// IsToken is a marker method that satisfies module.TokenRef (via
// module.ControllerRef, which embeds it), so *Controller can be passed to
// any of Module's builder methods.
func (c *Controller) IsToken() {}

// SetOwnerModule associates this controller with the module that owns it.
// It is called by module assembly once ownership is known (structural
// assembly walks Module.Controllers registrations); a later task wires
// this call in automatically. Exposed for now so callers of the DI
// bootstrap machinery can establish ownership explicitly.
func (c *Controller) SetOwnerModule(m *module.Module) {
	c.owner = m
}

// OwnerModule implements module.Owner. It returns nil until
// SetOwnerModule has been called on this controller.
func (c *Controller) OwnerModule() *module.Module {
	return c.owner
}

// ResolveDirect satisfies internal/inject's directResolver interface,
// scoping the search to a SINGLE module: this Controller's own OwnerModule
// (module-scoped, same encapsulation Controllers already have today via
// findOwn/findExported) -- unlike Middleware/Guard/Interceptor/Filter's
// union-of-referencing-modules scope, a Controller has exactly one owner,
// known since Stage 1 assembly, so no external scope needs to be passed in.
func (c *Controller) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect([]*module.Module{c.owner}, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// same single-module scope as ResolveDirect.
func (c *Controller) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll([]*module.Module{c.owner}, t)
}

// Path stores prefix as this controller's route path prefix. Route
// registration itself does not yet apply the prefix to individual routes --
// that composition happens later, when routes are wired into the HTTP
// server (out of scope for this task).
func (c *Controller) Path(prefix string) {
	c.pathPrefix = prefix
}

// PathPrefix returns the prefix stored via Path, or "" if Path was never
// called.
func (c *Controller) PathPrefix() string {
	return c.pathPrefix
}

// Route creates a *route.Route via route.New(method, path, fn) -- which
// runs fn immediately, see route.New's own doc comment -- and appends it to
// this controller's internal route list.
func (c *Controller) Route(method route.HttpMethod, path string, fn func(*route.Route)) {
	c.routes = append(c.routes, route.New(method, path, fn))
}

// RouteGet is shorthand for Route(route.HttpGet, path, fn).
func (c *Controller) RouteGet(path string, fn func(*route.Route)) {
	c.Route(route.HttpGet, path, fn)
}

// RoutePost is shorthand for Route(route.HttpPost, path, fn).
func (c *Controller) RoutePost(path string, fn func(*route.Route)) {
	c.Route(route.HttpPost, path, fn)
}

// RoutePut is shorthand for Route(route.HttpPut, path, fn).
func (c *Controller) RoutePut(path string, fn func(*route.Route)) {
	c.Route(route.HttpPut, path, fn)
}

// RoutePatch is shorthand for Route(route.HttpPatch, path, fn).
func (c *Controller) RoutePatch(path string, fn func(*route.Route)) {
	c.Route(route.HttpPatch, path, fn)
}

// RouteDelete is shorthand for Route(route.HttpDelete, path, fn).
func (c *Controller) RouteDelete(path string, fn func(*route.Route)) {
	c.Route(route.HttpDelete, path, fn)
}

// RouteHead is shorthand for Route(route.HttpHead, path, fn).
func (c *Controller) RouteHead(path string, fn func(*route.Route)) {
	c.Route(route.HttpHead, path, fn)
}

// RouteOptions is shorthand for Route(route.HttpOptions, path, fn).
func (c *Controller) RouteOptions(path string, fn func(*route.Route)) {
	c.Route(route.HttpOptions, path, fn)
}

// RouteTrace is shorthand for Route(route.HttpTrace, path, fn).
func (c *Controller) RouteTrace(path string, fn func(*route.Route)) {
	c.Route(route.HttpTrace, path, fn)
}

// RouteConnect is shorthand for Route(route.HttpConnect, path, fn).
func (c *Controller) RouteConnect(path string, fn func(*route.Route)) {
	c.Route(route.HttpConnect, path, fn)
}

// RouteQuery is shorthand for Route(route.HttpQuery, path, fn).
func (c *Controller) RouteQuery(path string, fn func(*route.Route)) {
	c.Route(route.HttpQuery, path, fn)
}

// OwnRoutes returns a copy of the routes registered on this controller via
// Route. Read-only: mutating the returned slice does not affect this
// Controller's internal state (same defensive-copy pattern as
// Module.OwnProviders/Module.OwnControllers).
func (c *Controller) OwnRoutes() []*route.Route {
	return append([]*route.Route(nil), c.routes...)
}

// Use registers items as general middleware for this controller, using the
// real *middleware.Middleware type (internal/middleware, feature
// "Middleware" T1). Use was never called by any shipped code path with the
// placeholder Middleware stub, so it is safe to change its signature
// directly rather than deprecate-and-migrate (same class of change as the
// earlier "Controller & Route Registration" feature's HttpAdapter.Listen
// signature change).
func (c *Controller) Use(items ...*middleware.Middleware) {
	c.middleware = append(c.middleware, items...)
}

// OwnMiddleware returns a copy of the middleware registered on this
// controller via Use. Read-only: mutating the returned slice does not
// affect this Controller's internal state (same defensive-copy pattern as
// OwnRoutes).
func (c *Controller) OwnMiddleware() []*middleware.Middleware {
	return append([]*middleware.Middleware(nil), c.middleware...)
}

// Guards registers items as route guards for this controller, using the
// real *guard.Guard type (internal/guard, feature "Guard" T1). Guards was
// never called by any shipped code path with the placeholder Middleware
// stub, so it is safe to change its signature directly rather than
// deprecate-and-migrate (same class of change as Use's earlier migration to
// *middleware.Middleware).
func (c *Controller) Guards(items ...*guard.Guard) {
	c.guards = append(c.guards, items...)
}

// OwnGuards returns a copy of the guards registered on this controller via
// Guards. Read-only: mutating the returned slice does not affect this
// Controller's internal state (same defensive-copy pattern as
// OwnMiddleware).
func (c *Controller) OwnGuards() []*guard.Guard {
	return append([]*guard.Guard(nil), c.guards...)
}

// Interceptors registers items as interceptors for this controller, using
// the real *interceptor.Interceptor type (internal/interceptor, feature
// "Interceptor" T1). Interceptors was never called by any shipped code path
// with the placeholder Middleware stub, so it is safe to change its
// signature directly rather than deprecate-and-migrate (same class of
// change as Use's/Guards' earlier migrations to their respective real
// types).
func (c *Controller) Interceptors(items ...*interceptor.Interceptor) {
	c.interceptors = append(c.interceptors, items...)
}

// OwnInterceptors returns a copy of the interceptors registered on this
// controller via Interceptors. Read-only: mutating the returned slice does
// not affect this Controller's internal state (same defensive-copy pattern
// as OwnGuards).
func (c *Controller) OwnInterceptors() []*interceptor.Interceptor {
	return append([]*interceptor.Interceptor(nil), c.interceptors...)
}

// Filters registers items as exception filters for this controller, using
// the real *filter.Filter type (internal/filter, feature "Filter" T1).
// Filters was never called by any shipped code path with the placeholder
// Middleware stub, so it is safe to change its signature directly rather
// than deprecate-and-migrate (same class of change as Use's/Guards'/
// Interceptors' earlier migrations to their respective real types).
func (c *Controller) Filters(items ...*filter.Filter) {
	c.filters = append(c.filters, items...)
}

// OwnFilters returns a copy of the filters registered on this controller via
// Filters. Read-only: mutating the returned slice does not affect this
// Controller's internal state (same defensive-copy pattern as
// OwnInterceptors).
func (c *Controller) OwnFilters() []*filter.Filter {
	return append([]*filter.Filter(nil), c.filters...)
}

// Tags stores tags as this controller's OpenAPI tags, inherited by every
// Route under this controller UNLESS the Route itself calls its own Tags,
// which REPLACES the controller's value entirely for that route (spec.md
// AC2 -- override resolution happens at generation time, not here).
func (c *Controller) Tags(tags ...string) *Controller {
	c.tags = append([]string(nil), tags...)
	return c
}

// OwnTags returns a copy of the tags registered on this controller via Tags.
// Read-only: mutating the returned slice does not affect this Controller's
// internal state (same defensive-copy pattern as OwnMiddleware/
// OwnInterceptors/OwnFilters).
func (c *Controller) OwnTags() []string {
	return append([]string(nil), c.tags...)
}

// BearerAuth marks this controller as requiring bearer authentication,
// inherited by every Route under this controller UNLESS the Route itself
// calls its own BearerAuth, which REPLACES the controller's value entirely
// for that route (spec.md AC2, same override semantics as Tags). Returns c
// so calls can chain.
func (c *Controller) BearerAuth() *Controller {
	c.bearerAuth = true
	return c
}

// HasBearerAuth reports whether BearerAuth was called on this controller.
func (c *Controller) HasBearerAuth() bool {
	return c.bearerAuth
}
