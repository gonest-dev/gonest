package controller

import (
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
)

// resolvedProvider builds a *provider.Provider whose Constructor resolves T
// and whose resolved value is already set (SetResolvedValue is a public
// method precisely so tests like this one can simulate "Stage 3 already
// ran" without actually running the whole resolver pipeline).
func resolvedProvider[T any](value T) *provider.Provider {
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() T { return value })
	})
	p.Declare()
	p.SetResolvedValue(reflect.ValueOf(value))
	return p
}

type ctrlFooService struct{ Name string }

type ctrlAnimal interface{ Sound() string }
type ctrlCat struct{}

func (ctrlCat) Sound() string { return "meow" }

func TestController_ResolveDirect_Pointer_FindsOwnModuleProvider(t *testing.T) {
	p := resolvedProvider(&ctrlFooService{Name: "real"})
	c := New(func(c *Controller) {})

	m := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Controllers(c)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	v, ok := c.ResolveDirect(reflect.TypeOf(&ctrlFooService{}))
	if !ok {
		t.Fatalf("ResolveDirect() ok=false, want true")
	}
	if v.Interface().(*ctrlFooService).Name != "real" {
		t.Fatalf("ResolveDirect() = %v, want Name==real", v.Interface())
	}
}

func TestController_ResolveDirect_Interface_SingleMatch(t *testing.T) {
	p := resolvedProvider[ctrlAnimal](ctrlCat{})
	c := New(func(c *Controller) {})

	m := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Controllers(c)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	v, ok := c.ResolveDirect(reflect.TypeOf((*ctrlAnimal)(nil)).Elem())
	if !ok {
		t.Fatalf("ResolveDirect() ok=false, want true")
	}
	if v.Interface().(ctrlAnimal).Sound() != "meow" {
		t.Fatalf("ResolveDirect() resolved wrong value")
	}
}

func TestController_ResolveDirectAll_ReturnsMatchesScopedToOwnModule(t *testing.T) {
	p := resolvedProvider[ctrlAnimal](ctrlCat{})
	c := New(func(c *Controller) {})

	m := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Controllers(c)
	})
	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	matches := c.ResolveDirectAll(reflect.TypeOf((*ctrlAnimal)(nil)).Elem())
	if len(matches) != 1 {
		t.Fatalf("ResolveDirectAll() len = %d, want 1", len(matches))
	}
}

func TestController_ResolveDirect_NotVisibleFromOtherModule(t *testing.T) {
	p := resolvedProvider(&ctrlFooService{Name: "other-module-only"})
	c := New(func(c *Controller) {})

	otherModule := module.New(func(m *module.Module) {
		m.Providers(p)
	})
	ownModule := module.New(func(m *module.Module) {
		m.Controllers(c)
	})
	if _, err := otherModule.Assemble(); err != nil {
		t.Fatalf("Assemble(otherModule) error = %v", err)
	}
	if _, err := ownModule.Assemble(); err != nil {
		t.Fatalf("Assemble(ownModule) error = %v", err)
	}

	if _, ok := c.ResolveDirect(reflect.TypeOf(&ctrlFooService{})); ok {
		t.Fatalf("ResolveDirect() found a provider from a module that never imported/exported to it, want not visible")
	}
}
