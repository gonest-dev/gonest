// gonest/di/provider.go
package di

import (
	"context"
	"fmt"
	"reflect"
)

// ClassProvider provides instances by calling a constructor function
type ClassProvider struct {
	constructor any
	scope       Scope
	resultType  reflect.Type
}

// Compile-time interface compliance
var _ Provider = (*ClassProvider)(nil)

// NewClassProvider creates a provider from a constructor function
func NewClassProvider(constructor any, opts ...ProviderOption) (*ClassProvider, error) {
	t := reflect.TypeOf(constructor)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("constructor must be a function")
	}

	if t.NumOut() == 0 {
		return nil, fmt.Errorf("constructor must return at least one value")
	}

	// Get return type (first return value)
	resultType := t.Out(0)

	options := &ProviderOptions{
		Scope: ScopeSINGLETON, // Default scope
	}
	for _, opt := range opts {
		opt(options)
	}

	return &ClassProvider{
		constructor: constructor,
		scope:       options.Scope,
		resultType:  resultType,
	}, nil
}

func (p *ClassProvider) Provide(ctx context.Context, container *Container) (any, error) {
	fn := reflect.ValueOf(p.constructor)
	fnType := fn.Type()

	// Resolve dependencies
	args := make([]reflect.Value, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		argType := fnType.In(i)

		// Special case: context.Context
		if argType.String() == "context.Context" {
			args[i] = reflect.ValueOf(ctx)
			continue
		}

		// Resolve from container
		dep, err := container.Resolve(ctx, argType)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dependency %s: %w", argType, err)
		}
		args[i] = reflect.ValueOf(dep)
	}

	// Call constructor
	results := fn.Call(args)

	// Handle error return (if constructor returns (T, error))
	if len(results) == 2 {
		if err, ok := results[1].Interface().(error); ok && err != nil {
			return nil, err
		}
	}

	return results[0].Interface(), nil
}

func (p *ClassProvider) Scope() Scope {
	return p.scope
}

func (p *ClassProvider) Type() reflect.Type {
	return p.resultType
}

// ValueProvider provides a pre-existing instance
type ValueProvider struct {
	value any
	scope Scope
}

var _ Provider = (*ValueProvider)(nil)

// NewValueProvider creates a provider from an existing value
func NewValueProvider(value any, opts ...ProviderOption) *ValueProvider {
	options := &ProviderOptions{
		Scope: ScopeSINGLETON,
	}
	for _, opt := range opts {
		opt(options)
	}

	return &ValueProvider{
		value: value,
		scope: options.Scope,
	}
}

func (p *ValueProvider) Provide(_ context.Context, _ *Container) (any, error) {
	return p.value, nil
}

func (p *ValueProvider) Scope() Scope {
	return p.scope
}

func (p *ValueProvider) Type() reflect.Type {
	return reflect.TypeOf(p.value)
}

// FactoryProvider provides instances using a factory function
type FactoryProvider struct {
	factory    any
	scope      Scope
	resultType reflect.Type
}

var _ Provider = (*FactoryProvider)(nil)

// NewFactoryProvider creates a provider from a factory function
// Factory signature: func(ctx context.Context, container *Container) (T, error)
func NewFactoryProvider(factory any, opts ...ProviderOption) (*FactoryProvider, error) {
	t := reflect.TypeOf(factory)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("factory must be a function")
	}

	if t.NumOut() == 0 {
		return nil, fmt.Errorf("factory must return at least one value")
	}

	resultType := t.Out(0)

	options := &ProviderOptions{
		Scope: ScopeSINGLETON,
	}
	for _, opt := range opts {
		opt(options)
	}

	return &FactoryProvider{
		factory:    factory,
		scope:      options.Scope,
		resultType: resultType,
	}, nil
}

func (p *FactoryProvider) Provide(ctx context.Context, container *Container) (any, error) {
	fn := reflect.ValueOf(p.factory)
	fnType := fn.Type()

	// Build arguments
	args := make([]reflect.Value, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		argType := fnType.In(i)

		switch argType.String() {
		case "context.Context":
			args[i] = reflect.ValueOf(ctx)
		case "*di.Container":
			args[i] = reflect.ValueOf(container)
		default:
			// Try to resolve from container
			dep, err := container.Resolve(ctx, argType)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve dependency %s: %w", argType, err)
			}
			args[i] = reflect.ValueOf(dep)
		}
	}

	// Call factory
	results := fn.Call(args)

	// Handle error return
	if len(results) == 2 {
		if err, ok := results[1].Interface().(error); ok && err != nil {
			return nil, err
		}
	}

	return results[0].Interface(), nil
}

func (p *FactoryProvider) Scope() Scope {
	return p.scope
}

func (p *FactoryProvider) Type() reflect.Type {
	return p.resultType
}

// AsyncProvider provides instances asynchronously
type AsyncProvider struct {
	factory    any
	scope      Scope
	resultType reflect.Type
}

var _ Provider = (*AsyncProvider)(nil)

// NewAsyncProvider creates an async provider
// Factory signature: func(ctx context.Context) (T, error)
func NewAsyncProvider(factory any, opts ...ProviderOption) (*AsyncProvider, error) {
	t := reflect.TypeOf(factory)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("factory must be a function")
	}

	if t.NumOut() < 1 {
		return nil, fmt.Errorf("factory must return at least one value")
	}

	resultType := t.Out(0)

	options := &ProviderOptions{
		Scope: ScopeSINGLETON,
	}
	for _, opt := range opts {
		opt(options)
	}

	return &AsyncProvider{
		factory:    factory,
		scope:      options.Scope,
		resultType: resultType,
	}, nil
}

func (p *AsyncProvider) Provide(ctx context.Context, _ *Container) (any, error) {
	fn := reflect.ValueOf(p.factory)

	// Call async factory
	args := []reflect.Value{reflect.ValueOf(ctx)}
	results := fn.Call(args)

	// Handle error
	if len(results) == 2 {
		if err, ok := results[1].Interface().(error); ok && err != nil {
			return nil, err
		}
	}

	return results[0].Interface(), nil
}

func (p *AsyncProvider) Scope() Scope {
	return p.scope
}

func (p *AsyncProvider) Type() reflect.Type {
	return p.resultType
}


