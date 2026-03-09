// gonest/interceptors/middleware.go
package interceptors

import (
	"time"

	"github.com/gonest-dev/gonest/core/common"
)

// UseInterceptors creates a middleware that applies interceptors
func UseInterceptors(interceptors ...Interceptor) common.MiddlewareFunc {
	return func(next common.HandlerFunc) common.HandlerFunc {
		return func(ctx *common.Context) error {
			// Create execution context
			execCtx := &ExecutionContext{
				Context:   ctx,
				Handler:   next,
				Metadata:  make(map[string]any),
				StartTime: time.Now(),
			}

			// Build chain of interceptors
			handler := next

			// Apply interceptors in reverse order (like middleware)
			for i := len(interceptors) - 1; i >= 0; i-- {
				interceptor := interceptors[i]
				currentHandler := handler

				handler = func(ctx *common.Context) error {
					return interceptor.Intercept(execCtx, func() error {
						return currentHandler(ctx)
					})
				}
			}

			// Execute the chain
			return handler(ctx)
		}
	}
}

// ApplyInterceptors is a helper to apply interceptors to a handler
func ApplyInterceptors(handler common.HandlerFunc, interceptors ...Interceptor) common.HandlerFunc {
	middleware := UseInterceptors(interceptors...)
	return middleware(handler)
}


