package controller

import (
	"testing"

	"github.com/gonest-dev/gonest/internal/module"
)

func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	executed := false

	c := New(func(c *Controller) {
		executed = true
	})

	if executed {
		t.Fatalf("New(fn) executed fn synchronously, want deferred until bootstrap runs it")
	}
	if c == nil {
		t.Fatalf("New(fn) returned nil *Controller")
	}
}

func TestController_SatisfiesModuleOwner(t *testing.T) {
	var owner module.Owner = New(func(c *Controller) {})
	if owner == nil {
		t.Fatalf("*Controller does not satisfy module.Owner")
	}
}

func TestOwnerModule_PopulatedAfterSetOwnerModule(t *testing.T) {
	c := New(func(c *Controller) {})

	m := module.New(func(m *module.Module) {})
	c.SetOwnerModule(m)

	if got := c.OwnerModule(); got != m {
		t.Fatalf("OwnerModule() = %v, want %v", got, m)
	}
}

func TestOwnerModule_NilBeforeAssociation(t *testing.T) {
	c := New(func(c *Controller) {})

	if got := c.OwnerModule(); got != nil {
		t.Fatalf("OwnerModule() = %v, want nil before any module associates this controller", got)
	}
}

// NOTE on module.Module.Controllers(cs ...controllerRef): Go's spec ties
// unexported interface methods to the package they are declared in --
// an unexported method identifier is only equal across packages if it is
// declared in the SAME package as the interface. internal/module's
// controllerRef ("isController()") therefore can only ever be satisfied by
// types defined inside package module itself; *controller.Controller from
// this package cannot implement it, no matter its method set, because
// package controller declares a distinct (name-identical but
// package-scoped) isController identifier. Verified empirically: adding
// `m.Controllers(c)` here (c being a *Controller from this package) fails
// to compile with "does not implement module.controllerRef (unexported
// method isController)". This is a cross-package structural-typing
// limitation in internal/module's current API, not something fixable from
// internal/controller -- see task report for details. *Controller still
// implements an isController() method locally (see controller.go) per the
// spec's stated contract, so it is ready the moment module.Module's API
// is adjusted (e.g. an exported adapter, or controllerRef moved/exported).
