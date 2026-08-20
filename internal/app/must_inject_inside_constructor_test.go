package app

import (
	"strings"
	"testing"

	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/provider"
)

// mustInjectInsideConstructorDep is a minimal target type for
// inject.Must[T] in TestMustInject_CalledInsideConstructor_* below -- its
// content does not matter, only its pointer identity.
type mustInjectInsideConstructorDep struct{}

type mustInjectInsideConstructorConsumer struct{}

// TestMustInject_CalledInsideConstructor_ProducesResolveError proves the
// resolving-phase guard (internal/inject's resolving flag, set for the
// exact duration of Stage 3): a Provider whose Constructor calls
// inject.Must[T](p) from INSIDE the Constructor closure itself -- instead
// of the builder fn, before Constructor(...) is even registered -- must
// fail loudly instead of silently returning a placeholder Stage 3 never
// fills in. callConstructor's own recover() converts the panic into a
// regular error (same as any other Constructor panic), so New() returns a
// non-nil error rather than propagating a raw Go panic.
func TestMustInject_CalledInsideConstructor_ProducesResolveError(t *testing.T) {
	depProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectInsideConstructorDep { return &mustInjectInsideConstructorDep{} })
	})

	consumerProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectInsideConstructorConsumer {
			// WRONG on purpose: MustInject called from inside Constructor
			// instead of the builder fn above it -- the exact mistake the
			// resolving-phase guard exists to catch.
			_ = inject.Must[*mustInjectInsideConstructorDep](p)
			return &mustInjectInsideConstructorConsumer{}
		})
	})

	root := module.New(func(m *module.Module) {
		m.Providers(depProvider, consumerProvider)
	})

	app, err := New[recordingFakeAdapter](root, Options{})
	if err == nil {
		t.Fatal("New() returned nil error, want an error naming Constructor-time MustInject misuse")
	}
	if app != nil {
		t.Fatal("New() returned a non-nil *App alongside a non-nil error")
	}
	const want = "MustInject[*app.mustInjectInsideConstructorDep] called from inside a Constructor"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want it to contain %q", err.Error(), want)
	}
}

// TestMustNewApp_CalledInsideConstructor_Panics proves the same failure
// through MustNewApp's own panic-on-error wrapper.
func TestMustNewApp_CalledInsideConstructor_Panics(t *testing.T) {
	depProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectInsideConstructorDep { return &mustInjectInsideConstructorDep{} })
	})

	consumerProvider := provider.New(func(p *provider.Provider) {
		p.Constructor(func() *mustInjectInsideConstructorConsumer {
			_ = inject.Must[*mustInjectInsideConstructorDep](p)
			return &mustInjectInsideConstructorConsumer{}
		})
	})

	root := module.New(func(m *module.Module) {
		m.Providers(depProvider, consumerProvider)
	})

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("MustNewApp() did not panic, want a panic naming Constructor-time MustInject misuse")
		}
	}()

	MustNewApp[recordingFakeAdapter](root, Options{})
}
