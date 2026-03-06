// gonest/interceptors/error_interceptor.go
package interceptors

import (
	"fmt"
	"log"
)

// ErrorInterceptor handles errors and transforms them
type ErrorInterceptor struct {
	logger        *log.Logger
	logErrors     bool
	transformFunc func(error) error
}

// ErrorInterceptorOptions configures the error interceptor
type ErrorInterceptorOptions struct {
	Logger        *log.Logger
	LogErrors     bool
	TransformFunc func(error) error
}

// NewErrorInterceptor creates a new error interceptor
func NewErrorInterceptor(opts ...*ErrorInterceptorOptions) *ErrorInterceptor {
	options := &ErrorInterceptorOptions{
		Logger:    log.Default(),
		LogErrors: true,
	}

	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	return &ErrorInterceptor{
		logger:        options.Logger,
		logErrors:     options.LogErrors,
		transformFunc: options.TransformFunc,
	}
}

// Intercept handles errors
func (i *ErrorInterceptor) Intercept(ctx *ExecutionContext, next func() error) error {
	// Execute handler
	err := next()

	if err != nil {
		// Log error
		if i.logErrors {
			method := ctx.Context.Get("method")
			path := ctx.Context.Get("path")
			i.logger.Printf("[ERROR] %s %s: %v", method, path, err)
		}

		// Transform error if function provided
		if i.transformFunc != nil {
			return i.transformFunc(err)
		}
	}

	return err
}

// SimpleErrorInterceptor creates a basic error interceptor
func SimpleErrorInterceptor() *ErrorInterceptor {
	return NewErrorInterceptor()
}

// ErrorToJSON creates an interceptor that transforms errors to JSON
func ErrorToJSON() *ErrorInterceptor {
	return NewErrorInterceptor(&ErrorInterceptorOptions{
		LogErrors: true,
		TransformFunc: func(err error) error {
			// Create structured error
			return fmt.Errorf(`{"error": "%s"}`, err.Error())
		},
	})
}
