package inject

import (
	"strings"
	"testing"

	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
	"gonest.dev/gonest/internal/scope"
)

// lazyConfig/otherLazyConfig are placeholder Lazy-injected value types --
// content doesn't matter, only pointer identity/type.
type lazyConfig struct {
	Driver string
}

type otherLazyConfig struct{}

func TestMustLazy_SuccessfulEagerResolve(t *testing.T) {
	resetPendingEdges()

	callCount := 0
	p := provider.New(func(pr *provider.Provider) {
		pr.Constructor(func() *lazyConfig {
			callCount++
			return &lazyConfig{Driver: "sms"}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Lazy(func(l *module.LazyModule) {
			got := Must[*lazyConfig](l)
			if got == nil || got.Driver != "sms" {
				t.Fatalf("Must[*lazyConfig](l) = %+v, want Driver=sms", got)
			}
		})
	})

	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble returned error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("Constructor called %d times, want exactly 1", callCount)
	}
}

func TestMustLazy_NoMatchingProvider_Panics(t *testing.T) {
	resetPendingEdges()

	m := module.New(func(m *module.Module) {
		m.Lazy(func(l *module.LazyModule) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Must[*lazyConfig](l) with no matching provider did not panic")
				}
				msg, ok := r.(string)
				if !ok {
					t.Fatalf("panic value = %v (%T), want string", r, r)
				}
				if !strings.Contains(msg, "lazyConfig") {
					t.Fatalf("panic message = %q, want it to name the target type", msg)
				}
			}()
			Must[*lazyConfig](l)
		})
	})

	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble returned error: %v", err)
	}
}

func TestMustLazy_NestedMustInject_Panics(t *testing.T) {
	resetPendingEdges()

	dep := provider.New(func(pr *provider.Provider) {
		pr.Constructor(func() *otherLazyConfig { return &otherLazyConfig{} })
	})

	p := provider.New(func(pr *provider.Provider) {
		pr.Constructor(func() *lazyConfig {
			// Recording a new pending edge during eager Lazy construction
			// is exactly what LAZY-06 forbids.
			Must[*otherLazyConfig](pr)
			return &lazyConfig{}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(p, dep)
		m.Lazy(func(l *module.LazyModule) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Must[*lazyConfig](l) with a self-referencing Constructor did not panic")
				}
			}()
			Must[*lazyConfig](l)
		})
	})

	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble returned error: %v", err)
	}
}

func TestMustLazy_NonSingletonProvider_Panics(t *testing.T) {
	resetPendingEdges()

	p := provider.New(func(pr *provider.Provider) {
		pr.Scope(scope.Transient)
		pr.Constructor(func() *lazyConfig { return &lazyConfig{} })
	})

	m := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Lazy(func(l *module.LazyModule) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Must[*lazyConfig](l) against a Transient provider did not panic")
				}
			}()
			Must[*lazyConfig](l)
		})
	})

	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble returned error: %v", err)
	}
}

func TestMustLazy_RepeatedCall_ReusesCachedValue(t *testing.T) {
	resetPendingEdges()

	callCount := 0
	p := provider.New(func(pr *provider.Provider) {
		pr.Constructor(func() *lazyConfig {
			callCount++
			return &lazyConfig{Driver: "sms"}
		})
	})

	m := module.New(func(m *module.Module) {
		m.Providers(p)
		m.Lazy(func(l *module.LazyModule) {
			first := Must[*lazyConfig](l)
			second := Must[*lazyConfig](l)
			if first != second {
				t.Fatalf("2nd Must[*lazyConfig](l) call returned a different pointer than the 1st")
			}
		})
	})

	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble returned error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("Constructor called %d times across 2 Must calls, want exactly 1", callCount)
	}
}

func TestMustLazy_NonPointerType_Panics(t *testing.T) {
	resetPendingEdges()

	m := module.New(func(m *module.Module) {
		m.Lazy(func(l *module.LazyModule) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Must[lazyConfig](l) (non-pointer) did not panic")
				}
			}()
			Must[lazyConfig](l)
		})
	})

	if _, err := m.Assemble(); err != nil {
		t.Fatalf("Assemble returned error: %v", err)
	}
}
