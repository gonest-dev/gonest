// gonest/validator/rules/comparison.go
package rules

import (
	"golang.org/x/exp/constraints"

	"github.com/gonest-dev/gonest/validator"
)

// EqualTo validates that value equals another value
func EqualTo[T comparable](other T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value != other {
			return validator.
				NewFieldError("", "equal_to", "Value must equal the expected value").
				WithParam("expected", other).
				WithParam("actual", value)
		}
		return nil
	}
}

// NotEqualTo validates that value doesn't equal another value
func NotEqualTo[T comparable](other T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value == other {
			return validator.
				NewFieldError("", "not_equal_to", "Value must not equal the rejected value").
				WithParam("rejected", other)
		}
		return nil
	}
}

// GreaterThanOrEqual validates value >= other
func GreaterThanOrEqual[T constraints.Ordered](other T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value < other {
			return validator.
				NewFieldError("", "greater_than_or_equal", "Value must be greater than or equal to threshold").
				WithParam("threshold", other).
				WithParam("actual", value)
		}
		return nil
	}
}

// LessThanOrEqual validates value <= other
func LessThanOrEqual[T constraints.Ordered](other T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value > other {
			return validator.
				NewFieldError("", "less_than_or_equal", "Value must be less than or equal to threshold").
				WithParam("threshold", other).
				WithParam("actual", value)
		}
		return nil
	}
}

// SameAs validates that two values are the same (for password confirmation)
func SameAs[T comparable](getter func() T, fieldName string) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		other := getter()
		if value != other {
			return validator.
				NewFieldError("", "same_as", "Values must match").
				WithParam("field", fieldName)
		}
		return nil
	}
}

// DifferentFrom validates that value is different from others
func DifferentFrom[T comparable](others ...T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		for _, other := range others {
			if value == other {
				return validator.
					NewFieldError("", "different_from", "Value must be different from specified values").
					WithParam("forbidden", others)
			}
		}
		return nil
	}
}

// NotIn validates that value is not in a list
func NotIn[T comparable](forbidden []T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		for _, item := range forbidden {
			if value == item {
				return validator.
					NewFieldError("", "not_in", "Value must not be in the forbidden list").
					WithParam("forbidden", forbidden)
			}
		}
		return nil
	}
}

// InRange validates value is in range (inclusive)
func InRange[T constraints.Ordered](ranges ...[2]T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		for _, r := range ranges {
			if value >= r[0] && value <= r[1] {
				return nil
			}
		}

		return validator.
			NewFieldError("", "in_range", "Value must be in one of the specified ranges").
			WithParam("ranges", ranges).
			WithParam("actual", value)
	}
}

// NotInRange validates value is not in range (exclusive)
func NotInRange[T constraints.Ordered](ranges ...[2]T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		for _, r := range ranges {
			if value >= r[0] && value <= r[1] {
				return validator.
					NewFieldError("", "not_in_range", "Value must not be in the forbidden ranges").
					WithParam("ranges", ranges).
					WithParam("actual", value)
			}
		}
		return nil
	}
}

// Compare validates using a custom comparison function
func Compare[T any](
	compareTo T,
	comparison func(value, other T) bool,
	code, message string,
) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if !comparison(value, compareTo) {
			return validator.
				NewFieldError("", code, message).
				WithParam("expected", compareTo).
				WithParam("actual", value)
		}
		return nil
	}
}

// When conditionally applies a validator
func When[T any](
	condition func(T) bool,
	thenValidator validator.Validator[T],
) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if condition(value) {
			return thenValidator(value)
		}
		return nil
	}
}

// Unless conditionally applies a validator (inverse of When)
func Unless[T any](
	condition func(T) bool,
	elseValidator validator.Validator[T],
) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if !condition(value) {
			return elseValidator(value)
		}
		return nil
	}
}
