// gonest/pipes/validation_pipe.go
package pipes

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/gonest-dev/gonest/core"
	"github.com/gonest-dev/gonest/validator"
)

// Validatable interface for DTOs that can validate themselves
type Validatable interface {
	Validate() *validator.ValidationResult
}

// ValidationPipe validates and transforms input data
type ValidationPipe struct {
	options *ValidationPipeOptions
}

// NewValidationPipe creates a new validation pipe
func NewValidationPipe(opts ...*ValidationPipeOptions) *ValidationPipe {
	options := DefaultValidationPipeOptions()

	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	return &ValidationPipe{
		options: options,
	}
}

// Transform validates and transforms the value
func (vp *ValidationPipe) Transform(value any, _ *core.Context, targetType reflect.Type) (any, error) {
	// If target type is provided, transform to that type
	if targetType != nil {
		transformed, err := vp.transformToType(value, targetType)
		if err != nil {
			return nil, err
		}
		value = transformed
	}

	// Validate if the value implements Validatable
	if validatable, ok := value.(Validatable); ok {
		result := validatable.Validate()

		if result.Invalid() {
			if vp.options.DisableErrorMessages {
				return nil, fmt.Errorf("validation failed")
			}

			return nil, &ValidationError{
				Result: result,
			}
		}
	}

	return value, nil
}

// transformToType transforms value to target type
func (vp *ValidationPipe) transformToType(value any, targetType reflect.Type) (any, error) {
	// If already correct type, return as is
	if reflect.TypeOf(value) == targetType {
		return value, nil
	}

	// Marshal and unmarshal to transform
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value: %w", err)
	}

	// Create new instance of target type
	result := reflect.New(targetType).Interface()

	if err := json.Unmarshal(data, result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to target type: %w", err)
	}

	// Return the dereferenced value
	return reflect.ValueOf(result).Elem().Interface(), nil
}

// ValidationError wraps validation result
type ValidationError struct {
	Result *validator.ValidationResult
}

func (e *ValidationError) Error() string {
	if e.Result.Count() > 0 {
		return e.Result.First().Error()
	}
	return "validation failed"
}

// ToJSON returns JSON representation of validation errors
func (e *ValidationError) ToJSON() map[string]any {
	return e.Result.ToJSON()
}
