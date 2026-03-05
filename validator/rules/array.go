package rules

import (
	"github.com/gonest-dev/gonest/validator"
)

// ArrayMinSize validates minimum array size
func ArrayMinSize[T any](min int) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if len(value) < min {
			err := validator.NewFieldError(
				"",
				"array_min_size",
				"Array is too small",
			)
			err.WithParam("min", min)
			err.WithParam("actual", len(value))
			return err
		}
		return nil
	}
}

// ArrayMaxSize validates maximum array size
func ArrayMaxSize[T any](max int) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if len(value) > max {
			err := validator.NewFieldError(
				"",
				"array_max_size",
				"Array is too large",
			)
			err.WithParam("max", max)
			err.WithParam("actual", len(value))
			return err
		}
		return nil
	}
}

// ArraySize validates exact array size
func ArraySize[T any](size int) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if len(value) != size {
			err := validator.NewFieldError(
				"",
				"array_size",
				"Array must have exact size",
			)
			err.WithParam("expected", size)
			err.WithParam("actual", len(value))
			return err
		}
		return nil
	}
}

// ArrayNotEmpty validates that array is not empty
func ArrayNotEmpty[T any]() validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if len(value) == 0 {
			return validator.NewFieldError(
				"",
				"array_not_empty",
				"Array cannot be empty",
			)
		}
		return nil
	}
}

// ArrayUnique validates that all array elements are unique
func ArrayUnique[T comparable]() validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		seen := make(map[T]bool)
		for i, item := range value {
			if seen[item] {
				err := validator.NewFieldError(
					"",
					"array_unique",
					"Array contains duplicate values",
				)
				err.WithParam("index", i)
				err.WithParam("value", item)
				return err
			}
			seen[item] = true
		}
		return nil
	}
}

// ArrayContains validates that array contains a specific value
func ArrayContains[T comparable](target T) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		for _, item := range value {
			if item == target {
				return nil
			}
		}

		err := validator.NewFieldError(
			"",
			"array_contains",
			"Array must contain the specified value",
		)
		err.WithParam("target", target)
		return err
	}
}

// ArrayDoesNotContain validates that array doesn't contain a specific value
func ArrayDoesNotContain[T comparable](forbidden T) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		for i, item := range value {
			if item == forbidden {
				err := validator.NewFieldError(
					"",
					"array_does_not_contain",
					"Array must not contain the specified value",
				)
				err.WithParam("forbidden", forbidden)
				err.WithParam("index", i)
				return err
			}
		}
		return nil
	}
}

// ArrayEvery validates that every element passes the predicate
func ArrayEvery[T any](predicate func(T) bool, message string) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		for i, item := range value {
			if !predicate(item) {
				err := validator.NewFieldError(
					"",
					"array_every",
					message,
				)
				err.WithParam("index", i)
				err.WithParam("value", item)
				return err
			}
		}
		return nil
	}
}

// ArraySome validates that at least one element passes the predicate
func ArraySome[T any](predicate func(T) bool, message string) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		for _, item := range value {
			if predicate(item) {
				return nil
			}
		}

		return validator.NewFieldError(
			"",
			"array_some",
			message,
		)
	}
}

// ArrayNone validates that no element passes the predicate
func ArrayNone[T any](predicate func(T) bool, message string) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		for i, item := range value {
			if predicate(item) {
				err := validator.NewFieldError(
					"",
					"array_none",
					message,
				)
				err.WithParam("index", i)
				err.WithParam("value", item)
				return err
			}
		}
		return nil
	}
}

// ArrayEach validates each element with a validator
func ArrayEach[T any](itemValidator validator.Validator[T]) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		for i, item := range value {
			if err := itemValidator(item); err != nil {
				err.WithParam("index", i)
				return err
			}
		}
		return nil
	}
}
