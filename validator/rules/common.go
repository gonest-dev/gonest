package rules

import (
	"slices"

	"github.com/gonest-dev/gonest/validator"
)

// Required validates that a value is not zero/empty
func Required[T comparable]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		var zero T
		if value == zero {
			return validator.
				NewFieldError("", string(validator.ErrorCodeREQUIRED), "This field is required")
		}
		return nil
	}
}

// NotEmpty validates that a value is not empty (for slices, maps, strings)
func NotEmpty[T ~string | ~[]any]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if len(value) == 0 {
			return validator.
				NewFieldError("", string(validator.ErrorCodeREQUIRED), "This field cannot be empty")
		}
		return nil
	}
}

// Optional always passes (useful for composition)
func Optional[T any]() validator.Validator[T] {
	return func(_ T) *validator.FieldError {
		return nil
	}
}

// Custom creates a custom validator with a predicate function
func Custom[T any](predicate func(T) bool, message string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if !predicate(value) {
			return validator.
				NewFieldError("", string(validator.ErrorCodeCUSTOM), message)
		}
		return nil
	}
}

// Must creates a validator that checks if a condition is true
func Must[T any](condition func(T) bool, code, message string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if !condition(value) {
			return validator.
				NewFieldError("", code, message)
		}
		return nil
	}
}

// Equal validates that value equals the expected value
func Equal[T comparable](expected T, message string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value != expected {
			return validator.
				NewFieldError("", "equal", message).
				WithParam("expected", expected)
		}
		return nil
	}
}

// NotEqual validates that value doesn't equal the rejected value
func NotEqual[T comparable](rejected T, message string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value == rejected {
			return validator.
				NewFieldError("", "not_equal", message).
				WithParam("rejected", rejected)
		}
		return nil
	}
}

// OneOf validates that value is one of the allowed values
func OneOf[T comparable](allowed []T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if slices.Contains(allowed, value) {
			return nil
		}

		return validator.
			NewFieldError("", string(validator.ErrorCodeONEOF), "Value must be one of the allowed values").
			WithParam("allowed", allowed)
	}
}

// In is an alias for OneOf
func In[T comparable](allowed []T) validator.Validator[T] {
	return OneOf(allowed)
}
