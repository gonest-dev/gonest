// gonest/validator/rules/number.go
package rules

import (
	"golang.org/x/exp/constraints"

	"github.com/gonest-dev/gonest/validator"
)

// Min validates minimum value
func Min[T constraints.Ordered](minValue T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value < minValue {
			return validator.
				NewFieldError("", string(validator.ErrorCodeMIN), "Value is below minimum").
				WithParam("min", minValue).
				WithParam("actual", value)
		}
		return nil
	}
}

// Max validates maximum value
func Max[T constraints.Ordered](maxValue T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value > maxValue {
			return validator.
				NewFieldError("", string(validator.ErrorCodeMAX), "Value is above maximum").
				WithParam("max", maxValue).
				WithParam("actual", value)
		}
		return nil
	}
}

// Range validates value is within range [min, max]
func Range[T constraints.Ordered](minValue, maxValue T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value < minValue || value > maxValue {
			return validator.
				NewFieldError("", "range", "Value is out of range").
				WithParam("min", minValue).
				WithParam("max", maxValue).
				WithParam("actual", value)
		}
		return nil
	}
}

// Positive validates that number is positive (> 0)
func Positive[T constraints.Signed | constraints.Float]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value <= 0 {
			return validator.
				NewFieldError("", "positive", "Value must be positive")
		}
		return nil
	}
}

// Negative validates that number is negative (< 0)
func Negative[T constraints.Signed | constraints.Float]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value >= 0 {
			return validator.
				NewFieldError("", "negative", "Value must be negative")
		}
		return nil
	}
}

// NonNegative validates that number is non-negative (>= 0)
func NonNegative[T constraints.Signed | constraints.Float]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value < 0 {
			return validator.
				NewFieldError("", "non_negative", "Value must be non-negative")
		}
		return nil
	}
}

// NonPositive validates that number is non-positive (<= 0)
func NonPositive[T constraints.Signed | constraints.Float]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value > 0 {
			return validator.
				NewFieldError("", "non_positive", "Value must be non-positive")
		}
		return nil
	}
}

// GreaterThan validates value is greater than the specified value
func GreaterThan[T constraints.Ordered](threshold T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value <= threshold {
			return validator.
				NewFieldError("", "greater_than", "Value must be greater than threshold").
				WithParam("threshold", threshold).
				WithParam("actual", value)
		}
		return nil
	}
}

// LessThan validates value is less than the specified value
func LessThan[T constraints.Ordered](threshold T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value >= threshold {
			return validator.
				NewFieldError("", "less_than", "Value must be less than threshold").
				WithParam("threshold", threshold).
				WithParam("actual", value)
		}
		return nil
	}
}

// Between validates value is strictly between min and max (exclusive)
func Between[T constraints.Ordered](minValue, maxValue T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value <= minValue || value >= maxValue {
			return validator.
				NewFieldError("", "between", "Value must be between min and max (exclusive)").
				WithParam("min", minValue).
				WithParam("max", maxValue).
				WithParam("actual", value)
		}
		return nil
	}
}

// MultipleOf validates that value is a multiple of the specified number
func MultipleOf[T constraints.Integer](divisor T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value%divisor != 0 {
			return validator.
				NewFieldError("", "multiple_of", "Value must be a multiple of the specified number").
				WithParam("divisor", divisor).
				WithParam("actual", value)
		}
		return nil
	}
}
