// gonest/validator/rules/async.go
package rules

import (
	"context"
	"fmt"

	"github.com/gonest-dev/gonest/packages/validator"
)

// AsyncCustom creates an async custom validator
func AsyncCustom[T any](
	predicate func(context.Context, T) (bool, error),
	code string,
	message string,
) validator.ContextValidator[T] {
	return func(ctx context.Context, value T) *validator.FieldError {
		valid, err := predicate(ctx, value)
		if err != nil {
			return validator.
				NewFieldError("", "async_error", fmt.Sprintf("Validation error: %v", err))
		}

		if !valid {
			return validator.
				NewFieldError("", code, message)
		}

		return nil
	}
}

// AsyncUnique validates uniqueness (e.g., checking database)
func AsyncUnique[T any](
	checker func(context.Context, T) (bool, error),
	resourceName string,
) validator.ContextValidator[T] {
	return func(ctx context.Context, value T) *validator.FieldError {
		exists, err := checker(ctx, value)
		if err != nil {
			return validator.
				NewFieldError("", "async_error", fmt.Sprintf("Failed to check uniqueness: %v", err))
		}

		if exists {
			return validator.
				NewFieldError("", "unique", fmt.Sprintf("%s already exists", resourceName)).
				WithParam("resource", resourceName)
		}

		return nil
	}
}

// AsyncExists validates that a resource exists (e.g., foreign key check)
func AsyncExists[T any](
	checker func(context.Context, T) (bool, error),
	resourceName string,
) validator.ContextValidator[T] {
	return func(ctx context.Context, value T) *validator.FieldError {
		exists, err := checker(ctx, value)
		if err != nil {
			return validator.
				NewFieldError("", "async_error", fmt.Sprintf("Failed to check existence: %v", err))
		}

		if !exists {
			return validator.
				NewFieldError("", "exists", fmt.Sprintf("%s not found", resourceName)).
				WithParam("resource", resourceName)
		}

		return nil
	}
}

// AsyncValidateWith validates using an external API or service
func AsyncValidateWith[T any](
	validator func(context.Context, T) *validator.FieldError,
) validator.ContextValidator[T] {
	return validator
}

// AsyncCompare compares value with a value fetched asynchronously
func AsyncCompare[T comparable](
	fetcher func(context.Context) (T, error),
	comparison func(T, T) bool,
	code string,
	message string,
) validator.ContextValidator[T] {
	return func(ctx context.Context, value T) *validator.FieldError {
		compareWith, err := fetcher(ctx)
		if err != nil {
			return validator.
				NewFieldError("", "async_error", fmt.Sprintf("Failed to fetch comparison value: %v", err))
		}

		if !comparison(value, compareWith) {
			return validator.
				NewFieldError("", code, message).
				WithParam("expected", compareWith).
				WithParam("actual", value)
		}

		return nil
	}
}


