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

func TestDeclare_ExecutesFn(t *testing.T) {
	executed := false
	c := New(func(c *Controller) {
		executed = true
	})

	c.Declare()

	if !executed {
		t.Fatalf("Declare() did not execute fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	c := New(func(c *Controller) {
		count++
	})

	c.Declare()
	c.Declare()
	c.Declare()

	if count != 1 {
		t.Fatalf("fn executed %d times across 3 Declare() calls, want exactly 1", count)
	}
}

func TestDeclare_NilFn_DoesNotPanic(t *testing.T) {
	c := &Controller{}

	c.Declare()
}

// TestController_SatisfiesModuleControllerRef confirms the cross-package
// blocker flagged during T5 (unexported controllerRef.isController couldn't
// be satisfied outside package module) is resolved now that internal/module
// exports ControllerRef/IsController.
func TestController_SatisfiesModuleControllerRef(t *testing.T) {
	// Compile-time proof: this line alone is the test. If *Controller
	// stopped satisfying module.ControllerRef, this file would fail to
	// build.
	c := New(func(c *Controller) {})
	var _ module.ControllerRef = c
}
