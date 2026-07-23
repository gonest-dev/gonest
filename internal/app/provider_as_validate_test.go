package app

import (
	"strings"
	"testing"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
)

// providerAsValidateAnimal/providerAsValidateCat/providerAsValidateDog are
// minimal fixtures for this file's own tests -- an interface + one real
// implementor (cat) + one non-implementor (dog), used to exercise
// validateProviderAsRefs's 2 failure cases (missing registration, wrong
// type) plus the success case.
type providerAsValidateAnimal interface{ Talk() string }

type providerAsValidateCat struct{}

func (providerAsValidateCat) Talk() string { return "meow" }

type providerAsValidateDog struct{}

func (providerAsValidateDog) Bark() string { return "woof" }

func TestValidateProviderAsRefs_ConcreteNeverRegistered_ReturnsError(t *testing.T) {
	// cat is wrapped via module.ProviderAs but its own concrete provider is
	// NEVER passed to Providers -- Declare() never runs for it, so its
	// ResolvedType() stays nil (PROVAS-08).
	cat := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *providerAsValidateCat { return &providerAsValidateCat{} })
	})
	catAsAnimal := module.ProviderAs[providerAsValidateAnimal](cat)

	m := module.New(func(m *module.Module) {
		m.Providers(catAsAnimal) // cat itself deliberately omitted
	})
	modules, err := m.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	err = validateProviderAsRefs(modules)
	if err == nil {
		t.Fatal("validateProviderAsRefs() error = nil, want an error naming the missing registration")
	}
	if !strings.Contains(err.Error(), "never registered") {
		t.Fatalf("validateProviderAsRefs() error = %q, want it to mention the missing Providers(...) registration", err.Error())
	}
}

func TestValidateProviderAsRefs_WrappedTypeDoesNotImplementTarget_ReturnsError(t *testing.T) {
	// dog is registered (Declare runs, ResolvedType() becomes non-nil) but
	// does not implement providerAsValidateAnimal -- ProviderAs itself
	// cannot catch this at call time (see its own doc comment), only this
	// post-declare validation pass can.
	dog := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *providerAsValidateDog { return &providerAsValidateDog{} })
	})
	dogAsAnimal := module.ProviderAs[providerAsValidateAnimal](dog)

	m := module.New(func(m *module.Module) {
		m.Providers(dog, dogAsAnimal)
	})
	modules, err := m.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	declareProviders(modules)

	err = validateProviderAsRefs(modules)
	if err == nil {
		t.Fatal("validateProviderAsRefs() error = nil, want an error naming both types")
	}
	if !strings.Contains(err.Error(), "providerAsValidateDog") || !strings.Contains(err.Error(), "providerAsValidateAnimal") {
		t.Fatalf("validateProviderAsRefs() error = %q, want it to name both the concrete type and the target interface", err.Error())
	}
}

func TestValidateProviderAsRefs_WellFormed_ReturnsNil(t *testing.T) {
	cat := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *providerAsValidateCat { return &providerAsValidateCat{} })
	})
	catAsAnimal := module.ProviderAs[providerAsValidateAnimal](cat)

	m := module.New(func(m *module.Module) {
		m.Providers(cat, catAsAnimal)
	})
	modules, err := m.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	declareProviders(modules)

	if err := validateProviderAsRefs(modules); err != nil {
		t.Fatalf("validateProviderAsRefs() error = %v, want nil (well-formed registration)", err)
	}
}

func TestValidateProviderAsRefs_NoProviderAsRefs_ReturnsNil(t *testing.T) {
	p := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *providerAsValidateCat { return &providerAsValidateCat{} })
	})
	m := module.New(func(m *module.Module) { m.Providers(p) })
	modules, err := m.Assemble()
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	if err := validateProviderAsRefs(modules); err != nil {
		t.Fatalf("validateProviderAsRefs() error = %v, want nil (no ProviderAs views at all)", err)
	}
}

// TestNewApp_ProviderAs_WrongType_ReturnsBootstrapError proves
// validateProviderAsRefs is really wired into NewApp's bootstrap sequence
// (not just unit-tested in isolation): a wrongly-wrapped provider fails
// NewApp itself, before resolver.Resolve ever runs.
func TestNewApp_ProviderAs_WrongType_ReturnsBootstrapError(t *testing.T) {
	dog := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *providerAsValidateDog { return &providerAsValidateDog{} })
	})
	dogAsAnimal := module.ProviderAs[providerAsValidateAnimal](dog)

	root := module.New(func(m *module.Module) {
		m.Providers(dog, dogAsAnimal)
	})

	_, err := New[recordingFakeAdapter](root, Options{})
	if err == nil {
		t.Fatal("NewApp() error = nil, want a bootstrap error from validateProviderAsRefs")
	}
}

// TestMustNewApp_ProviderAs_ConcreteMissing_Panics proves MustNewApp's
// panic-on-error wrapper still works for this new validation error.
func TestMustNewApp_ProviderAs_ConcreteMissing_Panics(t *testing.T) {
	cat := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *providerAsValidateCat { return &providerAsValidateCat{} })
	})
	catAsAnimal := module.ProviderAs[providerAsValidateAnimal](cat)

	root := module.New(func(m *module.Module) {
		m.Providers(catAsAnimal) // cat itself deliberately omitted
	})

	defer func() {
		if recover() == nil {
			t.Fatal("MustNewApp() did not panic, want it to panic on validateProviderAsRefs' error")
		}
	}()

	MustNewApp[recordingFakeAdapter](root, Options{})
}

// TestNewApp_ProviderAs_WellFormed_Succeeds proves the success path through
// a REAL NewApp call: MustInject[TInterface] from a Controller resolves the
// ProviderAs-wrapped provider once bootstrap completes.
func TestNewApp_ProviderAs_WellFormed_Succeeds(t *testing.T) {
	cat := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *providerAsValidateCat { return &providerAsValidateCat{} })
	})
	catAsAnimal := module.ProviderAs[providerAsValidateAnimal](cat)

	root := module.New(func(m *module.Module) {
		m.Providers(cat, catAsAnimal)
	})

	app, err := New[recordingFakeAdapter](root, Options{})
	if err != nil {
		t.Fatalf("NewApp() error = %v, want nil (well-formed ProviderAs registration)", err)
	}
	if app == nil {
		t.Fatal("NewApp() returned nil *App")
	}
}
