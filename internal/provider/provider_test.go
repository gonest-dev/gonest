package provider

import (
	"context"
	"reflect"
	"testing"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/scope"
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

// TestProvider_SatisfiesModuleProviderRef confirms the cross-package
// blocker flagged during T4 (unexported providerRef.isProvider couldn't be
// satisfied outside package module) is resolved now that internal/module
// exports ProviderRef/IsProvider.
func TestProvider_SatisfiesModuleProviderRef(t *testing.T) {
	// Compile-time proof: this line alone is the test. If *Provider stopped
	// satisfying module.ProviderRef, this file would fail to build. Stage 1
	// assembly behavior itself (fn deferral, registration) is module's own
	// concern and already covered by internal/module's test suite.
	p := New(func(p *Provider) {})
	var _ module.ProviderRef = p
}

func TestResolvedType_ReturnsConstructorReturnType_FuncReturningT(t *testing.T) {
	p := New(func(p *Provider) {
		p.Constructor(func() *fakeService {
			return &fakeService{}
		})
	})
	runFn(p)

	want := reflect.TypeOf(&fakeService{})
	if got := p.ResolvedType(); got != want {
		t.Fatalf("ResolvedType() = %v, want %v", got, want)
	}
}

func TestDeclare_ExecutesFn(t *testing.T) {
	executed := false
	p := New(func(p *Provider) {
		executed = true
	})

	p.Declare()

	if !executed {
		t.Fatalf("Declare() did not execute fn")
	}
}

func TestDeclare_DoesNotRunFnTwiceOnRepeatedCalls(t *testing.T) {
	count := 0
	p := New(func(p *Provider) {
		count++
	})

	p.Declare()
	p.Declare()
	p.Declare()

	if count != 1 {
		t.Fatalf("fn executed %d times across 3 Declare() calls, want exactly 1", count)
	}
}

func TestDeclare_NilFn_DoesNotPanic(t *testing.T) {
	p := &Provider{}

	p.Declare()
}

func TestConstructorFunc_ReturnsInvocableReflectValue(t *testing.T) {
	p := New(func(p *Provider) {
		p.Constructor(func() *fakeService {
			return &fakeService{}
		})
	})
	runFn(p)

	fn := p.ConstructorFunc()
	if !fn.IsValid() {
		t.Fatalf("ConstructorFunc() returned invalid reflect.Value")
	}

	out := fn.Call(nil)
	if len(out) != 1 {
		t.Fatalf("ConstructorFunc().Call(nil) returned %d values, want 1", len(out))
	}
	if _, ok := out[0].Interface().(*fakeService); !ok {
		t.Fatalf("ConstructorFunc().Call(nil)[0] = %v, want *fakeService", out[0].Interface())
	}
}

func TestResolvedScope_ReturnsSingletonByDefault(t *testing.T) {
	p := New(func(p *Provider) {})
	runFn(p)

	if got := p.ResolvedScope(); got != scope.Singleton {
		t.Fatalf("ResolvedScope() = %v, want %v", got, scope.Singleton)
	}
}

func TestResolvedScope_ReturnsExplicitScope(t *testing.T) {
	p := New(func(p *Provider) {
		p.Scope(scope.Transient)
	})
	runFn(p)

	if got := p.ResolvedScope(); got != scope.Transient {
		t.Fatalf("ResolvedScope() = %v, want %v", got, scope.Transient)
	}
}

func TestResolvedType_ReturnsConstructorReturnType_FuncReturningTAndError(t *testing.T) {
	p := New(func(p *Provider) {
		p.Constructor(func() (*fakeService, error) {
			return &fakeService{}, nil
		})
	})
	runFn(p)

	want := reflect.TypeOf(&fakeService{})
	if got := p.ResolvedType(); got != want {
		t.Fatalf("ResolvedType() = %v, want %v", got, want)
	}
}

func TestSetResolvedValue_ThenResolvedValue_ReturnsStoredValue(t *testing.T) {
	p := New(nil)
	real := &fakeService{}

	if _, ok := p.ResolvedValue(); ok {
		t.Fatalf("ResolvedValue() ok=true before SetResolvedValue, want false")
	}

	p.SetResolvedValue(reflect.ValueOf(real))

	v, ok := p.ResolvedValue()
	if !ok {
		t.Fatalf("ResolvedValue() ok=false after SetResolvedValue, want true")
	}
	if v.Interface().(*fakeService) != real {
		t.Fatalf("ResolvedValue() = %v, want %v", v.Interface(), real)
	}
}
