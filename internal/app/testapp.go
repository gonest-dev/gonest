package app

import (
	"context"
	"reflect"

	"github.com/gonest-dev/gonest/internal/adapter/fiber"
	"github.com/gonest-dev/gonest/internal/inject"
	"github.com/gonest-dev/gonest/internal/module"
	"github.com/gonest-dev/gonest/internal/resolver"
)

// TestBuilder collects overrides for MustNewTestApp's provider substitution
// (see MustOverride). Only ever obtained via the configure callback
// MustNewTestApp passes it to -- there is no exported constructor, since a
// zero-value *TestBuilder with a nil overrides map would panic on the
// first MustOverride call.
type TestBuilder struct {
	overrides map[reflect.Type]reflect.Value
}

// MustOverride registers mockValue as the substitute for any Provider whose
// ResolvedType() exactly matches T (a pointer override) or is implemented
// by T (an interface override, reflect.Type.Implements()) -- the matching
// Provider's real Constructor never runs; mockValue is used directly
// instead (see internal/resolver.ResolveWithOverrides). T may be a
// concrete pointer or an interface (spec.md P3 AC1).
func MustOverride[T any](b *TestBuilder, mockValue T) {
	t := reflect.TypeFor[T]()
	b.overrides[t] = reflect.ValueOf(mockValue)
}

// TestApp is MustNewTestApp's return value: the same fully-bootstrapped
// module tree NewApp produces (providers resolved override-aware, routes
// registered on an adapter), except no real network port is bound (no
// Listen call) -- see MustNewTestApp's own doc comment for the full
// 3-phase sequence this reuses.
type TestApp struct {
	root    *module.Module
	adapter HttpAdapter
}

// Adapter returns the HttpAdapter routes were registered on, for a future
// "HTTP Test Client" feature (MustRequest) to dispatch requests against.
func (a *TestApp) Adapter() HttpAdapter { return a.adapter }

// Close is a no-op today -- MustNewTestApp never binds a real network port,
// so there is nothing live to release. Exists so INSIGHT.md's `defer
// tester.Close()` call shape compiles and reads naturally, ahead of any
// future resource MustNewTestApp might need to release.
func (a *TestApp) Close() {}

// ResolveDirect satisfies internal/inject's directResolver interface,
// scoped to root ONLY (single-module scope, same as *controller.Controller's
// own ResolveDirect) -- so gonest.MustInject[T](tester) resolves exactly
// like a Controller's own MustInject would (spec.md P3 AC5).
func (a *TestApp) ResolveDirect(t reflect.Type) (reflect.Value, bool) {
	return resolver.FindDirect([]*module.Module{a.root}, t)
}

// ResolveDirectAll satisfies internal/inject's directResolver interface,
// same single-module scope as ResolveDirect.
func (a *TestApp) ResolveDirectAll(t reflect.Type) []reflect.Value {
	return resolver.FindDirectAll([]*module.Module{a.root}, t)
}

// MustNewTestApp runs the SAME 3-phase bootstrap NewApp does (see NewApp's
// own doc comment for the full sequence), except: (1) configure, if
// non-nil, receives a *TestBuilder to register MustOverride calls on
// BEFORE Stage 3 (provider resolution) runs -- a matching Provider's real
// Constructor never executes, the override's value is used directly; (2)
// the adapter is always fiber.FiberApp (v1's only real adapter -- no type
// parameter, unlike NewApp[T]) and no Listen call happens: routes are
// still registered (for a future "HTTP Test Client" feature to dispatch
// against), but no real network port is bound. Panics on any bootstrap
// error (same "Must"-prefixed convention as MustNewApp).
func MustNewTestApp(root *module.Module, configure func(*TestBuilder)) *TestApp {
	b := &TestBuilder{overrides: make(map[reflect.Type]reflect.Value)}
	if configure != nil {
		configure(b)
	}

	inject.Reset()
	registerFrameworkSingletons()

	modules, err := root.Assemble()
	if err != nil {
		panic(err)
	}

	declareProviders(modules)

	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()

	if err := resolver.ResolveWithOverrides(ctx, modules, b.overrides); err != nil {
		panic(err)
	}

	declareControllers(modules)
	declareListeners(modules)

	ownership := discoverPipelineStageOwnership(modules)
	declarePipelineStageTypes(ownership)

	adapter := newAdapter[fiber.FiberApp]()
	if err := registerRoutes(adapter, root, modules); err != nil {
		panic(err)
	}

	return &TestApp{root: root, adapter: adapter}
}
