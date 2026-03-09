package tester

import (
	"context"
	"reflect"

	"github.com/gonest-dev/gonest/core/common"
	"github.com/gonest-dev/gonest/core/di"
)

// ModuleBuilder helps build a testing module with overrides
type ModuleBuilder struct {
	rootModule common.Module
	overrides  map[reflect.Type]di.Provider
}

// CreateModule creates a new testing module builder
func CreateModule(module common.Module) *ModuleBuilder {
	return &ModuleBuilder{
		rootModule: module,
		overrides:  make(map[reflect.Type]di.Provider),
	}
}

// OverrideProvider starts the override process for a provider
func (b *ModuleBuilder) OverrideProvider(token any) *OverrideBuilder {
	t := reflect.TypeOf(token)
	if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		// Keep as is
	} else if t.Kind() == reflect.Interface {
		// Keep as is
	}

	return &OverrideBuilder{
		builder: b,
		token:   t,
	}
}

// Compile compiles the testing module into a Module
func (b *ModuleBuilder) Compile() (*Module, error) {
	// We'll create a special NestApplication just for testing
	// that allows us to access the container directly.

	app := common.NestFactory{}.Create(b.rootModule)
	container := app.GetContainer()

	// Apply overrides
	for _, provider := range b.overrides {
		if err := container.Override(provider, ""); err != nil {
			return nil, err
		}
	}

	return &Module{
		app: app,
	}, nil
}

// Module represents a compiled testing module
type Module struct {
	app *common.NestApplication
}

// Get retrieves an instance from the testing module
func (m *Module) Get(token any) (any, error) {
	t := reflect.TypeOf(token)
	return m.app.GetContainer().Resolve(context.Background(), t)
}

// CreateNestApplication creates a full NestApplication from the testing module
func (m *Module) CreateNestApplication(opts ...common.ApplicationOption) *common.NestApplication {
	// The application is already created during Compile, we just return it or
	// allow further configuration if needed.
	return m.app
}

// OverrideBuilder helps define how a provider is overridden
type OverrideBuilder struct {
	builder *ModuleBuilder
	token   reflect.Type
}

// UseValue overrides the provider with a constant value
func (o *OverrideBuilder) UseValue(val any) *ModuleBuilder {
	o.builder.overrides[o.token] = di.NewValueProvider(val)
	return o.builder
}

// UseClass overrides the provider with a new class instance
func (o *OverrideBuilder) UseClass(class any) *ModuleBuilder {
	// Note: di.RegisterType logic would be needed here if we want full autowire
	// For now, simpler value or factory might be more common in tests.
	return o.builder
}

// UseFactory overrides the provider with a factory function
func (o *OverrideBuilder) UseFactory(factory any) *ModuleBuilder {
	provider, _ := di.NewFactoryProvider(factory, di.Singleton())
	o.builder.overrides[o.token] = provider
	return o.builder
}
