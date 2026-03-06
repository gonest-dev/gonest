// gonest/pipes/decorators.go
package pipes

import (
	"fmt"
	"reflect"

	"github.com/gonest-dev/gonest/core"
)

// UsePipes creates a middleware that applies pipes to request data
func UsePipes(pipes ...any) core.MiddlewareFunc {
	return func(next core.HandlerFunc) core.HandlerFunc {
		return func(ctx *core.Context) error {
			// Pipes will be applied when parameters are extracted
			// Store pipes in context for later use
			ctx.Set("__pipes", pipes)
			return next(ctx)
		}
	}
}

// Body decorator with automatic validation
func Body[T any](validate bool) func(*core.Context) (*T, error) {
	return func(ctx *core.Context) (*T, error) {
		var dto T

		if err := ctx.BindJSON(&dto); err != nil {
			return nil, fmt.Errorf("failed to bind JSON: %w", err)
		}

		if validate {
			// Check if DTO implements Validatable
			if validatable, ok := any(&dto).(Validatable); ok {
				result := validatable.Validate()

				if result.Invalid() {
					return nil, &ValidationError{Result: result}
				}
			}
		}

		return &dto, nil
	}
}

// Param decorator with pipe
func Param(name string, pipes ...any) func(*core.Context) (any, error) {
	return func(ctx *core.Context) (any, error) {
		value := ctx.Param(name)

		// Apply pipes
		var result any = value
		var err error

		for _, pipe := range pipes {
			result, err = applyPipe(pipe, result, ctx)
			if err != nil {
				return nil, err
			}
		}

		return result, nil
	}
}

// Query decorator with pipe
func Query(name string, pipes ...any) func(*core.Context) (any, error) {
	return func(ctx *core.Context) (any, error) {
		value := ctx.Query(name)

		// Apply pipes
		var result any = value
		var err error

		for _, pipe := range pipes {
			result, err = applyPipe(pipe, result, ctx)
			if err != nil {
				return nil, err
			}
		}

		return result, nil
	}
}

// applyPipe applies a pipe to a value
func applyPipe(pipe any, value any, ctx *core.Context) (any, error) {
	pipeValue := reflect.ValueOf(pipe)

	// Look for Transform method
	transformMethod := pipeValue.MethodByName("Transform")
	if !transformMethod.IsValid() {
		return nil, fmt.Errorf("pipe does not have Transform method")
	}

	// Call Transform method
	methodType := transformMethod.Type()

	// Prepare arguments
	args := make([]reflect.Value, 0)

	// Add value argument
	if methodType.NumIn() > 0 {
		valueArg := reflect.ValueOf(value)
		args = append(args, valueArg)
	}

	// Add context argument if needed
	if methodType.NumIn() > 1 {
		ctxArg := reflect.ValueOf(ctx)
		args = append(args, ctxArg)
	}

	// Call transform
	results := transformMethod.Call(args)

	if len(results) != 2 {
		return nil, fmt.Errorf("Transform method must return (value, error)")
	}

	// Check error
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}

	return results[0].Interface(), nil
}

// ValidateBody is a helper to validate body in handlers
func ValidateBody[T any](ctx *core.Context) (*T, error) {
	var dto T

	if err := ctx.BindJSON(&dto); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	// Check if *T implements Validatable (pointer receiver)
	if validatable, ok := any(&dto).(Validatable); ok {
		result := validatable.Validate()
		if result.Invalid() {
			return nil, &ValidationError{Result: result}
		}
	}

	return &dto, nil
}
