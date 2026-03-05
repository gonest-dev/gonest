package rules

import (
	"golang.org/x/exp/constraints"

	"github.com/gonest-dev/gonest/validator"
)

// Min validates minimum value
func Min[T constraints.Ordered](min T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value < min {
			err := validator.NewFieldError(
				"",
				string(validator.ErrorCodeMIN),
				"Value is below minimum",
			)
			err.WithParam("min", min)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// Max validates maximum value
func Max[T constraints.Ordered](max T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value > max {
			err := validator.NewFieldError(
				"",
				string(validator.ErrorCodeMAX),
				"Value is above maximum",
			)
			err.WithParam("max", max)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// Range validates value is within range [min, max]
func Range[T constraints.Ordered](min, max T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value < min || value > max {
			err := validator.NewFieldError(
				"",
				"range",
				"Value is out of range",
			)
			err.WithParam("min", min)
			err.WithParam("max", max)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// Positive validates that number is positive (> 0)
func Positive[T constraints.Signed | constraints.Float]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value <= 0 {
			return validator.NewFieldError(
				"",
				"positive",
				"Value must be positive",
			)
		}
		return nil
	}
}

// Negative validates that number is negative (< 0)
func Negative[T constraints.Signed | constraints.Float]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value >= 0 {
			return validator.NewFieldError(
				"",
				"negative",
				"Value must be negative",
			)
		}
		return nil
	}
}

// NonNegative validates that number is non-negative (>= 0)
func NonNegative[T constraints.Signed | constraints.Float]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value < 0 {
			return validator.NewFieldError(
				"",
				"non_negative",
				"Value must be non-negative",
			)
		}
		return nil
	}
}

// NonPositive validates that number is non-positive (<= 0)
func NonPositive[T constraints.Signed | constraints.Float]() validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value > 0 {
			return validator.NewFieldError(
				"",
				"non_positive",
				"Value must be non-positive",
			)
		}
		return nil
	}
}

// GreaterThan validates value is greater than the specified value
func GreaterThan[T constraints.Ordered](threshold T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value <= threshold {
			err := validator.NewFieldError(
				"",
				"greater_than",
				"Value must be greater than threshold",
			)
			err.WithParam("threshold", threshold)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// LessThan validates value is less than the specified value
func LessThan[T constraints.Ordered](threshold T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value >= threshold {
			err := validator.NewFieldError(
				"",
				"less_than",
				"Value must be less than threshold",
			)
			err.WithParam("threshold", threshold)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// Between validates value is strictly between min and max (exclusive)
func Between[T constraints.Ordered](min, max T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value <= min || value >= max {
			err := validator.NewFieldError(
				"",
				"between",
				"Value must be between min and max (exclusive)",
			)
			err.WithParam("min", min)
			err.WithParam("max", max)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// MultipleOf validates that value is a multiple of the specified number
func MultipleOf[T constraints.Integer](divisor T) validator.Validator[T] {
	return func(value T) *validator.FieldError {
		if value%divisor != 0 {
			err := validator.NewFieldError(
				"",
				"multiple_of",
				"Value must be a multiple of the specified number",
			)
			err.WithParam("divisor", divisor)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}
