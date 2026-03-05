package exceptions

import (
	"github.com/leandroluk/gonest/core"
)

// NotFoundExceptionFilter handles 404 errors
type NotFoundExceptionFilter struct{}

func NewNotFoundExceptionFilter() *NotFoundExceptionFilter {
	return &NotFoundExceptionFilter{}
}

func (f *NotFoundExceptionFilter) Catch(err error, ctx *core.Context) error {
	if httpErr, ok := err.(*HttpException); ok {
		if httpErr.StatusCode == 404 {
			return ctx.JSON(404, map[string]any{
				"statusCode": 404,
				"message":    "Resource not found",
				"path":       ctx.Get("path"),
			})
		}
	}

	// Not a 404, pass through
	return err
}

// ValidationExceptionFilter handles validation errors
type ValidationExceptionFilter struct{}

func NewValidationExceptionFilter() *ValidationExceptionFilter {
	return &ValidationExceptionFilter{}
}

func (f *ValidationExceptionFilter) Catch(err error, ctx *core.Context) error {
	if validationErr, ok := err.(*ValidationException); ok {
		return ctx.JSON(400, validationErr.ToJSON())
	}

	// Not a validation error, pass through
	return err
}

// UnauthorizedExceptionFilter handles 401 errors
type UnauthorizedExceptionFilter struct{}

func NewUnauthorizedExceptionFilter() *UnauthorizedExceptionFilter {
	return &UnauthorizedExceptionFilter{}
}

func (f *UnauthorizedExceptionFilter) Catch(err error, ctx *core.Context) error {
	if httpErr, ok := err.(*HttpException); ok {
		if httpErr.StatusCode == 401 {
			return ctx.JSON(401, map[string]any{
				"statusCode": 401,
				"message":    "Unauthorized",
				"hint":       "Please provide valid authentication credentials",
			})
		}
	}

	return err
}

// ForbiddenExceptionFilter handles 403 errors
type ForbiddenExceptionFilter struct{}

func NewForbiddenExceptionFilter() *ForbiddenExceptionFilter {
	return &ForbiddenExceptionFilter{}
}

func (f *ForbiddenExceptionFilter) Catch(err error, ctx *core.Context) error {
	if httpErr, ok := err.(*HttpException); ok {
		if httpErr.StatusCode == 403 {
			return ctx.JSON(403, map[string]any{
				"statusCode": 403,
				"message":    "Forbidden",
				"hint":       "You don't have permission to access this resource",
			})
		}
	}

	return err
}

// ChainExceptionFilters chains multiple exception filters
func ChainExceptionFilters(filters ...ExceptionFilter) ExceptionFilter {
	return ExceptionFilterFunc(func(err error, ctx *core.Context) error {
		for _, filter := range filters {
			err = filter.Catch(err, ctx)
			if err == nil {
				return nil
			}
		}
		return err
	})
}
