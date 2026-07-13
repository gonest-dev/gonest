// Package controller implements the declarative Controller API of the DI
// graph. Controller.New(fn) defers fn until bootstrap runs it; it exists at
// this stage only as a minimal shell that satisfies module.Owner and the
// module package's unexported controllerRef marker interface, so it can be
// registered via Module.Controllers and later act as a MustInject
// consumer. Route/Path/Handler registration is out of scope here -- see the
// "Controller & Route Registration" feature.
package controller

import (
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/route"
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

	middleware   []Middleware
	guards       []Middleware
	interceptors []Middleware
	filters      []Middleware
}

// Middleware is a minimal placeholder type for the pipeline stubs (Use,
// Guards, Interceptors, Filters). It carries no behavior yet -- it exists so
// those methods have a plausible "list of middleware-like things" signature
// to grow into once a later feature defines what middleware actually does.
type Middleware struct{}

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

// OwnRoutes returns a copy of the routes registered on this controller via
// Route. Read-only: mutating the returned slice does not affect this
// Controller's internal state (same defensive-copy pattern as
// Module.OwnProviders/Module.OwnControllers).
func (c *Controller) OwnRoutes() []*route.Route {
	return append([]*route.Route(nil), c.routes...)
}

// Use registers items as general middleware for this controller. Stub only
// -- nothing reads the stored values yet.
func (c *Controller) Use(items ...Middleware) {
	c.middleware = append(c.middleware, items...)
}

// Guards registers items as route guards for this controller. Stub only --
// nothing reads the stored values yet.
func (c *Controller) Guards(items ...Middleware) {
	c.guards = append(c.guards, items...)
}

// Interceptors registers items as interceptors for this controller. Stub
// only -- nothing reads the stored values yet.
func (c *Controller) Interceptors(items ...Middleware) {
	c.interceptors = append(c.interceptors, items...)
}

// Filters registers items as exception filters for this controller. Stub
// only -- nothing reads the stored values yet.
func (c *Controller) Filters(items ...Middleware) {
	c.filters = append(c.filters, items...)
}
