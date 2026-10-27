package app

import (
	"context"
	"fmt"
	"reflect"

	"gonest.dev/gonest/internal/inject"
	"gonest.dev/gonest/internal/logger"
	"gonest.dev/gonest/internal/module"
	"gonest.dev/gonest/internal/resolver"
)

// testAdapterFactory is set by RegisterTestAdapter -- MustNewTestApp's only
// channel to a concrete HttpAdapter. This package must NEVER import
// internal/adapter/fiber directly: that package already imports this one
// (for the HttpAdapter/Options types its own Init/RegisterRoute signatures
// require), so a direct import here would be a cycle. Instead,
// internal/adapter/fiber registers itself via this hook from its own
// init(), inverting the dependency -- same idiom database/sql's own driver
// registration uses for the identical reason (a leaf package cannot know
// its own concrete implementations without importing them back).
var testAdapterFactory func() HttpAdapter

// RegisterTestAdapter lets an HttpAdapter implementation (in practice only
// internal/adapter/fiber.FiberApp exists today) register itself as
// MustNewTestApp's default adapter. Any program that imports the top-level
// gonest package (which always imports internal/adapter/fiber, see
// gonest.FiberApp) triggers this registration before MustNewTestApp could
// possibly run. Calling MustNewTestApp without ANY adapter having
// registered itself first is a caller/build misconfiguration, not a
// runtime condition to recover from -- see MustNewTestApp's own panic.
func RegisterTestAdapter(factory func() HttpAdapter) {
	testAdapterFactory = factory
}

// TestBuilder collects overrides for MustNewTestApp's provider substitution
// (see MustOverride). Only ever obtained via the configure callback
// MustNewTestApp passes it to -- there is no exported constructor, since a
// zero-value *TestBuilder with a nil overrides map would panic on the
// first MustOverride call.
type TestBuilder struct {
	overrides map[reflect.Type]reflect.Value
}

// Override registers mockValue as the substitute for any Provider whose
// ResolvedType() exactly matches T (a pointer override) or is implemented
// by T (an interface override, reflect.Type.Implements()) -- the matching
// Provider's real Constructor never runs; mockValue is used directly
// instead (see internal/resolver.ResolveWithOverrides). T may be a
// concrete pointer or an interface. Returns b for chaining.
func (b *TestBuilder) Override[T any](mockValue T) *TestBuilder {
	t := reflect.TypeFor[T]()
	b.overrides[t] = reflect.ValueOf(mockValue)
	return b
}

// Test is NewTestApp's return value: the same fully-bootstrapped
// module tree NewApp produces (providers resolved override-aware, routes
// registered on an adapter), except no real network port is bound (no
// Listen call).
type Test struct {
	root    *module.Module
	adapter HttpAdapter
}

// Adapter returns the HttpAdapter routes were registered on.
func (a *Test) Adapter() HttpAdapter { return a.adapter }

// Close is a no-op today.
func (a *Test) Close() {}

// Inject resolves an instance of T from the test app's root module, panicking on failure.
func (a *Test) Inject[T any]() T {
	return inject.Inject[T](a)
}

// InjectAll resolves all instances of T from the test app's root module.
func (a *Test) InjectAll[T any]() []T {
	return inject.InjectAll[T](a)
}

// SafeInject resolves an instance of T from the test app's root module, returning (T, error).
func (a *Test) SafeInject[T any]() (T, error) {
	return inject.SafeInject[T](a)
}

// ResolveDirect satisfies internal/inject's directResolver interface,
// scoped to root ONLY (single-module scope, same as *controller.Controller's
// own ResolveDirect).
func (a *Test) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect([]*module.Module{a.root}, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// same single-module scope as ResolveDirect.
func (a *Test) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll([]*module.Module{a.root}, t)
}

// SafeNewTestApp runs the same 3-phase bootstrap NewApp does, returning (*Test, error).
func SafeNewTestApp(root *module.Module, configure func(*TestBuilder)) (app *Test, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", r)
			}
		}
	}()
	b := &TestBuilder{overrides: make(map[reflect.Type]reflect.Value)}
	if configure != nil {
		configure(b)
	}

	logger.SetActive(nil)
	inject.Reset()
	registerFrameworkSingletons()

	modules, err := root.Assemble()
	if err != nil {
		return nil, err
	}

	declareProviders(modules)

	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()

	if err := resolver.ResolveWithOverrides(ctx, modules, b.overrides); err != nil {
		return nil, err
	}

	declareControllers(modules)
	declareListeners(modules)
	declareSchedulers(modules)

	ownership := discoverPipelineStageOwnership(modules)
	declarePipelineStageTypes(ownership)

	if testAdapterFactory == nil {
		return nil, fmt.Errorf("gonest: no HttpAdapter registered for NewTestApp -- import gonest.dev/gonest (or gonest.dev/gonest/internal/adapter/fiber directly) before calling it")
	}
	adapter := testAdapterFactory()
	adapter.Init(Options{})
	if err := registerRoutes(adapter, root, modules); err != nil {
		return nil, err
	}

	return &Test{root: root, adapter: adapter}, nil
}

// NewTestApp runs the same 3-phase bootstrap NewApp does, panicking on error.
func NewTestApp(root *module.Module, configure func(*TestBuilder)) *Test {
	testApp, err := SafeNewTestApp(root, configure)
	if err != nil {
		panic(err)
	}
	return testApp
}
