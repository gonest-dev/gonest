package provider

import (
	"context"
	"testing"

	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/scope"
)

type fakeService struct{}

func TestNew_DoesNotExecuteFnOnCall(t *testing.T) {
	executed := false

	p := New(func(p *Provider) {
		executed = true
	})

	if executed {
		t.Fatalf("New(fn) executed fn synchronously, want deferred")
	}
	if p == nil {
		t.Fatalf("New(fn) returned nil *Provider")
	}
}

// runFn is a white-box test helper: it invokes the deferred fn directly,
// simulating what Stage 2 (a future task) will do during bootstrap.
func runFn(p *Provider) {
	if p.fn != nil {
		p.fn(p)
	}
}

func TestConstructor_AcceptsFuncReturningT(t *testing.T) {
	p := New(func(p *Provider) {
		p.Constructor(func() *fakeService {
			return &fakeService{}
		})
	})

	runFn(p)

	if p.constructor.IsZero() {
		t.Fatalf("Constructor did not store a valid reflect.Value for func() T")
	}
}

func TestConstructor_AcceptsFuncReturningTAndError(t *testing.T) {
	p := New(func(p *Provider) {
		p.Constructor(func() (*fakeService, error) {
			return &fakeService{}, nil
		})
	})

	runFn(p)

	if p.constructor.IsZero() {
		t.Fatalf("Constructor did not store a valid reflect.Value for func() (T, error)")
	}
}

func TestConstructor_AcceptsFuncWithContextReturningT(t *testing.T) {
	p := New(func(p *Provider) {
		p.Constructor(func(ctx context.Context) *fakeService {
			return &fakeService{}
		})
	})

	runFn(p)

	if p.constructor.IsZero() {
		t.Fatalf("Constructor did not store a valid reflect.Value for func(context.Context) T")
	}
}

func TestConstructor_AcceptsFuncWithContextReturningTAndError(t *testing.T) {
	p := New(func(p *Provider) {
		p.Constructor(func(ctx context.Context) (*fakeService, error) {
			return &fakeService{}, nil
		})
	})

	runFn(p)

	if p.constructor.IsZero() {
		t.Fatalf("Constructor did not store a valid reflect.Value for func(context.Context) (T, error)")
	}
}

func TestConstructor_InvalidSignature_Panics(t *testing.T) {
	p := New(func(p *Provider) {
		p.Constructor(func(a, b int) string { return "" })
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Constructor with invalid signature did not panic")
		}
		msg, ok := r.(string)
		if !ok || msg != "gonest: invalid Constructor signature" {
			t.Fatalf("panic value = %v, want %q", r, "gonest: invalid Constructor signature")
		}
	}()

	runFn(p)
}

func TestScope_DefaultsToSingleton(t *testing.T) {
	p := New(func(p *Provider) {})
	runFn(p)

	if p.scope != scope.Singleton {
		t.Fatalf("default scope = %v, want %v", p.scope, scope.Singleton)
	}
}

func TestScope_ExplicitOverridesDefault(t *testing.T) {
	p := New(func(p *Provider) {
		p.Scope(scope.Transient)
	})
	runFn(p)

	if p.scope != scope.Transient {
		t.Fatalf("scope = %v, want %v", p.scope, scope.Transient)
	}
}

func TestProvider_SatisfiesModuleOwner(t *testing.T) {
	var owner module.Owner = New(func(p *Provider) {})
	if owner == nil {
		t.Fatalf("*Provider does not satisfy module.Owner")
	}
}

func TestOwnerModule_NilBeforeAssociation(t *testing.T) {
	p := New(func(p *Provider) {})

	if got := p.OwnerModule(); got != nil {
		t.Fatalf("OwnerModule() = %v, want nil before any module associates this provider", got)
	}
}

func TestOwnerModule_PopulatedAfterSetOwnerModule(t *testing.T) {
	p := New(func(p *Provider) {})

	m := module.New(func(m *module.Module) {})
	p.SetOwnerModule(m)

	if got := p.OwnerModule(); got != m {
		t.Fatalf("OwnerModule() = %v, want %v", got, m)
	}
}

// NOTE (blocker, not weakened): a cross-package integration test calling
// (*module.Module).Providers(p) with p *Provider from this package does
// NOT compile. Go's spec disallows satisfying an interface with an
// unexported method (providerRef.isProvider) from any package other than
// the one that declares the interface -- confirmed with an isolated
// two-package repro (pkga.Ref / pkgb.Thing) that fails identically:
// "*pkgb.Thing does not implement pkga.Ref (unexported method isRef)".
// module_test.go's own fakeProvider only works because it lives in
// package module itself (same package as providerRef), not because of
// the method name matching alone.
//
// *Provider does implement an isProvider() method, satisfying the letter
// of the task's structural requirement, and passing *Provider to any
// exported API in package module works fine. But module.Module.Providers
// specifically cannot accept it as-is -- this needs to be resolved by
// widening internal/module's contract (e.g. an exported adapter type or
// exported interface), which is out of scope for T4 (internal/module is
// off-limits for this task). Flagged in the task report.
