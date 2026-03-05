package rules

import (
	"github.com/leandroluk/gonest/validator"
)

// Required validates that a value is not zero/empty
func Required[T comparable]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		var zero T
		if value == zero {
			return validator.NewFieldError(
				"",
				string(validator.ErrorCodeREQUIRED),
				"This field is required",
			)
		}
		return nil
	}
}

// NotEmpty validates that a value is not empty (for slices, maps, strings)
func NotEmpty[T ~string | ~[]any]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if len(value) == 0 {
			return validator.NewFieldError(
				"",
				string(validator.ErrorCodeREQUIRED),
				"This field cannot be empty",
			)
		}
		return nil
	}
}

// Optional always passes (useful for composition)
func Optional[T any]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		return nil
	}
}

// Custom creates a custom validator with a predicate function
func Custom[T any](predicate func(T) bool, message string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if !predicate(value) {
			return validator.NewFieldError(
				"",
				string(validator.ErrorCodeCUSTOM),
				message,
			)
		}
		return nil
	}
}

// Must creates a validator that checks if a condition is true
func Must[T any](condition func(T) bool, code, message string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if !condition(value) {
			return validator.NewFieldError("", code, message)
		}
		return nil
	}
}

// Equal validates that value equals the expected value
func Equal[T comparable](expected T, message string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value != expected {
			err := validator.NewFieldError(
				"",
				"equal",
				message,
			)
			err.WithParam("expected", expected)
			return err
		}
		return nil
	}
}

// NotEqual validates that value doesn't equal the rejected value
func NotEqual[T comparable](rejected T, message string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value == rejected {
			err := validator.NewFieldError(
				"",
				"not_equal",
				message,
			)
			err.WithParam("rejected", rejected)
			return err
		}
		return nil
	}
}

// OneOf validates that value is one of the allowed values
func OneOf[T comparable](allowed []T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		for _, a := range allowed {
			if value == a {
				return nil
			}
		}

		err := validator.NewFieldError(
			"",
			string(validator.ErrorCodeONE_OF),
			"Value must be one of the allowed values",
		)
		err.WithParam("allowed", allowed)
		return err
	}
}

// In is an alias for OneOf
func In[T comparable](allowed []T) validator.Validator[T] {
	return OneOf(allowed)
}
