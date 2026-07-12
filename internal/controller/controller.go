// Package controller implements the declarative Controller API of the DI
// graph. Controller.New(fn) defers fn until bootstrap runs it; it exists at
// this stage only as a minimal shell that satisfies module.Owner and the
// module package's unexported controllerRef marker interface, so it can be
// registered via Module.Controllers and later act as a MustResolve
// consumer. Route/Path/Handler registration is out of scope here -- see the
// "Controller & Route Registration" feature.
package controller

import "github.com/gonest-dev/gonest/internal/module"

// Controller represents a declarative unit that consumes providers via
// MustResolve. It does not participate in the provider resolution graph --
// it only consumes placeholders it requests.
//
// New(fn) does not execute fn at call time. fn is deferred until bootstrap
// (Stage 2, builder execution) runs it, since that is the point at which
// MustResolve calls inside fn need a known owner module to resolve scope
// against.
type Controller struct {
	fn func(*Controller)

	owner *module.Module
}

// New creates a Controller that defers fn until bootstrap runs it. fn is
// expected to declare routes/handlers and call MustResolve for its
// dependencies -- neither is implemented yet at this shell stage.
func New(fn func(*Controller)) *Controller {
	return &Controller{fn: fn}
}

// isController is the unexported marker method that lets *Controller
// structurally satisfy module.controllerRef (internal/module) without
// either package importing a shared interface type.
func (c *Controller) isController() {}

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
