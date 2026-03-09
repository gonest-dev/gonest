// gonest/di/injector.go
package di

import (
	"context"
	"fmt"
	"reflect"
)

// Injector provides automatic dependency injection
type Injector struct {
	container *Container
}

// NewInjector creates a new injector
func NewInjector(container *Container) *Injector {
	return &Injector{
		container: container,
	}
}

// Inject injects dependencies into a struct's fields
// Fields marked with `inject:""` tag will be injected
func (i *Injector) Inject(ctx context.Context, target any) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer {
		return fmt.Errorf("target must be a pointer")
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct")
	}

	t := v.Type()
	for idx := 0; idx < t.NumField(); idx++ {
		field := t.Field(idx)
		fieldValue := v.Field(idx)

		// Check for inject tag
		injectTag, ok := field.Tag.Lookup("inject")
		if !ok || injectTag == "-" {
			continue
		}

		if !fieldValue.CanSet() {
			continue
		}

		// Get the name from tag (optional)
		name := ""
		if injectTag != "" && injectTag != "-" {
			name = injectTag
		}

		// Resolve dependency
		dep, err := i.container.ResolveNamed(ctx, field.Type, name)
		if err != nil {
			return fmt.Errorf("failed to inject field %s: %w", field.Name, err)
		}

		fieldValue.Set(reflect.ValueOf(dep))
	}

	return nil
}

// InjectMethod calls a method with injected dependencies
func (i *Injector) InjectMethod(ctx context.Context, target any, methodName string) ([]any, error) {
	v := reflect.ValueOf(target)
	method := v.MethodByName(methodName)

	if !method.IsValid() {
		return nil, fmt.Errorf("method %s not found", methodName)
	}

	methodType := method.Type()
	args := make([]reflect.Value, methodType.NumIn())

	for idx := 0; idx < methodType.NumIn(); idx++ {
		argType := methodType.In(idx)

		// Special case: context.Context
		if argType.String() == "context.Context" {
			args[idx] = reflect.ValueOf(ctx)
			continue
		}

		// Resolve from container
		dep, err := i.container.Resolve(ctx, argType)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve arg %d: %w", idx, err)
		}
		args[idx] = reflect.ValueOf(dep)
	}

	results := method.Call(args)

	// Convert results to []any
	resultValues := make([]any, len(results))
	for idx, result := range results {
		resultValues[idx] = result.Interface()
	}

	return resultValues, nil
}

// Call calls a function with injected dependencies
func (i *Injector) Call(ctx context.Context, fn any) ([]any, error) {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return nil, fmt.Errorf("not a function")
	}

	fnType := v.Type()
	args := make([]reflect.Value, fnType.NumIn())

	for idx := 0; idx < fnType.NumIn(); idx++ {
		argType := fnType.In(idx)

		// Special case: context.Context
		if argType.String() == "context.Context" {
			args[idx] = reflect.ValueOf(ctx)
			continue
		}

		// Resolve from container
		dep, err := i.container.Resolve(ctx, argType)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve arg %d (%s): %w", idx, argType, err)
		}
		args[idx] = reflect.ValueOf(dep)
	}

	results := v.Call(args)

	// Convert to []any
	resultValues := make([]any, len(results))
	for idx, result := range results {
		resultValues[idx] = result.Interface()
	}

	return resultValues, nil
}

// AutoWire automatically wires dependencies for a struct
// Returns a new instance with all dependencies injected
func (i *Injector) AutoWire(ctx context.Context, instance any) (any, error) {
	t := reflect.TypeOf(instance)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Create new instance
	v := reflect.New(t)

	// Inject dependencies
	if err := i.Inject(ctx, v.Interface()); err != nil {
		return nil, err
	}

	return v.Interface(), nil
}


