// gonest/validator/rules/array.go
package rules

import (
	"slices"

	"github.com/gonest-dev/gonest/validator"
)

// ArrayMinSize validates minimum array size
func ArrayMinSize[T any](minSize int) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if len(value) < minSize {
			return validator.
				NewFieldError("", "array_min_size", "Array is too small").
				WithParam("min", minSize).
				WithParam("actual", len(value))
		}
		return nil
	}
}

// ArrayMaxSize validates maximum array size
func ArrayMaxSize[T any](maxSize int) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if len(value) > maxSize {
			return validator.
				NewFieldError("", "array_max_size", "Array is too large").
				WithParam("max", maxSize).
				WithParam("actual", len(value))
		}
		return nil
	}
}

// ArraySize validates exact array size
func ArraySize[T any](size int) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if len(value) != size {
			return validator.
				NewFieldError("", "array_size", "Array must have exact size").
				WithParam("expected", size).
				WithParam("actual", len(value))
		}
		return nil
	}
}

// ArrayNotEmpty validates that array is not empty
func ArrayNotEmpty[T any]() validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if len(value) == 0 {
			return validator.
				NewFieldError("", "array_not_empty", "Array cannot be empty")
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
				return validator.
					NewFieldError("", "array_unique", "Array contains duplicate values").
					WithParam("index", i).
					WithParam("value", item)
			}
			seen[item] = true
		}
		return nil
	}
}

// ArrayContains validates that array contains a specific value
func ArrayContains[T comparable](target T) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if slices.Contains(value, target) {
			return nil
		}

		return validator.
			NewFieldError("", "array_contains", "Array must contain the specified value").
			WithParam("target", target)
	}
}

// ArrayDoesNotContain validates that array doesn't contain a specific value
func ArrayDoesNotContain[T comparable](forbidden T) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		for i, item := range value {
			if item == forbidden {
				return validator.
					NewFieldError("", "array_does_not_contain", "Array must not contain the specified value").
					WithParam("forbidden", forbidden).
					WithParam("index", i)
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
				return validator.
					NewFieldError("", "array_every", message).
					WithParam("index", i).
					WithParam("value", item)
			}
		}
		return nil
	}
}

// ArraySome validates that at least one element passes the predicate
func ArraySome[T any](predicate func(T) bool, message string) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		if slices.ContainsFunc(value, predicate) {
			return nil
		}

		return validator.
			NewFieldError("", "array_some", message)
	}
}

// ArrayNone validates that no element passes the predicate
func ArrayNone[T any](predicate func(T) bool, message string) validator.Validator[[]T] {
	return func(value []T) *validator.FieldError {
		for i, item := range value {
			if predicate(item) {
				return validator.
					NewFieldError("", "array_none", message).
					WithParam("index", i).
					WithParam("value", item)
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
				return err.WithParam("index", i)
			}
		}
		return nil
	}
}
