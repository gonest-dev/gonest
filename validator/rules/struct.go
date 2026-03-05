package rules

import (
	"context"

	"github.com/leandroluk/gonest/validator"
)

// Validatable interface for structs that can validate themselves
type Validatable interface {
	Validate() *validator.ValidationResult
}

// AsyncValidatable interface for structs with async validation
type AsyncValidatable interface {
	ValidateAsync(ctx context.Context) *validator.ValidationResult
}

// ValidStruct validates a nested struct that implements Validatable
func ValidStruct[T Validatable]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		result := value.Validate()

		if result.Invalid() {
			// Return first error from nested validation
			if first := result.First(); first != nil {
				return first
			}
		}

		return nil
	}
}

// ValidStructPtr validates a nested struct pointer
func ValidStructPtr[T any]() validator.Validator[*T] {
	return func(value *T) *validator.FieldError {
		if value == nil {
			return validator.NewFieldError(
				"",
				"required",
				"Value cannot be nil",
			)
		}

		// Check if value implements Validatable
		if validatable, ok := any(value).(Validatable); ok {
			result := validatable.Validate()

			if result.Invalid() {
				if first := result.First(); first != nil {
					return first
				}
			}
		}

		return nil
	}
}

// ValidStructAsync validates a nested struct with async validation
func ValidStructAsync[T AsyncValidatable]() validator.ContextValidator[T] {
	return func(ctx context.Context, value T) *validator.FieldError {
		result := value.ValidateAsync(ctx)

		if result.Invalid() {
			if first := result.First(); first != nil {
				return first
			}
		}

		return nil
	}
}

// ValidStructPtrAsync validates a nested struct pointer with async validation
func ValidStructPtrAsync[T AsyncValidatable]() validator.ContextValidator[*T] {
	return func(ctx context.Context, value *T) *validator.FieldError {
		if value == nil {
			return validator.NewFieldError(
				"",
				"required",
				"Value cannot be nil",
			)
		}

		result := (*value).ValidateAsync(ctx)

		if result.Invalid() {
			if first := result.First(); first != nil {
				return first
			}
		}

		return nil
	}
}

// StructField validates a specific field of a struct using a getter
func StructField[T any, F any](
	getter func(T) F,
	fieldValidator validator.Validator[F],
) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		fieldValue := getter(value)
		return fieldValidator(fieldValue)
	}
}

// StructHas validates that struct has a non-zero field
func StructHas[T any, F comparable](
	getter func(T) F,
	fieldName string,
) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		fieldValue := getter(value)
		var zero F

		if fieldValue == zero {
			err := validator.NewFieldError(
				"",
				"struct_has",
				"Required field is missing",
			)
			err.WithParam("field", fieldName)
			return err
		}

		return nil
	}
}
